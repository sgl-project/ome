package hf_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type HFConfig struct {
	Logger logging.Interface

	ModelName         string            `mapstructure:"model_name" validate:"required"`
	InternalModelName string            `mapstructure:"internal_model_name" validate:"required"`
	DownloadCommit    string            `mapstructure:"download_commit"`
	Token             string            `mapstructure:"hf_token"`
	ObjectStoreURI    *casper.ObjectURI `mapstructure:"object_store_uri" validate:"required"`

	// Injected client objects from DI
	CasperDataStore *casper.CasperDataStore `validate:"required"`
}

type Option func(*HFConfig) error

// Apply applies the given options to the configuration.
func (c *HFConfig) Apply(opts ...Option) error {
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

// NewHFConfig builds and returns a new configuration from the given options.
func NewHFConfig(opts ...Option) (*HFConfig, error) {
	c := &HFConfig{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithAnotherLog specifies the csr logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *HFConfig) error {
		if logger == nil {
			return errors.New("invalid logger nil")
		}

		c.Logger = logger
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *HFConfig) error {
		return nil
	}
}

// WithViper attempts to resolve the configuration for HF download agent using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *HFConfig) error {
		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		if len(c.DownloadCommit) == 0 {
			c.DownloadCommit = "main"
		}
		return nil
	}
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params appParams) Option {
	return func(c *HFConfig) error {
		c.CasperDataStore = params.CasperDataStore
		return nil
	}
}

func (c *HFConfig) ValidateHFConfig() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
