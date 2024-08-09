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

func (l zapWrapper) Debug(msg string)                          { l.logger.Debug(msg) }
func (l zapWrapper) Info(msg string)                           { l.logger.Info(msg) }
func (l zapWrapper) Warn(msg string)                           { l.logger.Warn(msg) }
func (l zapWrapper) Error(msg string)                          { l.logger.Error(msg) }
func (l zapWrapper) Fatal(msg string)                          { l.logger.Fatal(msg) }
func (l zapWrapper) Debugf(format string, args ...interface{}) { l.logger.Debug(fmtMsg(format, args)) }
func (l zapWrapper) Infof(format string, args ...interface{})  { l.logger.Info(fmtMsg(format, args)) }
func (l zapWrapper) Warnf(format string, args ...interface{})  { l.logger.Warn(fmtMsg(format, args)) }
func (l zapWrapper) Errorf(format string, args ...interface{}) { l.logger.Error(fmtMsg(format, args)) }
func (l zapWrapper) Fatalf(format string, args ...interface{}) { l.logger.Fatal(fmtMsg(format, args)) }

func ForZap(logger *zap.Logger) Interface {
	return zapWrapper{logger: logger}
}
