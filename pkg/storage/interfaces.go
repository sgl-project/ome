package storage

import (
	"context"
	"io"
	"time"

	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// Provider represents the storage provider type
type Provider string

const (
	ProviderOCI    Provider = "oci"
	ProviderS3     Provider = "s3"
	ProviderGCS    Provider = "gcs"
	ProviderAzure  Provider = "azure"
	ProviderGitHub Provider = "github"
	ProviderLocal  Provider = "local"
	ProviderPVC    Provider = "pvc"
)

// Storage is the main interface that all storage backends must implement
type Storage interface {
	// Provider returns the storage provider type
	Provider() Provider

	Download(ctx context.Context, source string, target string, opts ...DownloadOption) error
	Upload(ctx context.Context, source string, target string, opts ...UploadOption) error

	Get(ctx context.Context, uri string) (io.ReadCloser, error)
	Put(ctx context.Context, uri string, reader io.Reader, size int64, opts ...UploadOption) error

	Delete(ctx context.Context, uri string) error
	Exists(ctx context.Context, uri string) (bool, error)
	List(ctx context.Context, uri string, opts ...ListOption) ([]ObjectInfo, error)
	Stat(ctx context.Context, uri string) (*Metadata, error)
	Copy(ctx context.Context, source string, target string) error
}

// MultipartCapable interface for providers that support multipart uploads
type MultipartCapable interface {
	InitiateMultipartUpload(ctx context.Context, uri string, opts ...UploadOption) (string, error)
	UploadPart(ctx context.Context, uri string, uploadID string, partNumber int, reader io.Reader, size int64) (string, error)
	CompleteMultipartUpload(ctx context.Context, uri string, uploadID string, parts []Part) error
	AbortMultipartUpload(ctx context.Context, uri string, uploadID string) error
}

// BulkStorage interface for providers that support bulk operations
type BulkStorage interface {
	BulkDownload(ctx context.Context, downloads []BulkDownloadItem, opts ...BulkOption) (*BulkDownloadResult, error)
	BulkUpload(ctx context.Context, uploads []BulkUploadItem, opts ...BulkOption) (*BulkUploadResult, error)
}

// ObjectInfo contains information about a storage object
type ObjectInfo struct {
	Name         string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	IsDir        bool
}

// Metadata contains detailed metadata about a storage object
type Metadata struct {
	Name         string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	Metadata     map[string]string
	StorageClass string
}

// Part represents a part in a multipart upload
type Part struct {
	PartNumber int
	ETag       string
	Size       int64
}

// BulkDownloadItem represents a single item in a bulk download operation
type BulkDownloadItem struct {
	Source string
	Target string
}

// BulkUploadItem represents a single item in a bulk upload operation
type BulkUploadItem struct {
	Source string
	Target string
}

// BulkDownloadResult contains the results of a bulk download operation
type BulkDownloadResult struct {
	Successful []string
	Failed     map[string]error
	TotalBytes int64
	Duration   time.Duration
}

// BulkUploadResult contains the results of a bulk upload operation
type BulkUploadResult struct {
	Successful []string
	Failed     map[string]error
	TotalBytes int64
	Duration   time.Duration
}

// Config provides configuration for storage providers
type Config struct {
	Provider   Provider
	AuthConfig *AuthConfig // Authentication configuration
	Region     string
	Endpoint   string // For S3-compatible or custom endpoints
	Bucket     string // Default bucket/container
	Namespace  string // For OCI
	Extra      map[string]interface{}
}

// AuthConfig wraps authentication configuration for storage providers
type AuthConfig struct {
	Provider string // auth provider type (aws, azure, gcp, oci)
	Type     string // auth type (e.g., access_key, service_account)
	Region   string
	Extra    map[string]interface{} // Provider-specific auth config
}

// Factory creates storage providers based on configuration
type Factory interface {
	CreateStorage(ctx context.Context, config Config) (Storage, error)
	SupportedProviders() []Provider
}

// ProgressReporter reports operation progress
type ProgressReporter interface {
	Update(bytesTransferred, totalBytes int64)
	Done()
	Error(err error)
}

// Range specifies a byte range for partial downloads
type Range struct {
	Start int64
	End   int64
}

// Type is an alias to the existing StorageType for backward compatibility
type Type = utilstorage.StorageType

// Re-export storage type constants for convenience
const (
	TypeS3          = utilstorage.StorageTypeS3
	TypeAzure       = utilstorage.StorageTypeAzure
	TypeGCS         = utilstorage.StorageTypeGCS
	TypeOCI         = utilstorage.StorageTypeOCI
	TypePVC         = utilstorage.StorageTypePVC
	TypeLocal       = utilstorage.StorageTypeLocal
	TypeGitHub      = utilstorage.StorageTypeGitHub
	TypeHuggingFace = utilstorage.StorageTypeHuggingFace
	TypeVendor      = utilstorage.StorageTypeVendor
)
