package casper

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/spf13/viper"
)

const (
	CasperConfigViperKeyNameKey = "casper_config_viper_prefix"

	/*
	 * These Viper key name have to be consistent with mapstructure tags in the struct definition
	 */
	NameViperKeyName           = "name"
	AuthTypeViperKeyName       = "auth_type"
	CompartmentIdViperKeyName  = "compartment_id"
	RegionViperKeyName         = "region_override"
	EnableOboTokenViperKeyName = "enable_obo_token"
	OboTokenViperKeyName       = "obo_token"
)

type Config struct {
	AnotherLogger  logging.Interface
	Name           string                         `mapstructure:"name"`
	AuthType       *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	CompartmentId  *string                        `mapstructure:"compartment_id"`
	Region         string                         `mapstructure:"region_override"`
	EnableOboToken bool                           `mapstructure:"enable_obo_token"`
	OboToken       string                         `mapstructure:"obo_token" validate:"required_if=EnableOboToken true"`
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
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		prefix := v.GetString(CasperConfigViperKeyNameKey)
		if prefix != "" {
			prefix = prefix + "."
		}

		c.Name = v.GetString(fmt.Sprintf("%s%s", prefix, NameViperKeyName))
		c.CompartmentId = common.String(v.GetString(fmt.Sprintf("%s%s", prefix, CompartmentIdViperKeyName)))
		c.Region = v.GetString(fmt.Sprintf("%s%s", prefix, RegionViperKeyName))
		c.EnableOboToken = v.GetBool(fmt.Sprintf("%s%s", prefix, EnableOboTokenViperKeyName))
		c.OboToken = v.GetString(fmt.Sprintf("%s%s", prefix, OboTokenViperKeyName))

		if err := v.UnmarshalKey(fmt.Sprintf("%s%s", prefix, AuthTypeViperKeyName), &c.AuthType); err != nil {
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
	// Validate by using go-playground validator
	validate := validator.New()
	return validate.Struct(c)
}
