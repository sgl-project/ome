package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/schollz/progressbar/v3"
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

// SnapshotDownload downloads all files in a repository to a local directory with progress reporting
func SnapshotDownload(ctx context.Context, config *DownloadConfig) (string, error) {
	if config.LocalDir == "" {
		return "", fmt.Errorf("local_dir must be specified for snapshot download")
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
	var snapshotBar *progressbar.ProgressBar
	if progressManager != nil {
		snapshotBar = progressManager.CreateSnapshotProgressBar(fileCount, totalSize)
	}

	fmt.Printf("Found %d files to download (%s)\n", fileCount, formatSize(totalSize))

	// Track download timing
	startTime := time.Now()

	// Download each file
	for i, file := range filesToDownload {
		if progressManager != nil && progressManager.enableDetailedLogs {
			progressManager.logger.
				WithField("repo_id", config.RepoID).
				WithField("file_index", i+1).
				WithField("total_files", fileCount).
				WithField("file_path", file.Path).
				WithField("file_size", file.Size).
				Debug("Starting individual file download")
		} else {
			fmt.Printf("Downloading %d/%d: %s (%s)\n", i+1, fileCount, file.Path, formatSize(file.Size))
		}

		fileConfig := *config // Copy the config
		fileConfig.Filename = file.Path

		// Add hub config to context for progress reporting
		fileCtx := ctx
		if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
			fileCtx = context.WithValue(ctx, HubConfigKey, hubConfig)
		}

		_, err := HfHubDownload(fileCtx, &fileConfig)
		if err != nil {
			if progressManager != nil {
				progressManager.LogError("file_download", config.RepoID, err)
			}
			return "", fmt.Errorf("failed to download file %s: %w", file.Path, err)
		}

		// Update snapshot progress
		if snapshotBar != nil {
			_ = snapshotBar.Add(1) // Ignore error from progress bar update
		}
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

	// Log completion
	duration := time.Since(startTime)
	if progressManager != nil {
		progressManager.LogSnapshotComplete(config.RepoID, fileCount, duration, totalSize)
	}

	return config.LocalDir, nil
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
