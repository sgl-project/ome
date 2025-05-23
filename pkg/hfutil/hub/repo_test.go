package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
