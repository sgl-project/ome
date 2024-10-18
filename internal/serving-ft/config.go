package serving_ft

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config represents a struct to hold all configs for finetune serving init
type Config struct {
	AnotherLogger logging.Interface

	ModelDownloadDirectory string            `mapstructure:"model_download_directory"`
	ModelWeightDirectory   string            `mapstructure:"model_weight_directory"`
	IsFTWeightsMerged      bool              `mapstructure:"is_ft_weights_merged"`
	FinetuningStrategy     string            `mapstructure:"finetuning_strategy"`
	ModelFormat            string            `mapstructure:"model_format"`
	FineTunedWeightURI     *casper.ObjectURI `mapstructure:"ft_weight_object_store_uri"`

	// Injected client object from DI
	CasperDataStore *casper.CasperDataStore
}

// Option represents a server configuration option.
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

func WithAppParams(params appParams) Option {
	return func(c *Config) error {
		c.CasperDataStore = params.CasperDataStore
		return nil
	}
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *Config) error {
		return nil
	}
}

// WithAnotherLog specifies the csr logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

// Validate ensures the app Config is valid.
func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
