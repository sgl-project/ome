# OCI Object Storage Data Store

The OCI Object Storage package provides a robust data store abstraction backed by Oracle Object Storage (OCI). It supports object uploads, downloads (including multipart), metadata inspection, and local integrity validation via MD5 checksum comparison.

## Features

- **Functional Options API**: Clean, flexible configuration using functional options pattern
- **Downloads with Strategy**: Automatically chooses between standard and multipart downloads based on file size
- **Multipart Downloads**: Efficient parallel downloading for large files with configurable chunk sizes and thread counts
- **Integrity Validation**: MD5 checksum verification for downloaded files
- **Path Manipulation**: Flexible options for handling object paths during downloads
- **Retry Logic**: Built-in retry mechanisms for robust operations
- **Local Data Store**: File system-based implementation for testing and local development
- **Dependency Injection**: Full support for fx-based dependency injection
- **Comprehensive Testing**: Extensive test suite with 24.5% coverage focusing on all testable business logic

## Installation

```bash
go get github.com/sgl-project/sgl-ome/pkg/ociobjectstore
```

## Quick Start

### Basic Configuration

```go
package main

import (
    "github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
    "github.com/sgl-project/sgl-ome/pkg/principals"
    "github.com/spf13/viper"
)

func main() {
    // Configure using Viper
    v := viper.New()
    v.Set("auth_type", "InstancePrincipal")
    v.Set("region_override", "us-chicago-1")
    
    // Create logger (implement logging.Interface)
    logger := &YourLogger{}
    
    // Create data store
    cds, err := ociobjectstore.ProvideOCIOSDataStore(v, logger)
    if err != nil {
        panic(err)
    }
    
    // Use the data store...
}
```

### Manual Configuration

```go
authType := principals.InstancePrincipal
config := &ociobjectstore.Config{
    AuthType:      &authType,
    Name:          "my-ociobjectstore-store",
    Region:        "us-chicago-1",
    AnotherLogger: logger,
}

cds, err := ociobjectstore.NewOCIOSDataStore(config)
if err != nil {
    panic(err)
}
```

## API Usage

### Downloads with Functional Options

The functional options API provides a clean and flexible way to configure downloads:

#### Simple Downloads

```go
source := ociobjectstore.ObjectURI{
    BucketName: "my-bucket",
    ObjectName: "path/to/file.txt",
}

// Basic download with default options
err := cds.Download(source, "/local/target/dir")

// Download with custom thread count
err = cds.Download(source, "/local/target/dir", 
    ociobjectstore.WithThreads(10))

// Download with multiple options
err = cds.Download(source, "/local/target/dir",
    ociobjectstore.WithThreads(20),
    ociobjectstore.WithChunkSize(16), // 16MB chunks
    ociobjectstore.WithSizeThreshold(50)) // Use multipart for files > 50MB
```

#### Download with Strategy (Recommended)

Smart downloads automatically choose the best download method based on file size:

```go
// Smart download with automatic method selection
err := cds.DownloadWithStrategy(source, "/local/target/dir",
    ociobjectstore.WithSizeThreshold(100),  // Use multipart for files > 100MB
    ociobjectstore.WithChunkSize(8),        // 8MB chunks for multipart
    ociobjectstore.WithThreads(15))         // 15 concurrent threads

// Force multipart download regardless of size
err = cds.DownloadWithStrategy(source, "/local/target/dir",
    ociobjectstore.WithForceMultipart(true),
    ociobjectstore.WithChunkSize(32),
    ociobjectstore.WithThreads(25))

// Force standard download regardless of size
err = cds.DownloadWithStrategy(source, "/local/target/dir",
    ociobjectstore.WithForceStandard(true))
```

#### Bulk Downloads

Download multiple objects with concurrency control:

```go
objects := []ociobjectstore.ObjectURI{
    {BucketName: "bucket1", ObjectName: "file1.txt"},
    {BucketName: "bucket1", ObjectName: "file2.txt"},
    {BucketName: "bucket1", ObjectName: "large-file.bin"},
}

err := cds.BulkDownload(objects, "/local/target/dir", 5, // 5 concurrent downloads
    ociobjectstore.WithSizeThreshold(50),
    ociobjectstore.WithChunkSize(16),
    ociobjectstore.WithThreads(10),
    ociobjectstore.WithOverrideEnabled(false)) // Skip existing valid files
```

### Path Manipulation Options

Control how object paths are handled during downloads:

```go
// Strip prefix from object paths
err := cds.Download(source, "/local/target/dir",
    ociobjectstore.WithStripPrefix("models/v1/"))

// Use only the base filename
err = cds.Download(source, "/local/target/dir",
    ociobjectstore.WithBaseNameOnly(true))

// Join paths with tail overlap detection
err = cds.Download(source, "/local/target/dir",
    ociobjectstore.WithTailOverlap(true))

// Exclude certain patterns
err = cds.BulkDownload(objects, "/local/target/dir", 5,
    ociobjectstore.WithExcludePatterns([]string{"*.tmp", "*.log", ".DS_Store"}))
```

### File Integrity and Overrides

```go
// Enable file override (re-download existing files)
err := cds.Download(source, "/local/target/dir",
    ociobjectstore.WithOverrideEnabled(true))

// Disable override (skip existing valid files) - default behavior
err = cds.Download(source, "/local/target/dir",
    ociobjectstore.WithOverrideEnabled(false))

// Check if local copy is valid
valid, err := cds.IsLocalCopyValid(source, "/local/path/to/file.txt")
if err != nil {
    // Handle error
}
if !valid {
    // File needs to be re-downloaded
}
```

### Uploads

```go
target := ociobjectstore.ObjectURI{
    BucketName: "my-bucket",
    ObjectName: "uploaded/file.txt",
}

// Upload a file
err := cds.Upload("/local/path/to/file.txt", target)

// Upload string content directly
err = cds.Upload("Hello, World!", target)

// Multipart upload for large files
err = cds.MultipartFileUpload("/local/large-file.bin", target, 
    16, // 16MB chunks
    10) // 10 concurrent threads
```

### Object Operations

```go
// List objects with prefix
objects, err := cds.ListObjects(ociobjectstore.ObjectURI{
    BucketName: "my-bucket",
    Prefix:     "models/v1/",
})

// Get object metadata
metadata, err := cds.HeadObject(ociobjectstore.ObjectURI{
    BucketName: "my-bucket",
    ObjectName: "file.txt",
})

// Get object content
response, err := cds.GetObject(source)
defer response.Content.Close()
// Read from response.Content...
```

## Available Functional Options

| Option                          | Description                                | Example                                  |
|---------------------------------|--------------------------------------------|------------------------------------------|
| `WithSizeThreshold(mb int)`     | Set size threshold for multipart downloads | `WithSizeThreshold(100)`                 |
| `WithChunkSize(mb int)`         | Set chunk size for multipart downloads     | `WithChunkSize(16)`                      |
| `WithThreads(count int)`        | Set number of concurrent threads           | `WithThreads(20)`                        |
| `WithForceStandard(bool)`       | Force standard download                    | `WithForceStandard(true)`                |
| `WithForceMultipart(bool)`      | Force multipart download                   | `WithForceMultipart(true)`               |
| `WithOverrideEnabled(bool)`     | Enable/disable file override               | `WithOverrideEnabled(true)`              |
| `WithExcludePatterns([]string)` | Exclude files matching patterns            | `WithExcludePatterns([]string{"*.tmp"})` |
| `WithStripPrefix(string)`       | Strip prefix from object paths             | `WithStripPrefix("models/v1/")`          |
| `WithBaseNameOnly(bool)`        | Use only base filename                     | `WithBaseNameOnly(true)`                 |
| `WithTailOverlap(bool)`         | Enable tail overlap path joining           | `WithTailOverlap(true)`                  |

## Configuration Options

### Viper Configuration Keys

```yaml
# Required
auth_type: "InstancePrincipal"  # or "UserPrincipal", "ResourcePrincipal"

# Optional
name: "my-ociobjectstore-store"
region_override: "us-chicago-1"
compartment_id: "ocid1.compartment.oc1..example"
enable_obo_token: false
obo_token: "your-obo-token"  # Required if enable_obo_token is true
```

### Authentication Types

- `InstancePrincipal`: Use OCI instance principal authentication
- `UserPrincipal`: Use OCI user principal authentication  
- `ResourcePrincipal`: Use OCI resource principal authentication

### OBO Token Support

For On-Behalf-Of (OBO) token authentication:

```go
config := &ociobjectstore.Config{
    AuthType:       &authType,
    EnableOboToken: true,
    OboToken:       "your-obo-token",
    AnotherLogger:  logger,
}
```

## Dependency Injection with Fx

### Single Data Store

```go
package main

import (
    "github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
    "go.uber.org/fx"
)

func main() {
    app := fx.New(
        // Provide dependencies
        fx.Provide(NewViper),
        fx.Provide(NewLogger),
        
        // Add OCI Object Store module
        ociobjectstore.OCIOSDataStoreModule,
        
        // Use the data store
        fx.Invoke(func(cds *ociobjectstore.OCIOSDataStore) {
            // Your application logic here
        }),
    )
    
    app.Run()
}
```

### Multiple Data Stores

```go
func main() {
    app := fx.New(
        fx.Provide(NewLogger),
        fx.Provide(func() []*ociobjectstore.Config {
            return []*ociobjectstore.Config{
                {AuthType: &instancePrincipal, Name: "store1"},
                {AuthType: &instancePrincipal, Name: "store2"},
            }
        }),
        fx.Provide(ociobjectstore.ProvideListOfOCIOSDataStoreWithAppParams),
        fx.Invoke(func(stores []*ociobjectstore.OCIOSDataStore) {
            // Use multiple stores
        }),
    )
    
    app.Run()
}
```

## Local Data Store

For testing and local development:

```go
lds := &ociobjectstore.LocalDataStore{
    WorkingDirectory: "/local/storage/path",
}

// Implements the same DataStore interface
err := lds.Download(source, "/target/dir")
err = lds.Upload("/source/file.txt", target)
```

## Error Handling

The package provides detailed error messages for different failure scenarios:

```go
err := cds.Download(source, target)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "failed to apply download options"):
        // Invalid download options
    case strings.Contains(err.Error(), "object not found"):
        // Object doesn't exist
    case strings.Contains(err.Error(), "failed to get object"):
        // Network or permission error
    default:
        // Other errors
    }
}
```

## Performance Tuning

### Optimal Settings for Different Use Cases

#### Small Files (< 10MB)
```go
ociobjectstore.WithForceStandard(true),
ociobjectstore.WithThreads(5)
```

#### Medium Files (10MB - 100MB)
```go
ociobjectstore.WithSizeThreshold(50),
ociobjectstore.WithChunkSize(8),
ociobjectstore.WithThreads(10)
```

#### Large Files (> 100MB)
```go
ociobjectstore.WithForceMultipart(true),
ociobjectstore.WithChunkSize(16),
ociobjectstore.WithThreads(20)
```

#### Bulk Downloads
```go
// Use moderate concurrency to avoid overwhelming the service
cds.BulkDownload(objects, target, 5, // 5 concurrent downloads
    ociobjectstore.WithSizeThreshold(50),
    ociobjectstore.WithChunkSize(8),
    ociobjectstore.WithThreads(8))
```
