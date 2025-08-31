// Package main demonstrates basic Hugging Face Hub download functionality.
//
// This example shows how to:
// - Download a single file from a repository
// - Download all files from a repository (snapshot download)
// - Handle errors and confirmations
//
// Usage:
//
//	go run basic_download.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
)

func main() {
	fmt.Println("🤗 Hugging Face Hub - Basic Download Example")
	fmt.Println("============================================")

	// Configuration for the download
	config := &hub.DownloadConfig{
		RepoID:        "microsoft/DialoGPT-medium",
		RepoType:      hub.RepoTypeModel,
		Token:         os.Getenv("HF_TOKEN"), // Set your token in environment
		LocalDir:      "./downloads/DialoGPT-medium",
		ForceDownload: false,
	}

	// Ensure we have a token
	if config.Token == "" {
		fmt.Println("⚠️  Warning: No HF_TOKEN environment variable set.")
		fmt.Println("   Some repositories may require authentication.")
		fmt.Println("   Set your token with: export HF_TOKEN=your_token_here")
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(config.LocalDir, 0755); err != nil {
		log.Fatalf("Failed to create target directory: %v", err)
	}

	fmt.Printf("Repository: %s\n", config.RepoID)
	fmt.Printf("Target directory: %s\n", config.LocalDir)

	// Create HubConfig to enable progress bars
	hubConfig := &hub.HubConfig{
		Token:          config.Token,
		Endpoint:       hub.DefaultEndpoint,
		CacheDir:       hub.GetCacheDir(),
		MaxWorkers:     20,
		EnableProgress: false, // Enable progress bars
		MaxRetries:     3,
		RetryInterval:  10 * time.Second,
	}

	// Add HubConfig to context for progress reporting
	ctx := context.WithValue(context.Background(), hub.HubConfigKey, hubConfig)

	// Example 1: Download a single file
	fmt.Println("\n📄 Example 1: Single File Download")
	fmt.Println("----------------------------------")

	singleFileConfig := *config
	singleFileConfig.Filename = "config.json"
	singleFileConfig.LocalDir = "" // Use cache for single files

	filePath, err := hub.HfHubDownload(ctx, &singleFileConfig)
	if err != nil {
		log.Printf("Failed to download single file: %v", err)
	} else {
		fmt.Printf("✅ Downloaded config.json to: %s\n", filePath)
	}

	// Example 2: List repository files
	fmt.Println("\n📂 Example 2: Repository File Listing")
	fmt.Println("-------------------------------------")

	files, err := hub.ListRepoFiles(ctx, config)
	if err != nil {
		log.Fatalf("Failed to list repository files: %v", err)
	}

	fmt.Printf("Found %d items in repository:\n", len(files))
	var totalSize int64
	fileCount := 0

	for _, file := range files {
		if file.Type == "file" {
			fileCount++
			totalSize += file.Size
			fmt.Printf("  📄 %s (%s)\n", file.Path, formatSize(file.Size))
		} else {
			fmt.Printf("  📁 %s/\n", file.Path)
		}
	}

	fmt.Printf("\nSummary: %d files, %s total\n", fileCount, formatSize(totalSize))

	// Example 3: Snapshot download (all files)
	fmt.Println("\n📦 Example 3: Snapshot Download (All Files)")
	fmt.Println("-------------------------------------------")

	fmt.Printf("This will download %d files (%s) to %s\n",
		fileCount, formatSize(totalSize), config.LocalDir)

	fmt.Print("Proceed with snapshot download? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors

	if response == "y" || response == "Y" || response == "yes" || response == "Yes" {
		fmt.Println("Starting snapshot download...")

		downloadPath, err := hub.SnapshotDownload(ctx, config)
		if err != nil {
			log.Fatalf("Failed to download repository: %v", err)
		}

		fmt.Printf("✅ Snapshot download completed!\n")
		fmt.Printf("All files downloaded to: %s\n", downloadPath)

		// Verify downloaded files
		fmt.Println("\nVerifying downloaded files:")
		err = filepath.Walk(downloadPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				relPath, _ := filepath.Rel(downloadPath, path)
				fmt.Printf("  ✅ %s (%s)\n", relPath, formatSize(info.Size()))
			}
			return nil
		})
		if err != nil {
			log.Printf("Error verifying files: %v", err)
		}
	} else {
		fmt.Println("Snapshot download cancelled.")
	}

	fmt.Println("\n🎉 Basic download examples completed!")
	fmt.Println("Next: Try the enhanced_client.go example for more features.")

	fmt.Print("Press Enter to exit...")
	_, _ = fmt.Scanln(&response) // Ignore input parsing errors
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
