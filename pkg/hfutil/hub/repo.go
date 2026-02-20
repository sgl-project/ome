package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// RepoFile represents a file in a repository
type RepoFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"` // "file" or "directory"
}

// ListRepoFiles lists all files in a repository
func ListRepoFiles(ctx context.Context, config *DownloadConfig) ([]RepoFile, error) {
	if config.RepoID == "" {
		return nil, fmt.Errorf("repo_id cannot be empty")
	}

	// Set defaults
	repoType := config.RepoType
	if repoType == "" {
		repoType = RepoTypeModel
	}

	revision := config.Revision
	if revision == "" {
		revision = DefaultRevision
	}

	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	// Construct API URL for listing files (with recursive flag)
	var apiURL string
	switch repoType {
	case RepoTypeModel:
		apiURL = fmt.Sprintf("%s/api/models/%s/tree/%s?recursive=true", endpoint, config.RepoID, url.QueryEscape(revision))
	case RepoTypeDataset:
		apiURL = fmt.Sprintf("%s/api/datasets/%s/tree/%s?recursive=true", endpoint, url.PathEscape(config.RepoID), url.QueryEscape(revision))
	case RepoTypeSpace:
		apiURL = fmt.Sprintf("%s/api/spaces/%s/tree/%s?recursive=true", endpoint, url.PathEscape(config.RepoID), url.QueryEscape(revision))
	default:
		return nil, fmt.Errorf("invalid repo type: %s", repoType)
	}

	// Get retry configuration from context (HubConfig)
	maxRetries := 3                   // default
	retryInterval := 10 * time.Second // default

	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		maxRetries = hubConfig.MaxRetries
		retryInterval = hubConfig.RetryInterval
	}

	// Use exponential backoff with jitter for rate limiting
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create request
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add headers
		headers := BuildHeaders(config.Token, "huggingface-hub-go/1.0.0", config.Headers)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// Use pooled client with timeout
		client := NewHTTPClientWithTimeout(DefaultRequestTimeout)

		resp, err := client.Do(req)
		if err != nil {
			// Network errors are retryable
			if attempt < maxRetries {
				delay := exponentialBackoffWithJitter(attempt+1, retryInterval, 60*time.Second)
				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return nil, fmt.Errorf("failed to perform request: %w", err)
		}
		defer resp.Body.Close()

		// Handle successful response
		if resp.StatusCode == http.StatusOK {
			var files []RepoFile
			if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
				return nil, fmt.Errorf("failed to decode response: %w", err)
			}
			return files, nil
		}

		// Handle rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp)
			if retryAfter == 0 {
				// Use exponential backoff with jitter if no Retry-After header
				retryAfter = exponentialBackoffWithJitter(attempt+1, retryInterval, 300*time.Second) // Max 5 minutes
			}

			// Only retry if we haven't exhausted attempts
			if attempt < maxRetries {
				select {
				case <-time.After(retryAfter):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}

		// Handle other HTTP errors with retry for server errors
		if resp.StatusCode >= 500 && attempt < maxRetries {
			delay := exponentialBackoffWithJitter(attempt+1, retryInterval, 60*time.Second)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Handle non-retryable error responses
		return nil, handleHTTPError(resp, config.RepoID, repoType, revision, "")
	}

	// Should not reach here
	return nil, fmt.Errorf("failed to list repository files after %d attempts", maxRetries+1)
}

// downloadTask represents a file download task
type downloadTask struct {
	file   RepoFile
	config *DownloadConfig
	index  int
}

// downloadResult represents the result of a download task
type downloadResult struct {
	index    int
	filePath string
	err      error
	duration time.Duration
	size     int64
}

// SnapshotDownload downloads all files in a repository to a local directory with progress reporting and concurrent downloads
func SnapshotDownload(ctx context.Context, config *DownloadConfig) (string, error) {
	if config.LocalDir == "" {
		return "", fmt.Errorf("local_dir must be specified for snapshot download")
	}

	// Get concurrency configuration from context (HubConfig)
	var maxWorkers int = 4 // default
	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		maxWorkers = hubConfig.MaxWorkers
	}
	// Use MaxWorkers from config if available
	if config.MaxWorkers > 0 {
		maxWorkers = config.MaxWorkers
	}

	// Check if progress is enabled
	enableProgress := true
	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		enableProgress = hubConfig.ShouldEnableProgress()
	}

	// List all files in the repository
	files, err := ListRepoFiles(ctx, config)
	if err != nil {
		return "", fmt.Errorf("failed to list repository files: %w", err)
	}

	// Filter files (exclude directories) and calculate totals
	var filesToDownload []RepoFile
	var totalSize int64
	for _, file := range files {
		if file.Type == "file" {
			// Apply pattern filtering if specified
			if ShouldIgnoreFile(file.Path, config.AllowPatterns, config.IgnorePatterns) {
				continue
			}
			filesToDownload = append(filesToDownload, file)
			totalSize += file.Size
		}
	}

	fileCount := len(filesToDownload)

	// Create overall progress for snapshot download
	snapshotProgress := NewProgress(fmt.Sprintf("Downloading %s", config.RepoID), totalSize, enableProgress)

	// Track download timing
	startTime := time.Now()

	// Create channels for task distribution and result collection
	taskChan := make(chan downloadTask, fileCount)
	resultChan := make(chan downloadResult, fileCount)

	// Create worker pool
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			downloadWorker(ctx, workerID, taskChan, resultChan)
		}(i)
	}

	// Send download tasks to workers
	go func() {
		defer close(taskChan)
		for i, file := range filesToDownload {
			fileConfig := *config // Copy the config
			fileConfig.Filename = file.Path

			select {
			case taskChan <- downloadTask{
				file:   file,
				config: &fileConfig,
				index:  i,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	results := make([]downloadResult, fileCount)
	var completedFiles int
	var totalErrors int
	var downloadedBytes int64

	// Start goroutine to collect results
	go func() {
		for result := range resultChan {
			results[result.index] = result
			completedFiles++

			if result.err != nil {
				totalErrors++
			} else {
				// Update overall progress
				downloadedBytes += result.size
				snapshotProgress.Update(result.size)
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)

	// Wait for all results to be collected
	for completedFiles < fileCount {
		time.Sleep(10 * time.Millisecond)
	}

	// Finish progress tracking
	snapshotProgress.Finish()

	// Check if context was cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Report results
	duration := time.Since(startTime)
	fmt.Printf("Download completed: %d files, %d errors in %v\n", completedFiles, totalErrors, duration.Round(time.Second))

	if totalErrors > 0 {
		// Collect error details
		var errorFiles []string
		var errorMessages []string
		for _, result := range results {
			if result.err != nil {
				errorFiles = append(errorFiles, filesToDownload[result.index].Path)
				errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", filesToDownload[result.index].Path, result.err))
			}
		}

		fmt.Printf("Failed files: %v\n", errorFiles)

		// Log detailed error information at debug level
		if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok && hubConfig.Logger != nil {
			hubConfig.Logger.
				WithField("error_count", totalErrors).
				WithField("errors", errorMessages).
				Debug("Download error details")
		}

		// Return error with details but include the local directory
		return config.LocalDir, fmt.Errorf("failed to download %d out of %d files", totalErrors, fileCount)
	}

	return config.LocalDir, nil
}

// downloadWorker is a worker goroutine that processes download tasks
func downloadWorker(ctx context.Context, workerID int, taskChan <-chan downloadTask, resultChan chan<- downloadResult) {
	for {
		select {
		case task, ok := <-taskChan:
			if !ok {
				return // Channel closed, worker should exit
			}

			startTime := time.Now()

			// Perform the download (HfHubDownload handles its own progress per file)
			filePath, err := HfHubDownload(ctx, task.config)

			duration := time.Since(startTime)

			// Log download errors at debug level
			if err != nil {
				if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok && hubConfig.Logger != nil {
					hubConfig.Logger.
						WithField("worker_id", workerID).
						WithField("file", task.file.Path).
						WithError(err).
						Debug("Worker download failed")
				}
			}

			result := downloadResult{
				index:    task.index,
				filePath: filePath,
				err:      err,
				duration: duration,
				size:     task.file.Size,
			}

			// Send result
			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return // Context cancelled, worker should exit
		}
	}
}

// FilterByPatterns filters files based on allow and ignore patterns
func FilterByPatterns(files []RepoFile, allowPatterns, ignorePatterns []string) []RepoFile {
	var filtered []RepoFile
	for _, file := range files {
		if file.Type == "file" && !ShouldIgnoreFile(file.Path, allowPatterns, ignorePatterns) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
