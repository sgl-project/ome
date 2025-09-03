package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	"github.com/sgl-project/ome/pkg/storage"
)

const (
	defaultChunkSize = 4 * 1024 * 1024 // 4MB chunks for parallel download - matching old implementation
	maxPartRetries   = 3
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
	index int
	start int64
	end   int64
	size  int64
}

// downloadedPart represents a downloaded part with its temporary file
type downloadedPart struct {
	index        int
	tempFilePath string
	offset       int64
	size         int64
	err          error
}

// parallelDownload performs a parallel multi-threaded download
func (p *OCIProvider) parallelDownload(ctx context.Context, source *ociURI, target string, size int64, options storage.DownloadOptions) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Use temp file for safe download
	tempTarget := target + ".temp"

	// Clean up any existing temporary file
	os.Remove(tempTarget)

	// Determine chunk size and concurrency
	chunkSize := int64(defaultChunkSize)
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}

	// Calculate chunks
	chunks := calculateChunks(size, chunkSize)
	p.logger.WithField("chunks", len(chunks)).
		WithField("concurrency", concurrency).
		Debug("Starting parallel download")

	// Create channels for communication
	chunkChan := make(chan *downloadChunk, len(chunks))
	resultChan := make(chan *downloadedPart, len(chunks))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go p.downloadWorker(ctx, source, chunkChan, resultChan, &wg)
	}

	// Queue chunks
	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan)

	// Wait for workers to finish and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect downloaded parts
	downloadedParts := make(map[int]*downloadedPart)
	for part := range resultChan {
		if part.err != nil {
			// Clean up any temp files
			for _, p := range downloadedParts {
				if p.tempFilePath != "" {
					os.Remove(p.tempFilePath)
				}
			}
			if options.Progress != nil {
				options.Progress.Error(part.err)
			}
			return fmt.Errorf("error downloading part %d: %w", part.index, part.err)
		}
		downloadedParts[part.index] = part
	}

	// Create the final file
	file, err := os.Create(tempTarget)
	if err != nil {
		// Clean up temp files
		for _, p := range downloadedParts {
			if p.tempFilePath != "" {
				os.Remove(p.tempFilePath)
			}
		}
		return err
	}

	fileClosed := false
	defer func() {
		if !fileClosed {
			file.Close()
		}
	}()

	// Assemble the file from parts in order
	var totalWritten int64
	for i := 0; i < len(chunks); i++ {
		part, ok := downloadedParts[i]
		if !ok {
			// Clean up
			for _, p := range downloadedParts {
				if p.tempFilePath != "" {
					os.Remove(p.tempFilePath)
				}
			}
			os.Remove(tempTarget)
			return fmt.Errorf("missing part %d", i)
		}

		// Open temp file
		tempFile, err := os.Open(part.tempFilePath)
		if err != nil {
			// Clean up
			for _, p := range downloadedParts {
				if p.tempFilePath != "" {
					os.Remove(p.tempFilePath)
				}
			}
			os.Remove(tempTarget)
			return fmt.Errorf("failed to open temp file for part %d: %w", i, err)
		}

		// Copy from temp file to final file
		buf := BufferPool.Get().([]byte)
		written, err := io.CopyBuffer(file, tempFile, buf)
		BufferPool.Put(buf)
		tempFile.Close()

		if err != nil {
			// Clean up
			for _, p := range downloadedParts {
				if p.tempFilePath != "" {
					os.Remove(p.tempFilePath)
				}
			}
			os.Remove(tempTarget)
			return fmt.Errorf("failed to copy part %d: %w", i, err)
		}

		totalWritten += written

		// Remove temp file
		os.Remove(part.tempFilePath)

		// Update progress
		if options.Progress != nil {
			options.Progress.Update(totalWritten, size)
		}
	}

	// Ensure all data is flushed to disk
	if err := file.Sync(); err != nil {
		os.Remove(tempTarget)
		return fmt.Errorf("failed to sync file to disk: %w", err)
	}

	// Close the file explicitly before renaming
	if err := file.Close(); err != nil {
		os.Remove(tempTarget)
		return fmt.Errorf("failed to close file: %w", err)
	}
	fileClosed = true

	// Rename to final target
	if err := os.Rename(tempTarget, target); err != nil {
		os.Remove(tempTarget)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	// Double-check the final file size
	fileInfo, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("failed to stat final file: %w", err)
	}
	if fileInfo.Size() != size {
		return fmt.Errorf("file size mismatch: expected %d bytes, got %d bytes", size, fileInfo.Size())
	}

	// Verify MD5 integrity
	if err := p.verifyMD5(ctx, source, target); err != nil {
		// MD5 mismatch is critical - remove the corrupted file
		os.Remove(target)
		return fmt.Errorf("MD5 verification failed: %w", err)
	}

	// Verify download if ETag provided
	if options.VerifyETag != "" {
		// Get object metadata to verify
		headResponse, err := p.client.HeadObject(ctx, objectstorage.HeadObjectRequest{
			NamespaceName: &source.Namespace,
			BucketName:    &source.Bucket,
			ObjectName:    &source.Object,
		})
		if err == nil && headResponse.ETag != nil {
			if *headResponse.ETag != options.VerifyETag {
				return fmt.Errorf("ETag mismatch: expected %s, got %s", options.VerifyETag, *headResponse.ETag)
			}
		}
	}

	return nil
}

// downloadWorker is a worker that downloads chunks to temporary files
func (p *OCIProvider) downloadWorker(ctx context.Context, source *ociURI, chunks <-chan *downloadChunk, results chan<- *downloadedPart, wg *sync.WaitGroup) {
	defer wg.Done()

	for chunk := range chunks {
		part := p.downloadChunkToTemp(ctx, source, chunk)
		results <- part
	}
}

// downloadChunkToTemp downloads a single chunk to a temporary file with retry
func (p *OCIProvider) downloadChunkToTemp(ctx context.Context, source *ociURI, chunk *downloadChunk) *downloadedPart {
	var lastErr error
	var tempFilePath string
	start := time.Now()

	for attempt := 1; attempt <= maxPartRetries; attempt++ {
		// Create range header
		rangeHeader := fmt.Sprintf("bytes=%d-%d", chunk.start, chunk.end)

		// Get the chunk
		request := objectstorage.GetObjectRequest{
			NamespaceName: &source.Namespace,
			BucketName:    &source.Bucket,
			ObjectName:    &source.Object,
			Range:         &rangeHeader,
		}

		response, err := p.client.GetObject(ctx, request)
		if err != nil {
			p.logger.WithField("chunk", chunk.index).
				WithField("attempt", fmt.Sprintf("%d/%d", attempt, maxPartRetries)).
				Warn("Error getting object for chunk")
			lastErr = err
			if attempt < maxPartRetries {
				time.Sleep(2 * time.Second)
			}
			continue
		}

		// Create temporary file for this chunk
		tempFile, err := os.CreateTemp("", fmt.Sprintf("oci_download_part_%d_*.tmp", chunk.index))
		if err != nil {
			response.Content.Close()
			lastErr = fmt.Errorf("failed to create temp file for chunk %d: %w", chunk.index, err)
			if attempt < maxPartRetries {
				time.Sleep(2 * time.Second)
			}
			continue
		}
		tempFilePath = tempFile.Name()

		// Copy the chunk data to temp file using pooled buffer
		buf := BufferPool.Get().([]byte)
		written, err := io.CopyBuffer(tempFile, response.Content, buf)
		BufferPool.Put(buf)
		response.Content.Close()

		// Sync and close temp file
		syncErr := tempFile.Sync()
		closeErr := tempFile.Close()

		if err != nil {
			p.logger.WithField("chunk", chunk.index).
				WithField("attempt", fmt.Sprintf("%d/%d", attempt, maxPartRetries)).
				Warn("Error writing chunk to temp file")
			os.Remove(tempFilePath)
			lastErr = err
			if attempt < maxPartRetries {
				time.Sleep(2 * time.Second)
			}
			continue
		}

		if syncErr != nil || closeErr != nil {
			p.logger.WithField("chunk", chunk.index).
				Warn("Error syncing/closing temp file")
			os.Remove(tempFilePath)
			if syncErr != nil {
				lastErr = syncErr
			} else {
				lastErr = closeErr
			}
			if attempt < maxPartRetries {
				time.Sleep(2 * time.Second)
			}
			continue
		}

		if written != chunk.size {
			p.logger.WithField("chunk", chunk.index).
				WithField("expected", chunk.size).
				WithField("actual", written).
				Warn("Partial chunk write")
			os.Remove(tempFilePath)
			lastErr = fmt.Errorf("partial write: expected %d bytes, wrote %d bytes", chunk.size, written)
			if attempt < maxPartRetries {
				time.Sleep(2 * time.Second)
			}
			continue
		}

		// Success
		duration := time.Since(start)
		speedMBs := float64(written) / 1024.0 / 1024.0 / duration.Seconds()
		p.logger.WithField("chunk", chunk.index).
			WithField("bytes", written).
			WithField("speed_MB/s", fmt.Sprintf("%.2f", speedMBs)).
			Debug("Downloaded chunk")

		return &downloadedPart{
			index:        chunk.index,
			tempFilePath: tempFilePath,
			offset:       chunk.start,
			size:         written,
			err:          nil,
		}
	}

	return &downloadedPart{
		index: chunk.index,
		err:   fmt.Errorf("failed to download chunk %d after %d retries: %w", chunk.index, maxPartRetries, lastErr),
	}
}

// calculateChunks divides the file into chunks for parallel download
func calculateChunks(totalSize, chunkSize int64) []*downloadChunk {
	var chunks []*downloadChunk

	numChunks := (totalSize + chunkSize - 1) / chunkSize

	for i := int64(0); i < numChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize - 1

		// Adjust last chunk
		if end >= totalSize {
			end = totalSize - 1
		}

		chunks = append(chunks, &downloadChunk{
			index: int(i),
			start: start,
			end:   end,
			size:  end - start + 1,
		})
	}

	return chunks
}
