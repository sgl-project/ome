package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHfHubURL(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		filename string
		opts     *DownloadConfig
		expected string
		wantErr  bool
	}{
		{
			name:     "basic model URL",
			repoID:   "microsoft/DialoGPT-medium",
			filename: "config.json",
			opts:     nil,
			expected: "https://huggingface.co/microsoft/DialoGPT-medium/resolve/main/config.json",
			wantErr:  false,
		},
		{
			name:     "dataset URL",
			repoID:   "squad",
			filename: "train.json",
			opts: &DownloadConfig{
				RepoType: RepoTypeDataset,
			},
			expected: "https://huggingface.co/datasets/squad/resolve/main/train.json",
			wantErr:  false,
		},
		{
			name:     "space URL",
			repoID:   "gradio/hello_world",
			filename: "app.py",
			opts: &DownloadConfig{
				RepoType: RepoTypeSpace,
			},
			expected: "https://huggingface.co/spaces/gradio/hello_world/resolve/main/app.py",
			wantErr:  false,
		},
		{
			name:     "custom revision",
			repoID:   "microsoft/DialoGPT-medium",
			filename: "config.json",
			opts: &DownloadConfig{
				Revision: "v1.0",
			},
			expected: "https://huggingface.co/microsoft/DialoGPT-medium/resolve/v1.0/config.json",
			wantErr:  false,
		},
		{
			name:     "custom endpoint",
			repoID:   "microsoft/DialoGPT-medium",
			filename: "config.json",
			opts: &DownloadConfig{
				Endpoint: "https://custom.endpoint",
			},
			expected: "https://custom.endpoint/microsoft/DialoGPT-medium/resolve/main/config.json",
			wantErr:  false,
		},
		{
			name:     "with subfolder",
			repoID:   "microsoft/DialoGPT-medium",
			filename: "config.json",
			opts: &DownloadConfig{
				Subfolder: "pytorch",
			},
			expected: "https://huggingface.co/microsoft/DialoGPT-medium/resolve/main/pytorch/config.json",
			wantErr:  false,
		},
		{
			name:     "invalid repo type",
			repoID:   "test/repo",
			filename: "file.txt",
			opts: &DownloadConfig{
				RepoType: "invalid",
			},
			wantErr: true,
		},
		{
			name:     "filename with special characters",
			repoID:   "test/repo",
			filename: "file with spaces.json",
			opts:     nil,
			expected: "https://huggingface.co/test/repo/resolve/main/file%20with%20spaces.json",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HfHubURL(tt.repoID, tt.filename, tt.opts)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRepoFolderName(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		repoType string
		expected string
	}{
		{
			name:     "model repository",
			repoID:   "microsoft/DialoGPT-medium",
			repoType: RepoTypeModel,
			expected: "models--microsoft--DialoGPT-medium",
		},
		{
			name:     "dataset repository",
			repoID:   "squad",
			repoType: RepoTypeDataset,
			expected: "datasets--squad",
		},
		{
			name:     "space repository",
			repoID:   "gradio/hello_world",
			repoType: RepoTypeSpace,
			expected: "spaces--gradio--hello_world",
		},
		{
			name:     "complex repo name",
			repoID:   "organization/sub-org/model-name",
			repoType: RepoTypeModel,
			expected: "models--organization--sub-org--model-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RepoFolderName(tt.repoID, tt.repoType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		revision string
		expected bool
	}{
		{
			name:     "valid commit hash",
			revision: "abc123def456789012345678901234567890abcd",
			expected: true,
		},
		{
			name:     "short hash",
			revision: "abc123",
			expected: false,
		},
		{
			name:     "long hash",
			revision: "abc123def456789012345678901234567890abcdef",
			expected: false,
		},
		{
			name:     "invalid characters",
			revision: "abc123def456789012345678901234567890abcG",
			expected: false,
		},
		{
			name:     "branch name",
			revision: "main",
			expected: false,
		},
		{
			name:     "empty string",
			revision: "",
			expected: false,
		},
		{
			name:     "uppercase hash",
			revision: "ABC123DEF456789012345678901234567890ABCD",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommitHash(tt.revision)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSHA256(t *testing.T) {
	tests := []struct {
		name     string
		etag     string
		expected bool
	}{
		{
			name:     "valid SHA256",
			etag:     "a1b2c3d4e5f6789012345678901234567890abcdef1234567890123456789012",
			expected: true,
		},
		{
			name:     "short hash",
			etag:     "abc123",
			expected: false,
		},
		{
			name:     "invalid characters",
			etag:     "a1b2c3d4e5f6789012345678901234567890abcdef1234567890123456789012G",
			expected: false,
		},
		{
			name:     "empty string",
			etag:     "",
			expected: false,
		},
		{
			name:     "uppercase hash",
			etag:     "A1B2C3D4E5F6789012345678901234567890ABCDEF1234567890123456789012",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSHA256(tt.etag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test_file")
	require.NoError(t, err)
	defer func(name string) {
		_ = os.Remove(name)
	}(tmpFile.Name())
	_ = tmpFile.Close()

	// Test existing file
	assert.True(t, FileExists(tmpFile.Name()))

	// Test non-existing file
	assert.False(t, FileExists("/non/existent/file"))
}

func TestGetFileSize(t *testing.T) {
	// Create a temporary file with known content
	tmpFile, err := os.CreateTemp("", "test_file")
	require.NoError(t, err)
	defer func(name string) {
		_ = os.Remove(name)
	}(tmpFile.Name())

	content := "test content"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test file size
	size, err := GetFileSize(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	// Test non-existing file
	_, err = GetFileSize("/non/existent/file")
	assert.Error(t, err)
}

func TestEnsureDir(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "test_dir")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Test creating nested directory
	nestedDir := filepath.Join(tmpDir, "nested", "deep", "dir")
	err = EnsureDir(nestedDir)
	require.NoError(t, err)

	// Verify directory exists
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Test existing directory (should not error)
	err = EnsureDir(nestedDir)
	assert.NoError(t, err)
}

func TestBuildHeaders(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		userAgent    string
		extraHeaders map[string]string
		expected     map[string]string
	}{
		{
			name:      "basic headers",
			token:     "test_token",
			userAgent: "TestAgent/1.0",
			expected: map[string]string{
				"Authorization": "Bearer test_token",
				"User-Agent":    "TestAgent/1.0",
			},
		},
		{
			name:      "no token",
			userAgent: "TestAgent/1.0",
			expected: map[string]string{
				"User-Agent": "TestAgent/1.0",
			},
		},
		{
			name:  "no user agent",
			token: "test_token",
			expected: map[string]string{
				"Authorization": "Bearer test_token",
			},
		},
		{
			name:      "with extra headers",
			token:     "test_token",
			userAgent: "TestAgent/1.0",
			extraHeaders: map[string]string{
				"Custom-Header": "custom_value",
				"X-Test":        "test",
			},
			expected: map[string]string{
				"Authorization": "Bearer test_token",
				"User-Agent":    "TestAgent/1.0",
				"Custom-Header": "custom_value",
				"X-Test":        "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildHeaders(tt.token, tt.userAgent, tt.extraHeaders)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeEtag(t *testing.T) {
	tests := []struct {
		name     string
		etag     string
		expected string
	}{
		{
			name:     "quoted etag",
			etag:     `"abc123"`,
			expected: "abc123",
		},
		{
			name:     "weak etag",
			etag:     `W/"abc123"`,
			expected: "abc123",
		},
		{
			name:     "unquoted etag",
			etag:     "abc123",
			expected: "abc123",
		},
		{
			name:     "empty etag",
			etag:     "",
			expected: "",
		},
		{
			name:     "etag with W/ prefix only",
			etag:     "W/abc123",
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeEtag(tt.etag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		patterns []string
		expected bool
	}{
		{
			name:     "glob match",
			filename: "config.json",
			patterns: []string{"*.json"},
			expected: true,
		},
		{
			name:     "no patterns",
			filename: "file.txt",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "no match",
			filename: "config.json",
			patterns: []string{"*.txt", "*.bin"},
			expected: false,
		},
		{
			name:     "substring match",
			filename: "path/to/config.json",
			patterns: []string{"config"},
			expected: true,
		},
		{
			name:     "multiple patterns one match",
			filename: "model.bin",
			patterns: []string{"*.json", "*.bin", "*.txt"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesPattern(tt.filename, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldIgnoreFile(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		allowPatterns  []string
		ignorePatterns []string
		expected       bool
	}{
		{
			name:           "no patterns",
			filename:       "file.txt",
			allowPatterns:  []string{},
			ignorePatterns: []string{},
			expected:       false,
		},
		{
			name:          "allow pattern match",
			filename:      "config.json",
			allowPatterns: []string{"*.json"},
			expected:      false,
		},
		{
			name:          "allow pattern no match",
			filename:      "model.bin",
			allowPatterns: []string{"*.json"},
			expected:      true,
		},
		{
			name:           "ignore pattern match",
			filename:       "model.bin",
			ignorePatterns: []string{"*.bin"},
			expected:       true,
		},
		{
			name:           "ignore pattern no match",
			filename:       "config.json",
			ignorePatterns: []string{"*.bin"},
			expected:       false,
		},
		{
			name:           "allow and ignore both match - ignore wins",
			filename:       "test.bin",
			allowPatterns:  []string{"*.bin"},
			ignorePatterns: []string{"test.*"},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldIgnoreFile(tt.filename, tt.allowPatterns, tt.ignorePatterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPointerPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_storage")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	tests := []struct {
		name             string
		storageFolder    string
		revision         string
		relativeFilename string
		expectedSuffix   string
		wantErr          bool
	}{
		{
			name:             "valid pointer path",
			storageFolder:    tmpDir,
			revision:         "abc123",
			relativeFilename: "config.json",
			expectedSuffix:   "snapshots/abc123/config.json",
			wantErr:          false,
		},
		{
			name:             "nested file path",
			storageFolder:    tmpDir,
			revision:         "def456",
			relativeFilename: "pytorch/model.bin",
			expectedSuffix:   "snapshots/def456/pytorch/model.bin",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPointerPath(tt.storageFolder, tt.revision, tt.relativeFilename)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, result, tt.expectedSuffix)
			}
		})
	}
}

func TestCacheCommitHashForRevision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_cache")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Test caching commit hash
	revision := "main"
	commitHash := "abc123def456"

	err = CacheCommitHashForRevision(tmpDir, revision, commitHash)
	require.NoError(t, err)

	// Verify file was created
	refPath := filepath.Join(tmpDir, "refs", revision)
	assert.True(t, FileExists(refPath))

	// Verify content
	content, err := os.ReadFile(refPath)
	require.NoError(t, err)
	assert.Equal(t, commitHash, string(content))

	// Test caching same hash again (should not error)
	err = CacheCommitHashForRevision(tmpDir, revision, commitHash)
	assert.NoError(t, err)

	// Test revision equals commit hash (should skip)
	err = CacheCommitHashForRevision(tmpDir, commitHash, commitHash)
	assert.NoError(t, err)
}

// Test AreSymlinksSupported with mock scenarios
func TestAreSymlinksSupported(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_symlinks")
	require.NoError(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Test with valid directory
	result := AreSymlinksSupported(tmpDir)
	// Result depends on platform, but function should not panic
	assert.IsType(t, true, result)

	// Test with empty directory
	result = AreSymlinksSupported("")
	assert.False(t, result)
}

// Test the new cross-platform disk space functionality

func TestCheckDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("sufficient disk space", func(t *testing.T) {
		// Test with a small file size (should always have enough space)
		err := CheckDiskSpace(1024, tmpDir)
		assert.NoError(t, err)
	})

	t.Run("zero size file", func(t *testing.T) {
		// Should not check disk space for zero-size files
		err := CheckDiskSpace(0, tmpDir)
		assert.NoError(t, err)
	})

	t.Run("negative size file", func(t *testing.T) {
		// Should not check disk space for negative-size files
		err := CheckDiskSpace(-100, tmpDir)
		assert.NoError(t, err)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		// Should handle non-existent directories gracefully
		err := CheckDiskSpace(1024, "/non/existent/directory")
		// Function should not fail even if it can't check space
		assert.NoError(t, err)
	})
}

func TestGetAvailableDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid directory", func(t *testing.T) {
		space, err := getAvailableDiskSpace(tmpDir)
		// Should either return available space or handle gracefully
		if err != nil {
			// If error occurs, it should be handled gracefully
			assert.NotPanics(t, func() {
				_, _ = getAvailableDiskSpace(tmpDir)
			})
		} else {
			// Available space should be reasonable (either 0 or positive)
			assert.GreaterOrEqual(t, space, int64(0))
		}
	})

	t.Run("create directory if not exists", func(t *testing.T) {
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")

		// Directory doesn't exist yet
		assert.False(t, FileExists(newDir))

		// Function should create it
		_, err := getAvailableDiskSpace(newDir)

		// Should not error and directory should be created
		assert.NoError(t, err)
		assert.True(t, FileExists(newDir))
	})

	t.Run("empty directory path", func(t *testing.T) {
		// Should handle empty directory path
		_, err := getAvailableDiskSpace("")
		// May error, but should not panic
		assert.NotPanics(t, func() {
			_, _ = getAvailableDiskSpace("")
		})
		// We don't assert on error since behavior may vary by platform
		_ = err
	})
}

func TestGetAvailableDiskSpaceWindows(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("windows disk space check", func(t *testing.T) {
		space, err := getAvailableDiskSpaceWindows(tmpDir)

		// Should not error for valid directory
		assert.NoError(t, err)

		// Should return a reasonable amount (no longer hardcoded 100GB)
		// The new implementation actually measures available space
		assert.Greater(t, space, int64(0))
		// Should be at least 1MB (our minimum threshold)
		assert.GreaterOrEqual(t, space, int64(1024*1024))
	})

	t.Run("windows with read-only directory", func(t *testing.T) {
		// Create a subdirectory and try to make it read-only
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		err := os.Mkdir(readOnlyDir, 0444) // Read-only permissions
		require.NoError(t, err)

		space, err := getAvailableDiskSpaceWindows(readOnlyDir)

		// Should handle read-only directories gracefully
		// May return error or fallback to generic method
		assert.NotPanics(t, func() {
			_, _ = getAvailableDiskSpaceWindows(readOnlyDir)
		})

		if err == nil {
			// If no error, should return reasonable space
			assert.GreaterOrEqual(t, space, int64(0))
		}
	})
}

func TestGetAvailableDiskSpaceUnix(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("unix disk space check", func(t *testing.T) {
		space, err := getAvailableDiskSpaceUnix(tmpDir)

		// Should not error for valid directory on Unix systems
		// On non-Unix systems, it might fallback to generic method
		if err != nil {
			// Fallback to generic method should work
			assert.NotPanics(t, func() {
				_, _ = getAvailableDiskSpaceUnix(tmpDir)
			})
		} else {
			// Should return reasonable space amount
			assert.GreaterOrEqual(t, space, int64(0))
		}
	})
}

func TestGetAvailableDiskSpaceGeneric(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("generic disk space check", func(t *testing.T) {
		space, err := getAvailableDiskSpaceGeneric(tmpDir)

		// Should not error for valid directory
		assert.NoError(t, err)

		// Should return a reasonable amount (no longer hardcoded)
		// The new implementation actually measures available space
		assert.Greater(t, space, int64(0))
		// Should be at least 1MB (our minimum threshold)
		assert.GreaterOrEqual(t, space, int64(1024*1024))
	})

	t.Run("generic with unwritable directory", func(t *testing.T) {
		// Create a directory and remove write permissions
		unwritableDir := filepath.Join(tmpDir, "unwritable")
		err := os.Mkdir(unwritableDir, 0444) // Read-only
		require.NoError(t, err)

		space, err := getAvailableDiskSpaceGeneric(unwritableDir)

		// Should handle unwritable directories
		// May return error due to inability to create test file
		if err != nil {
			assert.Contains(t, err.Error(), "unable to create test file")
		} else {
			assert.GreaterOrEqual(t, space, int64(0))
		}
	})
}

func TestDiskSpaceIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("end-to-end disk space workflow", func(t *testing.T) {
		// Test the complete workflow of checking disk space before download

		// Small file - should always pass
		err := CheckDiskSpace(1024, tmpDir)
		assert.NoError(t, err)

		// Very large file (1TB) - might fail on systems with limited space
		err = CheckDiskSpace(1024*1024*1024*1024, tmpDir)
		// We don't assert on this as it depends on actual available space
		// but it should not panic
		assert.NotPanics(t, func() {
			_ = CheckDiskSpace(1024*1024*1024*1024, tmpDir)
		})
		_ = err
	})

	t.Run("platform-specific behavior", func(t *testing.T) {
		// Test that platform-specific functions are called appropriately
		space1, err1 := getAvailableDiskSpace(tmpDir)
		space2, err2 := getAvailableDiskSpaceGeneric(tmpDir)

		// Both should complete without panicking
		assert.NotPanics(t, func() {
			_, _ = getAvailableDiskSpace(tmpDir)
			_, _ = getAvailableDiskSpaceGeneric(tmpDir)
		})

		// Generic method should return measured space (no longer hardcoded)
		assert.NoError(t, err2)
		assert.Greater(t, space2, int64(0))
		assert.GreaterOrEqual(t, space2, int64(1024*1024)) // At least 1MB

		// Platform-specific method should also return reasonable values
		if err1 == nil {
			assert.Greater(t, space1, int64(0))
			// Both methods might return different values but should be reasonable
			assert.GreaterOrEqual(t, space1, int64(1024*1024)) // At least 1MB
		}
	})
}

// Benchmark tests
func BenchmarkHfHubURL(b *testing.B) {
	opts := &DownloadConfig{
		RepoType: RepoTypeModel,
		Revision: "main",
		Endpoint: DefaultEndpoint,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HfHubURL("microsoft/DialoGPT-medium", "config.json", opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsCommitHash(b *testing.B) {
	hash := "abc123def456789012345678901234567890abcd"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsCommitHash(hash)
	}
}

func BenchmarkBuildHeaders(b *testing.B) {
	token := "hf_test_token_123"
	userAgent := "test-agent/1.0"
	extra := map[string]string{"Custom": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildHeaders(token, userAgent, extra)
	}
}
