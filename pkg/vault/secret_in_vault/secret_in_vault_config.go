package secret_in_vault

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/spf13/viper"
)

const (
	SecretInVaultConfigViperKeyNameKey = "secret_in_vault_config_viper_prefix"

	/*
	 * These Viper key name have to be consistent with mapstructure tags in the struct definition
	 */
	NameViperKeyName     = "name"
	AuthTypeViperKeyName = "auth_type"
	RegionViperKeyName   = "region_override"
)

type SecretInVaultConfig struct {
	AnotherLogger logging.Interface
	Name          string                         `mapstructure:"name"`
	Region        string                         `mapstructure:"region_override"`
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
}

// Option represents a server configuration option.
type Option func(*SecretInVaultConfig) error

// Apply applies the given options to the configuration.
func (c *SecretInVaultConfig) Apply(opts ...Option) error {
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
func NewSecretInVaultConfig(opts ...Option) (*SecretInVaultConfig, error) {
	c := &SecretInVaultConfig{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *SecretInVaultConfig) error {
		prefix := v.GetString(SecretInVaultConfigViperKeyNameKey)
		if prefix != "" {
			prefix = prefix + "."
		}

		c.Name = v.GetString(fmt.Sprintf("%s%s", prefix, NameViperKeyName))
		c.Region = v.GetString(fmt.Sprintf("%s%s", prefix, RegionViperKeyName))
		if err := v.UnmarshalKey(fmt.Sprintf("%s%s", prefix, AuthTypeViperKeyName), &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithAnotherLog specifies the logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *SecretInVaultConfig) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

func (c *SecretInVaultConfig) Validate() error {
	// Validate by using package validator
	validate := validator.New()
	return validate.Struct(c)
}
