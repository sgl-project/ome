package logging

import (
	"fmt"
)

// Interface exists solely in order to decouple clients from various implementations
// of logging libraries (while we transition from logrus to zap).
//
// NOTE: This is temporary and not intended to be fast (printf-like methods are really bad).
// Use zap logger directly for new code (see benchmarks at https://github.com/uber-go/zap).
type Interface interface {
	WithField(key string, value interface{}) Interface
	WithError(err error) Interface

	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)

	// Avoid using Printf-like methods
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

func fmtMsg(format string, args []interface{}) string {
	msg := format
	if len(args) != 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return msg
}
