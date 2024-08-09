package secret_in_vault

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/principals"
	"errors"
	"fmt"
	"github.com/spf13/viper"
)

type SecretInVaultConfig struct {
	AnotherLogger logging.Interface
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type"`
	Region        string                         `mapstructure:"region"`
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
		if err := v.UnmarshalKey("auth_type", &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *SecretInVaultConfig) error {
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
	if c.AuthType == nil {
		return errors.New("missing config variable - no auth_type in SecretInVaultConfig struct")
	}
	return nil
}
