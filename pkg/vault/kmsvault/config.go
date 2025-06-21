package kmsvault

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/spf13/viper"
)

type Config struct {
	AnotherLogger  logging.Interface
	AuthType       *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	region         string                         `mapstructure:"region_override"`
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

// WithAnotherLogger sets the logger to use for the configuration.
func WithAnotherLogger(l logging.Interface) Option {
	return func(c *Config) error {
		c.AnotherLogger = l
		return nil
	}
}

// WithViper sets the viper to use for the configuration.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error unmarshalling viper configuration: %w", err)
		}
		return nil
	}
}

func (c *Config) Validate() error {
	// Validate the configuration
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return fmt.Errorf("error validating configuration: %w", err)
	}
	return nil
}
