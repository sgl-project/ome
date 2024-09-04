package secret_retrieval

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

const (
	RegionViperKeyNameSuffix = "region"
)

type SecretRetrievalConfig struct {
	AnotherLogger logging.Interface
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type"`
	Region        string                         `mapstructure:"region"`
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
func WithViper(v *viper.Viper, viperKeyNames []string) Option {
	return func(c *SecretRetrievalConfig) error {
		for _, keyName := range viperKeyNames {
			if strings.Contains(keyName, RegionViperKeyNameSuffix) {
				c.Region = v.GetString(keyName)
				continue
			}
		}

		if err := v.UnmarshalKey("auth_type", &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *SecretRetrievalConfig) error {
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
	if c.AuthType == nil {
		return errors.New("missing config variable - no auth_type in SecretRetrievalConfig struct")
	}
	return nil
}
