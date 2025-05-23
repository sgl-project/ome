package hub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHfFileMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata *HfFileMetadata
	}{
		{
			name: "complete metadata",
			metadata: &HfFileMetadata{
				CommitHash: stringPtr("abc123"),
				Etag:       stringPtr("def456"),
				Location:   "https://huggingface.co/test/file",
				Size:       int64Ptr(1024),
			},
		},
		{
			name: "minimal metadata",
			metadata: &HfFileMetadata{
				Location: "https://huggingface.co/test/file",
			},
		},
		{
			name: "nil fields",
			metadata: &HfFileMetadata{
				CommitHash: nil,
				Etag:       nil,
				Location:   "https://huggingface.co/test/file",
				Size:       nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.metadata)
			assert.NotEmpty(t, tt.metadata.Location)
		})
	}
}

func TestRepoFileInfo(t *testing.T) {
	lastCommit := &LastCommitInfo{
		OID:   "commit123",
		Title: "Initial commit",
		Date:  time.Now(),
	}

	lfsInfo := &LFSInfo{
		OID:         "sha256:abc123",
		Size:        1024 * 1024,
		PointerSize: 128,
	}

	fileInfo := &RepoFileInfo{
		Path:       "model.bin",
		Size:       1024 * 1024,
		BlobID:     "blob123",
		LFS:        lfsInfo,
		LastCommit: lastCommit,
	}

	assert.Equal(t, "model.bin", fileInfo.Path)
	assert.Equal(t, int64(1024*1024), fileInfo.Size)
	assert.Equal(t, "blob123", fileInfo.BlobID)
	assert.NotNil(t, fileInfo.LFS)
	assert.NotNil(t, fileInfo.LastCommit)
	assert.Equal(t, "sha256:abc123", fileInfo.LFS.OID)
	assert.Equal(t, "commit123", fileInfo.LastCommit.OID)
}

func TestLFSInfo(t *testing.T) {
	lfs := &LFSInfo{
		OID:         "sha256:abc123def456",
		Size:        1024 * 1024 * 10, // 10MB
		PointerSize: 256,
	}

	assert.Equal(t, "sha256:abc123def456", lfs.OID)
	assert.Equal(t, int64(1024*1024*10), lfs.Size)
	assert.Equal(t, 256, lfs.PointerSize)
}

func TestLastCommitInfo(t *testing.T) {
	now := time.Now()
	commit := &LastCommitInfo{
		OID:   "commit123abc",
		Title: "Fix model weights",
		Date:  now,
	}

	assert.Equal(t, "commit123abc", commit.OID)
	assert.Equal(t, "Fix model weights", commit.Title)
	assert.Equal(t, now, commit.Date)
}

func TestRepoInfo(t *testing.T) {
	now := time.Now()
	siblings := []RepoSibling{
		{
			RFilename: "config.json",
			Size:      int64Ptr(1024),
			BlobID:    stringPtr("blob123"),
		},
		{
			RFilename: "model.bin",
			Size:      int64Ptr(1024 * 1024),
			BlobID:    stringPtr("blob456"),
			LFS: &LFSInfo{
				OID:  "sha256:def789",
				Size: 1024 * 1024,
			},
		},
	}

	repo := &RepoInfo{
		ID:           "microsoft/DialoGPT-medium",
		Author:       stringPtr("microsoft"),
		SHA:          stringPtr("abc123"),
		CreatedAt:    &now,
		LastModified: &now,
		Private:      boolPtr(false),
		Disabled:     boolPtr(false),
		Downloads:    intPtr(1000),
		Likes:        intPtr(50),
		Tags:         []string{"text-generation", "pytorch"},
		PipelineTag:  stringPtr("text-generation"),
		LibraryName:  stringPtr("transformers"),
		ModelType:    stringPtr("gpt2"),
		Gated:        stringPtr("false"),
		Siblings:     siblings,
	}

	assert.Equal(t, "microsoft/DialoGPT-medium", repo.ID)
	assert.Equal(t, "microsoft", *repo.Author)
	assert.Equal(t, "abc123", *repo.SHA)
	assert.False(t, *repo.Private)
	assert.False(t, *repo.Disabled)
	assert.Equal(t, 1000, *repo.Downloads)
	assert.Equal(t, 50, *repo.Likes)
	assert.Contains(t, repo.Tags, "text-generation")
	assert.Contains(t, repo.Tags, "pytorch")
	assert.Equal(t, "text-generation", *repo.PipelineTag)
	assert.Equal(t, "transformers", *repo.LibraryName)
	assert.Equal(t, "gpt2", *repo.ModelType)
	assert.Equal(t, "false", *repo.Gated)
	assert.Len(t, repo.Siblings, 2)
	assert.Equal(t, "config.json", repo.Siblings[0].RFilename)
	assert.Equal(t, "model.bin", repo.Siblings[1].RFilename)
	assert.NotNil(t, repo.Siblings[1].LFS)
}

func TestRepoSibling(t *testing.T) {
	tests := []struct {
		name    string
		sibling RepoSibling
	}{
		{
			name: "simple file",
			sibling: RepoSibling{
				RFilename: "config.json",
				Size:      int64Ptr(1024),
				BlobID:    stringPtr("blob123"),
			},
		},
		{
			name: "LFS file",
			sibling: RepoSibling{
				RFilename: "model.bin",
				Size:      int64Ptr(1024 * 1024),
				BlobID:    stringPtr("blob456"),
				LFS: &LFSInfo{
					OID:  "sha256:abc123",
					Size: 1024 * 1024,
				},
			},
		},
		{
			name: "minimal file",
			sibling: RepoSibling{
				RFilename: "README.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.sibling.RFilename)
		})
	}
}

func TestDownloadConfig(t *testing.T) {
	config := &DownloadConfig{
		RepoID:         "microsoft/DialoGPT-medium",
		RepoType:       RepoTypeModel,
		Revision:       "main",
		Filename:       "config.json",
		Subfolder:      "pytorch",
		Token:          "hf_test_token",
		CacheDir:       "/cache",
		LocalDir:       "/local",
		ForceDownload:  true,
		LocalFilesOnly: false,
		ResumeDownload: true,
		Proxies:        map[string]string{"http": "proxy:8080"},
		EtagTimeout:    10 * time.Second,
		Headers:        map[string]string{"Custom": "header"},
		Endpoint:       "https://huggingface.co",
		MaxWorkers:     4,
		AllowPatterns:  []string{"*.json", "*.txt"},
		IgnorePatterns: []string{"*.bin"},
	}

	assert.Equal(t, "microsoft/DialoGPT-medium", config.RepoID)
	assert.Equal(t, RepoTypeModel, config.RepoType)
	assert.Equal(t, "main", config.Revision)
	assert.Equal(t, "config.json", config.Filename)
	assert.Equal(t, "pytorch", config.Subfolder)
	assert.Equal(t, "hf_test_token", config.Token)
	assert.Equal(t, "/cache", config.CacheDir)
	assert.Equal(t, "/local", config.LocalDir)
	assert.True(t, config.ForceDownload)
	assert.False(t, config.LocalFilesOnly)
	assert.True(t, config.ResumeDownload)
	assert.Equal(t, "proxy:8080", config.Proxies["http"])
	assert.Equal(t, 10*time.Second, config.EtagTimeout)
	assert.Equal(t, "header", config.Headers["Custom"])
	assert.Equal(t, "https://huggingface.co", config.Endpoint)
	assert.Equal(t, 4, config.MaxWorkers)
	assert.Contains(t, config.AllowPatterns, "*.json")
	assert.Contains(t, config.IgnorePatterns, "*.bin")
}

func TestSnapshotDownloadResult(t *testing.T) {
	result := &SnapshotDownloadResult{
		SnapshotPath:    "/path/to/snapshot",
		CachedFiles:     []string{"config.json", "tokenizer.json"},
		DownloadedFiles: []string{"model.bin", "vocab.txt"},
		SkippedFiles:    []string{"large_file.bin"},
		Errors: map[string]error{
			"failed_file.txt": assert.AnError,
		},
	}

	assert.Equal(t, "/path/to/snapshot", result.SnapshotPath)
	assert.Len(t, result.CachedFiles, 2)
	assert.Len(t, result.DownloadedFiles, 2)
	assert.Len(t, result.SkippedFiles, 1)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.CachedFiles, "config.json")
	assert.Contains(t, result.DownloadedFiles, "model.bin")
	assert.Contains(t, result.SkippedFiles, "large_file.bin")
	assert.Contains(t, result.Errors, "failed_file.txt")
}

func TestDownloadProgress(t *testing.T) {
	progress := &DownloadProgress{
		Filename:        "model.bin",
		BytesDownloaded: 1024 * 512,  // 512KB
		TotalBytes:      1024 * 1024, // 1MB
		Speed:           1024 * 100,  // 100KB/s
		ETA:             5 * time.Second,
	}

	assert.Equal(t, "model.bin", progress.Filename)
	assert.Equal(t, int64(1024*512), progress.BytesDownloaded)
	assert.Equal(t, int64(1024*1024), progress.TotalBytes)
	assert.Equal(t, float64(1024*100), progress.Speed)
	assert.Equal(t, 5*time.Second, progress.ETA)

	// Test progress percentage calculation
	percentage := float64(progress.BytesDownloaded) / float64(progress.TotalBytes) * 100
	assert.Equal(t, 50.0, percentage)
}

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	assert.NotNil(t, config)
	assert.Equal(t, DefaultEndpoint, config.Endpoint)
	assert.Equal(t, GetHfToken(), config.Token)
	assert.Equal(t, GetCacheDir(), config.CacheDir)
	assert.Equal(t, "huggingface-hub-go/1.0.0", config.UserAgent)
	assert.NotNil(t, config.Headers)
	assert.Equal(t, DefaultRequestTimeout, config.Timeout)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultRetryInterval, config.RetryDelay)
	assert.Equal(t, DefaultMaxWorkers, config.MaxWorkers)
}

func TestClientConfig(t *testing.T) {
	config := &ClientConfig{
		Endpoint:   "https://custom.endpoint",
		Token:      "custom_token",
		CacheDir:   "/custom/cache",
		UserAgent:  "CustomAgent/1.0",
		Headers:    map[string]string{"Custom": "Value"},
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 5 * time.Second,
		MaxWorkers: 6,
	}

	assert.Equal(t, "https://custom.endpoint", config.Endpoint)
	assert.Equal(t, "custom_token", config.Token)
	assert.Equal(t, "/custom/cache", config.CacheDir)
	assert.Equal(t, "CustomAgent/1.0", config.UserAgent)
	assert.Equal(t, "Value", config.Headers["Custom"])
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.Equal(t, 6, config.MaxWorkers)
}

// Test edge cases and validation scenarios
func TestEdgeCases(t *testing.T) {
	t.Run("empty download config", func(t *testing.T) {
		config := &DownloadConfig{}
		assert.Empty(t, config.RepoID)
		assert.Empty(t, config.RepoType)
		assert.Empty(t, config.Filename)
	})

	t.Run("nil LFS info", func(t *testing.T) {
		sibling := RepoSibling{
			RFilename: "file.txt",
			LFS:       nil,
		}
		assert.Nil(t, sibling.LFS)
	})

	t.Run("empty repo info", func(t *testing.T) {
		repo := &RepoInfo{}
		assert.Empty(t, repo.ID)
		assert.Nil(t, repo.Author)
		assert.Nil(t, repo.Private)
	})
}

// Test pointer helper functions
func TestPointerHelpers(t *testing.T) {
	t.Run("string pointer", func(t *testing.T) {
		s := "test"
		ptr := stringPtr(s)
		require.NotNil(t, ptr)
		assert.Equal(t, s, *ptr)
	})

	t.Run("int pointer", func(t *testing.T) {
		i := 42
		ptr := intPtr(i)
		require.NotNil(t, ptr)
		assert.Equal(t, i, *ptr)
	})

	t.Run("int64 pointer", func(t *testing.T) {
		i := int64(1024)
		ptr := int64Ptr(i)
		require.NotNil(t, ptr)
		assert.Equal(t, i, *ptr)
	})

	t.Run("bool pointer", func(t *testing.T) {
		b := true
		ptr := boolPtr(b)
		require.NotNil(t, ptr)
		assert.Equal(t, b, *ptr)
	})
}

// Benchmark tests
func BenchmarkDownloadConfigCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &DownloadConfig{
			RepoID:   "test/repo",
			RepoType: RepoTypeModel,
			Filename: "config.json",
		}
	}
}

func BenchmarkDefaultClientConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultClientConfig()
	}
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
