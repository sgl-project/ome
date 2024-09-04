package casper

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"errors"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/spf13/viper"
	"strings"
)

const (
	RegionViperKeyNameSuffix         = "region"
	CompartmentIdViperKeyNameSuffix  = "compartment_id"
	EnableOboTokenViperKeyNameSuffix = "enable_obo_token"
	OboTokenViperKeyNameSuffix       = "obo_token"
)

type Config struct {
	AnotherLogger  logging.Interface
	AuthType       *principals.AuthenticationType `mapstructure:"auth_type"`
	CompartmentId  *string                        `mapstructure:"compartment_id"`
	Region         string                         `mapstructure:"region"`
	EnableOboToken bool                           `mapstructure:"enable_obo_token"`
	OboToken       string                         `mapstructure:"obo_token"`
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

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper, viperKeyNames []string) Option {
	return func(c *Config) error {
		// Set up config using viperKeyNames
		for _, keyName := range viperKeyNames {
			if strings.Contains(keyName, RegionViperKeyNameSuffix) {
				c.Region = v.GetString(keyName)
				continue
			}
			if strings.Contains(keyName, CompartmentIdViperKeyNameSuffix) {
				c.CompartmentId = common.String(v.GetString(keyName))
				continue
			}

			if strings.Contains(keyName, EnableOboTokenViperKeyNameSuffix) {
				c.EnableOboToken = v.GetBool(keyName)
				continue
			}

			if strings.Contains(keyName, OboTokenViperKeyNameSuffix) {
				c.OboToken = v.GetString(keyName)
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
	return func(c *Config) error {
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

func (c *Config) Validate() error {
	if c.AuthType == nil {
		return errors.New("missing config variable - no auth_type in casper config struct")
	}

	// TODO: Implement
	return nil
}
