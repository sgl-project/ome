package merged_finetuned_adapter

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	AnotherLogger logging.Interface

	FineTunedWeightURI              *casper.ObjectURI       `mapstructure:"model" validate:"required"`
	unzippedFinetunedModelDirectory string                  `mapstructure:"unzipped_finetuned_model_directory" validate:"required"`
	zippedFinetunedModelDirectory   string                  `mapstructure:"zipped_finetuned_model_directory" validate:"required"`
	ObjectStorageDataStore          *casper.CasperDataStore `validate:"required"`
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

// defaultConfig returns a new configuration with default values.
func defaultConfig() *Config {
	return &Config{}
}

// NewMergedFinetunedAdapterConfig builds and returns a new configuration from the given options.
func NewMergedFinetunedAdapterConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params mergedFinetunedAdapterParams) Option {
	return func(c *Config) error {
		c.ObjectStorageDataStore = params.ObjectStorageDataStores
		return nil
	}
}

// WithAnotherLog sets the logger for the configuration.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		c.AnotherLogger = logger
		return nil
	}
}

// WithEnv sets the environment for the configuration.
func WithEnv(env *env.Environment) Option {
	return func(c *Config) error {
		return nil
	}
}

// WithViper sets the viper for the configuration.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {

		*c = *defaultConfig()
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error occurred when binding environment variables: %+v", err)
		}

		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		return nil
	}
}

func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
