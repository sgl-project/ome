package hub

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
)

// HubConfig represents the configuration for the Hugging Face Hub client
type HubConfig struct {
	Logger              logging.Interface
	Token               string        `mapstructure:"hf_token"`
	Endpoint            string        `mapstructure:"endpoint"`
	CacheDir            string        `mapstructure:"cache_dir"`
	UserAgent           string        `mapstructure:"user_agent"`
	RequestTimeout      time.Duration `mapstructure:"request_timeout"`
	EtagTimeout         time.Duration `mapstructure:"etag_timeout"`
	DownloadTimeout     time.Duration `mapstructure:"download_timeout"`
	MaxRetries          int           `mapstructure:"max_retries"`
	RetryInterval       time.Duration `mapstructure:"retry_interval"`
	MaxWorkers          int           `mapstructure:"max_workers"`
	ChunkSize           int64         `mapstructure:"chunk_size"`
	LocalFilesOnly      bool          `mapstructure:"local_files_only"`
	DisableProgressBars bool          `mapstructure:"disable_progress_bars"`
	EnableOfflineMode   bool          `mapstructure:"enable_offline_mode"`
	EnableSymlinks      bool          `mapstructure:"enable_symlinks"`
	VerifySSL           bool          `mapstructure:"verify_ssl"`
	EnableDetailedLogs  bool          `mapstructure:"enable_detailed_logs"`
	LogLevel            string        `mapstructure:"log_level"`
}

// defaultHubConfig returns a default configuration
func defaultHubConfig() *HubConfig {
	return &HubConfig{
		Endpoint:            DefaultEndpoint,
		CacheDir:            GetCacheDir(),
		UserAgent:           "huggingface-hub-go/1.0.0",
		RequestTimeout:      DefaultRequestTimeout,
		EtagTimeout:         DefaultEtagTimeout,
		DownloadTimeout:     DownloadTimeout,
		MaxRetries:          DefaultMaxRetries,
		RetryInterval:       DefaultRetryInterval,
		MaxWorkers:          DefaultMaxWorkers,
		ChunkSize:           DefaultChunkSize,
		LocalFilesOnly:      false,
		DisableProgressBars: false,
		EnableOfflineMode:   false,
		EnableSymlinks:      true,
		VerifySSL:           true,
		EnableDetailedLogs:  false,
		LogLevel:            "info",
		Token:               GetHfToken(),
	}
}

// HubOption represents a configuration option function
type HubOption func(*HubConfig) error

// Apply applies the given options to the configuration
func (c *HubConfig) Apply(opts ...HubOption) error {
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

// NewHubConfig builds and returns a new configuration from the given options
func NewHubConfig(opts ...HubOption) (*HubConfig, error) {
	c := defaultHubConfig()
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithLogger specifies the logger
func WithLogger(logger logging.Interface) HubOption {
	return func(c *HubConfig) error {
		if logger == nil {
			return errors.New("invalid logger nil")
		}

		c.Logger = logger
		return nil
	}
}

// WithToken specifies the HF token
func WithToken(token string) HubOption {
	return func(c *HubConfig) error {
		c.Token = token
		return nil
	}
}

// WithEndpoint specifies the Hub endpoint
func WithEndpoint(endpoint string) HubOption {
	return func(c *HubConfig) error {
		if endpoint == "" {
			return errors.New("endpoint cannot be empty")
		}
		c.Endpoint = endpoint
		return nil
	}
}

// WithCacheDir specifies the cache directory
func WithCacheDir(cacheDir string) HubOption {
	return func(c *HubConfig) error {
		if cacheDir == "" {
			return errors.New("cache directory cannot be empty")
		}
		c.CacheDir = cacheDir
		return nil
	}
}

// WithUserAgent specifies the user agent
func WithUserAgent(userAgent string) HubOption {
	return func(c *HubConfig) error {
		c.UserAgent = userAgent
		return nil
	}
}

// WithTimeouts specifies various timeout values
func WithTimeouts(request, etag, download time.Duration) HubOption {
	return func(c *HubConfig) error {
		if request > 0 {
			c.RequestTimeout = request
		}
		if etag > 0 {
			c.EtagTimeout = etag
		}
		if download > 0 {
			c.DownloadTimeout = download
		}
		return nil
	}
}

// WithRetryConfig specifies retry configuration
func WithRetryConfig(maxRetries int, retryInterval time.Duration) HubOption {
	return func(c *HubConfig) error {
		if maxRetries < 0 {
			return errors.New("max retries cannot be negative")
		}
		c.MaxRetries = maxRetries
		c.RetryInterval = retryInterval
		return nil
	}
}

// WithConcurrency specifies concurrency settings
func WithConcurrency(maxWorkers int, chunkSize int64) HubOption {
	return func(c *HubConfig) error {
		if maxWorkers <= 0 {
			return errors.New("max workers must be positive")
		}
		if chunkSize <= 0 {
			return errors.New("chunk size must be positive")
		}
		c.MaxWorkers = maxWorkers
		c.ChunkSize = chunkSize
		return nil
	}
}

// WithLocalFilesOnly enables local files only mode
func WithLocalFilesOnly(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.LocalFilesOnly = enabled
		return nil
	}
}

// WithOfflineMode enables offline mode
func WithOfflineMode(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.EnableOfflineMode = enabled
		return nil
	}
}

// WithSymlinks enables or disables symlink usage
func WithSymlinks(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.EnableSymlinks = enabled
		return nil
	}
}

// WithProgressBars enables or disables progress bars
func WithProgressBars(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.DisableProgressBars = !enabled
		return nil
	}
}

// WithSSLVerification enables or disables SSL verification
func WithSSLVerification(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.VerifySSL = enabled
		return nil
	}
}

// WithViper attempts to resolve the configuration using Viper
func WithViper(v *viper.Viper) HubOption {
	return func(c *HubConfig) error {
		// Initialize with defaults first
		*c = *defaultHubConfig()

		if err := configutils.BindEnvsRecursive(v, c, "hub"); err != nil {
			return fmt.Errorf("error occurred when binding envs: %+v", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Override with specific viper keys if they exist
		if v.IsSet("hf_token") {
			c.Token = v.GetString("hf_token")
		}
		if v.IsSet("endpoint") {
			c.Endpoint = v.GetString("endpoint")
		}
		if v.IsSet("cache_dir") {
			c.CacheDir = v.GetString("cache_dir")
		}

		return nil
	}
}

// ValidateConfig validates the configuration
func (c *HubConfig) ValidateConfig() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}

	// Additional custom validations
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.CacheDir == "" {
		return errors.New("cache directory is required")
	}
	if c.MaxWorkers <= 0 {
		return errors.New("max workers must be positive")
	}
	if c.ChunkSize <= 0 {
		return errors.New("chunk size must be positive")
	}

	return nil
}

// CreateProgressManager creates a progress manager from the configuration
func (c *HubConfig) CreateProgressManager() *ProgressManager {
	return NewProgressManager(
		c.Logger,
		!c.DisableProgressBars,
		c.EnableDetailedLogs,
	)
}

// ToDownloadConfig converts HubConfig to DownloadConfig for backward compatibility
func (c *HubConfig) ToDownloadConfig() *DownloadConfig {
	return &DownloadConfig{
		Token:       c.Token,
		CacheDir:    c.CacheDir,
		Endpoint:    c.Endpoint,
		EtagTimeout: c.EtagTimeout,
		Headers:     BuildHeaders(c.Token, c.UserAgent, nil),
		MaxWorkers:  c.MaxWorkers,
		// Set sensible defaults for common fields
		Revision: "main",        // Default git branch
		RepoType: RepoTypeModel, // Most common repository type
	}
}

// WithDetailedLogs enables or disables detailed logging
func WithDetailedLogs(enabled bool) HubOption {
	return func(c *HubConfig) error {
		c.EnableDetailedLogs = enabled
		return nil
	}
}

// WithLogLevel sets the logging level
func WithLogLevel(level string) HubOption {
	return func(c *HubConfig) error {
		validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
		for _, validLevel := range validLevels {
			if level == validLevel {
				c.LogLevel = level
				return nil
			}
		}
		return fmt.Errorf("invalid log level: %s. Valid levels: %v", level, validLevels)
	}
}
