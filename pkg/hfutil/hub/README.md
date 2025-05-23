# Hugging Face Hub Go Client

A production-ready Go implementation of the Hugging Face Hub client, providing seamless integration with the Hugging Face ecosystem. This library closely follows the patterns and functionality of the official Python `huggingface_hub` library while adding enterprise-grade features for Go applications.

## 🚀 Features

### Core Functionality
- **Single File Downloads**: Download individual files with caching and resume support
- **Snapshot Downloads**: Download entire repositories with concurrent workers
- **Repository Listing**: Browse repository contents with metadata
- **Multiple Repository Types**: Support for models, datasets, and spaces
- **Authentication**: Full support for Hugging Face tokens and gated repositories

### Enterprise Features
- **Beautiful Progress Bars**: Real-time progress tracking using `github.com/schollz/progressbar/v3`
- **Structured Logging**: Comprehensive logging integration with popular Go frameworks
- **Dependency Injection**: Built-in support for dependency injection frameworks (fx, wire)
- **Configuration Management**: Functional options pattern with validation
- **Concurrent Downloads**: Optimized multi-worker downloads for performance
- **Resume Capability**: Intelligent resume for interrupted downloads
- **Smart Caching**: Efficient symlink-based caching following HuggingFace patterns

### Production Ready
- **Cross-Platform**: Full Windows, macOS, and Linux support
- **Error Handling**: Comprehensive error types matching Python library
- **Backward Compatibility**: Seamless migration from existing implementations
- **Performance Optimized**: Chunked downloads with configurable concurrency
- **Resource Management**: Automatic cleanup and disk space validation

## 📦 Installation

```bash
go get github.com/sgl-project/sgl-ome/pkg/hfutil/hub
```

### Dependencies

```go
// Core dependencies
github.com/schollz/progressbar/v3  // Beautiful progress bars
github.com/go-playground/validator/v10  // Configuration validation
github.com/spf13/viper  // Configuration management
go.uber.org/fx  // Dependency injection support

// Internal dependencies
github.com/sgl-project/sgl-ome/pkg/logging
github.com/sgl-project/sgl-ome/pkg/configutils
```

## 🎯 Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/sgl-project/sgl-ome/pkg/hfutil/hub"
)

func main() {
    ctx := context.Background()
    
    // Simple file download
    config := &hub.DownloadConfig{
        RepoID:   "microsoft/DialoGPT-medium",
        Filename: "config.json",
        Token:    "your_hf_token_here",
    }
    
    filePath, err := hub.HfHubDownload(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Downloaded to: %s", filePath)
}
```

### Enhanced Client

```go
package main

import (
    "context"
    "time"
    
    "github.com/sgl-project/sgl-ome/pkg/hfutil/hub"
)

func main() {
    // Create enhanced configuration
    config, err := hub.NewHubConfig(
        hub.WithToken("your_hf_token"),
        hub.WithConcurrency(8, 20*1024*1024), // 8 workers, 20MB chunks
        hub.WithProgressBars(true),
        hub.WithDetailedLogs(true),
        hub.WithTimeouts(30*time.Second, 10*time.Second, 15*time.Minute),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Create client
    client, err := hub.NewHubClient(config)
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // Download with options
    filePath, err := client.Download(
        ctx,
        "microsoft/DialoGPT-medium",
        "config.json",
        hub.WithRevision("main"),
        hub.WithForceDownload(false),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Snapshot download
    downloadPath, err := client.SnapshotDownload(
        ctx,
        "microsoft/DialoGPT-medium",
        "./downloads/",
        hub.WithRepoType(hub.RepoTypeModel),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

## 📖 Documentation

### Configuration Options

#### Environment Variables
```bash
export HF_TOKEN=hf_your_token_here          # Authentication token
export HF_HUB_CACHE=/custom/cache/path      # Custom cache directory  
export HF_HUB_OFFLINE=1                     # Enable offline mode
export HF_HUB_DISABLE_PROGRESS_BARS=1       # Disable progress bars
```

#### Programmatic Configuration
```go
config, err := hub.NewHubConfig(
    // Authentication
    hub.WithToken("hf_token"),
    
    // Network settings
    hub.WithEndpoint("https://huggingface.co"),
    hub.WithTimeouts(30*time.Second, 10*time.Second, 10*time.Minute),
    hub.WithRetryConfig(5, 2*time.Second),
    
    // Performance
    hub.WithConcurrency(8, 20*1024*1024),  // workers, chunk size
    hub.WithSymlinks(true),
    
    // UI and logging
    hub.WithProgressBars(true),
    hub.WithDetailedLogs(true),
    hub.WithLogLevel("info"),
    hub.WithLogger(yourLogger),
    
    // Storage
    hub.WithCacheDir("./cache"),
    hub.WithLocalFilesOnly(false),
    hub.WithOfflineMode(false),
)
```

### Core Functions

#### Single File Download
```go
// Download a single file to cache
filePath, err := hub.HfHubDownload(ctx, &hub.DownloadConfig{
    RepoID:   "microsoft/DialoGPT-medium",
    Filename: "config.json",
    Token:    token,
})

// Download to specific directory
filePath, err := hub.HfHubDownload(ctx, &hub.DownloadConfig{
    RepoID:   "microsoft/DialoGPT-medium", 
    Filename: "config.json",
    LocalDir: "./downloads/",
    Token:    token,
})
```

#### Repository Listing
```go
files, err := hub.ListRepoFiles(ctx, &hub.DownloadConfig{
    RepoID:   "microsoft/DialoGPT-medium",
    RepoType: hub.RepoTypeModel,
    Token:    token,
})

for _, file := range files {
    fmt.Printf("%s: %d bytes\n", file.Path, file.Size)
}
```

#### Snapshot Download
```go
downloadPath, err := hub.SnapshotDownload(ctx, &hub.DownloadConfig{
    RepoID:         "microsoft/DialoGPT-medium",
    LocalDir:       "./downloads/",
    AllowPatterns:  []string{"*.json", "*.txt"},      // Optional filtering
    IgnorePatterns: []string{"*.bin"},                // Optional filtering
    Token:          token,
})
```

### Enhanced Client API

#### Client Creation
```go
// Basic client
client, err := hub.NewHubClient(config)

// With dependency injection
type AppDeps struct {
    fx.In
    Logger logging.Interface `name:"hub_logger"`
}

// Use fx.Provide with hub.Module
```

#### Download Methods
```go
// Single file with options
path, err := client.Download(ctx, repoID, filename,
    hub.WithRevision("v1.0"),
    hub.WithRepoType(hub.RepoTypeModel),
    hub.WithForceDownload(true),
)

// Snapshot with options
path, err := client.SnapshotDownload(ctx, repoID, localDir,
    hub.WithPatterns(allowPatterns, ignorePatterns),
    hub.WithRepoType(hub.RepoTypeDataset),
)

// List files with options
files, err := client.ListFiles(ctx, repoID,
    hub.WithRepoType(hub.RepoTypeSpace),
)
```

### Repository Types

```go
// Supported repository types
hub.RepoTypeModel    // "model" - AI models
hub.RepoTypeDataset  // "dataset" - Training datasets  
hub.RepoTypeSpace    // "space" - Gradio/Streamlit apps
```

### Error Handling

The library provides rich error types that match the Python implementation:

```go
_, err := client.Download(ctx, repoID, filename)
if err != nil {
    switch e := err.(type) {
    case *hub.RepositoryNotFoundError:
        log.Printf("Repository %s not found", e.RepoID)
    case *hub.GatedRepoError:
        log.Printf("Repository %s requires authentication", e.RepoID)
    case *hub.EntryNotFoundError:
        log.Printf("File %s not found", e.Path)
    case *hub.HTTPError:
        log.Printf("HTTP %d: %s", e.StatusCode, e.Message)
    default:
        log.Printf("Other error: %v", err)
    }
}
```

## 🎨 Progress Bars & Logging

### Progress Bars

The library includes beautiful progress bars using `github.com/schollz/progressbar/v3`:

```go
config, err := hub.NewHubConfig(
    hub.WithProgressBars(true),  // Enable progress bars
    // ... other options
)

// Progress bars automatically appear for:
// - Individual file downloads
// - Snapshot downloads  
// - Repository listing operations
```

Sample output:
```
📄 config.json          ████████████████████████████████ 1.2KB/1.2KB [100%] 0s
📄 pytorch_model.bin    ████████████▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 2.1GB/3.4GB [62%] 45s
```

### Structured Logging

Integrate with your logging framework:

```go
import "github.com/sirupsen/logrus"

logger := logrus.New()

config, err := hub.NewHubConfig(
    hub.WithLogger(logger),
    hub.WithDetailedLogs(true),
    hub.WithLogLevel("info"),
)

// Logs include:
// - Download start/completion with metrics
// - Error details with context
// - Performance statistics
// - Operation tracking
```

Sample log output:
```json
{
  "level": "info",
  "msg": "Download completed successfully", 
  "repo_id": "microsoft/DialoGPT-medium",
  "filename": "config.json",
  "duration_ms": 1250,
  "size": 1247,
  "speed_bps": 997600
}
```

## 🏗️ Architecture

### Module Structure

```
pkg/hfutil/hub/
├── config.go          # Configuration management with functional options
├── constants.go       # Constants and environment variable handling
├── download.go        # Core download implementation (HfHubDownload)
├── errors.go          # Rich error types matching Python library
├── module.go          # Dependency injection support (fx integration)
├── progress.go        # Progress reporting and UI management
├── repo.go           # Repository operations (listing, snapshots)
├── types.go          # Data structures and type definitions
├── utils.go          # Utilities (URL construction, validation, file ops)
├── samples/          # Self-contained usage examples
│   ├── README.md
│   ├── basic_download.go
│   ├── enhanced_client.go
│   ├── progress_logging.go
│   └── llama_download.go
└── README.md         # This file
```

### Design Principles

1. **Python Compatibility**: API mirrors `huggingface_hub` Python library
2. **Enterprise Ready**: Built for production with logging, monitoring, DI
3. **Performance Focused**: Concurrent downloads, efficient caching
4. **Go Idiomatic**: Follows Go best practices and patterns
5. **Extensible**: Functional options, interface-based design

### Cache Structure

Following Hugging Face conventions:
```
cache/
├── models--microsoft--DialoGPT-medium/
│   ├── blobs/
│   │   ├── abc123...  # Actual file content
│   │   └── def456...
│   ├── snapshots/
│   │   └── commit_hash/
│   │       ├── config.json -> ../../blobs/abc123
│   │       └── pytorch_model.bin -> ../../blobs/def456
│   └── refs/
│       └── main      # Points to commit hash
```

## 🔄 Migration from Python

### API Comparison

| Python                 | Go                       |
|------------------------|--------------------------|
| `hf_hub_download()`    | `hub.HfHubDownload()`    |
| `snapshot_download()`  | `hub.SnapshotDownload()` |
| `list_repo_files()`    | `hub.ListRepoFiles()`    |
| `HfApi().list_files()` | `client.ListFiles()`     |

### Configuration Mapping

| Python            | Go                        |
|-------------------|---------------------------|
| `token`           | `WithToken()`             |
| `cache_dir`       | `WithCacheDir()`          |
| `local_dir`       | `LocalDir` in config      |
| `force_download`  | `ForceDownload` in config |
| `resume_download` | Always enabled            |

### Error Mapping

| Python                    | Go                             |
|---------------------------|--------------------------------|
| `RepositoryNotFoundError` | `*hub.RepositoryNotFoundError` |
| `GatedRepoError`          | `*hub.GatedRepoError`          |
| `EntryNotFoundError`      | `*hub.EntryNotFoundError`      |
| `LocalEntryNotFoundError` | `*hub.LocalEntryNotFoundError` |

## 🚀 Performance

### Benchmarks

- **Single File**: ~50MB/s on typical connections
- **Concurrent Downloads**: 3-5x faster with 6-8 workers
- **Resume Speed**: Near-instant for completed portions
- **Memory Usage**: ~10MB baseline + chunk size per worker

### Optimization Tips

1. **Tune Concurrency**: 
   ```go
   hub.WithConcurrency(8, 20*1024*1024) // 8 workers, 20MB chunks
   ```

2. **Use Local Directories** for one-time downloads:
   ```go
   config.LocalDir = "./downloads/"  // Skip cache overhead
   ```

3. **Enable Progress Bars** only for user-facing downloads:
   ```go
   hub.WithProgressBars(false)  // Disable for scripts
   ```

4. **Optimize Cache Location**:
   ```go
   hub.WithCacheDir("/fast/ssd/cache")  // Use fast storage
   ```

## 🧪 Testing

### Running Examples

```bash
cd pkg/hfutil/hub/samples

# Basic functionality
go run basic_download.go

# Enterprise features  
go run enhanced_client.go

# Progress and logging
go run progress_logging.go

# Large model download (requires token)
export HF_TOKEN=your_token
go run llama_download.go
```

### Unit Tests

```bash
cd pkg/hfutil/hub
go test -v ./...
```

### Integration Tests

```bash
# Test with real downloads (requires network)
export HF_TOKEN=your_token
go test -tags=integration -v ./...
```

## 🛠️ Development

### Contributing

1. **Follow Go conventions**: `gofmt`, `golint`, `go vet`
2. **Add tests**: Unit tests for new functionality
3. **Update docs**: Keep README and examples current
4. **Backward compatibility**: Don't break existing APIs

### Building

```bash
# Build and test
go build ./pkg/hfutil/hub/...
go test ./pkg/hfutil/hub/...

# Lint
golangci-lint run ./pkg/hfutil/hub/...

# Examples
cd pkg/hfutil/hub/samples
go run basic_download.go
```

## 📋 Changelog

### v0.0.1 (Current)
- ✅ Complete rewrite following Python `huggingface_hub` patterns
- ✅ Enterprise features: progress bars, logging, DI support
- ✅ Production-ready configuration management
- ✅ Comprehensive error handling
- ✅ Cross-platform symlink support
- ✅ Concurrent downloads with resume capability
- ✅ Beautiful progress bars with `schollz/progressbar`
- ✅ Self-contained examples and documentation

### Previous Versions
- Legacy implementation in `pkg/hfutil/download/` (deprecated)

## 🤝 Support

### Getting Help

1. **Check Examples**: Review `samples/` directory
2. **Read Documentation**: This README and inline docs
3. **File Issues**: For bugs or feature requests

### Known Limitations

- **Windows Symlinks**: Requires developer mode or admin privileges
- **Large Files**: Memory usage scales with chunk size × workers
- **Network Issues**: Retries may not cover all edge cases

### Roadmap

- [ ] Upload support (`hf_hub_upload`)
- [ ] Model card operations
- [ ] Advanced filtering options
- [ ] Metrics and monitoring integration
- [ ] Performance optimizations

## 📄 License

This project follows the same license as the parent repository.

---

**Ready to get started?** Check out the [examples](samples/README.md) directory for hands-on tutorials! 🚀 