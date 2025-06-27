package hub

import (
	"errors"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultHubConfig(t *testing.T) {
	config := defaultHubConfig()

	assert.Equal(t, DefaultEndpoint, config.Endpoint)
	assert.Equal(t, GetCacheDir(), config.CacheDir)
	assert.Equal(t, "huggingface-hub-go/1.0.0", config.UserAgent)
	assert.Equal(t, DefaultRequestTimeout, config.RequestTimeout)
	assert.Equal(t, DefaultEtagTimeout, config.EtagTimeout)
	assert.Equal(t, DownloadTimeout, config.DownloadTimeout)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultRetryInterval, config.RetryInterval)
	assert.Equal(t, DefaultMaxWorkers, config.MaxWorkers)
	assert.Equal(t, int64(DefaultChunkSize), config.ChunkSize)
	assert.False(t, config.LocalFilesOnly)
	assert.False(t, config.DisableProgressBars)
	assert.False(t, config.EnableOfflineMode)
	assert.True(t, config.EnableSymlinks)
	assert.True(t, config.VerifySSL)
	assert.False(t, config.EnableDetailedLogs)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, GetHfToken(), config.Token)
}

func TestNewHubConfig(t *testing.T) {
	tests := []struct {
		name    string
		options []HubOption
		want    func(*HubConfig) bool
		wantErr bool
	}{
		{
			name:    "default config",
			options: []HubOption{},
			want: func(c *HubConfig) bool {
				return c.Endpoint == DefaultEndpoint
			},
			wantErr: false,
		},
		{
			name: "with token",
			options: []HubOption{
				WithToken("test_token"),
			},
			want: func(c *HubConfig) bool {
				return c.Token == "test_token"
			},
			wantErr: false,
		},
		{
			name: "with endpoint",
			options: []HubOption{
				WithEndpoint("https://custom.endpoint"),
			},
			want: func(c *HubConfig) bool {
				return c.Endpoint == "https://custom.endpoint"
			},
			wantErr: false,
		},
		{
			name: "with cache dir",
			options: []HubOption{
				WithCacheDir("/custom/cache"),
			},
			want: func(c *HubConfig) bool {
				return c.CacheDir == "/custom/cache"
			},
			wantErr: false,
		},
		{
			name: "with user agent",
			options: []HubOption{
				WithUserAgent("CustomApp/1.0"),
			},
			want: func(c *HubConfig) bool {
				return c.UserAgent == "CustomApp/1.0"
			},
			wantErr: false,
		},
		{
			name: "with timeouts",
			options: []HubOption{
				WithTimeouts(5*time.Second, 3*time.Second, 2*time.Minute),
			},
			want: func(c *HubConfig) bool {
				return c.RequestTimeout == 5*time.Second &&
					c.EtagTimeout == 3*time.Second &&
					c.DownloadTimeout == 2*time.Minute
			},
			wantErr: false,
		},
		{
			name: "with retry config",
			options: []HubOption{
				WithRetryConfig(10, 5*time.Second),
			},
			want: func(c *HubConfig) bool {
				return c.MaxRetries == 10 && c.RetryInterval == 5*time.Second
			},
			wantErr: false,
		},
		{
			name: "with concurrency",
			options: []HubOption{
				WithConcurrency(16, 50*1024*1024),
			},
			want: func(c *HubConfig) bool {
				return c.MaxWorkers == 16 && c.ChunkSize == 50*1024*1024
			},
			wantErr: false,
		},
		{
			name: "with boolean options",
			options: []HubOption{
				WithLocalFilesOnly(true),
				WithOfflineMode(true),
				WithSymlinks(false),
				WithProgressBars(false),
				WithSSLVerification(false),
			},
			want: func(c *HubConfig) bool {
				return c.LocalFilesOnly == true &&
					c.EnableOfflineMode == true &&
					c.EnableSymlinks == false &&
					c.DisableProgressBars == true &&
					c.VerifySSL == false
			},
			wantErr: false,
		},
		{
			name: "with logging options",
			options: []HubOption{
				WithDetailedLogs(true),
				WithLogLevel("debug"),
			},
			want: func(c *HubConfig) bool {
				return c.EnableDetailedLogs == true && c.LogLevel == "debug"
			},
			wantErr: false,
		},
		{
			name: "error on empty endpoint",
			options: []HubOption{
				WithEndpoint(""),
			},
			wantErr: true,
		},
		{
			name: "error on empty cache dir",
			options: []HubOption{
				WithCacheDir(""),
			},
			wantErr: true,
		},
		{
			name: "error on negative max workers",
			options: []HubOption{
				WithConcurrency(-1, 1024),
			},
			wantErr: true,
		},
		{
			name: "error on zero chunk size",
			options: []HubOption{
				WithConcurrency(1, 0),
			},
			wantErr: true,
		},
		{
			name: "error on negative retries",
			options: []HubOption{
				WithRetryConfig(-1, time.Second),
			},
			wantErr: true,
		},
		{
			name: "error on invalid log level",
			options: []HubOption{
				WithLogLevel("invalid"),
			},
			wantErr: true,
		},
		{
			name: "error on nil logger",
			options: []HubOption{
				WithLogger(nil),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewHubConfig(tt.options...)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				require.NoError(t, err)
				require.NotNil(t, config)
				assert.True(t, tt.want(config))
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	mockLogger := logging.Discard()

	config, err := NewHubConfig(WithLogger(mockLogger))
	require.NoError(t, err)
	assert.Equal(t, mockLogger, config.Logger)
}

func TestWithViper(t *testing.T) {
	v := viper.New()
	v.Set("hf_token", "viper_token")
	v.Set("endpoint", "https://viper.endpoint")
	v.Set("cache_dir", "/viper/cache")

	config, err := NewHubConfig(WithViper(v))
	require.NoError(t, err)
	assert.Equal(t, "viper_token", config.Token)
	assert.Equal(t, "https://viper.endpoint", config.Endpoint)
	assert.Equal(t, "/viper/cache", config.CacheDir)
}

func TestHubConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *HubConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  defaultHubConfig(),
			wantErr: false,
		},
		{
			name: "empty endpoint",
			config: &HubConfig{
				Endpoint:   "",
				CacheDir:   "/cache",
				MaxWorkers: 1,
				ChunkSize:  1024,
			},
			wantErr: true,
		},
		{
			name: "empty cache dir",
			config: &HubConfig{
				Endpoint:   "https://example.com",
				CacheDir:   "",
				MaxWorkers: 1,
				ChunkSize:  1024,
			},
			wantErr: true,
		},
		{
			name: "zero max workers",
			config: &HubConfig{
				Endpoint:   "https://example.com",
				CacheDir:   "/cache",
				MaxWorkers: 0,
				ChunkSize:  1024,
			},
			wantErr: true,
		},
		{
			name: "zero chunk size",
			config: &HubConfig{
				Endpoint:   "https://example.com",
				CacheDir:   "/cache",
				MaxWorkers: 1,
				ChunkSize:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateConfig()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateProgressManager(t *testing.T) {
	config := defaultHubConfig()
	config.Logger = logging.Discard()
	config.DisableProgressBars = false
	config.EnableDetailedLogs = true

	pm := config.CreateProgressManager()
	require.NotNil(t, pm)
	assert.Equal(t, config.Logger, pm.logger)
	// enableProgressBars depends on the display mode which is auto-detected
	// In test environment (non-terminal), it will be false even if DisableProgressBars is false
	// So we check the displayMode instead
	assert.Equal(t, ProgressModeAuto, config.ProgressDisplayMode)
	assert.True(t, pm.enableDetailedLogs)
}

func TestToDownloadConfig(t *testing.T) {
	hubConfig := &HubConfig{
		Token:       "test_token",
		CacheDir:    "/test/cache",
		Endpoint:    "https://test.endpoint",
		EtagTimeout: 5 * time.Second,
		UserAgent:   "TestAgent/1.0",
		MaxWorkers:  4,
	}

	downloadConfig := hubConfig.ToDownloadConfig()

	assert.Equal(t, "test_token", downloadConfig.Token)
	assert.Equal(t, "/test/cache", downloadConfig.CacheDir)
	assert.Equal(t, "https://test.endpoint", downloadConfig.Endpoint)
	assert.Equal(t, 5*time.Second, downloadConfig.EtagTimeout)
	assert.Equal(t, 4, downloadConfig.MaxWorkers)
	assert.NotNil(t, downloadConfig.Headers)

	// Test the new default values we added
	assert.Equal(t, "main", downloadConfig.Revision, "Revision should default to 'main'")
	assert.Equal(t, RepoTypeModel, downloadConfig.RepoType, "RepoType should default to 'model'")
}

func TestToDownloadConfigDefaults(t *testing.T) {
	tests := []struct {
		name         string
		hubConfig    *HubConfig
		expectedRev  string
		expectedType string
	}{
		{
			name: "minimal config - should get defaults",
			hubConfig: &HubConfig{
				Token:    "test",
				CacheDir: "/cache",
				Endpoint: "https://test.com",
			},
			expectedRev:  "main",
			expectedType: RepoTypeModel,
		},
		{
			name:         "empty config - should get defaults",
			hubConfig:    &HubConfig{},
			expectedRev:  "main",
			expectedType: RepoTypeModel,
		},
		{
			name: "config with other fields - should still get defaults",
			hubConfig: &HubConfig{
				Token:      "token",
				UserAgent:  "CustomAgent/1.0",
				MaxWorkers: 16,
				ChunkSize:  50 * 1024 * 1024,
			},
			expectedRev:  "main",
			expectedType: RepoTypeModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloadConfig := tt.hubConfig.ToDownloadConfig()

			assert.Equal(t, tt.expectedRev, downloadConfig.Revision,
				"Revision should always default to 'main'")
			assert.Equal(t, tt.expectedType, downloadConfig.RepoType,
				"RepoType should always default to RepoTypeModel")

			// Verify other essential fields are also properly set
			assert.NotNil(t, downloadConfig.Headers, "Headers should be initialized")
		})
	}
}

func TestApplyOptions(t *testing.T) {
	config := defaultHubConfig()

	// Test successful option application
	options := []HubOption{
		WithToken("applied_token"),
		WithEndpoint("https://applied.endpoint"),
	}

	err := config.Apply(options...)
	require.NoError(t, err)
	assert.Equal(t, "applied_token", config.Token)
	assert.Equal(t, "https://applied.endpoint", config.Endpoint)

	// Test option that returns error
	errorOption := func(c *HubConfig) error {
		return errors.New("test error")
	}

	err = config.Apply(errorOption)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")

	// Test nil option (should be skipped)
	err = config.Apply(nil, WithToken("after_nil"))
	require.NoError(t, err)
	assert.Equal(t, "after_nil", config.Token)
}

func TestLogLevelValidation(t *testing.T) {
	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}

	for _, level := range validLevels {
		t.Run("valid_level_"+level, func(t *testing.T) {
			config, err := NewHubConfig(WithLogLevel(level))
			require.NoError(t, err)
			assert.Equal(t, level, config.LogLevel)
		})
	}

	// Test invalid level
	_, err := NewHubConfig(WithLogLevel("invalid"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestTimeoutOptions(t *testing.T) {
	// Test partial timeout setting (others should remain default)
	config, err := NewHubConfig(WithTimeouts(5*time.Second, 0, 0))
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, config.RequestTimeout)
	assert.Equal(t, DefaultEtagTimeout, config.EtagTimeout)
	assert.Equal(t, DownloadTimeout, config.DownloadTimeout)

	// Test all timeouts
	config, err = NewHubConfig(WithTimeouts(1*time.Second, 2*time.Second, 3*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 1*time.Second, config.RequestTimeout)
	assert.Equal(t, 2*time.Second, config.EtagTimeout)
	assert.Equal(t, 3*time.Second, config.DownloadTimeout)
}

// Benchmark tests for configuration creation
func BenchmarkNewHubConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewHubConfig(
			WithToken("benchmark_token"),
			WithEndpoint("https://benchmark.endpoint"),
			WithConcurrency(8, 10*1024*1024),
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := defaultHubConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := config.ValidateConfig()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToDownloadConfig(b *testing.B) {
	config := defaultHubConfig()
	config.Token = "benchmark_token"
	config.UserAgent = "BenchmarkAgent/1.0"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		downloadConfig := config.ToDownloadConfig()
		if downloadConfig == nil {
			b.Fatal("ToDownloadConfig returned nil")
		}
	}
}
