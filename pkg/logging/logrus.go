package logging

import "github.com/sirupsen/logrus"

type logrusWrapper struct {
	logger *logrus.Entry
}

func (l logrusWrapper) WithField(key string, value interface{}) Interface {
	return logrusWrapper{logger: l.logger.WithField(key, value)}
}

func (l logrusWrapper) WithError(err error) Interface {
	return logrusWrapper{logger: l.logger.WithError(err)}
}

func (l logrusWrapper) Debug(msg string)                          { l.logger.Debug(msg) }
func (l logrusWrapper) Info(msg string)                           { l.logger.Info(msg) }
func (l logrusWrapper) Warn(msg string)                           { l.logger.Warn(msg) }
func (l logrusWrapper) Error(msg string)                          { l.logger.Error(msg) }
func (l logrusWrapper) Fatal(msg string)                          { l.logger.Fatal(msg) }
func (l logrusWrapper) Debugf(format string, args ...interface{}) { l.logger.Debugf(format, args...) }
func (l logrusWrapper) Infof(format string, args ...interface{})  { l.logger.Infof(format, args...) }
func (l logrusWrapper) Warnf(format string, args ...interface{})  { l.logger.Warnf(format, args...) }
func (l logrusWrapper) Errorf(format string, args ...interface{}) { l.logger.Errorf(format, args...) }
func (l logrusWrapper) Fatalf(format string, args ...interface{}) { l.logger.Fatalf(format, args...) }

func ForLogrus(logger *logrus.Entry) Interface {
	return logrusWrapper{logger}
}
