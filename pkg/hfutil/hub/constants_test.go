package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTrue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"1", "1", true},
		{"ON", "ON", true},
		{"YES", "YES", true},
		{"TRUE", "TRUE", true},
		{"lowercase on", "on", true},
		{"lowercase yes", "yes", true},
		{"lowercase true", "true", true},
		{"random string", "random", false},
		{"0", "0", false},
		{"OFF", "OFF", false},
		{"NO", "NO", false},
		{"FALSE", "FALSE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTrue(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestAsInt(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal int
		want       int
	}{
		{"empty string uses default", "", 42, 42},
		{"valid number", "123", 42, 123},
		{"zero", "0", 42, 0},
		{"negative number", "-10", 42, -10},
		{"invalid string uses default", "abc", 42, 42},
		{"mixed string uses default", "123abc", 42, 42},
		{"float string uses default", "123.45", 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := asInt(tt.value, tt.defaultVal)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetCacheDir(t *testing.T) {
	// Save original environment
	originalHfHubCache := os.Getenv(EnvHfHubCache)
	originalHuggingfaceHubCache := os.Getenv(EnvHuggingfaceHubCache)
	originalHfHome := os.Getenv(EnvHfHome)

	// Clean up after test
	defer func() {
		setEnvOrUnset(EnvHfHubCache, originalHfHubCache)
		setEnvOrUnset(EnvHuggingfaceHubCache, originalHuggingfaceHubCache)
		setEnvOrUnset(EnvHfHome, originalHfHome)
	}()

	tests := []struct {
		name                string
		hfHubCache          string
		huggingfaceHubCache string
		hfHome              string
		expectedContains    string
		expectedEqualTo     string
	}{
		{
			name:                "HF_HUB_CACHE has priority",
			hfHubCache:          "/custom/hf/hub/cache",
			huggingfaceHubCache: "/other/cache",
			hfHome:              "/other/home",
			expectedEqualTo:     "/custom/hf/hub/cache",
		},
		{
			name:                "HUGGINGFACE_HUB_CACHE second priority",
			hfHubCache:          "",
			huggingfaceHubCache: "/custom/huggingface/cache",
			hfHome:              "/other/home",
			expectedEqualTo:     "/custom/huggingface/cache",
		},
		{
			name:                "HF_HOME third priority",
			hfHubCache:          "",
			huggingfaceHubCache: "",
			hfHome:              "/custom/hf/home",
			expectedEqualTo:     "/custom/hf/home/hub",
		},
		{
			name:                "default when no env vars",
			hfHubCache:          "",
			huggingfaceHubCache: "",
			hfHome:              "",
			expectedContains:    DefaultCacheDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			setEnvOrUnset(EnvHfHubCache, tt.hfHubCache)
			setEnvOrUnset(EnvHuggingfaceHubCache, tt.huggingfaceHubCache)
			setEnvOrUnset(EnvHfHome, tt.hfHome)

			result := GetCacheDir()

			if tt.expectedEqualTo != "" {
				assert.Equal(t, tt.expectedEqualTo, result)
			} else if tt.expectedContains != "" {
				assert.Contains(t, result, tt.expectedContains)
			}

			// Ensure result is a valid path
			assert.NotEmpty(t, result)
			assert.True(t, filepath.IsAbs(result) || result == DefaultCacheDir)
		})
	}
}

func TestGetHfToken(t *testing.T) {
	// Save original token
	originalToken := os.Getenv(EnvHfToken)
	defer func() {
		setEnvOrUnset(EnvHfToken, originalToken)
	}()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"empty token", "", ""},
		{"valid token", "hf_test_token_123", "hf_test_token_123"},
		{"token with special chars", "hf_token!@#$%^&*()", "hf_token!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvOrUnset(EnvHfToken, tt.token)
			result := GetHfToken()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIsOfflineMode(t *testing.T) {
	// Save original environment
	originalOffline := os.Getenv(EnvHfHubOffline)
	defer func() {
		setEnvOrUnset(EnvHfHubOffline, originalOffline)
	}()

	tests := []struct {
		name        string
		offlineMode string
		want        bool
	}{
		{"not set", "", false},
		{"set to 1", "1", true},
		{"set to true", "true", true},
		{"set to 0", "0", false},
		{"set to false", "false", false},
		{"set to random", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvOrUnset(EnvHfHubOffline, tt.offlineMode)
			result := IsOfflineMode()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that constants have expected values
	assert.Equal(t, "https://huggingface.co", DefaultEndpoint)
	assert.Equal(t, "main", DefaultRevision)
	assert.Equal(t, ".cache/huggingface/hub", DefaultCacheDir)
	assert.Equal(t, 4, DefaultMaxWorkers) // Updated to reduce concurrent API calls
	assert.Equal(t, 10*1024*1024, DefaultChunkSize)
	assert.Equal(t, 10, DefaultMaxRetries) // Increased for better 429 handling

	// Test repository types
	assert.Equal(t, "model", RepoTypeModel)
	assert.Equal(t, "dataset", RepoTypeDataset)
	assert.Equal(t, "space", RepoTypeSpace)

	// Test headers
	assert.Equal(t, "User-Agent", UserAgentHeader)
	assert.Equal(t, "Authorization", AuthorizationHeader)
	assert.Equal(t, "X-Repo-Commit", HuggingfaceHeaderXRepoCommit)
	assert.Equal(t, "X-Linked-Etag", HuggingfaceHeaderXLinkedEtag)
	assert.Equal(t, "X-Linked-Size", HuggingfaceHeaderXLinkedSize)

	// Test environment variable names
	assert.Equal(t, "HF_TOKEN", EnvHfToken)
	assert.Equal(t, "HF_HOME", EnvHfHome)
	assert.Equal(t, "HUGGINGFACE_HUB_CACHE", EnvHuggingfaceHubCache)
	assert.Equal(t, "HF_HUB_CACHE", EnvHfHubCache)
	assert.Equal(t, "HF_HUB_OFFLINE", EnvHfHubOffline)
	assert.Equal(t, "HF_HUB_DISABLE_PROGRESS_BARS", EnvHfHubDisableProgress)
}

func TestRepoTypes(t *testing.T) {
	expectedTypes := []string{"model", "dataset", "space"}
	assert.Equal(t, expectedTypes, RepoTypes)

	// Test that all repo types are in the slice
	assert.Contains(t, RepoTypes, RepoTypeModel)
	assert.Contains(t, RepoTypes, RepoTypeDataset)
	assert.Contains(t, RepoTypes, RepoTypeSpace)
}

func TestRepoTypesURLPrefixes(t *testing.T) {
	expectedPrefixes := map[string]string{
		RepoTypeDataset: "datasets/",
		RepoTypeSpace:   "spaces/",
	}

	assert.Equal(t, expectedPrefixes, RepoTypesURLPrefixes)

	// Model should not have a prefix (empty key)
	_, hasModelPrefix := RepoTypesURLPrefixes[RepoTypeModel]
	assert.False(t, hasModelPrefix)

	// Test that prefixes are correct
	assert.Equal(t, "datasets/", RepoTypesURLPrefixes[RepoTypeDataset])
	assert.Equal(t, "spaces/", RepoTypesURLPrefixes[RepoTypeSpace])
}

func TestTimeoutConstants(t *testing.T) {
	// Test that timeout constants are reasonable
	assert.True(t, DefaultRequestTimeout > 0)
	assert.True(t, DefaultEtagTimeout > 0)
	assert.True(t, DownloadTimeout > DefaultRequestTimeout)
	assert.True(t, DefaultRetryInterval > 0)

	// Test specific values
	assert.Equal(t, "10s", DefaultRequestTimeout.String())
	assert.Equal(t, "10s", DefaultEtagTimeout.String())
	assert.Equal(t, "10m0s", DownloadTimeout.String())
	assert.Equal(t, "15s", DefaultRetryInterval.String())
}

func TestFileSizeThresholds(t *testing.T) {
	// Test LFS threshold
	assert.Equal(t, 10*1024*1024, LfsFileSizeThreshold)
	assert.Equal(t, 10*1024*1024, DefaultChunkSize)

	// Test download limits - MaxHTTPDownloadSize value is large so it might be int64
	assert.Equal(t, 50*1000*1000*1000, MaxHTTPDownloadSize)
	assert.Equal(t, 100, HFTransferConcurrency)
}

func TestSafetensorsConstants(t *testing.T) {
	assert.Equal(t, "model.safetensors", SafetensorsSingleFile)
	assert.Equal(t, "model.safetensors.index.json", SafetensorsIndexFile)
	assert.Equal(t, 25_000_000, SafetensorsMaxHeaderLength)
}

func TestFilePatterns(t *testing.T) {
	assert.Equal(t, "pytorch_model{suffix}.bin", PytorchWeightsFilePattern)
	assert.Equal(t, "model{suffix}.safetensors", SafetensorsWeightsFilePattern)
	assert.Equal(t, "tf_model{suffix}.h5", TF2WeightsFilePattern)

	// Test weight file names
	assert.Equal(t, "pytorch_model.bin", PytorchWeightsName)
	assert.Equal(t, "tf_model.h5", TF2WeightsName)
	assert.Equal(t, "model.ckpt", TFWeightsName)
	assert.Equal(t, "flax_model.msgpack", FlaxWeightsName)
	assert.Equal(t, "config.json", ConfigName)
	assert.Equal(t, "README.md", RepocardName)
}

func TestRepoIdSeparator(t *testing.T) {
	assert.Equal(t, "--", RepoIdSeparator)
}

// Helper function to set environment variable or unset if empty
func setEnvOrUnset(key, value string) {
	if value == "" {
		os.Unsetenv(key)
	} else {
		os.Setenv(key, value)
	}
}

// Test environment variable precedence in a comprehensive way
func TestEnvironmentVariablePrecedence(t *testing.T) {
	// Save all original environment variables
	originalEnvs := map[string]string{
		EnvHfToken:              os.Getenv(EnvHfToken),
		EnvHfHome:               os.Getenv(EnvHfHome),
		EnvHuggingfaceHubCache:  os.Getenv(EnvHuggingfaceHubCache),
		EnvHfHubCache:           os.Getenv(EnvHfHubCache),
		EnvHfHubOffline:         os.Getenv(EnvHfHubOffline),
		EnvHfHubDisableProgress: os.Getenv(EnvHfHubDisableProgress),
	}

	// Clean up after test
	defer func() {
		for key, value := range originalEnvs {
			setEnvOrUnset(key, value)
		}
	}()

	// Clear all environment variables first
	for key := range originalEnvs {
		os.Unsetenv(key)
	}

	// Test 1: HF_HUB_CACHE has highest priority
	os.Setenv(EnvHfHubCache, "/priority1")
	os.Setenv(EnvHuggingfaceHubCache, "/priority2")
	os.Setenv(EnvHfHome, "/priority3")

	cacheDir := GetCacheDir()
	assert.Equal(t, "/priority1", cacheDir)

	// Test 2: HUGGINGFACE_HUB_CACHE when HF_HUB_CACHE not set
	os.Unsetenv(EnvHfHubCache)
	cacheDir = GetCacheDir()
	assert.Equal(t, "/priority2", cacheDir)

	// Test 3: HF_HOME when others not set
	os.Unsetenv(EnvHuggingfaceHubCache)
	cacheDir = GetCacheDir()
	assert.Equal(t, "/priority3/hub", cacheDir)
}

// Benchmark tests
func BenchmarkGetCacheDir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetCacheDir()
	}
}

func BenchmarkGetHfToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetHfToken()
	}
}

func BenchmarkIsOfflineMode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsOfflineMode()
	}
}
