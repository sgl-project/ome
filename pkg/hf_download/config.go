package hf_download

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	Logger                 logging.Interface
	ModelName              string `mapstructure:"model_name" validate:"required"`
	Token                  string `mapstructure:"hf_token"`
	Branch                 string `mapstructure:"branch"`
	LocalPath              string `mapstructure:"local_path"`
	SkipSHA                bool   `mapstructure:"skip_sha"`
	MaxRetries             int    `mapstructure:"max_retries"`
	RetryInternalInSeconds int    `mapstructure:"retry_internal_in_seconds"`
	NumConnections         int    `mapstructure:"num_connections"`
}

func defaultConfig() *Config {
	return &Config{
		MaxRetries:             5,
		RetryInternalInSeconds: 10,
		NumConnections:         8,
		Branch:                 "main",
		LocalPath:              "./",
		SkipSHA:                false,
	}
}

type Option func(*Config) error

// Apply applies the given options to the configuration.
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

// NewConfig builds and returns a new configuration from the given options.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithLogger specifies the logger.
func WithLogger(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			if logger == nil {
				return errors.New("invalid logger nil")
			}
		}

		c.Logger = logger
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *Config) error {
		return nil
	}
}

// WithViper attempts to resolve the configuration for HF hf_download agent using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		// Initialize default configuration
		*c = *defaultConfig()
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error occurred when binding envs: %+v", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		c.ModelName = v.GetString("model_name")

		return nil
	}
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params downloadAgentParams) Option {
	return func(c *Config) error {
		return nil
	}
}

func (c *Config) ValidateConfig() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
