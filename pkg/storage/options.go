package storage

import "time"

// UploadOption configures upload operations
type UploadOption func(*uploadOptions)

// DownloadOption configures download operations
type DownloadOption func(*downloadOptions)

// ListOption configures list operations
type ListOption func(*listOptions)

// BulkOption configures bulk operations
type BulkOption func(*bulkOptions)

// uploadOptions contains configuration for upload operations
type uploadOptions struct {
	ContentType  string
	Metadata     map[string]string
	Progress     ProgressReporter
	PartSize     int64  // For multipart uploads
	Concurrency  int    // Number of parallel parts for multipart
	StorageClass string // Storage class/tier
}

// downloadOptions contains configuration for download operations
type downloadOptions struct {
	Range       *Range // For partial downloads
	Progress    ProgressReporter
	Concurrency int    // Number of parallel chunks
	VerifyETag  string // Verify ETag after download
}

// listOptions contains configuration for list operations
type listOptions struct {
	MaxResults    int
	Delimiter     string
	Recursive     bool
	IncludeHidden bool
	StartAfter    string // For pagination
}

// bulkOptions contains configuration for bulk operations
type bulkOptions struct {
	Concurrency     int
	Progress        ProgressReporter
	ContinueOnError bool
	RetryAttempts   int
	RetryDelay      time.Duration
}

// DefaultUploadOptions returns default upload options
func DefaultUploadOptions() uploadOptions {
	return uploadOptions{
		ContentType: "application/octet-stream",
		Metadata:    make(map[string]string),
		Concurrency: 5,
		PartSize:    5 * 1024 * 1024, // 5MB default part size
	}
}

// DefaultDownloadOptions returns default download options
func DefaultDownloadOptions() downloadOptions {
	return downloadOptions{
		Concurrency: 5,
	}
}

// DefaultListOptions returns default list options
func DefaultListOptions() listOptions {
	return listOptions{
		MaxResults: 1000,
		Recursive:  false,
	}
}

// DefaultBulkOptions returns default bulk options
func DefaultBulkOptions() bulkOptions {
	return bulkOptions{
		Concurrency:     5,
		ContinueOnError: true,
		RetryAttempts:   3,
		RetryDelay:      1 * time.Second,
	}
}

// Upload Options

// WithContentType sets the content type for upload
func WithContentType(contentType string) UploadOption {
	return func(o *uploadOptions) {
		o.ContentType = contentType
	}
}

// WithMetadata sets metadata for upload
func WithMetadata(metadata map[string]string) UploadOption {
	return func(o *uploadOptions) {
		o.Metadata = metadata
	}
}

// WithUploadProgress sets the progress reporter for upload
func WithUploadProgress(progress ProgressReporter) UploadOption {
	return func(o *uploadOptions) {
		o.Progress = progress
	}
}

// WithPartSize sets the part size for multipart upload
func WithPartSize(size int64) UploadOption {
	return func(o *uploadOptions) {
		o.PartSize = size
	}
}

// WithUploadConcurrency sets the concurrency for multipart upload
func WithUploadConcurrency(concurrency int) UploadOption {
	return func(o *uploadOptions) {
		o.Concurrency = concurrency
	}
}

// WithStorageClass sets the storage class/tier
func WithStorageClass(class string) UploadOption {
	return func(o *uploadOptions) {
		o.StorageClass = class
	}
}

// Download Options

// WithRange sets the byte range for partial download
func WithRange(start, end int64) DownloadOption {
	return func(o *downloadOptions) {
		o.Range = &Range{
			Start: start,
			End:   end,
		}
	}
}

// WithDownloadProgress sets the progress reporter for download
func WithDownloadProgress(progress ProgressReporter) DownloadOption {
	return func(o *downloadOptions) {
		o.Progress = progress
	}
}

// WithDownloadConcurrency sets the concurrency for parallel download
func WithDownloadConcurrency(concurrency int) DownloadOption {
	return func(o *downloadOptions) {
		o.Concurrency = concurrency
	}
}

// WithETagVerification sets the expected ETag for verification
func WithETagVerification(etag string) DownloadOption {
	return func(o *downloadOptions) {
		o.VerifyETag = etag
	}
}

// List Options

// WithMaxResults sets the maximum number of results
func WithMaxResults(max int) ListOption {
	return func(o *listOptions) {
		o.MaxResults = max
	}
}

// WithDelimiter sets the delimiter for hierarchical listing
func WithDelimiter(delimiter string) ListOption {
	return func(o *listOptions) {
		o.Delimiter = delimiter
	}
}

// WithRecursive enables recursive listing
func WithRecursive(recursive bool) ListOption {
	return func(o *listOptions) {
		o.Recursive = recursive
	}
}

// WithIncludeHidden includes hidden files in listing
func WithIncludeHidden(include bool) ListOption {
	return func(o *listOptions) {
		o.IncludeHidden = include
	}
}

// WithStartAfter sets the start position for pagination
func WithStartAfter(marker string) ListOption {
	return func(o *listOptions) {
		o.StartAfter = marker
	}
}

// BuildUploadOptions applies upload options and returns the configuration
func BuildUploadOptions(opts ...UploadOption) uploadOptions {
	options := DefaultUploadOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildDownloadOptions applies download options and returns the configuration
func BuildDownloadOptions(opts ...DownloadOption) downloadOptions {
	options := DefaultDownloadOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildListOptions applies list options and returns the configuration
func BuildListOptions(opts ...ListOption) listOptions {
	options := DefaultListOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildBulkOptions applies bulk options and returns the configuration
func BuildBulkOptions(opts ...BulkOption) bulkOptions {
	options := DefaultBulkOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// Bulk Options

// WithBulkConcurrency sets the concurrency for bulk operations
func WithBulkConcurrency(concurrency int) BulkOption {
	return func(o *bulkOptions) {
		o.Concurrency = concurrency
	}
}

// WithBulkProgress sets the progress reporter for bulk operations
func WithBulkProgress(progress ProgressReporter) BulkOption {
	return func(o *bulkOptions) {
		o.Progress = progress
	}
}

// WithContinueOnError sets whether to continue on error during bulk operations
func WithContinueOnError(continueOnError bool) BulkOption {
	return func(o *bulkOptions) {
		o.ContinueOnError = continueOnError
	}
}

// WithRetryAttempts sets the number of retry attempts for bulk operations
func WithRetryAttempts(attempts int) BulkOption {
	return func(o *bulkOptions) {
		o.RetryAttempts = attempts
	}
}

// WithRetryDelay sets the delay between retries for bulk operations
func WithRetryDelay(delay time.Duration) BulkOption {
	return func(o *bulkOptions) {
		o.RetryDelay = delay
	}
}
