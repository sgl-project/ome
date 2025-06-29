package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// BulkDownload downloads multiple objects concurrently
func BulkDownload(ctx context.Context, storage Storage, objects []ObjectURI, targetDir string, opts BulkDownloadOptions) ([]BulkDownloadResult, error) {
	if len(objects) == 0 {
		return nil, nil
	}

	// Create channels for job distribution
	jobs := make(chan ObjectURI, len(objects))
	results := make(chan BulkDownloadResult, len(objects))

	// Create worker pool
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go bulkDownloadWorker(ctx, &wg, storage, jobs, results, targetDir, opts)
	}

	// Queue all jobs
	for _, obj := range objects {
		select {
		case jobs <- obj:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(results)

	// Collect results
	var allResults []BulkDownloadResult
	completed := 0
	total := len(objects)

	for result := range results {
		allResults = append(allResults, result)
		completed++

		// Call progress callback if provided
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(completed, total, &result)
		}
	}

	return allResults, nil
}

// bulkDownloadWorker processes download jobs from the queue
func bulkDownloadWorker(ctx context.Context, wg *sync.WaitGroup, storage Storage, jobs <-chan ObjectURI, results chan<- BulkDownloadResult, targetDir string, opts BulkDownloadOptions) {
	defer wg.Done()

	for uri := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
			result := processBulkDownload(ctx, storage, uri, targetDir, opts)
			results <- result
		}
	}
}

// processBulkDownload handles a single download with retry logic
func processBulkDownload(ctx context.Context, storage Storage, uri ObjectURI, targetDir string, opts BulkDownloadOptions) BulkDownloadResult {
	start := time.Now()
	result := BulkDownloadResult{
		URI: uri,
	}

	// Determine target path
	targetPath := filepath.Join(targetDir, uri.ObjectName)
	result.TargetPath = targetPath

	// Download with retry
	var lastErr error
	err := RetryOperation(ctx, opts.RetryConfig, func() error {
		result.RetryAttempts++

		// Get object metadata first
		metadata, err := storage.Stat(ctx, uri)
		if err != nil {
			lastErr = err
			return err
		}
		result.Size = metadata.Size

		// Check if we should skip based on download options
		if opts.DownloadOptions.SkipExisting {
			if exists, _ := FileExists(targetPath); exists {
				// Validate if existing file is valid
				if valid, _ := IsLocalFileValid(targetPath, *metadata); valid {
					return nil // Skip download
				}
			}
		}

		// Perform download
		reader, err := storage.Get(ctx, uri)
		if err != nil {
			lastErr = err
			return err
		}
		defer reader.Close()

		// Write to file
		if err := WriteReaderToFile(reader, targetPath); err != nil {
			lastErr = err
			return err
		}

		// Validate downloaded file if MD5 is available
		if metadata.ContentMD5 != "" {
			if valid, err := ValidateFileMD5(targetPath, metadata.ContentMD5); err != nil {
				lastErr = err
				return err
			} else if !valid {
				lastErr = fmt.Errorf("MD5 validation failed for %s", uri.ObjectName)
				return lastErr
			}
		}

		return nil
	})

	result.Error = err
	result.Duration = time.Since(start)

	return result
}

// BulkUpload uploads multiple files concurrently
func BulkUpload(ctx context.Context, storage Storage, files []BulkUploadFile, opts BulkUploadOptions) ([]BulkUploadResult, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Create channels for job distribution
	jobs := make(chan BulkUploadFile, len(files))
	results := make(chan BulkUploadResult, len(files))

	// Create worker pool
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go bulkUploadWorker(ctx, &wg, storage, jobs, results, opts)
	}

	// Queue all jobs
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(results)

	// Collect results
	var allResults []BulkUploadResult
	completed := 0
	total := len(files)

	for result := range results {
		allResults = append(allResults, result)
		completed++

		// Call progress callback if provided
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(completed, total, &result)
		}
	}

	return allResults, nil
}

// bulkUploadWorker processes upload jobs from the queue
func bulkUploadWorker(ctx context.Context, wg *sync.WaitGroup, storage Storage, jobs <-chan BulkUploadFile, results chan<- BulkUploadResult, opts BulkUploadOptions) {
	defer wg.Done()

	for file := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
			result := processBulkUpload(ctx, storage, file, opts)
			results <- result
		}
	}
}

// processBulkUpload handles a single upload with retry logic
func processBulkUpload(ctx context.Context, storage Storage, file BulkUploadFile, opts BulkUploadOptions) BulkUploadResult {
	start := time.Now()
	result := BulkUploadResult{
		SourcePath: file.SourcePath,
		URI:        file.TargetURI,
	}

	// Get file info
	fileInfo, err := GetFileInfo(file.SourcePath)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}
	result.Size = fileInfo.Size()

	// Upload with retry
	err = RetryOperation(ctx, opts.RetryConfig, func() error {
		result.RetryAttempts++

		// Open file
		reader, err := OpenFile(file.SourcePath)
		if err != nil {
			return err
		}
		defer reader.Close()

		// Perform upload
		return storage.Put(ctx, file.TargetURI, reader, result.Size,
			WithUploadContentType(opts.UploadOptions.ContentType),
			WithUploadStorageClass(opts.UploadOptions.StorageClass))
	})

	result.Error = err
	result.Duration = time.Since(start)

	return result
}
