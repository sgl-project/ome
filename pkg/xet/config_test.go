package xet

import (
	"testing"

	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Apply(t *testing.T) {
	t.Run("apply single option", func(t *testing.T) {
		config := &Config{}
		logger := testingPkg.SetupMockLogger()

		option := WithLogger(logger)
		err := config.Apply(option)

		assert.NoError(t, err)
		assert.Equal(t, logger, config.Logger)
	})

	t.Run("apply multiple options", func(t *testing.T) {
		config := &Config{}
		logger := testingPkg.SetupMockLogger()

		err := config.Apply(
			WithLogger(logger),
			WithDefaults(),
		)

		assert.NoError(t, err)
		assert.Equal(t, logger, config.Logger)
		assert.Equal(t, DefaultHFEndpoint, config.Endpoint)
		assert.Equal(t, DefaultHFCacheDir, config.CacheDir)
		assert.Equal(t, uint32(DefaultMaxConcurrentDownloads), config.MaxConcurrentDownloads)
		assert.Equal(t, DefaultLogLevel, config.LogLevel)
	})

	t.Run("apply nil option", func(t *testing.T) {
		config := &Config{}

		err := config.Apply(nil)

		assert.NoError(t, err)
	})

	t.Run("apply option that returns error", func(t *testing.T) {
		config := &Config{}

		// WithLogger with nil logger should return error
		option := WithLogger(nil)
		err := config.Apply(option)

		assert.Error(t, err)
		assert.Equal(t, "invalid logger nil", err.Error())
	})

	t.Run("apply multiple options with one error", func(t *testing.T) {
		config := &Config{}
		logger := testingPkg.SetupMockLogger()

		err := config.Apply(
			WithLogger(logger),
			WithLogger(nil), // This should cause error
		)

		assert.Error(t, err)
		assert.Equal(t, "invalid logger nil", err.Error())
		// First option should still be applied
		assert.Equal(t, logger, config.Logger)
	})
}

func TestNewConfig(t *testing.T) {
	t.Run("create config with no options", func(t *testing.T) {
		config, err := NewConfig()

		assert.NoError(t, err)
		require.NotNil(t, config)
		assert.Empty(t, config.Endpoint)
		assert.Empty(t, config.CacheDir)
		assert.Equal(t, uint32(0), config.MaxConcurrentDownloads)
		assert.False(t, config.EnableDedup)
		assert.Empty(t, config.LogLevel)
		assert.False(t, config.EnableProgressReporting)
	})

	t.Run("create config with options", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()

		config, err := NewConfig(
			WithLogger(logger),
			WithDefaults(),
		)

		assert.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, logger, config.Logger)
		assert.Equal(t, DefaultHFEndpoint, config.Endpoint)
		assert.Equal(t, DefaultHFCacheDir, config.CacheDir)
		assert.Equal(t, uint32(DefaultMaxConcurrentDownloads), config.MaxConcurrentDownloads)
		assert.Equal(t, DefaultLogLevel, config.LogLevel)
	})

	t.Run("create config with error option", func(t *testing.T) {
		config, err := NewConfig(WithLogger(nil))

		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Equal(t, "invalid logger nil", err.Error())
	})
}

func TestWithViper(t *testing.T) {
	t.Run("with valid viper config", func(t *testing.T) {
		config := &Config{}
		v := viper.New()
		v.Set("endpoint", "https://test.com")
		v.Set("cache_dir", "/test/cache")
		v.Set("max_concurrent_downloads", 2)
		v.Set("enable_dedup", true)
		v.Set("log_level", "debug")

		option := WithViper(v)
		err := option(config)

		assert.NoError(t, err)
		assert.Equal(t, "https://test.com", config.Endpoint)
		assert.Equal(t, "/test/cache", config.CacheDir)
		assert.Equal(t, uint32(2), config.MaxConcurrentDownloads)
		assert.True(t, config.EnableDedup)
		assert.Equal(t, "debug", config.LogLevel)
	})

	t.Run("with empty viper", func(t *testing.T) {
		config := &Config{}
		v := viper.New()

		option := WithViper(v)
		err := option(config)

		assert.NoError(t, err)
		// All fields should remain empty/zero values
		assert.Empty(t, config.Endpoint)
		assert.Empty(t, config.CacheDir)
		assert.Equal(t, uint32(0), config.MaxConcurrentDownloads)
		assert.False(t, config.EnableDedup)
		assert.Empty(t, config.LogLevel)
	})
}

func TestWithLogger(t *testing.T) {
	t.Run("with valid logger", func(t *testing.T) {
		config := &Config{}
		logger := testingPkg.SetupMockLogger()

		option := WithLogger(logger)
		err := option(config)

		assert.NoError(t, err)
		assert.Equal(t, logger, config.Logger)
	})

	t.Run("with nil logger", func(t *testing.T) {
		config := &Config{}

		option := WithLogger(nil)
		err := option(config)

		assert.Error(t, err)
		assert.Equal(t, "invalid logger nil", err.Error())
		assert.Nil(t, config.Logger)
	})
}

func TestWithAppParams(t *testing.T) {
	t.Run("with valid hub params", func(t *testing.T) {
		config := &Config{}
		logger := testingPkg.SetupMockLogger()
		params := HubParams{
			Logger: logger,
		}

		option := WithAppParams(params)
		err := option(config)

		assert.NoError(t, err)
		// WithAppParams currently does nothing, just returns nil
	})
}

func TestWithDefaults(t *testing.T) {
	t.Run("apply defaults to empty config", func(t *testing.T) {
		config := &Config{}

		option := WithDefaults()
		err := option(config)

		assert.NoError(t, err)
		assert.Equal(t, DefaultHFEndpoint, config.Endpoint)
		assert.Equal(t, DefaultHFCacheDir, config.CacheDir)
		assert.Equal(t, uint32(DefaultMaxConcurrentDownloads), config.MaxConcurrentDownloads)
		assert.Equal(t, DefaultLogLevel, config.LogLevel)
		// EnableDedup and EnableProgressReporting are not set by WithDefaults
		assert.False(t, config.EnableDedup)
		assert.False(t, config.EnableProgressReporting)
	})

	t.Run("apply defaults to partially set config", func(t *testing.T) {
		config := &Config{
			Endpoint:                "https://custom.com",
			CacheDir:                "/custom/cache",
			MaxConcurrentDownloads:  8,
			LogLevel:                "debug",
			EnableDedup:             true,
			EnableProgressReporting: true,
		}

		option := WithDefaults()
		err := option(config)

		assert.NoError(t, err)
		// Existing values should not be overridden
		assert.Equal(t, "https://custom.com", config.Endpoint)
		assert.Equal(t, "/custom/cache", config.CacheDir)
		assert.Equal(t, uint32(8), config.MaxConcurrentDownloads)
		assert.Equal(t, "debug", config.LogLevel)
		assert.True(t, config.EnableDedup)
		assert.True(t, config.EnableProgressReporting)
	})

	t.Run("apply defaults with zero values", func(t *testing.T) {
		config := &Config{
			Endpoint:                "https://custom.com",
			CacheDir:                "/custom/cache",
			MaxConcurrentDownloads:  0, // Zero value should be overridden
			LogLevel:                "",
			EnableDedup:             false,
			EnableProgressReporting: false,
		}

		option := WithDefaults()
		err := option(config)

		assert.NoError(t, err)
		assert.Equal(t, "https://custom.com", config.Endpoint)
		assert.Equal(t, "/custom/cache", config.CacheDir)
		assert.Equal(t, uint32(DefaultMaxConcurrentDownloads), config.MaxConcurrentDownloads) // Should be overridden
		assert.Equal(t, DefaultLogLevel, config.LogLevel)                                     // Should be overridden
		assert.False(t, config.EnableDedup)                                                   // Not set by WithDefaults
		assert.False(t, config.EnableProgressReporting)                                       // Not set by WithDefaults
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Endpoint:               "https://huggingface.co",
			MaxConcurrentDownloads: 4,
		}

		err := config.Validate()

		assert.NoError(t, err)
	})

	t.Run("invalid config - missing required endpoint", func(t *testing.T) {
		config := &Config{
			MaxConcurrentDownloads: 4,
		}

		err := config.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
		assert.Contains(t, err.Error(), "Endpoint")
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("invalid config - zero max concurrent downloads", func(t *testing.T) {
		config := &Config{
			Endpoint:               "https://huggingface.co",
			MaxConcurrentDownloads: 0,
		}

		err := config.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
		assert.Contains(t, err.Error(), "MaxConcurrentDownloads")
		assert.Contains(t, err.Error(), "gt")
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("default config values", func(t *testing.T) {
		config := defaultConfig()

		assert.Equal(t, DefaultHFEndpoint, config.Endpoint)
		assert.Equal(t, DefaultHFCacheDir, config.CacheDir)
		assert.Equal(t, uint32(DefaultMaxConcurrentDownloads), config.MaxConcurrentDownloads)
		assert.True(t, config.EnableDedup) // Note: defaultConfig sets this to true
		assert.Equal(t, DefaultLogLevel, config.LogLevel)
		assert.False(t, config.EnableProgressReporting)
	})
}

func TestIntegration(t *testing.T) {
	t.Run("full config creation with all options", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()
		v := viper.New()
		v.Set("endpoint", "https://custom.com")
		v.Set("cache_dir", "/custom/cache")
		v.Set("max_concurrent_downloads", 6)
		v.Set("enable_dedup", true)
		v.Set("log_level", "debug")
		v.Set("enable_progress_reporting", true)

		config, err := NewConfig(
			WithViper(v),
			WithLogger(logger),
			WithAppParams(HubParams{Logger: logger}),
			WithDefaults(), // This should not override existing values
		)

		assert.NoError(t, err)
		require.NotNil(t, config)

		// Verify all values
		assert.Equal(t, logger, config.Logger)
		assert.Equal(t, "https://custom.com", config.Endpoint)
		assert.Equal(t, "/custom/cache", config.CacheDir)
		assert.Equal(t, uint32(6), config.MaxConcurrentDownloads)
		assert.True(t, config.EnableDedup)
		assert.Equal(t, "debug", config.LogLevel)
		assert.True(t, config.EnableProgressReporting)

		// Validate the config
		err = config.Validate()
		assert.NoError(t, err)
	})
}
