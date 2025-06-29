# Storage Package

The storage package provides a unified interface for interacting with multiple cloud storage providers including OCI Object Storage, AWS S3, Google Cloud Storage, Azure Blob Storage, and GitHub LFS.

## Features

- **Unified Interface**: Single API for all storage providers
- **High Performance**: Support for parallel uploads/downloads and multipart operations
- **Flexible Authentication**: Pluggable auth system with support for multiple credential types
- **URI Parsing**: Parse and validate storage URIs for all supported providers
- **Functional Options**: Configure operations with clean, composable options
- **Dependency Injection**: Built with dependency injection for testability

## Supported Providers

- **OCI Object Storage** (`oci://`)
- **AWS S3** (`s3://`)
- **Google Cloud Storage** (`gs://`)
- **Azure Blob Storage** (`azure://`)
- **GitHub LFS** (`github://`)

## Installation

```go
import "github.com/sgl-project/ome/pkg/storage"
```

## Quick Start

### Creating a Storage Instance

```go
// Create logger and factories
logger := logging.NewLogger()
authFactory := auth.NewDefaultFactory(logger)
storageFactory := storage.NewDefaultFactory(authFactory, logger)

// Configure storage
config := storage.StorageConfig{
    Provider: storage.ProviderOCI,
    Region:   "us-ashburn-1",
    AuthConfig: auth.Config{
        Provider: auth.ProviderOCI,
        AuthType: auth.OCIInstancePrincipal,
    },
}

// Create storage instance
ctx := context.Background()
store, err := storageFactory.Create(ctx, config.Provider, &config)
```

### Downloading Objects

```go
// Parse source URI
source, err := storage.ParseURI("oci://namespace/bucket/path/to/file.txt")

// Download with options
err = store.Download(ctx, *source, "/local/path/file.txt",
    storage.WithChunkSize(50),        // 50MB chunks
    storage.WithThreads(10),          // 10 parallel threads
    storage.WithOverrideEnabled(true), // Override existing files
)
```

### Uploading Objects

```go
// Parse target URI
target, err := storage.ParseURI("s3://my-bucket/uploads/data.json")

// Upload with options
err = store.Upload(ctx, "/local/data.json", *target,
    storage.WithContentType("application/json"),
    storage.WithStorageClass("STANDARD_IA"),
    storage.WithMetadata(map[string]string{
        "version": "1.0",
    }),
)
```

### Listing Objects

```go
uri, _ := storage.ParseURI("gs://bucket/prefix/")
objects, err := store.List(ctx, *uri, storage.ListOptions{
    Prefix:  "data/",
    MaxKeys: 100,
})

for _, obj := range objects {
    fmt.Printf("%s (size: %d)\n", obj.Name, obj.Size)
}
```

## URI Formats

### OCI Object Storage
- `oci://namespace@region/bucket/prefix`
- `oci://n/namespace/b/bucket/o/prefix`

### AWS S3
- `s3://bucket/prefix/object.txt`

### Google Cloud Storage
- `gs://bucket/prefix/object.txt`

### Azure Blob Storage
- `azure://container@storageaccount/prefix/blob.txt`

### GitHub LFS
- `github://owner/repo@branch/path/to/file.txt`

## Download Options

- `WithSizeThreshold(mb)` - File size threshold for multipart download
- `WithChunkSize(mb)` - Chunk size for multipart operations
- `WithThreads(n)` - Number of parallel threads
- `WithForceStandard(bool)` - Force standard download
- `WithForceMultipart(bool)` - Force multipart download
- `WithOverrideEnabled(bool)` - Override existing files
- `WithExcludePatterns(patterns)` - Patterns to exclude
- `WithStripPrefix(prefix)` - Strip prefix from paths
- `WithBaseNameOnly(bool)` - Use only base filename
- `WithTailOverlap(bool)` - Enable tail overlap detection

## Upload Options

- `WithUploadChunkSize(mb)` - Chunk size for multipart uploads
- `WithUploadThreads(n)` - Number of upload threads
- `WithContentType(type)` - Content type of the object
- `WithMetadata(map)` - Object metadata
- `WithStorageClass(class)` - Storage class/tier

## Authentication

The storage package uses the auth package for authentication. Each provider supports multiple authentication methods:

### OCI
- User Principal (API Key)
- Instance Principal
- Resource Principal
- OKE Workload Identity

### AWS
- Access Key
- Instance Profile
- Assume Role
- Web Identity

### GCP
- Service Account
- Application Default Credentials
- Workload Identity

### Azure
- Service Principal
- Managed Identity
- Device Flow

### GitHub
- Personal Access Token
- GitHub App

## Testing

The package includes comprehensive unit tests. Run tests with:

```bash
go test ./pkg/storage/...
```

## Future Enhancements

- Complete implementation of AWS S3, GCP, Azure, and GitHub providers
- Add support for more advanced features:
  - Server-side encryption
  - Lifecycle policies
  - Cross-region replication
  - Event notifications
- Performance optimizations for large-scale operations
- Metrics and observability integration