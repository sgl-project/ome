package hub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

// HFFileMetadata represents metadata about a file on the Hub
type HFFileMetadata struct {
	CommitHash string
	Etag       string
	Location   string
	Size       int64
}

// DownloadOptions represents options for downloading files
type DownloadOptions struct {
	// Destination
	CacheDir string
	LocalDir string

	// File info
	Subfolder string
	RepoType  string
	Revision  string

	// HTTP options
	Endpoint    string
	EtagTimeout time.Duration
	Headers     map[string]string
	Proxies     map[string]string
	Token       string

	// Download options
	LocalFilesOnly      bool
	ForceDownload       bool
	ResumeDownload      bool
	LocalDirUseSymlinks string // "auto", "true", "false"
	ForceFilename       string

	// Progress and user agent
	LibraryName    string
	LibraryVersion string
	UserAgent      string
}

// DefaultDownloadOptions returns default options for downloads
func DefaultDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		CacheDir:            GetCacheDir(),
		RepoType:            RepoTypeModel,
		Revision:            DefaultRevision,
		Endpoint:            DefaultEndpoint,
		EtagTimeout:         DefaultEtagTimeout,
		Headers:             make(map[string]string),
		LocalDirUseSymlinks: "auto",
		ResumeDownload:      true,
	}
}

// HfHubDownload downloads a file from the Hugging Face Hub
// This is the Go equivalent of the Python hf_hub_download function
func HfHubDownload(ctx context.Context, config *DownloadConfig) (string, error) {
	// Validate input
	if config.RepoID == "" {
		return "", fmt.Errorf("repo_id cannot be empty")
	}
	if config.Filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}

	// Set defaults
	if config.RepoType == "" {
		config.RepoType = RepoTypeModel
	}
	if config.Revision == "" {
		config.Revision = DefaultRevision
	}
	if config.Endpoint == "" {
		config.Endpoint = DefaultEndpoint
	}
	if config.EtagTimeout == 0 {
		config.EtagTimeout = DefaultEtagTimeout
	}

	// If local_dir is specified, download to local directory
	if config.LocalDir != "" {
		return hfHubDownloadToLocalDir(ctx, config)
	}

	// Otherwise, download to cache directory
	if config.CacheDir == "" {
		config.CacheDir = GetCacheDir()
	}

	return hfHubDownloadToCacheDir(ctx, config)
}

// hfHubDownloadToCacheDir downloads a file to the cache directory with symlinks
func hfHubDownloadToCacheDir(ctx context.Context, config *DownloadConfig) (string, error) {
	storageFolder := filepath.Join(config.CacheDir, RepoFolderName(config.RepoID, config.RepoType))

	// Cross platform transcription of filename
	relativeFilename := filepath.Join(strings.Split(config.Filename, "/")...)

	// If revision is a commit hash and file exists, return immediately
	if IsCommitHash(config.Revision) {
		pointerPath, err := GetPointerPath(storageFolder, config.Revision, relativeFilename)
		if err != nil {
			return "", fmt.Errorf("invalid pointer path: %w", err)
		}
		if FileExists(pointerPath) && !config.ForceDownload {
			return pointerPath, nil
		}
	}

	// Get metadata from server
	metadata, err := getMetadataOrCatchError(ctx, config, storageFolder, relativeFilename)
	if err != nil {
		// Try to load from cache if metadata fetch failed
		if !config.ForceDownload {
			cachedPath := tryToLoadFromCache(config, storageFolder, relativeFilename)
			if cachedPath != "" {
				return cachedPath, nil
			}
		}
		return "", err
	}

	// Create necessary directories
	blobPath := filepath.Join(storageFolder, "blobs", metadata.Etag)
	pointerPath, err := GetPointerPath(storageFolder, metadata.CommitHash, relativeFilename)
	if err != nil {
		return "", fmt.Errorf("invalid pointer path: %w", err)
	}

	if err := EnsureDir(filepath.Dir(blobPath)); err != nil {
		return "", fmt.Errorf("failed to create blob directory: %w", err)
	}
	if err := EnsureDir(filepath.Dir(pointerPath)); err != nil {
		return "", fmt.Errorf("failed to create pointer directory: %w", err)
	}

	// Cache the commit hash for this revision
	if err := CacheCommitHashForRevision(storageFolder, config.Revision, metadata.CommitHash); err != nil {
		return "", fmt.Errorf("failed to cache commit hash: %w", err)
	}

	// Return early if pointer already exists
	if !config.ForceDownload && FileExists(pointerPath) {
		return pointerPath, nil
	}

	// If blob exists but pointer is missing, create the pointer
	if !config.ForceDownload && FileExists(blobPath) {
		if err := CreateSymlink(blobPath, pointerPath); err != nil {
			return "", fmt.Errorf("failed to create symlink: %w", err)
		}
		return pointerPath, nil
	}

	// Download the file
	if err := downloadToTmpAndMove(ctx, config, metadata, blobPath); err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	// Create the pointer
	if !FileExists(pointerPath) {
		if err := CreateSymlink(blobPath, pointerPath); err != nil {
			return "", fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	return pointerPath, nil
}

// hfHubDownloadToLocalDir downloads a file directly to a local directory
func hfHubDownloadToLocalDir(ctx context.Context, config *DownloadConfig) (string, error) {
	localDir := config.LocalDir
	filePath := filepath.Join(localDir, config.Filename)

	// Check if file already exists and has correct metadata
	if !config.ForceDownload && FileExists(filePath) {
		// Try to get metadata to check if file is up to date
		metadata, err := getMetadataOrCatchError(ctx, config, "", "")
		if err == nil {
			// Check if the local file matches the expected size and etag
			if size, err := GetFileSize(filePath); err == nil && size == metadata.Size {
				// For LFS files, verify checksum
				if metadata.Etag != "" && IsSHA256(metadata.Etag) {
					if err := VerifyChecksum(filePath, metadata.Etag); err == nil {
						return filePath, nil
					}
				} else {
					// For non-LFS files, size match is sufficient
					return filePath, nil
				}
			}
		}
	}

	// Get metadata for download
	metadata, err := getMetadataOrCatchError(ctx, config, "", "")
	if err != nil {
		if !config.ForceDownload && FileExists(filePath) {
			// Fallback to local file if it exists
			return filePath, nil
		}
		return "", err
	}

	// Download the file
	if err := downloadToTmpAndMove(ctx, config, metadata, filePath); err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	return filePath, nil
}

// downloadToTmpAndMove downloads a file to a temporary location and then moves it
func downloadToTmpAndMove(ctx context.Context, config *DownloadConfig, metadata *FileMetadata, destPath string) error {
	incompletePath := destPath + ".incomplete"

	// Return early if file already exists and force_download is false
	if FileExists(destPath) && !config.ForceDownload {
		return nil
	}

	// Remove incomplete file if force_download is true
	if config.ForceDownload && FileExists(incompletePath) {
		os.Remove(incompletePath)
	}

	// Create directories
	if err := EnsureDir(filepath.Dir(destPath)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check disk space
	if metadata.Size > 0 {
		if err := CheckDiskSpace(metadata.Size, filepath.Dir(destPath)); err != nil {
			return err
		}
	}

	// Download the file
	if err := httpDownload(ctx, config, metadata, incompletePath); err != nil {
		return err
	}

	// Move the file to final destination
	if err := os.Rename(incompletePath, destPath); err != nil {
		return fmt.Errorf("failed to move file to final destination: %w", err)
	}

	return nil
}

// httpDownload performs the actual HTTP download with progress reporting
func httpDownload(ctx context.Context, config *DownloadConfig, metadata *FileMetadata, filePath string) error {
	// Create progress manager if config supports it
	var progressManager *ProgressManager
	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		progressManager = hubConfig.CreateProgressManager()
	}

	// Create or open file for writing (append mode for resume)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get current file size for resume
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	resumeSize := fileInfo.Size()

	// Skip download if file is already complete
	if resumeSize > 0 && metadata.Size > 0 && resumeSize >= metadata.Size {
		if progressManager != nil {
			progressManager.LogDownloadComplete(config.RepoID, config.Filename, 0, metadata.Size)
		}
		return nil
	}

	// Calculate remaining bytes to download
	remainingSize := metadata.Size - resumeSize
	if remainingSize <= 0 {
		remainingSize = metadata.Size
	}

	// Start download logging and progress tracking
	startTime := time.Now()
	if progressManager != nil {
		progressManager.LogDownloadStart(config.RepoID, config.Filename, remainingSize)
	}

	// Create progress bar for this file
	var progressBar *progressbar.ProgressBar
	var progressWriter io.Writer = file

	if progressManager != nil {
		filename := filepath.Base(config.Filename)
		progressBar = progressManager.CreateFileProgressBar(filename, remainingSize)
		if progressBar != nil {
			progressWriter = NewProgressWriter(progressBar, file)
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", metadata.Location, nil)
	if err != nil {
		if progressManager != nil {
			progressManager.LogError("http_request_creation", config.RepoID, err)
		}
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	headers := BuildHeaders(config.Token, "huggingface-hub-go/1.0.0", config.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add range header for resume
	if resumeSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeSize))
		if progressManager != nil && progressManager.enableDetailedLogs {
			logger := progressManager.logger.
				WithField("repo_id", config.RepoID).
				WithField("filename", config.Filename).
				WithField("resume_size", resumeSize)
			logger.Info("Resuming download from byte position")
		}
	}

	// Perform request
	client := &http.Client{
		Timeout: DownloadTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		if progressManager != nil {
			progressManager.LogError("http_request", config.RepoID, err)
		}
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
		if progressManager != nil {
			progressManager.LogError("http_response", config.RepoID, err)
		}
		return err
	}

	// Copy response body to file with progress tracking
	_, err = io.Copy(progressWriter, resp.Body)
	if err != nil {
		if progressManager != nil {
			progressManager.LogError("file_write", config.RepoID, err)
		}
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Complete progress bar
	if progressBar != nil {
		if err := progressBar.Finish(); err != nil {
			// Log error but don't fail the download
			if progressManager != nil && progressManager.logger != nil {
				progressManager.logger.Debug("Failed to finish progress bar")
			}
		}
	}

	// Log completion
	duration := time.Since(startTime)
	if progressManager != nil {
		progressManager.LogDownloadComplete(config.RepoID, config.Filename, duration, remainingSize)
	}

	return nil
}

// FileMetadata contains metadata about a file from the Hub
type FileMetadata struct {
	CommitHash string
	Etag       string
	Location   string
	Size       int64
}

// getMetadataOrCatchError gets file metadata from the Hub API
func getMetadataOrCatchError(ctx context.Context, config *DownloadConfig, storageFolder, relativeFilename string) (*FileMetadata, error) {
	if config.LocalFilesOnly {
		return nil, NewOfflineModeIsEnabledError("Cannot access file since local_files_only=true")
	}

	// Construct URL for HEAD request
	url, err := HfHubURL(config.RepoID, config.Filename, config)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	// Create HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	// Add headers
	headers := BuildHeaders(config.Token, "huggingface-hub-go/1.0.0", config.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept-Encoding", "identity") // Prevent compression

	// Perform request
	client := &http.Client{
		Timeout: config.EtagTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HEAD request: %w", err)
	}
	defer resp.Body.Close()

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, handleHTTPError(resp, config.RepoID, config.RepoType, config.Revision, config.Filename)
	}

	// Extract metadata from headers
	metadata := &FileMetadata{
		CommitHash: resp.Header.Get(HuggingfaceHeaderXRepoCommit),
		Etag:       NormalizeEtag(resp.Header.Get(HuggingfaceHeaderXLinkedEtag)),
		Location:   url, // Default to request URL
		Size:       0,
	}

	// Use ETag header if X-Linked-Etag is not available
	if metadata.Etag == "" {
		metadata.Etag = NormalizeEtag(resp.Header.Get("ETag"))
	}

	// Get file size
	if contentLength := resp.Header.Get(HuggingfaceHeaderXLinkedSize); contentLength != "" {
		if size, err := parseSize(contentLength); err == nil {
			metadata.Size = size
		}
	}
	if metadata.Size == 0 {
		if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
			if size, err := parseSize(contentLength); err == nil {
				metadata.Size = size
			}
		}
	}

	// Use redirect location if available
	if location := resp.Header.Get("Location"); location != "" {
		metadata.Location = location
	}

	// For local directory downloads, we can be more flexible with commit hash
	// as we don't need the cache structure
	isLocalDirDownload := config.LocalDir != ""

	// Validate required fields
	if metadata.CommitHash == "" && !isLocalDirDownload {
		// For cache downloads, we need commit hash for proper cache structure
		return nil, NewFileMetadataError(config.Filename, "Distant resource does not seem to be on huggingface.co")
	}
	if metadata.Etag == "" {
		return nil, NewFileMetadataError(config.Filename, "Distant resource does not have an ETag")
	}
	if metadata.Size == 0 {
		return nil, NewFileMetadataError(config.Filename, "Distant resource does not have a Content-Length")
	}

	// For local dir downloads without commit hash, use revision as fallback
	if metadata.CommitHash == "" && isLocalDirDownload {
		metadata.CommitHash = config.Revision
	}

	return metadata, nil
}

// handleHTTPError converts HTTP errors to appropriate Hub errors
func handleHTTPError(resp *http.Response, repoID, repoType, revision, filename string) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return NewEntryNotFoundError(repoID, repoType, revision, filename, resp)
	case http.StatusUnauthorized:
		return NewRepositoryNotFoundError(repoID, repoType, resp)
	case http.StatusForbidden:
		return NewGatedRepoError(repoID, repoType, resp)
	default:
		return NewHTTPError(fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode, resp)
	}
}

// tryToLoadFromCache attempts to load a file from the cache
func tryToLoadFromCache(config *DownloadConfig, storageFolder, relativeFilename string) string {
	var commitHash string

	if IsCommitHash(config.Revision) {
		commitHash = config.Revision
	} else {
		// Try to resolve revision to commit hash
		refPath := filepath.Join(storageFolder, "refs", config.Revision)
		if content, err := os.ReadFile(refPath); err == nil {
			commitHash = strings.TrimSpace(string(content))
		}
	}

	if commitHash != "" {
		pointerPath, err := GetPointerPath(storageFolder, commitHash, relativeFilename)
		if err == nil && FileExists(pointerPath) {
			return pointerPath
		}
	}

	return ""
}

// parseSize parses a size string to int64
func parseSize(s string) (int64, error) {
	var size int64
	if _, err := fmt.Sscanf(s, "%d", &size); err != nil {
		return 0, err
	}
	return size, nil
}
