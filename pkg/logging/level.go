package logging

import (
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

// Level is an enumeration encapsulating the logging level.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// ParseLevel parses the logging level.
func ParseLevel(level string) (Level, error) {
	if level == "" {
		return LevelInfo, nil
	}

	switch strings.ToUpper(level) {
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return "", fmt.Errorf("unknown log level: %s", level)
	}
}

// Validate validates whether this Level is valid.
func (l Level) Validate() error {
	lvl := string(l)

	switch strings.ToUpper(lvl) {
	case "", "DEBUG", "INFO", "WARN", "ERROR":
		return nil
	default:
		return fmt.Errorf("unknown log level: %s", l)
	}
}

// String implements fmt.Stringer.
func (l Level) String() string { return strings.ToUpper(string(l)) }

// toZapCoreLevel converts this Level into its zapcore equivalent.
func (l Level) toZapCoreLevel() (zapcore.Level, error) {
	switch strings.ToUpper(string(l)) {
	case "DEBUG":
		return zapcore.DebugLevel, nil
	case "", "INFO":
		return zapcore.InfoLevel, nil
	case "WARN":
		return zapcore.WarnLevel, nil
	case "ERROR":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("can't convert log level to zapcore.Level: %s", l)
	}
}

// toZapCoreLevel returns the zapcore.Level determined from this config.
func (c *Config) toZapCoreLevel() (zapcore.Level, error) {
	if c.Debug {
		return zapcore.DebugLevel, nil
	}

	return c.Level.toZapCoreLevel()
}
