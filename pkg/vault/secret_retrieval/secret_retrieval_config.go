package secret_retrieval

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
)

const (
	SecretRetrievalConfigViperKeyNameKey = "secret_retrieval_config_viper_prefix"

	/*
	 * These Viper key name have to be consistent with mapstructure tags in the struct definition
	 */
	NameViperKeyName     = "name"
	RegionViperKeyName   = "region_override"
	AuthTypeViperKeyName = "auth_type"
)

type SecretRetrievalConfig struct {
	AnotherLogger logging.Interface
	Name          string                         `mapstructure:"name"`
	Region        string                         `mapstructure:"region_override"`
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
}

// Option represents a server configuration option.
type Option func(*SecretRetrievalConfig) error

// Apply applies the given options to the configuration.
func (c *SecretRetrievalConfig) Apply(opts ...Option) error {
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

// NewSecretRetrievalConfig builds and returns a new kms configuration from the given options.
func NewSecretRetrievalConfig(opts ...Option) (*SecretRetrievalConfig, error) {
	c := &SecretRetrievalConfig{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *SecretRetrievalConfig) error {
		prefix := v.GetString(SecretRetrievalConfigViperKeyNameKey)
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
	return func(c *SecretRetrievalConfig) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

func (c *SecretRetrievalConfig) Validate() error {
	// Validate by using package validator
	validate := validator.New()
	return validate.Struct(c)
}
