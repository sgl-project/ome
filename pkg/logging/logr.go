package logging

import (
	"fmt"

	"github.com/go-logr/logr"
)

type logrWrapper struct {
	l logr.Logger
}

func ForLogr(l logr.Logger) Interface {
	return &logrWrapper{l: l}
}

func (a *logrWrapper) WithField(key string, value interface{}) Interface {
	return &logrWrapper{l: a.l.WithValues(key, value)}
}

func (a *logrWrapper) WithError(err error) Interface {
	// attach the error as a field; we'll still be able to log without forcing Error(...)
	return &logrWrapper{l: a.l.WithValues("error", err)}
}

func (a *logrWrapper) Debug(msg string) { a.l.V(1).Info(msg) } // V(1) ~= debug
func (a *logrWrapper) Info(msg string)  { a.l.Info(msg) }
func (a *logrWrapper) Warn(msg string)  { a.l.V(0).Info(msg, "severity", "warn") }
func (a *logrWrapper) Error(msg string) { a.l.Error(nil, msg) }
func (a *logrWrapper) Fatal(msg string) {
	// Don't os.Exit() inside libraries; emit as error with severity
	a.l.Error(nil, msg, "severity", "fatal")
}

func (a *logrWrapper) Debugf(format string, args ...interface{}) {
	a.Debug(fmt.Sprintf(format, args...))
}
func (a *logrWrapper) Infof(format string, args ...interface{}) { a.Info(fmt.Sprintf(format, args...)) }
func (a *logrWrapper) Warnf(format string, args ...interface{}) { a.Warn(fmt.Sprintf(format, args...)) }
func (a *logrWrapper) Errorf(format string, args ...interface{}) {
	a.Error(fmt.Sprintf(format, args...))
}
func (a *logrWrapper) Fatalf(format string, args ...interface{}) {
	a.Fatal(fmt.Sprintf(format, args...))
}
