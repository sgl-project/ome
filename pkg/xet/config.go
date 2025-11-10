package xet

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/ome/pkg/configutils"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
)

const (
	DefaultHFEndpoint             = "https://huggingface.co"
	DefaultHFCacheDir             = "/tmp/.cache/huggingface"
	DefaultMaxConcurrentDownloads = 4
	DefaultLogLevel               = "info"
	RepoTypeModel                 = "model"
)

// Config holds configuration for the xet client
type Config struct {
	Logger                  logging.Interface
	Endpoint                string `mapstructure:"endpoint" validate:"required"`
	Token                   string `mapstructure:"hf_token"`
	CacheDir                string `mapstructure:"cache_dir"`
	MaxConcurrentDownloads  uint32 `mapstructure:"max_concurrent_downloads" validate:"gt=0"`
	EnableDedup             bool   `mapstructure:"enable_dedup"`
	LogLevel                string `mapstructure:"log_level"` // Optional: error, warn, info, debug, trace
	EnableProgressReporting bool   `mapstructure:"enable_progress_reporting"`
}

// Option represents a configuration option function
type Option func(*Config) error

// Apply applies the given options to the configuration
func (c *Config) Apply(opts ...Option) error {
	for _, o := range opts {
		if o == nil {
			continue
		}

		if err := o(c); err != nil {
			return err
		}
	}
	return nil
}

// NewConfig builds and returns a new configuration from the given options
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// defaultHubConfig returns a default configuration
func defaultConfig() *Config {
	return &Config{
		Endpoint:               DefaultHFEndpoint,
		CacheDir:               DefaultHFCacheDir,
		MaxConcurrentDownloads: DefaultMaxConcurrentDownloads,
		EnableDedup:            true,
		LogLevel:               DefaultLogLevel,
	}
}

// WithViper attempts to resolve the configuration using Viper
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error occurred when binding envs: %+v", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}
		return nil
	}
}

// WithLogger specifies the logger
func WithLogger(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("invalid logger nil")
		}

		c.Logger = logger
		return nil
	}
}

// WithAppParams applies configuration parameters from Hub params.
func WithAppParams(params HubParams) Option {
	return func(c *Config) error {
		return nil
	}
}

// WithEnableProgressReporting specifies if enable progress reporting
func WithEnableProgressReporting(enableProgressReporting bool) Option {
	return func(c *Config) error {
		c.EnableProgressReporting = enableProgressReporting
		return nil
	}
}

// WithDefaults specifies the default values for the configuration if not already set
func WithDefaults() Option {
	return func(c *Config) error {
		if c.Endpoint == "" {
			c.Endpoint = DefaultHFEndpoint
		}
		if c.CacheDir == "" {
			c.CacheDir = DefaultHFCacheDir
		}
		if c.MaxConcurrentDownloads == 0 {
			c.MaxConcurrentDownloads = DefaultMaxConcurrentDownloads
		}
		if c.LogLevel == "" {
			c.LogLevel = DefaultLogLevel
		}

		return nil
	}
}

func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Additional custom validations
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.CacheDir == "" {
		return errors.New("cache directory is required")
	}
	if c.MaxConcurrentDownloads <= 0 {
		return errors.New("max workers must be positive")
	}

	return nil
}

// ToDownloadConfig converts Config to DownloadConfig
func (c *Config) ToDownloadConfig() *DownloadConfig {
	return &DownloadConfig{
		Token:      c.Token,
		CacheDir:   c.CacheDir,
		Endpoint:   c.Endpoint,
		MaxWorkers: int(c.MaxConcurrentDownloads),
		// Set sensible defaults for common fields
		Revision: "main",        // Default git branch
		RepoType: RepoTypeModel, // Most common repository type
	}
}
