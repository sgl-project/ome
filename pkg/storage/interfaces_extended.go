package storage

import (
	"context"
	"io"
)

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

	// Stat retrieves extended metadata about an object
	Stat(ctx context.Context, uri ObjectURI) (*Metadata, error)

	// SetMetadata updates object metadata
	SetMetadata(ctx context.Context, uri ObjectURI, metadata Metadata) error

	// GetPresignedURL generates a pre-signed URL for an object
	GetPresignedURL(ctx context.Context, uri ObjectURI, expiry int) (string, error)

	// ListVersions lists all versions of an object
	ListVersions(ctx context.Context, uri ObjectURI) ([]Metadata, error)

	// RestoreVersion restores a specific version of an object
	RestoreVersion(ctx context.Context, uri ObjectURI, versionID string) error
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

// UploadPlan contains information about how an upload will be performed
type UploadPlan struct {
	UseMultipart bool
	PartSize     int64
	NumParts     int
	MD5          string
	ContentType  string
	Metadata     map[string]string
}

// StorageMetrics provides metrics about storage operations
type StorageMetrics interface {
	// GetMetrics returns current metrics
	GetMetrics() Metrics

	// ResetMetrics resets all metrics
	ResetMetrics()
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

// Update DownloadOptions to include new fields
type DownloadOptionsExtended struct {
	DownloadOptions
	SkipExisting bool // Skip files that already exist locally
	ValidateMD5  bool // Validate MD5 checksums
}

// Update UploadOptions to include new fields
type UploadOptionsExtended struct {
	UploadOptions
	CalculateMD5 bool              // Calculate MD5 before upload
	Tags         map[string]string // Tags to apply to uploaded objects
}
