package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sgl-project/ome/pkg/storage"
)

// downloadChunk represents a chunk to be downloaded
type downloadChunk struct {
	index      int
	start      int64
	end        int64
	tempFile   string
	retryCount int
}

// downloadResult represents the result of a chunk download
type downloadResult struct {
	index        int
	tempFilePath string
	err          error
}

// parallelDownload performs a parallel download of a large object
func (p *S3Provider) parallelDownload(ctx context.Context, key string, targetFile string, size int64, options storage.DownloadOptions) error {
	// Track total bytes downloaded for progress reporting
	var totalBytesDownloaded int64
	var progressMutex sync.Mutex
	// Determine number of chunks and chunk size
	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	// Calculate chunk size (minimum 5MB per chunk)
	minChunkSize := int64(5 * 1024 * 1024) // 5MB
	chunkSize := size / int64(concurrency)
	if chunkSize < minChunkSize {
		chunkSize = minChunkSize
		concurrency = int(size / chunkSize)
		if concurrency == 0 {
			concurrency = 1
		}
	}

	// Create chunks
	chunks := make([]downloadChunk, 0, concurrency)
	for i := 0; i < concurrency; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == concurrency-1 {
			// Last chunk goes to the end of the file
			end = size - 1
		}

		chunks = append(chunks, downloadChunk{
			index: i,
			start: start,
			end:   end,
		})
	}

	// Create a temporary directory for chunk files
	tempDir, err := os.MkdirTemp("", "s3_download_*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory when done

	// Download chunks in parallel
	var wg sync.WaitGroup
	resultChan := make(chan downloadResult, len(chunks))
	semaphore := make(chan struct{}, concurrency)

	for _, chunk := range chunks {
		wg.Add(1)
		go func(ch downloadChunk) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			tempFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", ch.index))
			chunkSize := ch.end - ch.start + 1
			err := p.downloadChunk(ctx, key, tempFile, ch.start, ch.end, options)

			if err == nil && options.Progress != nil {
				// Thread-safe progress update
				newTotal := atomic.AddInt64(&totalBytesDownloaded, chunkSize)
				progressMutex.Lock()
				options.Progress.Update(newTotal, size)
				progressMutex.Unlock()
			}

			resultChan <- downloadResult{
				index:        ch.index,
				tempFilePath: tempFile,
				err:          err,
			}
		}(chunk)
	}

	// Wait for all downloads to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make(map[int]downloadResult)
	var downloadErrors []error
	for result := range resultChan {
		if result.err != nil {
			downloadErrors = append(downloadErrors, fmt.Errorf("chunk %d failed: %w", result.index, result.err))
		} else {
			results[result.index] = result
		}
	}

	// Check if any downloads failed
	if len(downloadErrors) > 0 {
		return fmt.Errorf("parallel download failed with %d errors: %v", len(downloadErrors), downloadErrors[0])
	}

	// Assemble the file from chunks
	if err := p.assembleChunks(targetFile, results, len(chunks)); err != nil {
		return fmt.Errorf("failed to assemble chunks: %w", err)
	}

	// Report progress if configured
	if options.Progress != nil {
		options.Progress.Done()
	}

	return nil
}

// downloadChunk downloads a specific chunk of the object
func (p *S3Provider) downloadChunk(ctx context.Context, key string, tempFile string, start, end int64, options storage.DownloadOptions) error {
	// Create the range header
	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)

	// Get the object with range
	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return fmt.Errorf("failed to get object range %s: %w", rangeHeader, err)
	}
	defer result.Body.Close()

	// Create temp file
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	// Copy the chunk data
	bytesWritten, err := io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	expectedSize := end - start + 1
	if bytesWritten != expectedSize {
		return fmt.Errorf("chunk size mismatch: expected %d, got %d", expectedSize, bytesWritten)
	}

	// Note: Progress reporting is now handled by parallelDownload to avoid concurrent updates

	return nil
}

// assembleChunks assembles the downloaded chunks into the final file
func (p *S3Provider) assembleChunks(targetFile string, results map[int]downloadResult, totalChunks int) error {
	// Create the target file
	target, err := os.Create(targetFile)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer target.Close()

	// Get a buffer from the pool
	bufPtr := p.bufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer func() {
		p.bufferPool.Put(bufPtr)
	}()

	// Write chunks in order
	for i := 0; i < totalChunks; i++ {
		result, ok := results[i]
		if !ok {
			return fmt.Errorf("missing chunk %d", i)
		}

		// Open the chunk file
		chunkFile, err := os.Open(result.tempFilePath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", i, err)
		}

		// Copy chunk to target file
		_, err = io.CopyBuffer(target, chunkFile, buf)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", i, err)
		}

		// Remove the temp file immediately after copying
		os.Remove(result.tempFilePath)
	}

	return nil
}

// downloadParallelWithRetry downloads a file with retry logic
func (p *S3Provider) downloadParallelWithRetry(ctx context.Context, key string, targetFile string, size int64, options storage.DownloadOptions) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			p.logger.WithField("attempt", attempt+1).
				WithField("key", key).
				Info("Retrying parallel download")
		}

		err := p.parallelDownload(ctx, key, targetFile, size, options)
		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.WithError(err).
			WithField("attempt", attempt+1).
			WithField("key", key).
			Warn("Parallel download attempt failed")
	}

	return fmt.Errorf("parallel download failed after %d attempts: %w", maxRetries, lastErr)
}
