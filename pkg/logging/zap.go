package logging

import "go.uber.org/zap"

type zapWrapper struct {
	logger *zap.Logger
}

func (l zapWrapper) WithField(key string, value interface{}) Interface {
	return zapWrapper{l.logger.With(zap.Any(key, value))}
}

func (l zapWrapper) WithError(err error) Interface {
	return zapWrapper{l.logger.With(zap.Error(err))}
}

// Add Skip parameter to all logging methods to ensure caller information is preserved
func (l zapWrapper) Debug(msg string) { l.logger.WithOptions(zap.AddCallerSkip(1)).Debug(msg) }
func (l zapWrapper) Info(msg string)  { l.logger.WithOptions(zap.AddCallerSkip(1)).Info(msg) }
func (l zapWrapper) Warn(msg string)  { l.logger.WithOptions(zap.AddCallerSkip(1)).Warn(msg) }
func (l zapWrapper) Error(msg string) { l.logger.WithOptions(zap.AddCallerSkip(1)).Error(msg) }
func (l zapWrapper) Fatal(msg string) { l.logger.WithOptions(zap.AddCallerSkip(1)).Fatal(msg) }
func (l zapWrapper) Debugf(format string, args ...interface{}) {
	l.logger.WithOptions(zap.AddCallerSkip(1)).Debug(fmtMsg(format, args))
}
func (l zapWrapper) Infof(format string, args ...interface{}) {
	l.logger.WithOptions(zap.AddCallerSkip(1)).Info(fmtMsg(format, args))
}
func (l zapWrapper) Warnf(format string, args ...interface{}) {
	l.logger.WithOptions(zap.AddCallerSkip(1)).Warn(fmtMsg(format, args))
}
func (l zapWrapper) Errorf(format string, args ...interface{}) {
	l.logger.WithOptions(zap.AddCallerSkip(1)).Error(fmtMsg(format, args))
}
func (l zapWrapper) Fatalf(format string, args ...interface{}) {
	l.logger.WithOptions(zap.AddCallerSkip(1)).Fatal(fmtMsg(format, args))
}

// Create Zap logger with caller information enabled
func ForZap(logger *zap.Logger) Interface {
	// If caller isn't already enabled, add it
	if !logger.Core().Enabled(zap.DebugLevel) {
		logger = logger.WithOptions(zap.AddCaller())
	}
	return zapWrapper{logger: logger}
}
