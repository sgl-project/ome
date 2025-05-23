// Package main demonstrates Hugging Face Hub with progress bars and logging.
//
// This example shows how to:
// - Enable beautiful progress bars with schollz/progressbar
// - Configure comprehensive logging
// - Use the enhanced UI features
// - Download large models with visual feedback
//
// Usage:
//
//	go run progress_logging.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/hfutil/hub"
	"github.com/sgl-project/sgl-ome/pkg/logging"
)

func main() {
	fmt.Println("🎨 Hugging Face Hub - Progress & Logging Example")
	fmt.Println("================================================")

	// Create a logger for demonstration
	logger := logging.Discard() // Replace with your production logger

	// Configuration with progress bars and logging enabled
	config, err := hub.NewHubConfig(
		hub.WithToken(os.Getenv("HF_TOKEN")),
		hub.WithEndpoint(hub.DefaultEndpoint),
		hub.WithCacheDir("./cache"),
		hub.WithTimeouts(30*time.Second, 10*time.Second, 15*time.Minute),
		hub.WithConcurrency(4, 10*1024*1024), // 4 workers, 10MB chunks
		hub.WithSymlinks(true),
		hub.WithProgressBars(true), // 🎨 Enable beautiful progress bars
		hub.WithDetailedLogs(true), // 📝 Enable detailed logging
		hub.WithLogLevel("info"),   // Set appropriate log level
		hub.WithLogger(logger),     // Add structured logging
	)
	if err != nil {
		log.Fatalf("Failed to create hub config: %v", err)
	}

	// Create enhanced client
	client, err := hub.NewHubClient(config)
	if err != nil {
		log.Fatalf("Failed to create hub client: %v", err)
	}

	fmt.Printf("✅ Enhanced Hub Client with Progress & Logging:\n")
	fmt.Printf("   Progress Bars: %t\n", !config.DisableProgressBars)
	fmt.Printf("   Detailed Logs: %t\n", config.EnableDetailedLogs)
	fmt.Printf("   Log Level: %s\n", config.LogLevel)
	fmt.Printf("   Max Workers: %d\n", config.MaxWorkers)

	ctx := context.Background()

	// Example 1: Small file download with progress
	fmt.Println("\n📄 Example 1: Single File with Progress Bar")
	fmt.Println("-------------------------------------------")

	fmt.Println("Downloading a small file to see the progress bar...")

	filePath, err := client.Download(
		ctx,
		"microsoft/DialoGPT-medium",
		"config.json",
		hub.WithRepoType(hub.RepoTypeModel),
	)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
	} else {
		fmt.Printf("✅ Downloaded with progress tracking to: %s\n", filePath)
	}

	// Example 2: Repository listing with spinner
	fmt.Println("\n📂 Example 2: Repository Listing with Spinner")
	fmt.Println("---------------------------------------------")

	// Use a model with more files to see better progress
	repoID := "microsoft/DialoGPT-medium"

	fmt.Printf("Listing files in %s (with spinner)...\n", repoID)
	files, err := client.ListFiles(ctx, repoID)
	if err != nil {
		log.Fatalf("Failed to list files: %v", err)
	}

	// Display files with enhanced formatting
	var totalSize int64
	fileCount := 0

	fmt.Printf("\n📊 Repository Analysis:\n")
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│ File                                │ Size        │ Type  │")
	fmt.Println("├─────────────────────────────────────────────────────────┤")

	for _, file := range files {
		if file.Type == "file" {
			fileCount++
			totalSize += file.Size
			fileName := truncateString(file.Path, 35)
			size := formatSize(file.Size)
			fmt.Printf("│ %-35s │ %-11s │ File  │\n", fileName, size)
		} else {
			dirName := truncateString(file.Path+"/", 35)
			fmt.Printf("│ %-35s │ %-11s │ Dir   │\n", dirName, "-")
		}
	}
	fmt.Println("└─────────────────────────────────────────────────────────┘")

	fmt.Printf("\n📋 Summary:\n")
	fmt.Printf("   Files: %d\n", fileCount)
	fmt.Printf("   Total Size: %s\n", formatSize(totalSize))
	fmt.Printf("   Estimated Download Time: %s\n", estimateDownloadTime(totalSize))

	// Example 3: Snapshot download with comprehensive progress
	fmt.Println("\n📦 Example 3: Snapshot Download with Progress Tracking")
	fmt.Println("------------------------------------------------------")

	fmt.Printf("This will download %d files (%s) with:\n", fileCount, formatSize(totalSize))
	fmt.Printf("  🎨 Individual file progress bars\n")
	fmt.Printf("  📊 Overall download progress\n")
	fmt.Printf("  📝 Detailed logging for each operation\n")
	fmt.Printf("  ⚡ Concurrent downloads (%d workers)\n", config.MaxWorkers)
	fmt.Printf("  🔄 Resume capability if interrupted\n")

	fmt.Print("\nProceed with enhanced download? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors

	if response == "y" || response == "Y" || response == "yes" || response == "Yes" {
		fmt.Printf("\n🚀 Starting Enhanced Download with Progress Tracking...\n")
		fmt.Println("========================================================")

		startTime := time.Now()

		downloadPath, err := client.SnapshotDownload(
			ctx,
			repoID,
			"./downloads/"+repoID,
			hub.WithRepoType(hub.RepoTypeModel),
			hub.WithForceDownload(false),
		)
		if err != nil {
			log.Fatalf("Failed to download repository: %v", err)
		}

		duration := time.Since(startTime)
		avgSpeed := float64(totalSize) / duration.Seconds()

		// Success summary with enhanced visuals
		fmt.Printf("\n🎉 Download Completed with Progress Tracking!\n")
		fmt.Println("============================================")
		fmt.Printf("📁 Location: %s\n", downloadPath)
		fmt.Printf("⏱️  Duration: %v\n", duration.Round(time.Second))
		fmt.Printf("🚀 Avg Speed: %s/s\n", formatSize(int64(avgSpeed)))
		fmt.Printf("📊 Total: %s\n", formatSize(totalSize))

		fmt.Printf("\n✨ Features Demonstrated:\n")
		fmt.Printf("   ✅ Real-time progress bars for each file\n")
		fmt.Printf("   ✅ Overall download progress tracking\n")
		fmt.Printf("   ✅ Structured logging with metrics\n")
		fmt.Printf("   ✅ Beautiful terminal UI with Unicode\n")
		fmt.Printf("   ✅ Concurrent downloads with progress\n")
		fmt.Printf("   ✅ Resume capability\n")
		fmt.Printf("   ✅ Performance monitoring\n")

	} else {
		fmt.Println("Download cancelled - Progress features demonstrated in listing.")
	}

	// Example 4: Error handling with logging
	fmt.Println("\n❌ Example 4: Error Handling with Logging")
	fmt.Println("-----------------------------------------")

	fmt.Println("Attempting to download from non-existent repository...")
	_, err = client.Download(ctx, "nonexistent/repo", "config.json")
	if err != nil {
		fmt.Printf("✅ Error properly logged and handled: %s\n",
			truncateString(err.Error(), 80))
	}

	fmt.Println("\n🎉 Progress & Logging examples completed!")
	fmt.Println("Benefits demonstrated:")
	fmt.Printf("  🎨 Beautiful progress bars using %s\n", "github.com/schollz/progressbar/v3")
	fmt.Printf("  📝 Structured logging integration\n")
	fmt.Printf("  📊 Real-time download statistics\n")
	fmt.Printf("  🎯 Enhanced user experience\n")
	fmt.Printf("  ⚡ Performance monitoring\n")
	fmt.Printf("  🔍 Detailed operation tracking\n")

	fmt.Print("Press Enter to exit...")
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return s[:length]
	}
	return s[:length-3] + "..."
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

// estimateDownloadTime provides a rough estimate of download time
func estimateDownloadTime(bytes int64) string {
	// Assume average speed of 10 MB/s (adjust based on your network)
	avgSpeedMBps := 10.0
	estimatedSeconds := float64(bytes) / (avgSpeedMBps * 1024 * 1024)

	if estimatedSeconds < 60 {
		return fmt.Sprintf("%.0f seconds", estimatedSeconds)
	} else if estimatedSeconds < 3600 {
		return fmt.Sprintf("%.1f minutes", estimatedSeconds/60)
	} else {
		return fmt.Sprintf("%.1f hours", estimatedSeconds/3600)
	}
}
