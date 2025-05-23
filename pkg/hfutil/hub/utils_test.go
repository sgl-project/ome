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
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test existing file
	assert.True(t, FileExists(tmpFile.Name()))

	// Test non-existing file
	assert.False(t, FileExists("/non/existent/file"))
}

func TestGetFileSize(t *testing.T) {
	// Create a temporary file with known content
	tmpFile, err := os.CreateTemp("", "test_file")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := "test content"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

	// Test with valid directory
	result := AreSymlinksSupported(tmpDir)
	// Result depends on platform, but function should not panic
	assert.IsType(t, true, result)

	// Test with empty directory
	result = AreSymlinksSupported("")
	assert.False(t, result)
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
