package xet

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/logging"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestHubParams(t *testing.T) {
	t.Run("HubParams struct", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()
		params := HubParams{
			Logger: logger,
		}

		assert.NotNil(t, params.Logger)
		assert.Equal(t, logger, params.Logger)
	})
}

func TestModule(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func(*viper.Viper)
		expectError bool
		// Expected config values after defaults are applied
		expectedConfig struct {
			Endpoint                string
			CacheDir                string
			MaxConcurrentDownloads  uint32
			EnableDedup             bool
			LogLevel                string
			EnableProgressReporting bool
		}
	}{
		{
			name: "successful module creation with defaults",
			setupViper: func(v *viper.Viper) {
				v.Set("endpoint", "https://huggingface.co")
				v.Set("cache_dir", "/tmp/test-cache")
				v.Set("max_concurrent_downloads", 2)
			},
			expectError: false,
			expectedConfig: struct {
				Endpoint                string
				CacheDir                string
				MaxConcurrentDownloads  uint32
				EnableDedup             bool
				LogLevel                string
				EnableProgressReporting bool
			}{
				Endpoint:                "https://huggingface.co",
				CacheDir:                "/tmp/test-cache",
				MaxConcurrentDownloads:  2,
				EnableDedup:             false,  // Zero value (WithDefaults doesn't set this)
				LogLevel:                "info", // Default value
				EnableProgressReporting: false,  // Default value
			},
		},
		{
			name: "successful module creation with progress reporting",
			setupViper: func(v *viper.Viper) {
				v.Set("endpoint", "https://huggingface.co")
				v.Set("cache_dir", "/tmp/test-cache")
				v.Set("max_concurrent_downloads", 2)
				v.Set("enable_progress_reporting", true)
			},
			expectError: false,
			expectedConfig: struct {
				Endpoint                string
				CacheDir                string
				MaxConcurrentDownloads  uint32
				EnableDedup             bool
				LogLevel                string
				EnableProgressReporting bool
			}{
				Endpoint:                "https://huggingface.co",
				CacheDir:                "/tmp/test-cache",
				MaxConcurrentDownloads:  2,
				EnableDedup:             false,  // Zero value (WithDefaults doesn't set this)
				LogLevel:                "info", // Default value
				EnableProgressReporting: true,
			},
		},
		{
			name: "module creation with minimal config - defaults applied",
			setupViper: func(v *viper.Viper) {
				// Only set minimal values, let defaults fill the rest
				v.Set("endpoint", "https://custom-endpoint.com")
			},
			expectError: false,
			expectedConfig: struct {
				Endpoint                string
				CacheDir                string
				MaxConcurrentDownloads  uint32
				EnableDedup             bool
				LogLevel                string
				EnableProgressReporting bool
			}{
				Endpoint:                "https://custom-endpoint.com",
				CacheDir:                "/tmp/.cache/huggingface", // Default value
				MaxConcurrentDownloads:  4,                         // Default value
				EnableDedup:             false,                     // Zero value (WithDefaults doesn't set this)
				LogLevel:                "info",                    // Default value
				EnableProgressReporting: false,                     // Default value
			},
		},
		{
			name: "module creation with full custom config",
			setupViper: func(v *viper.Viper) {
				v.Set("endpoint", "https://custom-endpoint.com")
				v.Set("cache_dir", "/custom/cache")
				v.Set("max_concurrent_downloads", 8)
				v.Set("enable_dedup", false)
				v.Set("log_level", "debug")
				v.Set("enable_progress_reporting", true)
			},
			expectError: false,
			expectedConfig: struct {
				Endpoint                string
				CacheDir                string
				MaxConcurrentDownloads  uint32
				EnableDedup             bool
				LogLevel                string
				EnableProgressReporting bool
			}{
				Endpoint:                "https://custom-endpoint.com",
				CacheDir:                "/custom/cache",
				MaxConcurrentDownloads:  8,
				EnableDedup:             false,
				LogLevel:                "debug",
				EnableProgressReporting: true,
			},
		},
		{
			name: "module creation with invalid config - no endpoint",
			setupViper: func(v *viper.Viper) {
				// Don't set endpoint, but WithDefaults will provide it
				v.Set("cache_dir", "/tmp/test-cache")
				v.Set("max_concurrent_downloads", 0) // This will be overridden by WithDefaults
			},
			expectError: false, // WithDefaults makes this valid
			expectedConfig: struct {
				Endpoint                string
				CacheDir                string
				MaxConcurrentDownloads  uint32
				EnableDedup             bool
				LogLevel                string
				EnableProgressReporting bool
			}{
				Endpoint:                "https://huggingface.co", // Default value
				CacheDir:                "/tmp/test-cache",
				MaxConcurrentDownloads:  4,      // Default value (overrides 0)
				EnableDedup:             false,  // Zero value (WithDefaults doesn't set this)
				LogLevel:                "info", // Default value
				EnableProgressReporting: false,  // Default value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a custom module that captures the config for verification
			var client *Client
			var capturedConfig *Config
			var logger *testingPkg.MockLogger

			customModule := fx.Options(
				fx.Provide(
					func(v *viper.Viper, params HubParams) (*Client, error) {
						config, err := NewConfig(
							WithViper(v),
							WithAppParams(params),
							WithLogger(params.Logger),
							WithDefaults(),
						)
						if err != nil {
							return nil, fmt.Errorf("error creating hub config: %+v", err)
						}

						// Capture the config for verification
						capturedConfig = config

						client, err := NewClient(config)
						if err != nil {
							return nil, fmt.Errorf("error creating xet client: %+v", err)
						}

						if config.EnableProgressReporting {
							// Enable progress reporting for the xet client
							if err := client.EnableConsoleProgress("direct", 250*time.Millisecond); err != nil {
								params.Logger.Warnf("warning: unable to enable progress reporting: %v", err)
							}
						}

						return client, nil
					},
				),
				fx.Invoke(func(lc fx.Lifecycle, client *Client, logger logging.Interface) {
					lc.Append(fx.Hook{
						OnStop: func(ctx context.Context) error {
							if err := client.Close(); err != nil {
								logger.Warnf("warning: error closing xet client: %v", err)
							}
							return nil
						},
					})
				}),
			)

			app := fx.New(
				fx.Provide(func() *viper.Viper {
					v := viper.New()
					tt.setupViper(v)
					return v
				}),
				fx.Provide(func() logging.Interface {
					logger = testingPkg.SetupMockLogger()
					return logger
				}),
				fx.Provide(fx.Annotate(func() logging.Interface {
					return logger
				}, fx.ResultTags(`name:"another_log"`))),
				customModule,
				fx.Populate(&client),
			)

			if tt.expectError {
				assert.Error(t, app.Err())
				return
			}

			require.NoError(t, app.Err())
			require.NotNil(t, client)
			require.NotNil(t, capturedConfig)

			// Verify the configuration values
			assert.Equal(t, tt.expectedConfig.Endpoint, capturedConfig.Endpoint, "Endpoint should match expected value")
			assert.Equal(t, tt.expectedConfig.CacheDir, capturedConfig.CacheDir, "CacheDir should match expected value")
			assert.Equal(t, tt.expectedConfig.MaxConcurrentDownloads, capturedConfig.MaxConcurrentDownloads, "MaxConcurrentDownloads should match expected value")
			assert.Equal(t, tt.expectedConfig.EnableDedup, capturedConfig.EnableDedup, "EnableDedup should match expected value")
			assert.Equal(t, tt.expectedConfig.LogLevel, capturedConfig.LogLevel, "LogLevel should match expected value")
			assert.Equal(t, tt.expectedConfig.EnableProgressReporting, capturedConfig.EnableProgressReporting, "EnableProgressReporting should match expected value")

			// Test that the client can be closed (lifecycle hook)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := app.Stop(ctx)
			assert.NoError(t, err)
		})
	}
}
