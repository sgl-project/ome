// Package main demonstrates the enhanced Hugging Face Hub client with enterprise features.
//
// This example shows how to:
// - Use the enhanced HubClient with configuration options
// - Configure logging and timeouts
// - Use functional options pattern
// - Handle different repository types
// - Use download options
//
// Usage:
//
//	go run enhanced_client.go
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
	fmt.Println("🚀 Hugging Face Hub - Enhanced Client Example")
	fmt.Println("==============================================")

	// Create a logger for demonstration (use your production logger)
	logger := logging.Discard() // Replace with your actual logger

	// Example 1: Create enhanced configuration
	fmt.Println("\n⚙️  Example 1: Enhanced Configuration")
	fmt.Println("------------------------------------")

	config, err := hub.NewHubConfig(
		hub.WithToken(os.Getenv("HF_TOKEN")),
		hub.WithEndpoint(hub.DefaultEndpoint),
		hub.WithCacheDir("./cache"),
		hub.WithUserAgent("MyApp/1.0.0"),
		hub.WithTimeouts(30*time.Second, 10*time.Second, 5*time.Minute),
		hub.WithConcurrency(4, 8*1024*1024), // 4 workers, 8MB chunks
		hub.WithRetryConfig(3, 2*time.Second),
		hub.WithSymlinks(true),
		hub.WithProgressBars(false), // Disable for this example
		hub.WithDetailedLogs(true),
		hub.WithLogLevel("info"),
		hub.WithLogger(logger),
	)
	if err != nil {
		log.Fatalf("Failed to create hub config: %v", err)
	}

	fmt.Printf("✅ Configuration created:\n")
	fmt.Printf("   Endpoint: %s\n", config.Endpoint)
	fmt.Printf("   Cache Dir: %s\n", config.CacheDir)
	fmt.Printf("   User Agent: %s\n", config.UserAgent)
	fmt.Printf("   Max Workers: %d\n", config.MaxWorkers)
	fmt.Printf("   Chunk Size: %d MB\n", config.ChunkSize/1024/1024)
	fmt.Printf("   Max Retries: %d\n", config.MaxRetries)

	// Example 2: Create enhanced client
	fmt.Println("\n🔧 Example 2: Enhanced Client Creation")
	fmt.Println("-------------------------------------")

	client, err := hub.NewHubClient(config)
	if err != nil {
		log.Fatalf("Failed to create hub client: %v", err)
	}

	fmt.Println("✅ Enhanced hub client created successfully")

	ctx := context.Background()

	// Example 3: Download single file with options
	fmt.Println("\n📄 Example 3: Single File Download with Options")
	fmt.Println("-----------------------------------------------")

	filePath, err := client.Download(
		ctx,
		"microsoft/DialoGPT-medium",
		"config.json",
		hub.WithRevision("main"),
		hub.WithRepoType(hub.RepoTypeModel),
		hub.WithForceDownload(false),
	)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
	} else {
		fmt.Printf("✅ Downloaded to: %s\n", filePath)
	}

	// Example 4: List repository files
	fmt.Println("\n📂 Example 4: Repository File Listing")
	fmt.Println("-------------------------------------")

	files, err := client.ListFiles(
		ctx,
		"microsoft/DialoGPT-medium",
		hub.WithRepoType(hub.RepoTypeModel),
	)
	if err != nil {
		log.Printf("Failed to list files: %v", err)
	} else {
		fmt.Printf("✅ Found %d files in repository\n", len(files))

		// Show first 5 files
		for i, file := range files {
			if i >= 5 {
				fmt.Printf("   ... and %d more files\n", len(files)-5)
				break
			}
			if file.Type == "file" {
				fmt.Printf("   📄 %s (%s)\n", file.Path, formatSize(file.Size))
			} else {
				fmt.Printf("   📁 %s/\n", file.Path)
			}
		}
	}

	// Example 5: Different repository types
	fmt.Println("\n🗂️  Example 5: Different Repository Types")
	fmt.Println("----------------------------------------")

	// Dataset example
	datasetFiles, err := client.ListFiles(
		ctx,
		"squad",
		hub.WithRepoType(hub.RepoTypeDataset),
	)
	if err != nil {
		log.Printf("Failed to list dataset files: %v", err)
	} else {
		fmt.Printf("✅ Dataset 'squad' has %d files\n", len(datasetFiles))
	}

	// Space example (if accessible)
	spaceFiles, err := client.ListFiles(
		ctx,
		"gradio/hello_world",
		hub.WithRepoType(hub.RepoTypeSpace),
	)
	if err != nil {
		log.Printf("Note: Space listing failed (may require auth): %v\n", err)
	} else {
		fmt.Printf("✅ Space 'gradio/hello_world' has %d files\n", len(spaceFiles))
	}

	// Example 6: Configuration validation
	fmt.Println("\n✅ Example 6: Configuration Validation")
	fmt.Println("-------------------------------------")

	if err := config.ValidateConfig(); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Configuration is valid\n")
	}

	// Example 7: Backward compatibility
	fmt.Println("\n🔄 Example 7: Backward Compatibility")
	fmt.Println("------------------------------------")

	legacyConfig := config.ToDownloadConfig()
	legacyConfig.RepoID = "microsoft/DialoGPT-medium"
	legacyConfig.Filename = "README.md"

	legacyPath, err := hub.HfHubDownload(ctx, legacyConfig)
	if err != nil {
		log.Printf("Legacy download failed: %v", err)
	} else {
		fmt.Printf("✅ Legacy API download to: %s\n", legacyPath)
	}

	// Example 8: Error handling demonstration
	fmt.Println("\n❌ Example 8: Error Handling")
	fmt.Println("----------------------------")

	_, err = client.Download(ctx, "nonexistent/repo", "config.json")
	if err != nil {
		errMsg := err.Error()
		if len(errMsg) > 60 {
			errMsg = errMsg[:60] + "..."
		}
		fmt.Printf("✅ Properly caught error: %s\n", errMsg)
	}

	fmt.Println("\n🎉 Enhanced client examples completed!")
	fmt.Println("Features demonstrated:")
	fmt.Println("✅ Functional options configuration pattern")
	fmt.Println("✅ Enterprise-grade client with logging")
	fmt.Println("✅ Multiple repository types support")
	fmt.Println("✅ Download options and customization")
	fmt.Println("✅ Configuration validation")
	fmt.Println("✅ Backward compatibility")
	fmt.Println("✅ Comprehensive error handling")
	fmt.Println("\nNext: Try the progress_logging.go example for UI features.")
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
