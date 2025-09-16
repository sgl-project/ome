# xet-core Go Binding

This package provides a Go binding to the Rust xet-core library, offering improved performance for Hugging Face Hub operations through deduplication, chunking, and advanced caching strategies.

## Features

- **High Performance**: Leverages xet-core's content-addressable storage (CAS) system
- **Deduplication**: Automatic content-based deduplication across downloads
- **Chunking**: Intelligent file chunking for efficient storage and transfer
- **Caching**: Two-tier cache system (chunks and shards)
- **Compatibility**: Drop-in replacement for existing HF Hub client
- **Concurrent Downloads**: Configurable parallel download support
- **Progress Tracking**: Real-time download progress callbacks

## Project Structure

```
pkg/xet/
├── src/                    # Rust source code
│   ├── lib.rs             # Main library entry point
│   ├── ffi.rs             # FFI layer for C interop
│   ├── error.rs           # Error handling
│   └── hf_adapter.rs      # HF-specific adaptations
├── xet.go                  # Go binding implementation
├── hf_compat.go           # HF Hub compatibility layer
├── xet_test.go            # Go tests
├── xet.h                  # C header file
├── Cargo.toml             # Rust dependencies
├── Makefile               # Build automation
└── README.md              # This file
```

## Building

### Prerequisites

- Go 1.22+
- Rust 1.75+
- C compiler (gcc/clang)
- Make

### Build Commands

```bash
# Build the library
make build

# Run tests
make test

# Build for specific platform
make release-darwin-aarch64
make release-linux-amd64

# Clean build artifacts
make clean
```

## Usage

### Direct API

```go
package main

import (
    "fmt"
    "github.com/ome/ome/pkg/xet"
)

func main() {
    // Create client
    config := &xet.Config{
        Endpoint: "https://huggingface.co",
        Token:    "your-hf-token",
        CacheDir: "/path/to/cache",
    }
    
    client, err := xet.NewClient(config)
    if err != nil {
        panic(err)
    }
    defer client.Close()
    
    // Download a file
    req := &xet.DownloadRequest{
        RepoID:   "bert-base-uncased",
        Filename: "config.json",
        LocalDir: "/path/to/download",
    }
    
    path, err := client.DownloadFile(req)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Downloaded to: %s\n", path)
}
```

### HF Hub Compatibility Layer

```go
package main

import (
    "context"
    "github.com/ome/ome/pkg/xet"
)

func main() {
    config := &xet.DownloadConfig{
        RepoID:   "gpt2",
        Filename: "pytorch_model.bin",
        LocalDir: "/path/to/download",
    }
    
    ctx := context.Background()
    path, err := xet.HfHubDownload(ctx, config)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Downloaded to: %s\n", path)
}
```

## Feature Flag

The xet binding can be enabled/disabled via environment variable:

```bash
# Enable xet binding
export OME_USE_XET_BINDING=1

# Or use experimental flag
export OME_EXPERIMENTAL_XET=1
```

## Testing

### Run Tests

```bash
# Run all tests
make test

# Run Go tests only
go test -v ./...

# Run with specific test
go test -v -run TestDownloadFile

# Run benchmarks
go test -bench=.
```

### PoC Example

```bash
# Build the PoC
cd cmd/xet-poc
go build

# List files in a repository
./xet-poc -repo bert-base-uncased -list

# Download a single file
./xet-poc -repo bert-base-uncased -file config.json

# Download entire repository
./xet-poc -repo bert-base-uncased -snapshot

# Use compatibility layer
./xet-poc -repo bert-base-uncased -file config.json -compat
```

## Performance

Expected improvements over the current HF Hub client:

- **Download Speed**: 10-40% faster for large files
- **Deduplication**: 30%+ storage savings for model variants
- **Cache Hit Rate**: 95%+ for repeated downloads
- **Memory Usage**: Comparable or lower
- **Concurrent Downloads**: 2x throughput

## Migration Guide

### Phase 1: Testing
1. Enable feature flag: `export OME_USE_XET_BINDING=1`
2. Run existing workflows
3. Monitor performance and errors
4. Report issues

### Phase 2: Gradual Rollout
1. Enable for specific operations
2. A/B test performance
3. Collect metrics
4. Fix any compatibility issues

### Phase 3: Full Migration
1. Make xet binding default
2. Keep old implementation for fallback
3. Remove feature flag after stability

## Known Limitations (PoC)

- Mock data for file listing (not connected to real HF API)
- Basic error handling
- Limited progress tracking
- No actual xet-core integration (uses mock implementation)
- Missing pattern filtering for snapshot downloads
- No token refresh support
- No LFS pointer resolution

## TODO

- [ ] Connect to real xet-core library
- [ ] Implement proper HF API integration
- [ ] Add comprehensive error handling
- [ ] Implement progress callbacks
- [ ] Add context cancellation support
- [ ] Implement pattern filtering
- [ ] Add token refresh mechanism
- [ ] Support LFS files
- [ ] Add retry logic
- [ ] Implement caching strategies
- [ ] Add metrics collection
- [ ] Performance benchmarks

## Contributing

1. Make changes in appropriate files
2. Run `make fmt` to format code
3. Run `make test` to ensure tests pass
4. Submit PR with description of changes

## License

Same as OME project