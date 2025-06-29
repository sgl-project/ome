package storage

import (
	"context"
	"io"
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

	// Copy copies an object within the same storage system
	Copy(ctx context.Context, source, target ObjectURI) error
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

// CompletedPart represents a completed part of a multipart upload
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// StorageFactory creates storage instances
type StorageFactory interface {
	// Create creates a storage instance for the given provider
	Create(ctx context.Context, provider Provider, config interface{}) (Storage, error)
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
