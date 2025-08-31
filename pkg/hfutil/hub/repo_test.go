package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRepoFiles(t *testing.T) {
	mockFiles := []RepoFile{
		{Path: "config.json", Size: 1024, Type: "file"},
		{Path: "model.bin", Size: 1024 * 1024, Type: "file"},
		{Path: "tokenizer/", Size: 0, Type: "directory"},
		{Path: "tokenizer/vocab.json", Size: 2048, Type: "file"},
	}

	tests := []struct {
		name       string
		config     *DownloadConfig
		repoType   string
		statusCode int
		wantErr    bool
		errType    interface{}
	}{
		{
			name: "successful model listing",
			config: &DownloadConfig{
				RepoID:   "test/model",
				RepoType: RepoTypeModel,
			},
			statusCode: 200,
			wantErr:    false,
		},
		{
			name: "successful dataset listing",
			config: &DownloadConfig{
				RepoID:   "test/dataset",
				RepoType: RepoTypeDataset,
			},
			statusCode: 200,
			wantErr:    false,
		},
		{
			name: "successful space listing",
			config: &DownloadConfig{
				RepoID:   "test/space",
				RepoType: RepoTypeSpace,
			},
			statusCode: 200,
			wantErr:    false,
		},
		{
			name: "empty repo ID",
			config: &DownloadConfig{
				RepoID: "",
			},
			wantErr: true,
		},
		{
			name: "repository not found",
			config: &DownloadConfig{
				RepoID:   "test/notfound",
				RepoType: RepoTypeModel,
			},
			statusCode: 404,
			wantErr:    true,
			errType:    &EntryNotFoundError{},
		},
		{
			name: "unauthorized access",
			config: &DownloadConfig{
				RepoID:   "test/private",
				RepoType: RepoTypeModel,
			},
			statusCode: 401,
			wantErr:    true,
			errType:    &RepositoryNotFoundError{},
		},
		{
			name: "gated repository",
			config: &DownloadConfig{
				RepoID:   "test/gated",
				RepoType: RepoTypeModel,
			},
			statusCode: 403,
			wantErr:    true,
			errType:    &GatedRepoError{},
		},
		{
			name: "invalid repo type",
			config: &DownloadConfig{
				RepoID:   "test/repo",
				RepoType: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr || tt.statusCode > 0 {
				server := createMockRepoServerForTest(t, mockFiles, tt.statusCode)
				defer server.Close()
				if tt.config.Endpoint == "" {
					tt.config.Endpoint = server.URL
				}
			}

			ctx := context.Background()
			files, err := ListRepoFiles(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.IsType(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, files, len(mockFiles))
				assert.Equal(t, "config.json", files[0].Path)
				assert.Equal(t, int64(1024), files[0].Size)
				assert.Equal(t, "file", files[0].Type)
			}
		})
	}
}

func TestListRepoFilesWithDefaults(t *testing.T) {
	mockFiles := []RepoFile{
		{Path: "README.md", Size: 512, Type: "file"},
	}

	server := createMockRepoServerForTest(t, mockFiles, 200)
	defer server.Close()

	config := &DownloadConfig{
		RepoID:   "test/repo",
		Endpoint: server.URL,
		// Test defaults: no RepoType, Revision, or Endpoint specified initially
	}

	ctx := context.Background()
	files, err := ListRepoFiles(ctx, config)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "README.md", files[0].Path)
}

func TestSnapshotDownloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *DownloadConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing local dir",
			config: &DownloadConfig{
				RepoID: "test/repo",
				// LocalDir not set
			},
			wantErr: true,
			errMsg:  "local_dir must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := SnapshotDownload(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestFilterByPatterns(t *testing.T) {
	files := []RepoFile{
		{Path: "config.json", Type: "file"},
		{Path: "model.bin", Type: "file"},
		{Path: "tokenizer/", Type: "directory"},
		{Path: "vocab.txt", Type: "file"},
		{Path: "README.md", Type: "file"},
	}

	tests := []struct {
		name           string
		allowPatterns  []string
		ignorePatterns []string
		expectedCount  int
		expectedPaths  []string
	}{
		{
			name:          "no patterns - all files",
			expectedCount: 4, // Excludes directory
			expectedPaths: []string{"config.json", "model.bin", "vocab.txt", "README.md"},
		},
		{
			name:          "allow JSON files only",
			allowPatterns: []string{"*.json"},
			expectedCount: 1,
			expectedPaths: []string{"config.json"},
		},
		{
			name:           "ignore binary files",
			ignorePatterns: []string{"*.bin"},
			expectedCount:  3,
			expectedPaths:  []string{"config.json", "vocab.txt", "README.md"},
		},
		{
			name:           "allow all but ignore markdown",
			ignorePatterns: []string{"*.md"},
			expectedCount:  3,
			expectedPaths:  []string{"config.json", "model.bin", "vocab.txt"},
		},
		{
			name:           "allow JSON but ignore config",
			allowPatterns:  []string{"*.json"},
			ignorePatterns: []string{"config*"},
			expectedCount:  0, // config.json is allowed by pattern but ignored by name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterByPatterns(files, tt.allowPatterns, tt.ignorePatterns)

			assert.Len(t, result, tt.expectedCount)

			if tt.expectedPaths != nil {
				var resultPaths []string
				for _, file := range result {
					resultPaths = append(resultPaths, file.Path)
				}
				assert.ElementsMatch(t, tt.expectedPaths, resultPaths)
			}
		})
	}
}

func TestRepoFileStructure(t *testing.T) {
	file := RepoFile{
		Path: "test/file.json",
		Size: 1024,
		Type: "file",
	}

	assert.Equal(t, "test/file.json", file.Path)
	assert.Equal(t, int64(1024), file.Size)
	assert.Equal(t, "file", file.Type)
}

// Helper functions for testing

func createMockRepoServerForTest(t *testing.T, files []RepoFile, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a GET request with the right path format
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/tree/")
		assert.Contains(t, r.URL.RawQuery, "recursive=true")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if statusCode == 200 {
			_ = json.NewEncoder(w).Encode(files)
		} else {
			// Return error response for non-200 status codes
			_, _ = w.Write([]byte(`{"error": "Repository not found"}`))
		}
	}))
}

// Benchmark tests

func BenchmarkListRepoFiles(b *testing.B) {
	mockFiles := make([]RepoFile, 1000)
	for i := 0; i < 1000; i++ {
		mockFiles[i] = RepoFile{
			Path: fmt.Sprintf("file%d.json", i),
			Size: int64(i * 100),
			Type: "file",
		}
	}

	server := createMockRepoServerForBench(b, mockFiles, 200)
	defer server.Close()

	config := &DownloadConfig{
		RepoID:   "test/repo",
		Endpoint: server.URL,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ListRepoFiles(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFilterByPatterns(b *testing.B) {
	files := make([]RepoFile, 1000)
	for i := 0; i < 1000; i++ {
		var ext string
		switch i % 4 {
		case 0:
			ext = ".json"
		case 1:
			ext = ".bin"
		case 2:
			ext = ".txt"
		default:
			ext = ".md"
		}
		files[i] = RepoFile{
			Path: fmt.Sprintf("file%d%s", i, ext),
			Type: "file",
		}
	}

	allowPatterns := []string{"*.json", "*.txt"}
	ignorePatterns := []string{"file5*"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FilterByPatterns(files, allowPatterns, ignorePatterns)
	}
}

// Helper for benchmarks
func createMockRepoServerForBench(b *testing.B, files []RepoFile, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if statusCode == 200 {
			_ = json.NewEncoder(w).Encode(files)
		}
	}))
}

// Test the new concurrent download functionality

func TestDownloadWorker(t *testing.T) {
	t.Run("successful file download", func(t *testing.T) {
		content := "test worker content"
		server := createMockHubServerForWorkerTest(t, content, 200)
		defer server.Close()

		tmpDir := t.TempDir()

		taskChan := make(chan downloadTask, 1)
		resultChan := make(chan downloadResult, 1)

		// Create test task
		task := downloadTask{
			file: RepoFile{
				Path: "test.txt",
				Size: int64(len(content)),
				Type: "file",
			},
			config: &DownloadConfig{
				RepoID:   "test/repo",
				Filename: "test.txt",
				LocalDir: tmpDir,
				Endpoint: server.URL,
			},
			index: 0,
		}

		ctx := context.Background()

		// Start worker
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadWorker(ctx, 1, taskChan, resultChan)
		}()

		// Send task
		taskChan <- task
		close(taskChan)

		// Get result
		result := <-resultChan
		wg.Wait()

		assert.NoError(t, result.err)
		assert.Equal(t, 0, result.index)
		assert.Greater(t, result.duration, time.Duration(0))
		assert.True(t, FileExists(filepath.Join(tmpDir, "test.txt")))
	})

	t.Run("download error handling", func(t *testing.T) {
		server := createMockHubServerForWorkerTest(t, "", 500) // Server error
		defer server.Close()

		tmpDir := t.TempDir()

		taskChan := make(chan downloadTask, 1)
		resultChan := make(chan downloadResult, 1)

		task := downloadTask{
			file: RepoFile{
				Path: "test.txt",
				Size: 1024,
				Type: "file",
			},
			config: &DownloadConfig{
				RepoID:   "test/repo",
				Filename: "test.txt",
				LocalDir: tmpDir,
				Endpoint: server.URL,
			},
			index: 0,
		}

		ctx := context.Background()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadWorker(ctx, 1, taskChan, resultChan)
		}()

		taskChan <- task
		close(taskChan)

		result := <-resultChan
		wg.Wait()

		assert.Error(t, result.err)
		assert.Equal(t, 0, result.index)
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		taskChan := make(chan downloadTask, 1)
		resultChan := make(chan downloadResult, 1)

		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadWorker(ctx, 1, taskChan, resultChan)
		}()

		// Cancel context immediately
		cancel()

		// Worker should exit gracefully
		close(taskChan)
		wg.Wait()

		// Channel should be empty since worker exited due to cancellation
		select {
		case <-resultChan:
			t.Error("Expected no results due to cancellation")
		default:
			// Expected - no results
		}
	})
}

func TestSnapshotDownloadConcurrent(t *testing.T) {
	t.Run("concurrent download with multiple workers", func(t *testing.T) {
		// Create test files
		testFiles := []RepoFile{
			{Path: "file1.txt", Size: 100, Type: "file"},
			{Path: "file2.txt", Size: 200, Type: "file"},
			{Path: "file3.txt", Size: 300, Type: "file"},
			{Path: "subdir/file4.txt", Size: 400, Type: "file"},
		}

		server := createMockRepoAndFileServer(t, testFiles, 200)
		defer server.Close()

		tmpDir := t.TempDir()
		config := &DownloadConfig{
			RepoID:     "test/repo",
			LocalDir:   tmpDir,
			Endpoint:   server.URL,
			MaxWorkers: 2, // Use 2 workers for testing concurrency
		}

		// Create HubConfig for worker configuration
		hubConfig := &HubConfig{
			MaxWorkers:          3,    // This should be overridden by config.MaxWorkers
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		result, err := SnapshotDownload(ctx, config)

		require.NoError(t, err)
		assert.Equal(t, tmpDir, result)

		// Verify all files were downloaded
		for _, file := range testFiles {
			filePath := filepath.Join(tmpDir, file.Path)
			assert.True(t, FileExists(filePath), "File %s should exist", file.Path)
		}
	})

	t.Run("partial failure handling", func(t *testing.T) {
		testFiles := []RepoFile{
			{Path: "file1.txt", Size: 100, Type: "file"},
			{Path: "file2.txt", Size: 200, Type: "file"}, // This will fail
			{Path: "file3.txt", Size: 300, Type: "file"},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle repository listing
			if r.URL.Path == "/api/models/test/repo/tree/main" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(testFiles)
				return
			}

			// Fail file2.txt immediately with non-retryable error
			if strings.Contains(r.URL.Path, "file2.txt") {
				w.WriteHeader(http.StatusNotFound) // Non-retryable error
				return
			}

			// Handle HEAD requests for metadata (successful files)
			if r.Method == "HEAD" {
				w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
				w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
				w.Header().Set(HuggingfaceHeaderXLinkedSize, "100")
				w.WriteHeader(http.StatusOK)
				return
			}

			// Handle GET requests for file downloads (successful files)
			if r.Method == "GET" {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(make([]byte, 100))
				return
			}

			// Default response
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		config := &DownloadConfig{
			RepoID:     "test/repo",
			LocalDir:   tmpDir,
			Endpoint:   server.URL,
			MaxWorkers: 2,
		}

		ctx := context.Background()
		result, err := SnapshotDownload(ctx, config)

		// Should return error but still provide the directory
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download")
		assert.Equal(t, tmpDir, result)

		// Verify successful files were downloaded
		assert.True(t, FileExists(filepath.Join(tmpDir, "file1.txt")))
		assert.True(t, FileExists(filepath.Join(tmpDir, "file3.txt")))

		// Verify failed file was not downloaded
		assert.False(t, FileExists(filepath.Join(tmpDir, "file2.txt")))
	})

	t.Run("context cancellation during concurrent download", func(t *testing.T) {
		testFiles := []RepoFile{
			{Path: "file1.txt", Size: 100, Type: "file"},
			{Path: "file2.txt", Size: 200, Type: "file"},
			{Path: "file3.txt", Size: 300, Type: "file"},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle repository listing quickly
			if r.URL.Path == "/api/models/test/repo/tree/main" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(testFiles)
				return
			}

			// For metadata and downloads, check if context is still valid
			// If not, just return immediately to avoid hanging
			select {
			case <-r.Context().Done():
				return
			default:
			}

			// Handle HEAD requests for metadata
			if r.Method == "HEAD" {
				w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
				w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
				w.Header().Set(HuggingfaceHeaderXLinkedSize, "100")
				w.WriteHeader(http.StatusOK)
				return
			}

			// Handle GET requests - respond immediately
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, 100))
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		config := &DownloadConfig{
			RepoID:     "test/repo",
			LocalDir:   tmpDir,
			Endpoint:   server.URL,
			MaxWorkers: 2,
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context immediately to test cancellation
		cancel()

		result, err := SnapshotDownload(ctx, config)

		assert.Error(t, err)
		// Check if the error is or contains context.Canceled
		assert.True(t, err == context.Canceled || strings.Contains(err.Error(), "context canceled"))
		assert.Empty(t, result)
	})
}

func TestConcurrentDownloadTasks(t *testing.T) {
	t.Run("downloadTask and downloadResult structures", func(t *testing.T) {
		file := RepoFile{
			Path: "test.txt",
			Size: 1024,
			Type: "file",
		}

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "test.txt",
		}

		task := downloadTask{
			file:   file,
			config: config,
			index:  5,
		}

		assert.Equal(t, file, task.file)
		assert.Equal(t, config, task.config)
		assert.Equal(t, 5, task.index)

		result := downloadResult{
			index:    5,
			filePath: "/path/to/file",
			err:      nil,
			duration: time.Second,
			size:     1024,
		}

		assert.Equal(t, 5, result.index)
		assert.Equal(t, "/path/to/file", result.filePath)
		assert.NoError(t, result.err)
		assert.Equal(t, time.Second, result.duration)
		assert.Equal(t, int64(1024), result.size)
	})
}

// Helper functions for concurrent testing

func createMockHubServerForWorkerTest(t *testing.T, content string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			// Mock metadata response
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, fmt.Sprintf("%d", len(content)))
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "GET" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.WriteHeader(statusCode)
			if statusCode == 200 {
				_, _ = w.Write([]byte(content))
			}
		}
	}))
}

func createMockRepoAndFileServer(t *testing.T, files []RepoFile, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/models/test/repo/tree/main" {
			// Repository listing
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			if statusCode == 200 {
				_ = json.NewEncoder(w).Encode(files)
			}
			return
		}

		// Handle HEAD requests for metadata
		if r.Method == "HEAD" {
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, "100")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle file downloads
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 100))
	}))
}
