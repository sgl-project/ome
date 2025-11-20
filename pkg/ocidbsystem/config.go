package ocidbsystem

import (
	"errors"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
)

type Config struct {
	AuthType      *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	AnotherLogger logging.Interface
	Region        string `mapstructure:"region_override"`
}

// Option defines a functional configuration override for building a Config.
type Option func(*Config) error

// NewConfig constructs and returns a new Config by applying the given options.
// Returns an error if any option application fails.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}
	return c, nil
}

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
