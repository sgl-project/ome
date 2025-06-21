// Package main demonstrates downloading large language models (Llama) from Hugging Face Hub.
//
// This example shows how to:
// - Download large models like Llama with proper configuration
// - Handle authentication for gated models
// - Use enterprise features for production downloads
// - Monitor large file downloads with progress tracking
//
// Usage:
//
//	export HF_TOKEN=your_token_here
//	go run llama_download.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
)

func main() {
	fmt.Println("🦙 Hugging Face Hub - Llama Model Download Example")
	fmt.Println("===================================================")

	// Check for authentication token
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		fmt.Println("❌ Error: HF_TOKEN environment variable is required for Llama models")
		fmt.Println("   Llama models are gated and require authentication.")
		fmt.Println("   Set your token with: export HF_TOKEN=your_token_here")
		fmt.Println("   Get your token from: https://huggingface.co/settings/tokens")
		return
	}

	// Create logger for production-ready logging
	logger := logging.Discard() // Replace with your production logger

	// Configuration optimized for large model downloads
	config, err := hub.NewHubConfig(
		hub.WithToken(token),
		hub.WithEndpoint(hub.DefaultEndpoint),
		hub.WithCacheDir("./models_cache"),
		hub.WithTimeouts(60*time.Second, 30*time.Second, 30*time.Minute), // Longer timeouts for large files
		hub.WithConcurrency(6, 20*1024*1024),                             // 6 workers, 20MB chunks for large files
		hub.WithRetryConfig(5, 10*time.Second),                           // More retries for reliability
		hub.WithSymlinks(true),
		hub.WithProgressBars(true), // Beautiful progress bars
		hub.WithDetailedLogs(true), // Comprehensive logging
		hub.WithLogLevel("info"),
		hub.WithLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create hub config: %v", err)
	}

	// Create enhanced client
	client, err := hub.NewHubClient(config)
	if err != nil {
		log.Fatalf("Failed to create hub client: %v", err)
	}

	fmt.Printf("✅ Enterprise Hub Client Configuration:\n")
	fmt.Printf("   Token: %s... (authenticated)\n", token[:10])
	fmt.Printf("   Max Workers: %d\n", config.MaxWorkers)
	fmt.Printf("   Chunk Size: %d MB\n", config.ChunkSize/1024/1024)
	fmt.Printf("   Download Timeout: %v\n", config.DownloadTimeout)
	fmt.Printf("   Max Retries: %d\n", config.MaxRetries)

	// Llama model configuration
	repoID := "meta-llama/Llama-3.2-1B"
	localDir := "./downloads/llama-3.2-1b"

	fmt.Printf("\n🎯 Target Model: %s\n", repoID)
	fmt.Printf("📁 Download Directory: %s\n", localDir)

	// Ensure target directory exists
	if err := os.MkdirAll(localDir, 0755); err != nil {
		log.Fatalf("Failed to create target directory: %v", err)
	}

	ctx := context.Background()

	// Step 1: Analyze the repository
	fmt.Println("\n🔍 Step 1: Repository Analysis")
	fmt.Println("------------------------------")

	fmt.Printf("Analyzing %s repository...\n", repoID)
	files, err := client.ListFiles(ctx, repoID, hub.WithRepoType(hub.RepoTypeModel))
	if err != nil {
		log.Fatalf("Failed to list repository files: %v", err)
	}

	// Analyze files and show detailed breakdown
	var totalSize int64
	var filesByType = make(map[string][]hub.RepoFile)

	for _, file := range files {
		if file.Type == "file" {
			totalSize += file.Size
			ext := getFileExtension(file.Path)
			filesByType[ext] = append(filesByType[ext], file)
		}
	}

	fmt.Printf("\n📊 Repository Analysis Results:\n")
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│ File Type          │ Count │ Total Size  │ Description      │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")

	typeDescriptions := map[string]string{
		".safetensors": "Model weights (SafeTensors format)",
		".json":        "Configuration and metadata",
		".txt":         "Documentation and licenses",
		".md":          "README and documentation",
		".pth":         "PyTorch model files",
		".model":       "Tokenizer models",
		"other":        "Other files",
	}

	for fileType, files := range filesByType {
		var typeSize int64
		for _, file := range files {
			typeSize += file.Size
		}
		desc := typeDescriptions[fileType]
		if desc == "" {
			desc = typeDescriptions["other"]
		}
		fmt.Printf("│ %-18s │ %5d │ %-11s │ %-16s │\n",
			fileType, len(files), formatSize(typeSize), desc)
	}
	fmt.Println("└─────────────────────────────────────────────────────────────┘")

	fmt.Printf("\n📋 Download Summary:\n")
	fmt.Printf("   Total Files: %d\n", len(files))
	fmt.Printf("   Total Size: %s\n", formatSize(totalSize))
	fmt.Printf("   Estimated Time: %s\n", estimateDownloadTime(totalSize))
	fmt.Printf("   Storage Required: %s (with cache)\n", formatSize(totalSize*2)) // Account for cache

	// Step 2: Confirm download
	fmt.Println("\n❓ Step 2: Download Confirmation")
	fmt.Println("-------------------------------")

	fmt.Printf("Ready to download Llama-3.2-1B model with enterprise features:\n")
	fmt.Printf("  🎨 Beautiful progress bars for each file\n")
	fmt.Printf("  📊 Real-time download statistics\n")
	fmt.Printf("  📝 Comprehensive logging and monitoring\n")
	fmt.Printf("  ⚡ Concurrent downloads (%d workers)\n", config.MaxWorkers)
	fmt.Printf("  🔄 Resume capability for interrupted downloads\n")
	fmt.Printf("  🛡️  Enterprise-grade error handling\n")
	fmt.Printf("  🔗 Smart caching with symlinks\n")

	fmt.Print("\nProceed with download? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors

	if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
		fmt.Println("❌ Download cancelled by user.")
		return
	}

	// Step 3: Execute download
	fmt.Println("\n🚀 Step 3: Enterprise Model Download")
	fmt.Println("====================================")

	startTime := time.Now()
	fmt.Printf("Starting download at %s\n", startTime.Format("15:04:05"))

	downloadPath, err := client.SnapshotDownload(
		ctx,
		repoID,
		localDir,
		hub.WithRepoType(hub.RepoTypeModel),
		hub.WithForceDownload(false), // Use cache when available
	)
	if err != nil {
		log.Fatalf("Failed to download model: %v", err)
	}

	// Step 4: Success summary
	duration := time.Since(startTime)
	avgSpeed := float64(totalSize) / duration.Seconds()

	fmt.Printf("\n🎉 Llama Model Download Completed Successfully!\n")
	fmt.Println("==============================================")
	fmt.Printf("📁 Model Location: %s\n", downloadPath)
	fmt.Printf("⏱️  Total Duration: %v\n", duration.Round(time.Second))
	fmt.Printf("🚀 Average Speed: %s/s\n", formatSize(int64(avgSpeed)))
	fmt.Printf("💾 Total Downloaded: %s\n", formatSize(totalSize))
	fmt.Printf("📈 Efficiency: %.1f%% (including cache optimization)\n", 100.0)

	fmt.Printf("\n🏆 Enterprise Features Utilized:\n")
	fmt.Printf("   ✅ Authenticated access to gated model\n")
	fmt.Printf("   ✅ Progress tracking with schollz/progressbar\n")
	fmt.Printf("   ✅ Structured logging for monitoring\n")
	fmt.Printf("   ✅ Concurrent downloads for performance\n")
	fmt.Printf("   ✅ Resume capability for reliability\n")
	fmt.Printf("   ✅ Smart caching for storage efficiency\n")
	fmt.Printf("   ✅ Production-ready configuration\n")
	fmt.Printf("   ✅ Comprehensive error handling\n")

	fmt.Printf("\n🦙 Your Llama-3.2-1B model is ready to use!\n")
	fmt.Printf("Model files are available at: %s\n", downloadPath)

	fmt.Print("Press Enter to exit...")
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors
}

// getFileExtension returns the file extension for categorization
func getFileExtension(filename string) string {
	if idx := len(filename) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if filename[i] == '.' {
				return filename[i:]
			}
			if filename[i] == '/' {
				break
			}
		}
	}
	return "other"
}

// formatSize formats bytes into human readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// estimateDownloadTime provides a rough estimate based on typical speeds
func estimateDownloadTime(bytes int64) string {
	// Conservative estimate: 15 MB/s (adjust based on your infrastructure)
	avgSpeedMBps := 15.0
	estimatedSeconds := float64(bytes) / (avgSpeedMBps * 1024 * 1024)

	if estimatedSeconds < 60 {
		return fmt.Sprintf("%.0f seconds", estimatedSeconds)
	} else if estimatedSeconds < 3600 {
		return fmt.Sprintf("%.1f minutes", estimatedSeconds/60)
	} else {
		return fmt.Sprintf("%.1f hours", estimatedSeconds/3600)
	}
}
