package hub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDownloadOptions(t *testing.T) {
	opts := DefaultDownloadOptions()

	assert.Equal(t, GetCacheDir(), opts.CacheDir)
	assert.Equal(t, RepoTypeModel, opts.RepoType)
	assert.Equal(t, DefaultRevision, opts.Revision)
	assert.Equal(t, DefaultEndpoint, opts.Endpoint)
	assert.Equal(t, DefaultEtagTimeout, opts.EtagTimeout)
	assert.Equal(t, "auto", opts.LocalDirUseSymlinks)
	assert.True(t, opts.ResumeDownload)
	assert.NotNil(t, opts.Headers)
}

func TestHfHubDownload(t *testing.T) {
	tests := []struct {
		name    string
		config  *DownloadConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &DownloadConfig{
				RepoID:   "test/repo",
				Filename: "config.json",
			},
			wantErr: false,
		},
		{
			name: "empty repo ID",
			config: &DownloadConfig{
				RepoID:   "",
				Filename: "config.json",
			},
			wantErr: true,
			errMsg:  "repo_id cannot be empty",
		},
		{
			name: "empty filename",
			config: &DownloadConfig{
				RepoID:   "test/repo",
				Filename: "",
			},
			wantErr: true,
			errMsg:  "filename cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server for valid configs
			if !tt.wantErr {
				server := createMockHubServer(t)
				defer server.Close()
				tt.config.Endpoint = server.URL
				tt.config.CacheDir = t.TempDir()
			}

			ctx := context.Background()
			_, err := HfHubDownload(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHfHubDownloadToLocalDir(t *testing.T) {
	server := createMockHubServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	config := &DownloadConfig{
		RepoID:   "test/repo",
		Filename: "config.json",
		LocalDir: tmpDir,
		Endpoint: server.URL,
	}

	ctx := context.Background()
	filePath, err := HfHubDownload(ctx, config)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "config.json"), filePath)
	assert.True(t, FileExists(filePath))
}

func TestHfHubDownloadToCacheDir(t *testing.T) {
	server := createMockHubServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	config := &DownloadConfig{
		RepoID:   "test/repo",
		Filename: "config.json",
		CacheDir: tmpDir,
		Endpoint: server.URL,
	}

	ctx := context.Background()
	filePath, err := HfHubDownload(ctx, config)

	require.NoError(t, err)
	assert.Contains(t, filePath, "snapshots")
	assert.True(t, FileExists(filePath))
}

func TestGetMetadataOrCatchError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		headers        map[string]string
		localFilesOnly bool
		wantErr        bool
		errType        interface{}
	}{
		{
			name:       "success with full metadata",
			statusCode: 200,
			headers: map[string]string{
				HuggingfaceHeaderXRepoCommit: "abc123",
				HuggingfaceHeaderXLinkedEtag: "def456",
				HuggingfaceHeaderXLinkedSize: "1024",
				"Location":                   "https://example.com/file",
			},
			wantErr: false,
		},
		{
			name:           "local files only mode",
			localFilesOnly: true,
			wantErr:        true,
			errType:        &OfflineModeIsEnabledError{},
		},
		{
			name:       "not found error",
			statusCode: 404,
			wantErr:    true,
			errType:    &EntryNotFoundError{},
		},
		{
			name:       "unauthorized error",
			statusCode: 401,
			wantErr:    true,
			errType:    &RepositoryNotFoundError{},
		},
		{
			name:       "forbidden error",
			statusCode: 403,
			wantErr:    true,
			errType:    &GatedRepoError{},
		},
		{
			name:       "missing commit hash",
			statusCode: 200,
			headers: map[string]string{
				HuggingfaceHeaderXLinkedEtag: "def456",
				HuggingfaceHeaderXLinkedSize: "1024",
			},
			wantErr: true,
			errType: &FileMetadataError{},
		},
		{
			name:       "missing etag",
			statusCode: 200,
			headers: map[string]string{
				HuggingfaceHeaderXRepoCommit: "abc123",
				HuggingfaceHeaderXLinkedSize: "1024",
			},
			wantErr: true,
			errType: &FileMetadataError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if !tt.localFilesOnly {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Set headers
					for k, v := range tt.headers {
						w.Header().Set(k, v)
					}
					w.WriteHeader(tt.statusCode)
				}))
				defer server.Close()
			}

			config := &DownloadConfig{
				RepoID:         "test/repo",
				Filename:       "config.json",
				LocalFilesOnly: tt.localFilesOnly,
			}

			if server != nil {
				config.Endpoint = server.URL
			}

			ctx := context.Background()
			metadata, err := getMetadataOrCatchError(ctx, config, "", "")

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.IsType(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, metadata)
				assert.Equal(t, "abc123", metadata.CommitHash)
				assert.Equal(t, "def456", metadata.Etag)
				assert.Equal(t, int64(1024), metadata.Size)
			}
		})
	}
}

func TestHandleHTTPError(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectedType interface{}
	}{
		{
			name:         "404 not found",
			statusCode:   404,
			expectedType: &EntryNotFoundError{},
		},
		{
			name:         "401 unauthorized",
			statusCode:   401,
			expectedType: &RepositoryNotFoundError{},
		},
		{
			name:         "403 forbidden",
			statusCode:   403,
			expectedType: &GatedRepoError{},
		},
		{
			name:         "500 server error",
			statusCode:   500,
			expectedType: &HTTPError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			resp, err := http.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			err = handleHTTPError(resp, "test/repo", RepoTypeModel, "main", "config.json")
			assert.Error(t, err)
			assert.IsType(t, tt.expectedType, err)
		})
	}
}

func TestTryToLoadFromCache(t *testing.T) {
	tmpDir := t.TempDir()
	storageFolder := filepath.Join(tmpDir, "models--test--repo")
	relativeFilename := "config.json"

	// Test with commit hash
	commitHash := "abc123def456789012345678901234567890abcd"
	config := &DownloadConfig{
		Revision: commitHash,
	}

	// Create the cache structure
	snapshotPath := filepath.Join(storageFolder, "snapshots", commitHash, relativeFilename)
	require.NoError(t, EnsureDir(filepath.Dir(snapshotPath)))
	require.NoError(t, os.WriteFile(snapshotPath, []byte("test content"), 0644))

	result := tryToLoadFromCache(config, storageFolder, relativeFilename)
	assert.Equal(t, snapshotPath, result)

	// Test with non-existent file
	config.Revision = "nonexistent"
	result = tryToLoadFromCache(config, storageFolder, relativeFilename)
	assert.Empty(t, result)

	// Test with revision that needs resolution
	config.Revision = "main"
	refPath := filepath.Join(storageFolder, "refs", "main")
	require.NoError(t, EnsureDir(filepath.Dir(refPath)))
	require.NoError(t, os.WriteFile(refPath, []byte(commitHash), 0644))

	result = tryToLoadFromCache(config, storageFolder, relativeFilename)
	assert.Equal(t, snapshotPath, result)
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid size",
			input:    "1024",
			expected: 1024,
			wantErr:  false,
		},
		{
			name:     "zero size",
			input:    "0",
			expected: 0,
			wantErr:  false,
		},
		{
			name:    "invalid size",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDownloadToTmpAndMove(t *testing.T) {
	server := createMockFileServer(t, "test content")
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.txt")

	config := &DownloadConfig{
		RepoID:   "test/repo",
		Filename: "test.txt",
	}

	metadata := &FileMetadata{
		Location: server.URL + "/test.txt",
		Size:     12, // "test content" is 12 bytes
		Etag:     "test-etag",
	}

	ctx := context.Background()
	err := downloadToTmpAndMove(ctx, config, metadata, destPath)

	require.NoError(t, err)
	assert.True(t, FileExists(destPath))

	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestHttpDownload(t *testing.T) {
	content := "test file content"
	server := createMockFileServer(t, content)
	defer server.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	config := &DownloadConfig{
		RepoID:   "test/repo",
		Filename: "test.txt",
	}

	metadata := &FileMetadata{
		Location: server.URL + "/test.txt",
		Size:     int64(len(content)),
		Etag:     "test-etag",
	}

	ctx := context.Background()
	err := httpDownload(ctx, config, metadata, filePath)

	require.NoError(t, err)
	assert.True(t, FileExists(filePath))

	downloadedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, content, string(downloadedContent))
}

func TestHttpDownloadWithResume(t *testing.T) {
	content := "test file content for resume"
	server := createMockFileServer(t, content)
	defer server.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Pre-write partial content
	partialContent := content[:10]
	require.NoError(t, os.WriteFile(filePath, []byte(partialContent), 0644))

	config := &DownloadConfig{
		RepoID:         "test/repo",
		Filename:       "test.txt",
		ResumeDownload: true,
	}

	metadata := &FileMetadata{
		Location: server.URL + "/test.txt",
		Size:     int64(len(content)),
		Etag:     "test-etag",
	}

	ctx := context.Background()
	err := httpDownload(ctx, config, metadata, filePath)

	require.NoError(t, err)
	assert.True(t, FileExists(filePath))

	// Note: Our mock server doesn't actually support range requests,
	// so this test just verifies the download completes without error
}

// Test the new retry and context functionality

func TestExponentialBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond

	tests := []struct {
		name        string
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name:        "attempt 0",
			attempt:     0,
			expectedMin: 0,
			expectedMax: 0,
		},
		{
			name:        "attempt 1",
			attempt:     1,
			expectedMin: 100 * time.Millisecond,
			expectedMax: 100 * time.Millisecond,
		},
		{
			name:        "attempt 2",
			attempt:     2,
			expectedMin: 200 * time.Millisecond,
			expectedMax: 200 * time.Millisecond,
		},
		{
			name:        "attempt 3",
			attempt:     3,
			expectedMin: 400 * time.Millisecond,
			expectedMax: 400 * time.Millisecond,
		},
		{
			name:        "attempt 10 (capped at 30s)",
			attempt:     10,
			expectedMin: 30 * time.Second,
			expectedMax: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exponentialBackoff(tt.attempt, baseDelay)
			assert.GreaterOrEqual(t, result, tt.expectedMin)
			assert.LessOrEqual(t, result, tt.expectedMax)
		})
	}
}

func TestRetryableHTTPError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		statusCode  int
		shouldRetry bool
	}{
		{
			name:        "network error",
			err:         fmt.Errorf("network error"),
			statusCode:  0,
			shouldRetry: true,
		},
		{
			name:        "500 server error",
			err:         nil,
			statusCode:  500,
			shouldRetry: true,
		},
		{
			name:        "502 bad gateway",
			err:         nil,
			statusCode:  502,
			shouldRetry: true,
		},
		{
			name:        "429 rate limit",
			err:         nil,
			statusCode:  429,
			shouldRetry: true,
		},
		{
			name:        "408 timeout",
			err:         nil,
			statusCode:  408,
			shouldRetry: true,
		},
		{
			name:        "404 not found",
			err:         nil,
			statusCode:  404,
			shouldRetry: false,
		},
		{
			name:        "401 unauthorized",
			err:         nil,
			statusCode:  401,
			shouldRetry: false,
		},
		{
			name:        "400 bad request",
			err:         nil,
			statusCode:  400,
			shouldRetry: false,
		},
		{
			name:        "200 success",
			err:         nil,
			statusCode:  200,
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retryableHTTPError(tt.err, tt.statusCode)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestContextReader(t *testing.T) {
	t.Run("normal read", func(t *testing.T) {
		ctx := context.Background()
		data := []byte("test data")
		reader := strings.NewReader(string(data))
		contextReader := &contextReader{ctx: ctx, r: reader}

		buf := make([]byte, len(data))
		n, err := contextReader.Read(buf)

		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf)
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		reader := strings.NewReader("test data")
		contextReader := &contextReader{ctx: ctx, r: reader}

		buf := make([]byte, 10)
		_, err := contextReader.Read(buf)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Create a reader that will block, allowing context to timeout
		reader := &blockingReader{}
		contextReader := &contextReader{ctx: ctx, r: reader}

		buf := make([]byte, 10)

		// Wait for context to timeout
		time.Sleep(20 * time.Millisecond)

		_, err := contextReader.Read(buf)

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

func TestCopyWithContext(t *testing.T) {
	t.Run("successful copy", func(t *testing.T) {
		ctx := context.Background()
		src := strings.NewReader("test data for copying")
		dst := &strings.Builder{}

		n, err := copyWithContext(ctx, dst, src)

		assert.NoError(t, err)
		assert.Equal(t, int64(21), n) // Length of "test data for copying"
		assert.Equal(t, "test data for copying", dst.String())
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Create a slow reader that will trigger context check
		slowReader := &slowReader{data: []byte("test data"), delay: 100 * time.Millisecond}
		dst := &strings.Builder{}

		// Cancel context after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		_, err := copyWithContext(ctx, dst, slowReader)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

// Helper types for testing
type slowReader struct {
	data  []byte
	pos   int
	delay time.Duration
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	// Simulate slow reading
	time.Sleep(r.delay)

	n = copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}

// blockingReader blocks indefinitely to test context cancellation
type blockingReader struct{}

func (r *blockingReader) Read(p []byte) (n int, err error) {
	// Block indefinitely by sleeping
	time.Sleep(1 * time.Hour)
	return 0, io.EOF
}

// Helper functions

func createMockHubServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			// Mock metadata response
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123def456789012345678901234567890abcd")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, "1024")
			// Don't set Location header - let it use the current server
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "GET" {
			// Mock file content
			w.Header().Set("Content-Length", "1024")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, 1024)) // Write 1024 zero bytes
		}
	}))
}

func createMockFileServer(t *testing.T, content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
}

// Benchmark tests
func BenchmarkHfHubDownload(b *testing.B) {
	server := createMockHubServerForBench(b)
	defer server.Close()

	tmpDir := b.TempDir()
	config := &DownloadConfig{
		RepoID:   "test/repo",
		Filename: "config.json",
		CacheDir: tmpDir,
		Endpoint: server.URL,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Filename = fmt.Sprintf("config%d.json", i) // Avoid cache hits
		_, err := HfHubDownload(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseSize(b *testing.B) {
	testSizes := []string{"1024", "2048", "4096", "8192"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range testSizes {
			_, _ = parseSize(size) // Ignore return values for benchmark
		}
	}
}

// Helper for benchmarks
func createMockHubServerForBench(b *testing.B) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123def456789012345678901234567890abcd")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, "1024")
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "GET" {
			w.Header().Set("Content-Length", "1024")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, 1024))
		}
	}))
}

func TestHttpDownloadWithRetries(t *testing.T) {
	t.Run("successful download on first attempt", func(t *testing.T) {
		content := "test file content"
		server := createMockFileServer(t, content)
		defer server.Close()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")

		// Create a HubConfig with retry settings
		hubConfig := &HubConfig{
			MaxRetries:          3,
			RetryInterval:       10 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "test.txt",
		}

		metadata := &FileMetadata{
			Location: server.URL + "/test.txt",
			Size:     int64(len(content)),
			Etag:     "test-etag",
		}

		err := httpDownload(ctx, config, metadata, filePath)

		require.NoError(t, err)
		assert.True(t, FileExists(filePath))

		downloadedContent, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, content, string(downloadedContent))
	})

	t.Run("retry on server error", func(t *testing.T) {
		content := "test file content"
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attemptCount, 1)
			if attempt < 3 {
				// Fail first 2 attempts
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Succeed on 3rd attempt
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(content))
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")

		hubConfig := &HubConfig{
			MaxRetries:          2,
			RetryInterval:       1 * time.Millisecond, // Very short for testing
			DisableProgressBars: true,                 // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "test.txt",
		}

		metadata := &FileMetadata{
			Location: server.URL + "/test.txt",
			Size:     int64(len(content)),
			Etag:     "test-etag",
		}

		err := httpDownload(ctx, config, metadata, filePath)

		require.NoError(t, err)
		assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
		assert.True(t, FileExists(filePath))

		downloadedContent, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, content, string(downloadedContent))
	})

	t.Run("fail after max retries", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			w.WriteHeader(http.StatusInternalServerError) // Always fail
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")

		hubConfig := &HubConfig{
			MaxRetries:          2,
			RetryInterval:       1 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "test.txt",
		}

		metadata := &FileMetadata{
			Location: server.URL + "/test.txt",
			Size:     1024,
			Etag:     "test-etag",
		}

		err := httpDownload(ctx, config, metadata, filePath)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP error 500")
		assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount)) // Initial + 2 retries
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")

		hubConfig := &HubConfig{
			MaxRetries:          3,
			RetryInterval:       10 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), HubConfigKey, hubConfig))

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "test.txt",
		}

		metadata := &FileMetadata{
			Location: server.URL + "/test.txt",
			Size:     1024,
			Etag:     "test-etag",
		}

		// Cancel context after short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := httpDownload(ctx, config, metadata, filePath)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestGetMetadataWithRetries(t *testing.T) {
	t.Run("successful metadata fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, "1024")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "config.json",
			Endpoint: server.URL,
		}

		hubConfig := &HubConfig{
			MaxRetries:          3,
			RetryInterval:       10 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		metadata, err := getMetadataOrCatchError(ctx, config, "", "")

		require.NoError(t, err)
		assert.Equal(t, "abc123", metadata.CommitHash)
		assert.Equal(t, "def456", metadata.Etag)
		assert.Equal(t, int64(1024), metadata.Size)
	})

	t.Run("retry on server error", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attemptCount, 1)
			if attempt < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Succeed on 3rd attempt
			w.Header().Set(HuggingfaceHeaderXRepoCommit, "abc123")
			w.Header().Set(HuggingfaceHeaderXLinkedEtag, "def456")
			w.Header().Set(HuggingfaceHeaderXLinkedSize, "1024")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "config.json",
			Endpoint: server.URL,
		}

		hubConfig := &HubConfig{
			MaxRetries:          3,
			RetryInterval:       1 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		metadata, err := getMetadataOrCatchError(ctx, config, "", "")

		require.NoError(t, err)
		assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
		assert.Equal(t, "abc123", metadata.CommitHash)
	})

	t.Run("non-retryable error (404)", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			w.WriteHeader(http.StatusNotFound) // Non-retryable
		}))
		defer server.Close()

		config := &DownloadConfig{
			RepoID:   "test/repo",
			Filename: "config.json",
			Endpoint: server.URL,
		}

		hubConfig := &HubConfig{
			MaxRetries:          3,
			RetryInterval:       1 * time.Millisecond,
			DisableProgressBars: true, // Disable progress bars for testing
		}
		ctx := context.WithValue(context.Background(), HubConfigKey, hubConfig)

		_, err := getMetadataOrCatchError(ctx, config, "", "")

		assert.Error(t, err)
		assert.IsType(t, &EntryNotFoundError{}, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&attemptCount)) // No retries for 404
	})
}
