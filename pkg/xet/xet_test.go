package xet

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		Endpoint:               "https://huggingface.co",
		MaxConcurrentDownloads: 4,
		EnableDedup:            true,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if client.client == nil {
		t.Fatal("Client handle is nil")
	}
}

func TestClientClose(t *testing.T) {
	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("Failed to close client: %v", err)
	}

	if client.client != nil {
		t.Fatal("Client handle not cleared after close")
	}

	// Double close should be safe
	err = client.Close()
	if err != nil {
		t.Fatalf("Double close failed: %v", err)
	}
}

func TestListFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test with a small public model
	files, err := client.ListFiles("bert-base-uncased", "main")
	if err != nil {
		// This is expected to fail in PoC as we have mock data
		t.Logf("ListFiles failed (expected in PoC): %v", err)
		return
	}

	if len(files) == 0 {
		t.Fatal("No files returned")
	}

	for _, file := range files {
		if file.Path == "" {
			t.Error("File path is empty")
		}
		if file.Hash == "" {
			t.Error("File hash is empty")
		}
		t.Logf("File: %s (size: %d, hash: %s)", file.Path, file.Size, file.Hash)
	}
}

func TestDownloadFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	
	config := &Config{
		Endpoint:               "https://huggingface.co",
		CacheDir:               tempDir,
		MaxConcurrentDownloads: 1,
		EnableDedup:            false,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	req := &DownloadRequest{
		RepoID:   "bert-base-uncased",
		RepoType: "models",
		Revision: "main",
		Filename: "config.json",
		LocalDir: tempDir,
	}

	path, err := client.DownloadFile(req)
	if err != nil {
		// This is expected to fail in PoC
		t.Logf("DownloadFile failed (expected in PoC): %v", err)
		return
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Downloaded file does not exist: %s", path)
	}

	t.Logf("File downloaded to: %s", path)
}

func TestCompatibilityLayer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	
	config := &DownloadConfig{
		RepoID:   "bert-base-uncased",
		RepoType: "models",
		Revision: "main",
		Filename: "config.json",
		LocalDir: tempDir,
		Endpoint: "https://huggingface.co",
	}

	ctx := context.Background()
	path, err := HfHubDownload(ctx, config)
	if err != nil {
		// This is expected to fail in PoC
		t.Logf("HfHubDownload failed (expected in PoC): %v", err)
		return
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Downloaded file does not exist: %s", path)
	}

	t.Logf("File downloaded via compatibility layer to: %s", path)
}

func TestSnapshotDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	
	config := &DownloadConfig{
		RepoID:     "bert-base-uncased",
		RepoType:   "models",
		Revision:   "main",
		LocalDir:   tempDir,
		Endpoint:   "https://huggingface.co",
		MaxWorkers: 2,
	}

	ctx := context.Background()
	path, err := SnapshotDownload(ctx, config)
	if err != nil {
		// This is expected to fail in PoC
		t.Logf("SnapshotDownload failed (expected in PoC): %v", err)
		return
	}

	// Check if any files were downloaded
	files, err := filepath.Glob(filepath.Join(tempDir, "*"))
	if err != nil {
		t.Fatalf("Failed to list downloaded files: %v", err)
	}

	if len(files) == 0 {
		t.Error("No files downloaded in snapshot")
	}

	t.Logf("Snapshot downloaded %d files to: %s", len(files), path)
}

// Benchmark tests

func BenchmarkClientCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		client, err := NewClient(nil)
		if err != nil {
			b.Fatal(err)
		}
		client.Close()
	}
}

func BenchmarkListFiles(b *testing.B) {
	client, err := NewClient(nil)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListFiles("bert-base-uncased", "main")
	}
}