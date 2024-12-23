package logging

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ConfigKey is the root configuration key (in Viper) for this module.
var ConfigKey = "logging"

// Config holds the configuration for logging.
type Config struct {
	// Debug sets the logging level to debug.
	//
	// If debug is true, any value in `level` is ignored and
	// it forces the logger to use Console encoder (instead of JSON).
	//
	// If you want to use JSON decoder while producing debug logs, use "debug=false, level=debug" combination.
	//
	// This field is here for legacy reasons only, maybe at some point we'll be able to get rid of it.
	Debug bool `mapstructure:"debug"`

	// Level controls the logging level.
	//
	// Defaults to INFO if not set.
	Level Level `mapstructure:"level"`

	// If set, timestamps will be serialized as RFC3339Nano time format.
	// Otherwise, default EncodeTime formatter will be used (ISO8601 if debug is set, Epoch otherwise).
	//
	// See getZapEncoderConfig() for details.
	EncodeTimeAsRFC3339Nano bool `mapstructure:"encodeTimeAsRFC3339Nano"`

	// DisableConsoleOutput disables logs to be written to the console.
	// This will prevent ODO from copying these logs into journalctl -> syslog -> /var/log/user.log
	// which can cause disk space usage issues.
	DisableConsoleOutput bool `mapstructure:"disableConsoleOutput"`

	// Logger contains various knobs of lumberjack logging functionality.
	lumberjack.Logger `mapstructure:",squash"`
}

// Option is a configuration option for logging.
type Option func(*Config) error

// Validate ensures the logging Config is valid.
func (c *Config) Validate() error {
	if c.MaxSize < 0 {
		return fmt.Errorf("maxsize must be >= 0, not %d", c.MaxSize)
	}
	if c.MaxBackups < 0 {
		return fmt.Errorf("maxbackups must be >= 0, not %d", c.MaxBackups)
	}
	if c.MaxAge < 0 {
		return fmt.Errorf("maxage days must be >= 0, not %d", c.MaxAge)
	}
	if err := c.Level.Validate(); err != nil {
		return fmt.Errorf("invalid level: %w", err)
	}

	return nil
}

// WithViper applies the configuration using Viper root configuration key "logging".
// It assumes that Viper has already been configured to read from a config file,
// the environment, or flags.
//
// By its nature, calling WithViper ensures the resulting config will never fail Validate.
func WithViper(v *viper.Viper) Option {
	return WithViperKey(v, ConfigKey)
}

// WithViperKey applies the configuration using Viper using a specified configuration key.
// It assumes that Viper has already been configured to read from a config file,
// the environment, or flags.
//
// By its nature, calling WithViperKey ensures the resulting config will never fail Validate.
func WithViperKey(v *viper.Viper, configKey string) Option {
	return func(c *Config) error {
		if v == nil {
			return errors.New("nil Viper")
		}

		return v.UnmarshalKey(configKey, c)
	}
}

// Apply takes the supplied options and applies them to the configuration.
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

// NewConfig creates a new logging config with the given options.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}
