package hub

import (
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// HubConfigKey is the context key for storing HubConfig
	HubConfigKey contextKey = "hubConfig"
	// WorkerIDKey is the context key for storing worker ID in concurrent downloads
	WorkerIDKey contextKey = "workerID"
)

// HfFileMetadata contains information about a file versioned on the Hub
type HfFileMetadata struct {
	CommitHash *string `json:"commit_hash,omitempty"`
	Etag       *string `json:"etag,omitempty"`
	Location   string  `json:"location"`
	Size       *int64  `json:"size,omitempty"`
}

// RepoFileInfo contains information about a file in a repository
type RepoFileInfo struct {
	Path       string          `json:"path"` // File path relative to repo root
	Size       int64           `json:"size"` // File size in bytes
	BlobID     string          `json:"oid"`  // Git object ID
	LFS        *LFSInfo        `json:"lfs,omitempty"`
	LastCommit *LastCommitInfo `json:"lastCommit,omitempty"`
}

// LFSInfo contains LFS metadata for large files
type LFSInfo struct {
	OID         string `json:"oid"`         // SHA256 hash of the file
	Size        int64  `json:"size"`        // Size in bytes
	PointerSize int    `json:"pointerSize"` // Size of the LFS pointer file
}

// LastCommitInfo contains information about the last commit that modified a file
type LastCommitInfo struct {
	OID   string    `json:"id"`
	Title string    `json:"title"`
	Date  time.Time `json:"date"`
}

// RepoInfo contains metadata about a repository
type RepoInfo struct {
	ID           string        `json:"id"`
	Author       *string       `json:"author,omitempty"`
	SHA          *string       `json:"sha,omitempty"`
	CreatedAt    *time.Time    `json:"createdAt,omitempty"`
	LastModified *time.Time    `json:"lastModified,omitempty"`
	Private      *bool         `json:"private,omitempty"`
	Disabled     *bool         `json:"disabled,omitempty"`
	Downloads    *int          `json:"downloads,omitempty"`
	Likes        *int          `json:"likes,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	PipelineTag  *string       `json:"pipeline_tag,omitempty"`
	LibraryName  *string       `json:"library_name,omitempty"`
	ModelType    *string       `json:"model_type,omitempty"`
	Gated        *string       `json:"gated,omitempty"` // "auto", "manual", or false
	Siblings     []RepoSibling `json:"siblings,omitempty"`
}

// RepoSibling contains basic information about a file in a repository
type RepoSibling struct {
	RFilename string   `json:"rfilename"`        // Relative filename
	Size      *int64   `json:"size,omitempty"`   // File size in bytes
	BlobID    *string  `json:"blobId,omitempty"` // Git object ID
	LFS       *LFSInfo `json:"lfs,omitempty"`    // LFS metadata if applicable
}

// DownloadConfig contains configuration for downloads
type DownloadConfig struct {
	// Repository information
	RepoID    string
	RepoType  string
	Revision  string
	Filename  string
	Subfolder string

	// Authentication
	Token string

	// Destination paths
	CacheDir string
	LocalDir string

	// Download behavior
	ForceDownload  bool
	LocalFilesOnly bool
	ResumeDownload bool

	// Network configuration
	Proxies     map[string]string
	EtagTimeout time.Duration
	Headers     map[string]string
	Endpoint    string

	// Concurrent downloads (for snapshots)
	MaxWorkers int

	// Pattern filtering (for snapshots)
	AllowPatterns  []string
	IgnorePatterns []string
}

// SnapshotDownloadResult contains the result of a snapshot download
type SnapshotDownloadResult struct {
	SnapshotPath    string           // Path to the downloaded snapshot
	CachedFiles     []string         // List of files that were already cached
	DownloadedFiles []string         // List of files that were downloaded
	SkippedFiles    []string         // List of files that were skipped
	Errors          map[string]error // Map of file paths to download errors
}

// DownloadProgress represents download progress information
type DownloadProgress struct {
	Filename        string
	BytesDownloaded int64
	TotalBytes      int64
	Speed           float64 // bytes per second
	ETA             time.Duration
}

// ProgressCallback is called during downloads to report progress
type ProgressCallback func(progress DownloadProgress)

// Client configuration options
type ClientConfig struct {
	Endpoint   string
	Token      string
	CacheDir   string
	UserAgent  string
	Headers    map[string]string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	MaxWorkers int
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Endpoint:   DefaultEndpoint,
		Token:      GetHfToken(),
		CacheDir:   GetCacheDir(),
		UserAgent:  "huggingface-hub-go/1.0.0",
		Headers:    make(map[string]string),
		Timeout:    DefaultRequestTimeout,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: DefaultRetryInterval,
		MaxWorkers: DefaultMaxWorkers,
	}
}
