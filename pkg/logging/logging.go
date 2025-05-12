package logging

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TimeNowFunc lets you specify the function for obtaining the current time.
// This is mainly to aid in testing.
var TimeNowFunc = time.Now

// TimeFormat is the time format to be used when writing logs.
var TimeFormat = time.RFC3339

// RequestIDKey is the key for the request ID in the context.
const RequestIDKey = "request-id"

// RequestIDHeader is the header to look for the request ID.
const RequestIDHeader = "opc-request-id"

// RequestLoggerKey is the key for the logger in the context.
const RequestLoggerKey = "request-logger"

// NewLogger takes a logging config and returns a new Zap logger that writes to
// the log file pointed to by the config with the options applied and stdout.
func NewLogger(config *Config) (*zap.Logger, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logging config: %w", err)
	}

	encoder, level, err := constructEncoderAndLevel(config)
	if err != nil {
		return nil, fmt.Errorf("constructing log encoder and level: %w", err)
	}

	logFile := zapcore.AddSync(&config.Logger)
	logCore := zapcore.NewCore(encoder, logFile, level)

	var core zapcore.Core
	if config.DisableConsoleOutput {
		core = logCore
	} else {
		console := zapcore.Lock(os.Stdout)
		consoleCore := zapcore.NewCore(encoder, console, level)
		core = zapcore.NewTee(logCore, consoleCore)
	}

	// Add caller information with proper skip level to show actual source file, not zap internal files
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)), nil
}

func constructEncoderAndLevel(config *Config) (zapcore.Encoder, zapcore.Level, error) {
	zapLevel, err := config.toZapCoreLevel()
	if err != nil {
		return nil, zapLevel, err
	}

	// TODO: Should this behave differently?
	//
	// Right now debug logs get written to the log file. That can be a bit
	// weird, but having debug logs written to a file can be useful when sifting
	// through lots of data while debugging a series of calls. The debug logs
	// are also in a more readable (i.e., different) format.
	//
	// Moreover, the lumberjack library is smart enough to write logs to a temp
	// file if the filename isn't specified, but that also means that this
	// silently writes files all the time, which can be annoying if you're
	// trying to run this in an immutable container.
	encoderConfig := getZapEncoderConfig(config)
	if config.Debug {
		return zapcore.NewConsoleEncoder(encoderConfig), zapLevel, nil
	}

	return zapcore.NewJSONEncoder(encoderConfig), zapLevel, nil
}

func getZapEncoderConfig(config *Config) zapcore.EncoderConfig {
	encoderConfig := zap.NewProductionEncoderConfig()
	if config.Debug {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	if config.EncodeTimeAsRFC3339Nano {
		encoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	}

	return encoderConfig
}

func NewTestLogger() Interface {
	return ForLogrus(logrus.NewEntry(logrus.New()))
}
