package logging

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// ConfigKey is the root configuration key (in Viper) for this module.
var ConfigKey = "logging"

// Config holds the configuration for logging.
type Config struct {
	Debug bool

	// If set, timestamps will be serialized as RFC3339Nano time format.
	// Otherwise, default EncodeTime formatter will be used (ISO8601 if debug is set, Epoch otherwise).
	//
	// See getZapEncoderConfig() for details.
	EncodeTimeAsRFC3339Nano bool

	// DisableConsoleOutput disables logs to be written to the console.
	// This will prevent ODO from copying these logs into journalctl -> syslog -> /var/log/user.log
	// which can cause disk space usage issues.
	DisableConsoleOutput bool

	LumberjackLogger *lumberjack.Logger
}

// Option is a configuration option for logging.
type Option func(*Config) error

// ensureLogger ensures that the LumberjackLogger pointer is not nil by creating
// a logger struct if necessary.
func (c *Config) ensureLogger() {
	if c.LumberjackLogger == nil {
		c.LumberjackLogger = &lumberjack.Logger{}
	}
}

// Validate ensures the logging Config is valid.
func (c *Config) Validate() error {
	if c.LumberjackLogger == nil {
		return errors.New("nil logger")
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

		if v.GetBool("debug") {
			if err := WithDebugging(c); err != nil {
				return err
			}
		}

		if v.GetBool(configKey + ".encodeTimeAsRFC3339Nano") {
			if err := WithEncodeTimeAsRFC3339Nano(c, true); err != nil {
				return err
			}
		}

		if v.GetBool(configKey + ".disableConsoleOutput") {
			if err := WithDisableConsoleOutput(c, true); err != nil {
				return err
			}
		}

		// just unmarshal directly to the struct
		c.ensureLogger()
		if err := v.UnmarshalKey(configKey, c.LumberjackLogger); err != nil {
			return err
		}

		return nil
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

// WithDebugging specifies use of debug log levels and settings.
func WithDebugging(c *Config) error {
	c.Debug = true
	return nil
}

// WithEncodeTimeAsRFC3339Nano instructs the logger to serialize timestamps
// as RFC3339Nano time format.
func WithEncodeTimeAsRFC3339Nano(c *Config, value bool) error {
	c.EncodeTimeAsRFC3339Nano = value
	return nil
}

// WithDisableConsoleOutput instructs the logger to disable console emitter.
func WithDisableConsoleOutput(c *Config, value bool) error {
	c.DisableConsoleOutput = value
	return nil
}

// WithLumberjackLogger specifies the lumberjack logger.
func WithLumberjackLogger(logger *lumberjack.Logger) Option {
	return func(c *Config) error {
		c.LumberjackLogger = logger
		return nil
	}
}

// WithFilename sets the log filename.
func WithFilename(filename string) Option {
	return func(c *Config) error {
		c.ensureLogger()
		c.LumberjackLogger.Filename = filename
		return nil
	}
}

// WithMaxAge specifies the maximum age of log files in days.
func WithMaxAge(days int) Option {
	return func(c *Config) error {
		if days < 0 {
			return fmt.Errorf("max age days must be >= 0, not %d", days)
		}

		c.ensureLogger()
		c.LumberjackLogger.MaxAge = days
		return nil
	}
}

// WithMaxBackups specifies the number of backup logs to keep.
func WithMaxBackups(backups int) Option {
	return func(c *Config) error {
		if backups < 0 {
			return fmt.Errorf("max backups must be >= 0, not %d", backups)
		}

		c.ensureLogger()
		c.LumberjackLogger.MaxBackups = backups
		return nil
	}
}

// WithCompression enables log compression.
func WithCompression(c *Config) error {
	c.ensureLogger()
	c.LumberjackLogger.Compress = true
	return nil
}

// WithLocalTime uses local time instead of UTC for filenames.
func WithLocalTime(c *Config) error {
	c.ensureLogger()
	c.LumberjackLogger.LocalTime = true
	return nil
}
