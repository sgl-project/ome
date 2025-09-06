package gcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sync/errgroup"

	storageTypes "github.com/sgl-project/ome/pkg/storage"
)

const (
	defaultChunkSize   = 4 * 1024 * 1024 // 4MB chunks for parallel operations
	maxWorkers         = 10              // Maximum number of parallel workers
	defaultParallelism = 5               // Default parallelism level
)

// BufferPool provides reusable buffers to reduce memory allocations
var BufferPool = sync.Pool{
	New: func() interface{} {
		// Use 1MB buffer by default
		return make([]byte, 1024*1024)
	},
}

// downloadChunk represents a chunk to download
type downloadChunk struct {
	index  int
	offset int64
	size   int64
}

// downloadedPart represents a downloaded part
type downloadedPart struct {
	index  int
	data   []byte
	offset int64
	size   int64
	err    error
}

// parallelDownload performs a parallel multi-threaded download
func (p *Provider) parallelDownload(ctx context.Context, bucketName, objectName, target string, options storageTypes.DownloadOptions) error {
	// Get object size
	obj := p.client.Bucket(bucketName).Object(objectName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes: %w", err)
	}

	size := attrs.Size
	if size == 0 {
		// Empty file, just create it
		return os.WriteFile(target, []byte{}, 0644)
	}

	// Determine number of chunks
	chunkSize := int64(defaultChunkSize)

	numChunks := (size + chunkSize - 1) / chunkSize
	parallelism := defaultParallelism
	if options.Concurrency > 0 {
		parallelism = options.Concurrency
	}
	if parallelism > maxWorkers {
		parallelism = maxWorkers
	}

	p.logger.WithField("size", size).
		WithField("chunks", numChunks).
		WithField("chunkSize", chunkSize).
		WithField("parallelism", parallelism).
		Debug("Starting parallel download")

	// Ensure target directory exists
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create the target file
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer file.Close()

	// Pre-allocate file space for better performance
	if err := file.Truncate(size); err != nil {
		return fmt.Errorf("failed to allocate file space: %w", err)
	}

	// Download chunks in parallel
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallelism)

	chunkChan := make(chan downloadChunk, numChunks)
	resultChan := make(chan downloadedPart, numChunks)

	// Producer: generate chunks
	go func() {
		for i := int64(0); i < numChunks; i++ {
			offset := i * chunkSize
			remainingSize := size - offset
			currentChunkSize := chunkSize
			if remainingSize < chunkSize {
				currentChunkSize = remainingSize
			}

			chunkChan <- downloadChunk{
				index:  int(i),
				offset: offset,
				size:   currentChunkSize,
			}
		}
		close(chunkChan)
	}()

	// Workers: download chunks
	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		g.Go(func() error {
			defer wg.Done()
			for chunk := range chunkChan {
				part := p.downloadChunk(ctx, bucketName, objectName, chunk)
				resultChan <- part
				if part.err != nil {
					return part.err
				}

				// Report progress if callback provided
				if options.Progress != nil {
					options.Progress.Update(chunk.offset+chunk.size, size)
				}
			}
			return nil
		})
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Consumer: write chunks to file
	for part := range resultChan {
		if part.err != nil {
			continue // Error already captured by errgroup
		}

		// Write to the correct position in the file
		if _, err := file.WriteAt(part.data, part.offset); err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", part.index, err)
		}
	}

	// Wait for all downloads to complete
	if err := g.Wait(); err != nil {
		return fmt.Errorf("parallel download failed: %w", err)
	}

	// Verify integrity if ETag provided
	if options.VerifyETag != "" {
		// TODO: Implement ETag verification
		p.logger.Debug("ETag verification not yet implemented")
	}

	p.logger.WithField("file", target).Debug("Parallel download completed")
	return nil
}

// downloadChunk downloads a single chunk of an object
func (p *Provider) downloadChunk(ctx context.Context, bucketName, objectName string, chunk downloadChunk) downloadedPart {
	obj := p.client.Bucket(bucketName).Object(objectName)

	// Create reader with range
	reader, err := obj.NewRangeReader(ctx, chunk.offset, chunk.size)
	if err != nil {
		return downloadedPart{
			index:  chunk.index,
			offset: chunk.offset,
			err:    fmt.Errorf("failed to create range reader for chunk %d: %w", chunk.index, err),
		}
	}
	defer reader.Close()

	// Get a buffer from the pool if the chunk size is reasonable
	var data []byte
	var poolBuffer []byte
	if chunk.size <= 1024*1024 { // Use pool for chunks up to 1MB
		poolBuffer = BufferPool.Get().([]byte)
		defer BufferPool.Put(poolBuffer)
		data = poolBuffer[:chunk.size]
	} else {
		// For larger chunks, allocate a new buffer
		data = make([]byte, chunk.size)
	}

	n, err := io.ReadFull(reader, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return downloadedPart{
			index:  chunk.index,
			offset: chunk.offset,
			err:    fmt.Errorf("failed to read chunk %d: %w", chunk.index, err),
		}
	}

	// If we used a pool buffer, we need to copy the data to avoid reuse issues
	result := make([]byte, n)
	copy(result, data[:n])

	return downloadedPart{
		index:  chunk.index,
		data:   result,
		offset: chunk.offset,
		size:   int64(n),
		err:    nil,
	}
}

// parallelUpload performs a parallel multi-part upload
func (p *Provider) parallelUpload(ctx context.Context, bucketName, objectName, source string, options storageTypes.UploadOptions) error {
	// Open source file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	size := stat.Size()

	// For small files, use regular upload
	if size < defaultChunkSize {
		return p.uploadFile(ctx, bucketName, objectName, source)
	}

	// Determine chunk size and parallelism
	chunkSize := int64(defaultChunkSize)
	if options.PartSize > 0 {
		chunkSize = options.PartSize
	}

	parallelism := defaultParallelism
	if options.Concurrency > 0 {
		parallelism = options.Concurrency
	}
	if parallelism > maxWorkers {
		parallelism = maxWorkers
	}

	numChunks := (size + chunkSize - 1) / chunkSize

	p.logger.WithField("size", size).
		WithField("chunks", numChunks).
		WithField("chunkSize", chunkSize).
		WithField("parallelism", parallelism).
		Debug("Starting parallel upload")

	// Initiate multipart upload
	uri := buildGCSURI(bucketName, objectName)
	uploadID, err := p.InitiateMultipartUpload(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// Upload chunks in parallel
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallelism)

	completedParts := make([]storageTypes.Part, numChunks)
	var completedMu sync.Mutex

	for i := int64(0); i < numChunks; i++ {
		partNum := i + 1
		offset := i * chunkSize
		currentChunkSize := chunkSize
		if offset+chunkSize > size {
			currentChunkSize = size - offset
		}

		g.Go(func() error {
			// Create a section reader for this chunk
			sectionReader := io.NewSectionReader(file, offset, currentChunkSize)

			// Upload the part
			etag, err := p.UploadPart(ctx, uri, uploadID, int(partNum), sectionReader, currentChunkSize)
			if err != nil {
				return fmt.Errorf("failed to upload part %d: %w", partNum, err)
			}

			// Store completed part info
			completedMu.Lock()
			completedParts[partNum-1] = storageTypes.Part{
				PartNumber: int(partNum),
				ETag:       etag,
				Size:       currentChunkSize,
			}
			completedMu.Unlock()

			// Report progress if callback provided
			if options.Progress != nil {
				options.Progress.Update(offset+currentChunkSize, size)
			}

			return nil
		})
	}

	// Wait for all uploads to complete
	if err := g.Wait(); err != nil {
		// Abort the multipart upload on error
		p.AbortMultipartUpload(ctx, uri, uploadID)
		return fmt.Errorf("parallel upload failed: %w", err)
	}

	// Complete the multipart upload
	if err := p.CompleteMultipartUpload(ctx, uri, uploadID, completedParts); err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	p.logger.WithField("object", objectName).Debug("Parallel upload completed")
	return nil
}

// uploadFile uploads a file using standard (non-parallel) method
func (p *Provider) uploadFile(ctx context.Context, bucketName, objectName, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	obj := p.client.Bucket(bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, file); err != nil {
		writer.Close()
		return fmt.Errorf("failed to upload file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}
