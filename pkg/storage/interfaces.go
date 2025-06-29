package storage

import (
	"context"
	"io"
	"time"
)

// Provider represents a storage provider type
type Provider string

const (
	ProviderOCI    Provider = "oci"
	ProviderAWS    Provider = "aws"
	ProviderGCP    Provider = "gcp"
	ProviderAzure  Provider = "azure"
	ProviderGitHub Provider = "github"
)

// ObjectInfo represents metadata about a storage object
type ObjectInfo struct {
	Name         string
	Size         int64
	LastModified string
	ETag         string
	ContentType  string
	Metadata     map[string]string
	StorageClass string
}

// Metadata represents detailed metadata about a storage object
type Metadata struct {
	ObjectInfo
	ContentMD5   string            // MD5 checksum of the content
	CacheControl string            // Cache control header
	Expires      string            // Expiration time
	VersionID    string            // Version ID for versioned objects
	IsMultipart  bool              // Whether object was uploaded via multipart
	Parts        int               // Number of parts (for multipart objects)
	Headers      map[string]string // Additional headers
}

// ListOptions provides options for listing objects
type ListOptions struct {
	Prefix     string
	MaxKeys    int
	Delimiter  string
	StartAfter string
	Marker     string
}

// DownloadOptions provides options for downloading objects
type DownloadOptions struct {
	// Size threshold in MB above which multipart download is used
	SizeThresholdInMB int
	// Chunk size in MB for multipart downloads
	ChunkSizeInMB int
	// Number of concurrent download threads
	Threads int
	// Force standard download regardless of file size
	ForceStandard bool
	// Force multipart download regardless of file size
	ForceMultipart bool
	// Whether to override existing files
	DisableOverride bool
	// Patterns for object names to exclude from download
	ExcludePatterns []string
	// Strip prefix from object paths during download
	StripPrefix   bool
	PrefixToStrip string
	// Use only the object's base name (filename)
	UseBaseNameOnly bool
	// Join paths with tail overlap detection
	JoinWithTailOverlap bool
	// Skip downloading files that already exist locally
	SkipExisting bool
	// Validate MD5 checksums
	ValidateMD5 bool
}

// UploadOptions provides options for uploading objects
type UploadOptions struct {
	// Chunk size in MB for multipart uploads
	ChunkSizeInMB int
	// Number of concurrent upload threads
	Threads int
	// Content type of the object
	ContentType string
	// Metadata to attach to the object
	Metadata map[string]string
	// Storage class/tier
	StorageClass string
	// Calculate MD5 before upload
	CalculateMD5 bool
	// Tags to apply to uploaded objects
	Tags map[string]string
}

// DownloadOption represents a functional option for configuring download operations
type DownloadOption func(*DownloadOptions) error

// UploadOption represents a functional option for configuring upload operations
type UploadOption func(*UploadOptions) error

// ObjectURI defines the identity and location of an object in a storage system
type ObjectURI struct {
	Provider   Provider               `json:"provider"`
	Namespace  string                 `json:"namespace,omitempty"` // OCI-specific
	BucketName string                 `json:"bucket_name" validate:"required"`
	ObjectName string                 `json:"object_name,omitempty"`
	Prefix     string                 `json:"prefix,omitempty"`
	Region     string                 `json:"region,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"` // Provider-specific fields
}

// Storage defines the interface for storage operations across different providers
type Storage interface {
	// Provider returns the storage provider type
	Provider() Provider

	// Download retrieves the object and writes it to the target path
	Download(ctx context.Context, source ObjectURI, target string, opts ...DownloadOption) error

	// Upload stores the file at source path as the target object
	Upload(ctx context.Context, source string, target ObjectURI, opts ...UploadOption) error

	// Get retrieves an object and returns a reader
	Get(ctx context.Context, uri ObjectURI) (io.ReadCloser, error)

	// Put stores data from reader as an object
	Put(ctx context.Context, uri ObjectURI, reader io.Reader, size int64, opts ...UploadOption) error

	// Delete removes an object
	Delete(ctx context.Context, uri ObjectURI) error

	// Exists checks if an object exists
	Exists(ctx context.Context, uri ObjectURI) (bool, error)

	// List returns a list of objects matching the criteria
	List(ctx context.Context, uri ObjectURI, opts ListOptions) ([]ObjectInfo, error)

	// GetObjectInfo retrieves metadata about an object
	GetObjectInfo(ctx context.Context, uri ObjectURI) (*ObjectInfo, error)

	// Stat retrieves metadata about an object (alias for GetObjectInfo)
	Stat(ctx context.Context, uri ObjectURI) (*Metadata, error)

	// Copy copies an object within the same storage system
	Copy(ctx context.Context, source, target ObjectURI) error
}

// BulkStorage extends Storage with bulk operations support
type BulkStorage interface {
	Storage

	// BulkDownload downloads multiple objects concurrently
	BulkDownload(ctx context.Context, objects []ObjectURI, targetDir string, opts BulkDownloadOptions) ([]BulkDownloadResult, error)

	// BulkUpload uploads multiple files concurrently
	BulkUpload(ctx context.Context, files []BulkUploadFile, opts BulkUploadOptions) ([]BulkUploadResult, error)
}

// ValidatingStorage extends Storage with validation support
type ValidatingStorage interface {
	Storage

	// GetWithValidation retrieves an object with MD5 validation
	GetWithValidation(ctx context.Context, uri ObjectURI, expectedMD5 string) (io.ReadCloser, error)

	// PutWithValidation stores data with MD5 validation
	PutWithValidation(ctx context.Context, uri ObjectURI, reader io.Reader, size int64, expectedMD5 string, opts ...UploadOption) error

	// ValidateLocalFile checks if a local file matches the remote object
	ValidateLocalFile(ctx context.Context, localPath string, uri ObjectURI) (bool, error)
}

// ProgressStorage extends Storage with progress tracking support
type ProgressStorage interface {
	Storage

	// DownloadWithProgress downloads with progress tracking
	DownloadWithProgress(ctx context.Context, source ObjectURI, target string, progress ProgressCallback, opts ...DownloadOption) error

	// UploadWithProgress uploads with progress tracking
	UploadWithProgress(ctx context.Context, source string, target ObjectURI, progress ProgressCallback, opts ...UploadOption) error
}

// ExtendedStorage provides additional storage operations
type ExtendedStorage interface {
	Storage

	// SetMetadata updates object metadata
	SetMetadata(ctx context.Context, uri ObjectURI, metadata Metadata) error

	// GetPresignedURL generates a pre-signed URL for an object
	GetPresignedURL(ctx context.Context, uri ObjectURI, expiry int) (string, error)

	// ListVersions lists all versions of an object
	ListVersions(ctx context.Context, uri ObjectURI) ([]Metadata, error)

	// RestoreVersion restores a specific version of an object
	RestoreVersion(ctx context.Context, uri ObjectURI, versionID string) error
}

// MultipartCapable indicates storage supports multipart operations
type MultipartCapable interface {
	// InitiateMultipartUpload starts a multipart upload
	InitiateMultipartUpload(ctx context.Context, uri ObjectURI, opts ...UploadOption) (string, error)

	// UploadPart uploads a part of a multipart upload
	UploadPart(ctx context.Context, uri ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error)

	// CompleteMultipartUpload completes a multipart upload
	CompleteMultipartUpload(ctx context.Context, uri ObjectURI, uploadID string, parts []CompletedPart) error
	// AbortMultipartUpload cancels a multipart upload
	AbortMultipartUpload(ctx context.Context, uri ObjectURI, uploadID string) error
}

// RetryableStorage wraps storage operations with retry logic
type RetryableStorage interface {
	Storage

	// SetRetryConfig sets the retry configuration
	SetRetryConfig(config RetryConfig)

	// GetRetryConfig returns the current retry configuration
	GetRetryConfig() RetryConfig
}

// DownloadStrategy defines how downloads should be performed
type DownloadStrategy interface {
	// ShouldUseMultipart determines if multipart download should be used
	ShouldUseMultipart(size int64, opts DownloadOptions) bool

	// CalculatePartSize calculates optimal part size for downloads
	CalculatePartSize(totalSize int64, opts DownloadOptions) int64

	// ValidateDownload validates a downloaded file
	ValidateDownload(localPath string, metadata Metadata) error
}

// UploadStrategy defines how uploads should be performed
type UploadStrategy interface {
	// ShouldUseMultipart determines if multipart upload should be used
	ShouldUseMultipart(size int64, opts UploadOptions) bool

	// CalculatePartSize calculates optimal part size for uploads
	CalculatePartSize(totalSize int64, opts UploadOptions) int64

	// PrepareUpload prepares an upload (e.g., calculates checksums)
	PrepareUpload(localPath string, opts UploadOptions) (*UploadPlan, error)
}

// StorageMetrics provides metrics about storage operations
type StorageMetrics interface {
	// GetMetrics returns current metrics
	GetMetrics() Metrics

	// ResetMetrics resets all metrics
	ResetMetrics()
}

// ValidatingReader wraps a reader with MD5 validation
type ValidatingReader interface {
	io.ReadCloser
	// Valid returns true if the data read so far is valid
	Valid() bool
	// Expected returns the expected MD5 checksum
	Expected() string
	// Actual returns the actual MD5 checksum calculated so far
	Actual() string
}

// ValidatingWriter wraps a writer with MD5 calculation
type ValidatingWriter interface {
	io.WriteCloser
	// Sum returns the MD5 checksum of data written
	Sum() string
	// SumBase64 returns the MD5 checksum as base64
	SumBase64() string
}

// StorageFactory creates storage instances
type StorageFactory interface {
	// Create creates a storage instance for the given provider
	Create(ctx context.Context, provider Provider, config interface{}) (Storage, error)
}

// CompletedPart represents a completed part of a multipart upload
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// UploadPlan contains information about how an upload will be performed
type UploadPlan struct {
	UseMultipart bool
	PartSize     int64
	NumParts     int
	MD5          string
	ContentType  string
	Metadata     map[string]string
}

// Metrics contains storage operation metrics
type Metrics struct {
	DownloadCount      int64
	DownloadBytes      int64
	DownloadErrors     int64
	DownloadDuration   int64 // nanoseconds
	UploadCount        int64
	UploadBytes        int64
	UploadErrors       int64
	UploadDuration     int64 // nanoseconds
	DeleteCount        int64
	DeleteErrors       int64
	ListCount          int64
	ListErrors         int64
	RetryCount         int64
	ValidationFailures int64
}

// ProgressCallback is called to report progress
type ProgressCallback func(progress Progress)

// Progress represents the progress of an operation
type Progress struct {
	TotalBytes     int64         // Total bytes to process
	ProcessedBytes int64         // Bytes processed so far
	TotalFiles     int           // Total number of files
	ProcessedFiles int           // Files processed so far
	CurrentFile    string        // Current file being processed
	StartTime      time.Time     // When the operation started
	CurrentSpeed   float64       // Current speed in bytes per second
	AverageSpeed   float64       // Average speed in bytes per second
	EstimatedTime  time.Duration // Estimated time remaining
	ElapsedTime    time.Duration // Time elapsed
	Error          error         // Last error encountered
}

// BulkDownloadResult represents the result of a single download in a bulk operation
type BulkDownloadResult struct {
	URI           ObjectURI
	TargetPath    string
	Size          int64
	Duration      time.Duration
	Error         error
	RetryAttempts int
}

// BulkDownloadOptions configures bulk download operations
type BulkDownloadOptions struct {
	Concurrency      int             // Number of concurrent downloads
	RetryConfig      RetryConfig     // Retry configuration
	DownloadOptions  DownloadOptions // Options for individual downloads
	ProgressCallback func(completed, total int, current *BulkDownloadResult)
}

// BulkUploadResult represents the result of a single upload in a bulk operation
type BulkUploadResult struct {
	SourcePath    string
	URI           ObjectURI
	Size          int64
	Duration      time.Duration
	Error         error
	RetryAttempts int
}

// BulkUploadOptions configures bulk upload operations
type BulkUploadOptions struct {
	Concurrency      int           // Number of concurrent uploads
	RetryConfig      RetryConfig   // Retry configuration
	UploadOptions    UploadOptions // Options for individual uploads
	ProgressCallback func(completed, total int, current *BulkUploadResult)
}

// BulkUploadFile represents a file to upload
type BulkUploadFile struct {
	SourcePath string
	TargetURI  ObjectURI
}

// RetryConfig defines the configuration for retry operations
type RetryConfig struct {
	MaxRetries     int              // Maximum number of retry attempts
	InitialDelay   time.Duration    // Initial delay between retries
	MaxDelay       time.Duration    // Maximum delay between retries
	Multiplier     float64          // Exponential backoff multiplier
	Jitter         bool             // Add random jitter to delays
	RetryableError func(error) bool // Function to determine if error is retryable
}

// DefaultDownloadOptions returns default download options
func DefaultDownloadOptions() DownloadOptions {
	return DownloadOptions{
		SizeThresholdInMB: 10,
		ChunkSizeInMB:     10,
		Threads:           10,
		DisableOverride:   false,
	}
}

// DefaultUploadOptions returns default upload options
func DefaultUploadOptions() UploadOptions {
	return UploadOptions{
		ChunkSizeInMB: 10,
		Threads:       10,
	}
}

// DefaultBulkDownloadOptions returns default bulk download options
func DefaultBulkDownloadOptions() BulkDownloadOptions {
	return BulkDownloadOptions{
		Concurrency:     4,
		RetryConfig:     DefaultRetryConfig(),
		DownloadOptions: DefaultDownloadOptions(),
	}
}

// DefaultBulkUploadOptions returns default bulk upload options
func DefaultBulkUploadOptions() BulkUploadOptions {
	return BulkUploadOptions{
		Concurrency:   4,
		RetryConfig:   DefaultRetryConfig(),
		UploadOptions: DefaultUploadOptions(),
	}
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableError: func(err error) bool {
			// By default, retry all errors
			// In production, this should check for specific error types
			return true
		},
	}
}

// WithDownloadProgress adds progress tracking to downloads
func WithDownloadProgress(callback ProgressCallback) DownloadOption {
	return func(opts *DownloadOptions) error {
		// This would be implemented by storage providers that support progress
		return nil
	}
}

// WithUploadProgress adds progress tracking to uploads
func WithUploadProgress(callback ProgressCallback) UploadOption {
	return func(opts *UploadOptions) error {
		// This would be implemented by storage providers that support progress
		return nil
	}
}

// WithSkipExisting skips downloading files that already exist locally
func WithSkipExisting() DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.SkipExisting = true
		return nil
	}
}

// WithValidation enables MD5 validation
func WithValidation() DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.ValidateMD5 = true
		return nil
	}
}

// WithUploadContentType sets the content type for uploads
func WithUploadContentType(contentType string) UploadOption {
	return func(opts *UploadOptions) error {
		opts.ContentType = contentType
		return nil
	}
}

// WithUploadStorageClass sets the storage class for uploads
func WithUploadStorageClass(storageClass string) UploadOption {
	return func(opts *UploadOptions) error {
		opts.StorageClass = storageClass
		return nil
	}
}
