package hub

import (
	"context"
	"testing"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHubClient(t *testing.T) {
	logger := logging.Discard()

	tests := []struct {
		name    string
		config  *HubConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &HubConfig{
				Token:               "test-token",
				Endpoint:            DefaultEndpoint,
				CacheDir:            "/tmp/test-cache",
				Logger:              logger,
				UserAgent:           "test-agent",
				RequestTimeout:      DefaultRequestTimeout,
				EtagTimeout:         DefaultEtagTimeout,
				DownloadTimeout:     DownloadTimeout,
				DisableProgressBars: true,
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config != nil {
				// Ensure config validation passes
				tt.config.MaxWorkers = 4
				tt.config.ChunkSize = DefaultChunkSize
				tt.config.MaxRetries = DefaultMaxRetries
				tt.config.RetryInterval = DefaultRetryInterval
			}

			client, err := NewHubClient(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.config, client.config)
				assert.Equal(t, logger, client.logger)
			}
		})
	}
}

func TestHubClientGetConfig(t *testing.T) {
	logger := logging.Discard()
	config := &HubConfig{
		Token:               "test-token",
		Endpoint:            DefaultEndpoint,
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          4,
		ChunkSize:           DefaultChunkSize,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		DisableProgressBars: true,
	}

	client, err := NewHubClient(config)
	require.NoError(t, err)

	retrievedConfig := client.GetConfig()
	assert.Equal(t, config, retrievedConfig)
}

func TestHubClientDownloadValidation(t *testing.T) {
	logger := logging.Discard()
	config := &HubConfig{
		Token:               "test-token",
		Endpoint:            DefaultEndpoint,
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          4,
		ChunkSize:           DefaultChunkSize,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		DisableProgressBars: true,
	}

	client, err := NewHubClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test validation - these will fail because we don't have a real server
	// but we can test that the validation and option handling works
	_, err = client.Download(ctx, "", "config.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repo_id cannot be empty")

	_, err = client.Download(ctx, "test/repo", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "filename cannot be empty")
}

func TestHubClientSnapshotDownloadValidation(t *testing.T) {
	logger := logging.Discard()
	config := &HubConfig{
		Token:               "test-token",
		Endpoint:            DefaultEndpoint,
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          4,
		ChunkSize:           DefaultChunkSize,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		DisableProgressBars: true,
	}

	client, err := NewHubClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test snapshot download validation
	_, err = client.SnapshotDownload(ctx, "test/repo", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local_dir must be specified")
}

func TestHubClientListFilesValidation(t *testing.T) {
	logger := logging.Discard()
	config := &HubConfig{
		Token:               "test-token",
		Endpoint:            DefaultEndpoint,
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          4,
		ChunkSize:           DefaultChunkSize,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		DisableProgressBars: true,
	}

	client, err := NewHubClient(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test list files validation
	_, err = client.ListFiles(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repo_id cannot be empty")
}

func TestDownloadOptions(t *testing.T) {
	config := &DownloadConfig{}

	// Test WithRevision
	err := WithRevision("v1.0.0")(config)
	assert.NoError(t, err)
	assert.Equal(t, "v1.0.0", config.Revision)

	// Test WithSubfolder
	err = WithSubfolder("subfolder")(config)
	assert.NoError(t, err)
	assert.Equal(t, "subfolder", config.Subfolder)

	// Test WithRepoType
	err = WithRepoType(RepoTypeDataset)(config)
	assert.NoError(t, err)
	assert.Equal(t, RepoTypeDataset, config.RepoType)

	// Test WithForceDownload
	err = WithForceDownload(true)(config)
	assert.NoError(t, err)
	assert.True(t, config.ForceDownload)

	// Test WithLocalOnly
	err = WithLocalOnly(true)(config)
	assert.NoError(t, err)
	assert.True(t, config.LocalFilesOnly)

	// Test WithPatterns
	allowPatterns := []string{"*.json"}
	ignorePatterns := []string{"*.bin"}
	err = WithPatterns(allowPatterns, ignorePatterns)(config)
	assert.NoError(t, err)
	assert.Equal(t, allowPatterns, config.AllowPatterns)
	assert.Equal(t, ignorePatterns, config.IgnorePatterns)
}

func TestDownloadOptionsChaining(t *testing.T) {
	config := &DownloadConfig{}

	// Test chaining multiple options
	options := []DownloadOption{
		WithRevision("main"),
		WithSubfolder("models"),
		WithRepoType(RepoTypeModel),
		WithForceDownload(false),
		WithLocalOnly(false),
		WithPatterns([]string{"*.json"}, []string{"*.bin"}),
	}

	for _, opt := range options {
		err := opt(config)
		assert.NoError(t, err)
	}

	assert.Equal(t, "main", config.Revision)
	assert.Equal(t, "models", config.Subfolder)
	assert.Equal(t, RepoTypeModel, config.RepoType)
	assert.False(t, config.ForceDownload)
	assert.False(t, config.LocalFilesOnly)
	assert.Equal(t, []string{"*.json"}, config.AllowPatterns)
	assert.Equal(t, []string{"*.bin"}, config.IgnorePatterns)
}

func TestHubClientParams(t *testing.T) {
	// Test the structure exists and has the expected fields
	params := HubClientParams{}

	// The fields are intended to be nil until dependency injection occurs
	// We just verify the struct can be created and has the correct type
	assert.IsType(t, logging.Interface(nil), params.Logger)
	assert.IsType(t, logging.Interface(nil), params.AnotherLogger)
}

func TestDownloadConfigConversion(t *testing.T) {
	logger := logging.Discard()
	hubConfig := &HubConfig{
		Token:               "test-token",
		Endpoint:            "https://test.example.com",
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent/1.0",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          8,
		ChunkSize:           1024 * 1024,
		MaxRetries:          5,
		RetryInterval:       DefaultRetryInterval,
		EnableSymlinks:      true,
		DisableProgressBars: true,
		EnableDetailedLogs:  false,
		LogLevel:            "info",
	}

	downloadConfig := hubConfig.ToDownloadConfig()

	assert.Equal(t, hubConfig.Token, downloadConfig.Token)
	assert.Equal(t, hubConfig.Endpoint, downloadConfig.Endpoint)
	assert.Equal(t, hubConfig.CacheDir, downloadConfig.CacheDir)
	assert.Equal(t, hubConfig.EtagTimeout, downloadConfig.EtagTimeout)
	assert.NotNil(t, downloadConfig.Headers)
}

// Benchmark tests

func BenchmarkNewHubClient(b *testing.B) {
	logger := logging.Discard()
	config := &HubConfig{
		Token:               "test-token",
		Endpoint:            DefaultEndpoint,
		CacheDir:            "/tmp/test-cache",
		Logger:              logger,
		UserAgent:           "test-agent",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxWorkers:          4,
		ChunkSize:           DefaultChunkSize,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		DisableProgressBars: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := NewHubClient(config)
		if err != nil {
			b.Fatal(err)
		}
		_ = client
	}
}

func BenchmarkDownloadOptions(b *testing.B) {
	options := []DownloadOption{
		WithRevision("main"),
		WithSubfolder("models"),
		WithRepoType(RepoTypeModel),
		WithForceDownload(false),
		WithLocalOnly(false),
		WithPatterns([]string{"*.json"}, []string{"*.bin"}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset config for each iteration
		config := &DownloadConfig{}
		for _, opt := range options {
			if err := opt(config); err != nil {
				b.Fatal(err)
			}
		}
	}
}
