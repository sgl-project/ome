package logging

type discard struct {
}

func (d discard) WithField(key string, value interface{}) Interface { return d }
func (d discard) WithError(err error) Interface                     { return d }
func (d discard) Debug(msg string)                                  {}
func (d discard) Info(msg string)                                   {}
func (d discard) Warn(msg string)                                   {}
func (d discard) Error(msg string)                                  {}
func (d discard) Fatal(msg string)                                  {}
func (d discard) Debugf(format string, args ...interface{})         {}
func (d discard) Infof(format string, args ...interface{})          {}
func (d discard) Warnf(format string, args ...interface{})          {}
func (d discard) Errorf(format string, args ...interface{})         {}
func (d discard) Fatalf(format string, args ...interface{})         {}

// Discard constructs a logger that discards any logging message.
//
// Used as a null-object pattern.
func Discard() Interface {
	return discard{}
}
