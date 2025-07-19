package oci_vault

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"

	"github.com/sgl-project/ome/pkg/configutils"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
)

type Config struct {
	AnotherLogger logging.Interface
	Name          string                         `mapstructure:"name"`
	Region        string                         `mapstructure:"region_override"`
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
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

// NewSecretInVaultConfig builds and returns a new config for SecretInVault from the given options.
func NewSecretInVaultConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {

		if err := configutils.BindEnvsRecursive(v, c, "SECRET_IN_VAULT"); err != nil {
			return fmt.Errorf("error occurred when binding envs: %+v", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithAnotherLog specifies the logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

// WithAppParams specifies the application parameters.
func WithAppParams(params vaultParams) Option {
	return func(c *Config) error {
		return nil
	}
}

func (c *Config) Validate() error {
	// Validate by using package validator
	validate := validator.New()
	return validate.Struct(c)
}
