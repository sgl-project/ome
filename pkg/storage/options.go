package storage

import "time"

// UploadOption configures upload operations
type UploadOption func(*UploadOptions)

// DownloadOption configures download operations
type DownloadOption func(*DownloadOptions)

// ListOption configures list operations
type ListOption func(*ListOptions)

// BulkOption configures bulk operations
type BulkOption func(*BulkOptions)

// UploadOptions contains configuration for upload operations
type UploadOptions struct {
	ContentType  string
	Metadata     map[string]string
	Progress     ProgressReporter
	PartSize     int64  // For multipart uploads
	Concurrency  int    // Number of parallel parts for multipart
	StorageClass string // Storage class/tier
}

// DownloadOptions contains configuration for download operations
type DownloadOptions struct {
	Range                   *Range // For partial downloads
	Progress                ProgressReporter
	Concurrency             int    // Number of parallel chunks (>1 enables parallel download for large files)
	VerifyETag              string // Verify ETag after download
	SkipIfValid             bool   // Skip download if valid local copy exists (similar to DisableOverride)
	ForceRedownload         bool   // Force download even if local copy exists
	DisableParallelDownload bool   // Disable parallel download even for large files

	// Path manipulation options
	StripPrefix     bool   // If true, remove a specified prefix from the object path
	PrefixToStrip   string // The prefix to strip when StripPrefix is true
	UseBaseNameOnly bool   // If true, download using only the object's base name

	// Advanced options
	ExcludePatterns     []string // Object names to exclude (glob patterns)
	JoinWithTailOverlap bool     // Join with tail overlap if true (for chunked downloads)
}

// ListOptions contains configuration for list operations
type ListOptions struct {
	MaxResults    int
	Delimiter     string
	Recursive     bool
	IncludeHidden bool
	StartAfter    string // For pagination
}

// BulkOptions contains configuration for bulk operations
type BulkOptions struct {
	Concurrency     int
	Progress        ProgressReporter
	ContinueOnError bool
	RetryAttempts   int
	RetryDelay      time.Duration
}

// DefaultUploadOptions returns default upload options
func DefaultUploadOptions() UploadOptions {
	return UploadOptions{
		ContentType: "application/octet-stream",
		Metadata:    make(map[string]string),
		Concurrency: 5,
		PartSize:    5 * 1024 * 1024, // 5MB default part size
	}
}

// DefaultDownloadOptions returns default download options
func DefaultDownloadOptions() DownloadOptions {
	return DownloadOptions{
		Concurrency: 5,
	}
}

// DefaultListOptions returns default list options
func DefaultListOptions() ListOptions {
	return ListOptions{
		MaxResults: 1000,
		Recursive:  false,
	}
}

// DefaultBulkOptions returns default bulk options
func DefaultBulkOptions() BulkOptions {
	return BulkOptions{
		Concurrency:     5,
		ContinueOnError: true,
		RetryAttempts:   3,
		RetryDelay:      1 * time.Second,
	}
}

// Upload Options

// WithContentType sets the content type for upload
func WithContentType(contentType string) UploadOption {
	return func(o *UploadOptions) {
		o.ContentType = contentType
	}
}

// WithMetadata sets metadata for upload
func WithMetadata(metadata map[string]string) UploadOption {
	return func(o *UploadOptions) {
		o.Metadata = metadata
	}
}

// WithUploadProgress sets the progress reporter for upload
func WithUploadProgress(progress ProgressReporter) UploadOption {
	return func(o *UploadOptions) {
		o.Progress = progress
	}
}

// WithPartSize sets the part size for multipart upload
func WithPartSize(size int64) UploadOption {
	return func(o *UploadOptions) {
		o.PartSize = size
	}
}

// WithUploadConcurrency sets the concurrency for multipart upload
func WithUploadConcurrency(concurrency int) UploadOption {
	return func(o *UploadOptions) {
		o.Concurrency = concurrency
	}
}

// WithStorageClass sets the storage class/tier
func WithStorageClass(class string) UploadOption {
	return func(o *UploadOptions) {
		o.StorageClass = class
	}
}

// Download Options

// WithRange sets the byte range for partial download
func WithRange(start, end int64) DownloadOption {
	return func(o *DownloadOptions) {
		o.Range = &Range{
			Start: start,
			End:   end,
		}
	}
}

// WithDownloadProgress sets the progress reporter for download
func WithDownloadProgress(progress ProgressReporter) DownloadOption {
	return func(o *DownloadOptions) {
		o.Progress = progress
	}
}

// WithDownloadConcurrency sets the concurrency for parallel download
func WithDownloadConcurrency(concurrency int) DownloadOption {
	return func(o *DownloadOptions) {
		o.Concurrency = concurrency
	}
}

// WithETagVerification sets the expected ETag for verification
func WithETagVerification(etag string) DownloadOption {
	return func(o *DownloadOptions) {
		o.VerifyETag = etag
	}
}

// WithSkipIfValid skips download if a valid local copy exists
func WithSkipIfValid(skip bool) DownloadOption {
	return func(o *DownloadOptions) {
		o.SkipIfValid = skip
	}
}

// WithForceRedownload forces download even if local copy exists
func WithForceRedownload(force bool) DownloadOption {
	return func(o *DownloadOptions) {
		o.ForceRedownload = force
	}
}

// WithStripPrefix enables prefix stripping from object paths
func WithStripPrefix(prefix string) DownloadOption {
	return func(o *DownloadOptions) {
		o.StripPrefix = true
		o.PrefixToStrip = prefix
	}
}

// WithUseBaseNameOnly downloads using only the object's base name
func WithUseBaseNameOnly(useBaseName bool) DownloadOption {
	return func(o *DownloadOptions) {
		o.UseBaseNameOnly = useBaseName
	}
}

// WithExcludePatterns sets patterns for objects to exclude
func WithExcludePatterns(patterns []string) DownloadOption {
	return func(o *DownloadOptions) {
		o.ExcludePatterns = patterns
	}
}

// WithJoinWithTailOverlap enables joining with tail overlap for chunked downloads
func WithJoinWithTailOverlap(join bool) DownloadOption {
	return func(o *DownloadOptions) {
		o.JoinWithTailOverlap = join
	}
}

// WithDisableParallelDownload disables parallel download for large files
func WithDisableParallelDownload(disable bool) DownloadOption {
	return func(o *DownloadOptions) {
		o.DisableParallelDownload = disable
	}
}

// List Options

// WithMaxResults sets the maximum number of results
func WithMaxResults(max int) ListOption {
	return func(o *ListOptions) {
		o.MaxResults = max
	}
}

// WithDelimiter sets the delimiter for hierarchical listing
func WithDelimiter(delimiter string) ListOption {
	return func(o *ListOptions) {
		o.Delimiter = delimiter
	}
}

// WithRecursive enables recursive listing
func WithRecursive(recursive bool) ListOption {
	return func(o *ListOptions) {
		o.Recursive = recursive
	}
}

// WithIncludeHidden includes hidden files in listing
func WithIncludeHidden(include bool) ListOption {
	return func(o *ListOptions) {
		o.IncludeHidden = include
	}
}

// WithStartAfter sets the start position for pagination
func WithStartAfter(marker string) ListOption {
	return func(o *ListOptions) {
		o.StartAfter = marker
	}
}

// BuildUploadOptions applies upload options and returns the configuration
func BuildUploadOptions(opts ...UploadOption) UploadOptions {
	options := DefaultUploadOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildDownloadOptions applies download options and returns the configuration
func BuildDownloadOptions(opts ...DownloadOption) DownloadOptions {
	options := DefaultDownloadOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildListOptions applies list options and returns the configuration
func BuildListOptions(opts ...ListOption) ListOptions {
	options := DefaultListOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// BuildBulkOptions applies bulk options and returns the configuration
func BuildBulkOptions(opts ...BulkOption) BulkOptions {
	options := DefaultBulkOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// Bulk Options

// WithBulkConcurrency sets the concurrency for bulk operations
func WithBulkConcurrency(concurrency int) BulkOption {
	return func(o *BulkOptions) {
		o.Concurrency = concurrency
	}
}

// WithBulkProgress sets the progress reporter for bulk operations
func WithBulkProgress(progress ProgressReporter) BulkOption {
	return func(o *BulkOptions) {
		o.Progress = progress
	}
}

// WithContinueOnError sets whether to continue on error during bulk operations
func WithContinueOnError(continueOnError bool) BulkOption {
	return func(o *BulkOptions) {
		o.ContinueOnError = continueOnError
	}
}

// WithRetryAttempts sets the number of retry attempts for bulk operations
func WithRetryAttempts(attempts int) BulkOption {
	return func(o *BulkOptions) {
		o.RetryAttempts = attempts
	}
}

// WithRetryDelay sets the delay between retries for bulk operations
func WithRetryDelay(delay time.Duration) BulkOption {
	return func(o *BulkOptions) {
		o.RetryDelay = delay
	}
}
