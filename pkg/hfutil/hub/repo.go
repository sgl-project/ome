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
		apiURL = fmt.Sprintf("%s/api/models/%s/tree/%s?recursive=true", endpoint, url.PathEscape(config.RepoID), url.QueryEscape(revision))
	case RepoTypeDataset:
		apiURL = fmt.Sprintf("%s/api/datasets/%s/tree/%s?recursive=true", endpoint, url.PathEscape(config.RepoID), url.QueryEscape(revision))
	case RepoTypeSpace:
		apiURL = fmt.Sprintf("%s/api/spaces/%s/tree/%s?recursive=true", endpoint, url.PathEscape(config.RepoID), url.QueryEscape(revision))
	default:
		return nil, fmt.Errorf("invalid repo type: %s", repoType)
	}

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

	// Perform request
	client := &http.Client{
		Timeout: DefaultRequestTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, handleHTTPError(resp, config.RepoID, repoType, revision, "")
	}

	// Parse response
	var files []RepoFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return files, nil
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

	// Create progress manager if config supports it
	var progressManager *ProgressManager
	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		progressManager = hubConfig.CreateProgressManager()
	}

	// List all files in the repository
	files, err := ListRepoFiles(ctx, config)
	if err != nil {
		if progressManager != nil {
			progressManager.LogError("list_files", config.RepoID, err)
		}
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
	if progressManager != nil {
		progressManager.LogRepoListing(config.RepoID, len(files))
		progressManager.LogSnapshotStart(config.RepoID, fileCount, totalSize)
	}

	// Create overall progress bar for snapshot download
	var snapshotBar ProgressBar
	if progressManager != nil {
		snapshotBar = progressManager.CreateSnapshotProgressBar(fileCount, totalSize)
	}

	fmt.Printf("Found %d files to download (%s) using %d workers\n", fileCount, formatSize(totalSize), maxWorkers)

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
			downloadWorker(ctx, workerID, taskChan, resultChan, progressManager)
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

	// Start goroutine to collect results
	go func() {
		for result := range resultChan {
			results[result.index] = result
			completedFiles++

			if result.err != nil {
				totalErrors++
				if progressManager != nil {
					progressManager.LogError("file_download", config.RepoID, result.err)
				}
			}

			// Update snapshot progress
			if snapshotBar != nil {
				_ = snapshotBar.Add(1) // Ignore error from progress bar update
			}

			// Log progress if detailed logging is enabled
			if progressManager != nil && progressManager.enableDetailedLogs {
				progressManager.logger.
					WithField("repo_id", config.RepoID).
					WithField("completed", completedFiles).
					WithField("total", fileCount).
					WithField("errors", totalErrors).
					Info("Download progress update")
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
	if snapshotBar != nil {
		if err := snapshotBar.Finish(); err != nil {
			// Log error but don't fail the operation
			if progressManager != nil && progressManager.logger != nil {
				progressManager.logger.Debug("Failed to finish snapshot progress bar")
			}
		}
	}

	// Check if context was cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Log completion
	duration := time.Since(startTime)
	if progressManager != nil {
		progressManager.LogSnapshotComplete(config.RepoID, fileCount, duration, totalSize)
		// Shutdown the progress manager to ensure multi-progress container is closed
		progressManager.Shutdown()
	}

	// Report results
	fmt.Printf("Download completed: %d files, %d errors\n", completedFiles, totalErrors)

	if totalErrors > 0 {
		// Collect error details
		var errorFiles []string
		for _, result := range results {
			if result.err != nil {
				errorFiles = append(errorFiles, filesToDownload[result.index].Path)
			}
		}

		if progressManager != nil {
			progressManager.logger.
				WithField("repo_id", config.RepoID).
				WithField("failed_files", errorFiles).
				WithField("error_count", totalErrors).
				Error("Some files failed to download")
		}

		// Return error with details but include the local directory
		return config.LocalDir, fmt.Errorf("failed to download %d out of %d files", totalErrors, fileCount)
	}

	return config.LocalDir, nil
}

// downloadWorker is a worker goroutine that processes download tasks
func downloadWorker(ctx context.Context, workerID int, taskChan <-chan downloadTask, resultChan chan<- downloadResult, progressManager *ProgressManager) {
	for {
		select {
		case task, ok := <-taskChan:
			if !ok {
				return // Channel closed, worker should exit
			}

			startTime := time.Now()

			if progressManager != nil && progressManager.enableDetailedLogs {
				progressManager.logger.
					WithField("worker_id", workerID).
					WithField("repo_id", task.config.RepoID).
					WithField("file_path", task.file.Path).
					WithField("file_size", task.file.Size).
					Debug("Worker starting file download")
			}

			// Add hub config and worker ID to context for progress reporting
			fileCtx := ctx
			if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
				fileCtx = context.WithValue(ctx, HubConfigKey, hubConfig)
			}
			// Add worker ID to context for multi-progress tracking
			fileCtx = context.WithValue(fileCtx, WorkerIDKey, workerID)

			// Perform the download
			filePath, err := HfHubDownload(fileCtx, task.config)

			duration := time.Since(startTime)

			result := downloadResult{
				index:    task.index,
				filePath: filePath,
				err:      err,
				duration: duration,
				size:     task.file.Size,
			}

			if progressManager != nil && progressManager.enableDetailedLogs {
				if err != nil {
					progressManager.logger.
						WithField("worker_id", workerID).
						WithField("repo_id", task.config.RepoID).
						WithField("file_path", task.file.Path).
						WithField("error", err.Error()).
						WithField("duration", duration).
						Error("Worker failed to download file")
				} else {
					progressManager.logger.
						WithField("worker_id", workerID).
						WithField("repo_id", task.config.RepoID).
						WithField("file_path", task.file.Path).
						WithField("duration", duration).
						Debug("Worker completed file download")
				}
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
