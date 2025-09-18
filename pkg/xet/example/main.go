package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sgl-project/ome/pkg/xet"
)

// Test-time indirection hooks (overridden in unit tests)
var (
	newClient          = xet.NewClient
	enableProgressHook = func(c *xet.Client, label string, d time.Duration) error { return c.EnableConsoleProgress(label, d) }
	downloadFileHook   = func(c *xet.Client, req *xet.DownloadRequest) (string, error) { return c.DownloadFile(req) }
	downloadSnapHook   = func(c *xet.Client, req *xet.SnapshotRequest) (string, error) { return c.DownloadSnapshot(req) }

	hfHubDownloadHook    = xet.HfHubDownload
	snapshotDownloadHook = xet.SnapshotDownload
)

func main() {
	// Command-line flags
	var (
		repoID    = flag.String("repo", "bert-base-uncased", "Hugging Face repository ID")
		filename  = flag.String("file", "config.json", "File to download")
		localDir  = flag.String("dir", "", "Local directory for download (default: temp dir)")
		token     = flag.String("token", "", "Hugging Face API token")
		endpoint  = flag.String("endpoint", "https://huggingface.co", "API endpoint")
		snapshot  = flag.Bool("snapshot", false, "Download entire repository")
		useCompat = flag.Bool("compat", false, "Use HF compatibility layer")
	)

	flag.Parse()

	// Set up local directory
	if *localDir == "" {
		tempDir, err := os.MkdirTemp("", "example-")
		if err != nil {
			log.Fatalf("Failed to create temp directory: %v", err)
		}
		*localDir = tempDir
	}

	// Always print where downloads will be saved
	defer func() {
		fmt.Printf("Downloads saved to: %s\n", *localDir)
	}()

	// Get token from environment if not provided
	if *token == "" {
		*token = os.Getenv("HF_TOKEN")
	}

	if *useCompat {
		// Test compatibility layer
		testCompatibilityLayer(*repoID, *filename, *localDir, *token, *endpoint, *snapshot)
	} else {
		// Test direct xet client
		testDirectClient(*repoID, *filename, *localDir, *token, *endpoint, *snapshot)
	}
}

func testDirectClient(repoID, filename, localDir, token, endpoint string, snapshot bool) {
	// Create xet client
	config := &xet.Config{
		Endpoint:               endpoint,
		Token:                  token,
		CacheDir:               filepath.Join(localDir, ".cache"),
		MaxConcurrentDownloads: 4,
		EnableDedup:            true,
	}

	client, err := newClient(config)
	if err != nil {
		log.Fatalf("Failed to create xet client: %v", err)
	}
	defer func(client *xet.Client) {
		err := client.Close()
		if err != nil {
			log.Printf("warning: error closing xet client: %v", err)
		}
	}(client)

	if err := enableProgressHook(client, "direct", 250*time.Millisecond); err != nil {
		log.Printf("warning: unable to enable progress reporting: %v", err)
	}

	fmt.Printf("Created xet client with endpoint: %s\n", endpoint)

	// Download file or snapshot
	if snapshot {
		fmt.Printf("\nDownloading entire repository to: %s\n", localDir)
		fmt.Println("Using PARALLEL downloads with caching...")

		// Use the new parallel snapshot download
		req := &xet.SnapshotRequest{
			RepoID:   repoID,
			RepoType: "models",
			Revision: "main",
			LocalDir: localDir,
		}

		path, err := downloadSnapHook(client, req)

		if err != nil {
			log.Fatalf("Failed to download snapshot: %v", err)
		}

		fmt.Printf("\nSnapshot download complete! All files saved to: %s\n", path)
	} else {
		// Download single file
		fmt.Printf("\nDownloading file: %s/%s\n", repoID, filename)

		req := &xet.DownloadRequest{
			RepoID:   repoID,
			RepoType: "models",
			Revision: "main",
			Filename: filename,
			LocalDir: localDir,
		}

		path, err := downloadFileHook(client, req)

		if err != nil {
			log.Fatalf("Failed to download file: %v", err)
		}

		fmt.Printf("\n\nFile downloaded successfully to: %s\n", path)

		// Verify file exists
		if stat, err := os.Stat(path); err == nil {
			fmt.Printf("File size: %d bytes\n", stat.Size())
		}
	}
}

func testCompatibilityLayer(repoID, filename, localDir, token, endpoint string, snapshot bool) {
	fmt.Println("Testing HF compatibility layer...")

	config := &xet.DownloadConfig{
		RepoID:   repoID,
		RepoType: "models",
		Revision: "main",
		Filename: filename,
		LocalDir: localDir,
		Endpoint: endpoint,
		Token:    token,
	}

	ctx := context.Background()

	if snapshot {
		fmt.Printf("\nDownloading snapshot of %s to %s\n", repoID, localDir)
		path, err := xet.SnapshotDownload(ctx, config)
		if err != nil {
			log.Fatalf("Snapshot download failed: %v", err)
		}
		fmt.Printf("Snapshot downloaded to: %s\n", path)
	} else {
		fmt.Printf("\nDownloading %s/%s using compatibility layer\n", repoID, filename)
		path, err := xet.HfHubDownload(ctx, config)
		if err != nil {
			log.Fatalf("Download failed: %v", err)
		}
		fmt.Printf("File downloaded to: %s\n", path)
	}

	// List downloaded files
	fmt.Printf("\nFiles in %s:\n", localDir)
	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(localDir, path)
			fmt.Printf("  - %s (%d bytes)\n", relPath, info.Size())
		}
		return nil
	})
	if err != nil {
		log.Printf("Failed to list files: %v", err)
	}
}
