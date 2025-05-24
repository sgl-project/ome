package kmscrypto

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/principals"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmsvault"
	"github.com/spf13/viper"
)

type Config struct {
	AnotherLogger     logging.Interface
	AuthType          *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	KmsCryptoEndpoint string                         `mapstructure:"kms_crypto_endpoint"`
	VaultId           string                         `mapstructure:"vault_id"`
	KmsVaultClient    *kmsvault.KMSVault
}

// NewConfig builds and returns a new configuration from the given options.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}
	return c, nil
}

// Apply applies the given options to the configuration.
func (c *Config) Apply(opts ...Option) error {
	for _, o := range opts {
		if o != nil {
			if err := o(c); err != nil {
				return err
			}
		}
	}
	return nil
}

// Option represents a configuration option for the server.
type Option func(*Config) error

// WithAnotherLog sets an alternative logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("logger cannot be nil")
		}
		c.AnotherLogger = logger
		return nil
	}
}

// WithViper loads configuration using Viper.
func WithViper(v *viper.Viper, logger logging.Interface) Option {
	return func(c *Config) error {

		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error binding envs: %w", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error unmarshalling config: %w", err)
		}

		return nil
	}
}

// WithAppParams sets the application parameters.
func WithAppParams(params kmsCryptoParams) Option {
	return func(c *Config) error {
		c.KmsVaultClient = params.KmsVaultClient
		endpoint, err := c.KmsVaultClient.GetCryptoEndpoint(c.VaultId)
		if err != nil {
			return err
		}
		c.KmsCryptoEndpoint = endpoint
		return nil
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}
