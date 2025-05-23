# Hugging Face Hub Go Examples

This directory contains comprehensive examples demonstrating the Hugging Face Hub Go implementation. Each example is self-contained and showcases different aspects of the library.

## Quick Start

1. **Set up your environment:**
   ```bash
   export HF_TOKEN=your_token_here  # Required for gated models
   cd pkg/hfutil/hub/samples
   ```

2. **Choose an example and run it:**
   ```bash
   go run basic_download.go         # Start here for basic functionality
   go run enhanced_client.go        # Enterprise features and configuration
   go run progress_logging.go       # Beautiful UI and logging
   go run llama_download.go         # Large model downloads
   ```

## Examples Overview

### 1. `basic_download.go` - Getting Started 🟢

**Best for:** First-time users, simple use cases

**Features demonstrated:**
- Single file downloads
- Repository file listing
- Snapshot downloads (all files)
- Basic error handling
- File verification

**Usage:**
```bash
go run basic_download.go
```

**What it downloads:** Microsoft DialoGPT-medium (moderate size, no auth required)

---

### 2. `enhanced_client.go` - Enterprise Features 🔵

**Best for:** Production deployments, advanced configuration

**Features demonstrated:**
- Functional options configuration pattern
- Enhanced HubClient with enterprise features
- Multiple repository types (models, datasets, spaces)
- Download options and customization
- Configuration validation
- Backward compatibility
- Comprehensive error handling

**Usage:**
```bash
go run enhanced_client.go
```

**What it shows:** Configuration patterns, client creation, different repo types

---

### 3. `progress_logging.go` - Beautiful UI & Monitoring 🟡

**Best for:** User-facing applications, monitoring needs

**Features demonstrated:**
- Beautiful progress bars using `github.com/schollz/progressbar/v3`
- Structured logging integration
- Real-time download statistics
- Enhanced terminal UI with Unicode characters
- Performance monitoring
- Detailed operation tracking

**Usage:**
```bash
go run progress_logging.go
```

**Dependencies:** This example showcases the progress bar integration

---

### 4. `llama_download.go` - Large Model Production Downloads 🔴

**Best for:** Production model deployments, large file handling

**Features demonstrated:**
- Authentication for gated models (Llama)
- Large file download optimization
- Enterprise-grade configuration for production
- Repository analysis and file categorization
- Storage requirement estimation
- Production monitoring and logging

**Usage:**
```bash
export HF_TOKEN=your_token_here  # Required!
go run llama_download.go
```

**Requirements:** 
- HF token required (Llama models are gated)
- ~5GB storage space
- Good internet connection

## Running Multiple Examples

Each example is completely self-contained. You can run them in any order:

```bash
# Run all examples in sequence
for example in basic_download.go enhanced_client.go progress_logging.go; do
    echo "Running $example..."
    go run "$example"
    echo "---"
done

# Run Llama example separately (requires token)
export HF_TOKEN=your_token_here
go run llama_download.go
```

## Sample Output

### Basic Download Example
```
🤗 Hugging Face Hub - Basic Download Example
============================================

Repository: microsoft/DialoGPT-medium
Target directory: ./downloads/DialoGPT-medium

📄 Example 1: Single File Download
----------------------------------
✅ Downloaded config.json to: /cache/models--microsoft--DialoGPT-medium/snapshots/abc123/config.json

📂 Example 2: Repository File Listing
-------------------------------------
Found 12 items in repository:
  📄 config.json (1.2 KB)
  📄 pytorch_model.bin (346.3 MB)
  📄 tokenizer.json (2.1 MB)
  ...

📦 Example 3: Snapshot Download (All Files)
-------------------------------------------
This will download 8 files (348.7 MB) to ./downloads/DialoGPT-medium
Proceed with snapshot download? (y/N): y
Starting snapshot download...
✅ Snapshot download completed!
```

### Enhanced Client Example
```
🚀 Hugging Face Hub - Enhanced Client Example
==============================================

⚙️  Example 1: Enhanced Configuration
------------------------------------
✅ Configuration created:
   Endpoint: https://huggingface.co
   Cache Dir: ./cache
   User Agent: MyApp/1.0.0
   Max Workers: 4
   Chunk Size: 8 MB
   Max Retries: 3
```

## Configuration Examples

### Environment Variables
```bash
export HF_TOKEN=hf_...                    # Your Hugging Face token
export HF_HUB_CACHE=/custom/cache/path    # Custom cache directory
export HF_HUB_OFFLINE=1                   # Enable offline mode
```

### Programmatic Configuration
```go
config, err := hub.NewHubConfig(
    hub.WithToken("your_token"),
    hub.WithCacheDir("./custom_cache"),
    hub.WithConcurrency(8, 20*1024*1024),  // 8 workers, 20MB chunks
    hub.WithProgressBars(true),
    hub.WithDetailedLogs(true),
    hub.WithRetryConfig(5, 10*time.Second),
)
```

## Troubleshooting

### Common Issues

1. **Authentication errors:**
   ```
   Error: Repository not found or requires authentication
   ```
   **Solution:** Set your HF_TOKEN environment variable

2. **Permission errors:**
   ```
   Error: Failed to create directory
   ```
   **Solution:** Check write permissions for target directories

3. **Network timeouts:**
   ```
   Error: context deadline exceeded
   ```
   **Solution:** Increase timeouts in configuration or check network

### Getting Help

- Check the main README in `pkg/hfutil/hub/README.md`
- Review the source code for detailed documentation
- File issues for bugs or feature requests

## Next Steps

After trying these examples:

1. **Integration:** Integrate the hub package into your application
2. **Production:** Use the enterprise features for production deployments
3. **Customization:** Extend the examples for your specific use cases
4. **Monitoring:** Add your logging and monitoring solutions

## Dependencies

The examples use these external dependencies:
- `github.com/schollz/progressbar/v3` - Beautiful progress bars
- Your logging framework (examples use a mock logger)

Install dependencies:
```bash
go mod download
``` 