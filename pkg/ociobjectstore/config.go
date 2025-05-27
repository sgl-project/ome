package ociobjectstore

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/principals"
	"github.com/spf13/viper"
)

// Viper keys must match the `mapstructure` tags defined in the Config struct
const (
	NameViperKeyName           = "name"
	AuthTypeViperKeyName       = "auth_type"
	CompartmentIdViperKeyName  = "compartment_id"
	RegionViperKeyName         = "region_override"
	EnableOboTokenViperKeyName = "enable_obo_token"
	OboTokenViperKeyName       = "obo_token"
)

// Config holds the configuration parameters required to initialize a OCIOSDataStore.
// Fields are populated using `viper`, environment values, or explicitly through Options.
type Config struct {
	AnotherLogger  logging.Interface              // Optional: Named logger for diagnostics
	Name           string                         `mapstructure:"name"`                                                 // Name for the configuration (useful in multi-store setup)
	AuthType       *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`                        // Authentication method (e.g., instance principal, API key)
	CompartmentId  *string                        `mapstructure:"compartment_id"`                                       // OCI Compartment OCID
	Region         string                         `mapstructure:"region_override"`                                      // Optional region override
	EnableOboToken bool                           `mapstructure:"enable_obo_token"`                                     // Whether OBO token should be used
	OboToken       string                         `mapstructure:"obo_token" validate:"required_if=EnableOboToken true"` // Token used when OBO is enabled
}

// Option defines a functional configuration override for building a Config.
type Option func(*Config) error

// Apply applies a sequence of configuration options to the Config instance.
// It returns the first error encountered or nil if all options succeed.
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

// NewConfig constructs and returns a new Config by applying the given options.
// Returns an error if any option application fails.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}
	return c, nil
}

// WithViper returns a configuration Option that populates the Config fields using Viper.
// Assumes the config keys match the constants defined above.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		c.Name = v.GetString(NameViperKeyName)
		c.CompartmentId = common.String(v.GetString(CompartmentIdViperKeyName))
		c.Region = v.GetString(RegionViperKeyName)
		c.EnableOboToken = v.GetBool(EnableOboTokenViperKeyName)
		c.OboToken = v.GetString(OboTokenViperKeyName)

		if err := v.UnmarshalKey(AuthTypeViperKeyName, &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithAnotherLog sets the logger to be used by the Config.
// Returns an error if the logger is nil.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("nil another logger")
		}
		c.AnotherLogger = logger
		return nil
	}
}

// Validate performs struct validation on the Config using go-playground/validator.
// Returns an error if required fields or conditions are not satisfied.
func (c *Config) Validate() error {
	validate := validator.New()
	return validate.Struct(c)
}
