package hub

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Package-level random source for jitter calculation
var (
	jitterRand     *rand.Rand
	jitterRandOnce sync.Once
)

// initJitterRand initializes the random source for jitter calculation
func initJitterRand() {
	jitterRandOnce.Do(func() {
		// Use current time nanoseconds as seed for non-deterministic randomness
		source := rand.NewSource(time.Now().UnixNano())
		jitterRand = rand.New(source)
	})
}

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

	// Cross platform transcription of filename - WITH SECURITY VALIDATION
	if strings.Contains(config.Filename, "..") {
		return "", fmt.Errorf("invalid filename: path traversal detected in %s", config.Filename)
	}
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

	// If server returned 304 Not Modified, use cached version
	if metadata.NotModified {
		pointerPath, err := GetPointerPath(storageFolder, metadata.CommitHash, relativeFilename)
		if err == nil && FileExists(pointerPath) {
			return pointerPath, nil
		}
		// If pointer doesn't exist, continue to download (shouldn't happen normally)
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
		// Validate cached blob if ETag is available
		if metadata.Etag != "" && IsSHA256(metadata.Etag) {
			if err := VerifyChecksum(blobPath, metadata.Etag); err != nil {
				// Cached file is corrupted, remove it and re-download
				os.Remove(blobPath)
				// Continue to download section below
			} else {
				// Cached file is valid, create symlink and return
				if err := CreateSymlink(blobPath, pointerPath); err != nil {
					return "", fmt.Errorf("failed to create symlink: %w", err)
				}
				return pointerPath, nil
			}
		} else {
			// No ETag validation possible, trust cached file
			if err := CreateSymlink(blobPath, pointerPath); err != nil {
				return "", fmt.Errorf("failed to create symlink: %w", err)
			}
			return pointerPath, nil
		}
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
				//// For LFS files, verify checksum
				//if metadata.Etag != "" && IsSHA256(metadata.Etag) {
				//	if err := VerifyChecksum(filePath, metadata.Etag); err == nil {
				//		return filePath, nil
				//	}
				//} else {
				//	// For non-LFS files, size match is sufficient
				//	return filePath, nil
				//}
				return filePath, nil
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
		err := os.Remove(incompletePath)
		if err != nil {
			return err
		}
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

	// Validate ETag if available (SHA256 hash)
	//if metadata.Etag != "" && IsSHA256(metadata.Etag) {
	//	if err := VerifyChecksum(incompletePath, metadata.Etag); err != nil {
	//		// Remove the invalid file
	//		os.Remove(incompletePath)
	//		return fmt.Errorf("ETag validation failed: %w", err)
	//	}
	//}

	// Ensure the incomplete file exists before trying to rename
	if _, err := os.Stat(incompletePath); err != nil {
		// Check if the destination file already exists (might have been moved by another worker)
		if _, destErr := os.Stat(destPath); destErr == nil {
			// Destination exists, verify size
			destInfo, _ := os.Stat(destPath)
			if metadata.Size > 0 && destInfo.Size() == metadata.Size {
				// File already moved successfully, remove incomplete file if it exists
				os.Remove(incompletePath)
				return nil
			}
		}
		return fmt.Errorf("incomplete file missing before rename: %w", err)
	}

	// Move the file to final destination
	if err := os.Rename(incompletePath, destPath); err != nil {
		// Check if it's a cross-device error and try copy instead
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to move file to final destination: %w (incomplete file may have been deleted)", err)
		}
		return fmt.Errorf("failed to move file to final destination: %w", err)
	}

	return nil
}

// httpDownload performs the actual HTTP download with progress reporting and retry logic
func httpDownload(ctx context.Context, config *DownloadConfig, metadata *FileMetadata, filePath string) error {
	// Get retry configuration from context (HubConfig)
	var maxRetries int = 3                             // default
	var retryInterval time.Duration = 10 * time.Second // default

	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		maxRetries = hubConfig.MaxRetries
		retryInterval = hubConfig.RetryInterval
	}

	// Determine if progress should be shown
	showProgress := true
	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		showProgress = hubConfig.ShouldEnableProgress()
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
		return nil
	}

	// Calculate remaining bytes to download
	remainingSize := metadata.Size - resumeSize
	if remainingSize <= 0 {
		remainingSize = metadata.Size
	}

	// Create simple progress reporter with resume support
	filename := filepath.Base(config.Filename)
	var progress Progress
	if resumeSize > 0 && metadata.Size > 0 {
		// Use progress with resume support
		progress = NewProgressWithResume(filename, metadata.Size, resumeSize, showProgress)
	} else {
		// Normal progress for new downloads
		progress = NewProgress(filename, remainingSize, showProgress)
	}
	defer progress.Finish()

	// Wrap the file writer with progress reporting
	var progressWriter io.Writer = NewSimpleProgressWriter(file, progress)

	// Retry loop for HTTP download
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "GET", metadata.Location, nil)
		if err != nil {
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
		}

		// Use pooled client with download timeout
		client := NewHTTPClientWithTimeout(DownloadTimeout)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err

			// Check if this is the last attempt
			if attempt == maxRetries {
				return fmt.Errorf("failed to perform request after %d attempts: %w", maxRetries+1, lastErr)
			}

			// Wait with exponential backoff before retrying
			delay := exponentialBackoff(attempt+1, retryInterval)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}()

		// Check status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)

			// Check if this error is retryable
			if !retryableHTTPError(nil, resp.StatusCode) || attempt == maxRetries {
				return lastErr
			}

			// Calculate delay based on response type
			var delay time.Duration
			if resp.StatusCode == 429 {
				// First check for Retry-After header
				if retryAfterDelay := parseRetryAfter(resp); retryAfterDelay > 0 {
					delay = retryAfterDelay
				} else {
					// Fall back to exponential backoff with jitter
					delay = exponentialBackoffWithJitter(attempt+1, retryInterval, 5*time.Minute)
				}
			} else {
				// For other errors, use regular exponential backoff
				delay = exponentialBackoff(attempt+1, retryInterval)
			}

			// Wait before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		// Download successful - copy response body to file with context awareness
		_, err = copyWithContext(ctx, progressWriter, resp.Body)
		if err != nil {
			lastErr = err

			// Check if context was cancelled
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Check if this is the last attempt
			if attempt == maxRetries {
				return fmt.Errorf("failed to write file after %d attempts: %w", maxRetries+1, lastErr)
			}

			// Reset file position for retry
			if _, err := file.Seek(resumeSize, 0); err != nil {
				return fmt.Errorf("failed to reset file position: %w", err)
			}

			// Wait with exponential backoff before retrying
			delay := exponentialBackoff(attempt+1, retryInterval)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		// Success! Break out of retry loop
		break
	}

	// Validate checksum if this was a resumed download and we have an ETag
	if resumeSize > 0 && metadata.Etag != "" && IsSHA256(metadata.Etag) {
		// Close the file to flush all writes
		file.Close()

		// Verify the complete file
		if err := VerifyChecksum(filePath, metadata.Etag); err != nil {
			// File is corrupted, delete it
			os.Remove(filePath)
			return fmt.Errorf("checksum validation failed after resume: %w", err)
		}
	}

	return nil
}

// FileMetadata contains metadata about a file from the Hub
type FileMetadata struct {
	CommitHash  string
	Etag        string
	Location    string
	Size        int64
	NotModified bool // True if server returned 304 Not Modified
}

// GetHfFileMetadata fetches metadata for a file from the Hugging Face Hub
// This is a clean, standalone function for getting file metadata via HEAD request
func GetHfFileMetadata(ctx context.Context, repoID, filename string, opts ...DownloadOption) (*FileMetadata, error) {
	// Build config from options
	config := &DownloadConfig{
		RepoID:   repoID,
		Filename: filename,
		RepoType: "model",
		Revision: "main",
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Construct URL
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

	// Use pooled client with custom timeout
	client := NewHTTPClientWithTimeout(config.EtagTimeout)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HEAD request: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Extract metadata
	metadata := &FileMetadata{
		CommitHash: resp.Header.Get(HuggingfaceHeaderXRepoCommit),
		Etag:       NormalizeEtag(resp.Header.Get(HuggingfaceHeaderXLinkedEtag)),
		Location:   url,
	}

	// Fallback to standard ETag if X-Linked-Etag not available
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

	return metadata, nil
}

// getMetadataOrCatchError gets file metadata from the Hub API with retry logic
func getMetadataOrCatchError(ctx context.Context, config *DownloadConfig, storageFolder, relativeFilename string) (*FileMetadata, error) {
	if config.LocalFilesOnly {
		return nil, NewOfflineModeIsEnabledError("Cannot access file since local_files_only=true")
	}

	// Get retry configuration from context (HubConfig)
	var maxRetries int = 3                             // default
	var retryInterval time.Duration = 10 * time.Second // default

	if hubConfig, ok := ctx.Value(HubConfigKey).(*HubConfig); ok {
		maxRetries = hubConfig.MaxRetries
		retryInterval = hubConfig.RetryInterval
	}

	// Construct URL for HEAD request
	url, err := HfHubURL(config.RepoID, config.Filename, config)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	// Try to get cached ETag for conditional request
	var cachedEtag string
	if storageFolder != "" && relativeFilename != "" {
		// Try to resolve revision to commit hash if it's not already
		commitHash := config.Revision
		if !IsCommitHash(config.Revision) {
			// Try to read from refs to get the actual commit hash
			refPath := filepath.Join(storageFolder, "refs", config.Revision)
			if data, err := os.ReadFile(refPath); err == nil {
				commitHash = strings.TrimSpace(string(data))
			}
		}

		// Now check if we have a cached pointer for this file
		pointerPath, _ := GetPointerPath(storageFolder, commitHash, relativeFilename)
		if FileExists(pointerPath) {
			// Read the symlink to get the blob path and extract ETag
			if target, err := os.Readlink(pointerPath); err == nil {
				cachedEtag = filepath.Base(target)
			}
		}
	}

	// Retry loop for metadata fetching
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
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

		// Add If-None-Match header for conditional request
		if cachedEtag != "" {
			req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, cachedEtag))
		}

		// Use pooled client with custom timeout
		client := NewHTTPClientWithTimeout(config.EtagTimeout)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err

			// Check if this is the last attempt
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to perform HEAD request after %d attempts: %w", maxRetries+1, err)
			}

			// Wait with exponential backoff before retrying
			delay := exponentialBackoff(attempt+1, retryInterval)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}()

		// Handle 304 Not Modified response
		if resp.StatusCode == http.StatusNotModified {
			// Try to get size from cached file
			var fileSize int64 = -1
			if storageFolder != "" && relativeFilename != "" && cachedEtag != "" {
				blobPath := filepath.Join(storageFolder, "blobs", cachedEtag)
				if info, err := os.Stat(blobPath); err == nil {
					fileSize = info.Size()
				}
			}

			// File hasn't changed, return metadata indicating no modification needed
			return &FileMetadata{
				CommitHash:  config.Revision, // Use provided revision
				Etag:        cachedEtag,
				Location:    url,
				Size:        fileSize, // Use actual file size if available
				NotModified: true,
			}, nil
		}

		// Handle error responses
		if resp.StatusCode != http.StatusOK {
			lastErr = handleHTTPError(resp, config.RepoID, config.RepoType, config.Revision, config.Filename)

			// Check if this error is retryable
			if !retryableHTTPError(nil, resp.StatusCode) || attempt == maxRetries {
				return nil, lastErr
			}

			// Calculate delay based on response type
			var delay time.Duration
			if resp.StatusCode == 429 {
				// First check for Retry-After header
				if retryAfterDelay := parseRetryAfter(resp); retryAfterDelay > 0 {
					delay = retryAfterDelay
				} else {
					// Fall back to exponential backoff with jitter
					delay = exponentialBackoffWithJitter(attempt+1, retryInterval, 5*time.Minute)
				}
			} else {
				// For other errors, use regular exponential backoff
				delay = exponentialBackoff(attempt+1, retryInterval)
			}

			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
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

	// This should never be reached due to the loop structure, but just in case
	return nil, fmt.Errorf("failed to get metadata after %d attempts: %w", maxRetries+1, lastErr)
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
	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(resp)
		return NewRateLimitError(resp, retryAfter)
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

// contextReader wraps an io.Reader to respect context cancellation
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (n int, err error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		return cr.r.Read(p)
	}
}

// retryableHTTPError checks if an HTTP error is retryable
func retryableHTTPError(err error, statusCode int) bool {
	if err != nil {
		return true // Network errors are retryable
	}

	// Retry on server errors and rate limiting
	return statusCode >= 500 || statusCode == 429 || statusCode == 408
}

// exponentialBackoff calculates the delay for exponential backoff
func exponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	// Cap at 30 seconds to avoid extremely long delays
	delay := time.Duration(math.Min(float64(baseDelay)*math.Pow(2, float64(attempt-1)), float64(30*time.Second)))
	return delay
}

// exponentialBackoffWithJitter calculates the delay with jitter to prevent thundering herd
func exponentialBackoffWithJitter(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Ensure random source is initialized
	initJitterRand()

	// Calculate exponential delay
	delay := time.Duration(math.Min(
		float64(baseDelay)*math.Pow(2, float64(attempt-1)),
		float64(maxDelay),
	))

	// Add jitter (±25% of calculated delay)
	jitter := time.Duration(jitterRand.Float64() * 0.5 * float64(delay))
	if jitterRand.Intn(2) == 0 {
		delay -= jitter
	} else {
		delay += jitter
	}

	return delay
}

// parseRetryAfter parses the Retry-After header from HTTP 429 responses
func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try to parse as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as HTTP date
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// copyWithContext copies data from src to dst while respecting context cancellation
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	// Wrap the source reader to respect context cancellation
	contextSrc := &contextReader{ctx: ctx, r: src}

	// Use io.CopyBuffer with a smaller buffer for more frequent progress updates
	// 32KB buffer provides good balance between performance and update frequency
	buf := make([]byte, 32*1024)
	return io.CopyBuffer(dst, contextSrc, buf)
}
