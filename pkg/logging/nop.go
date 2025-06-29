package logging

// NopLogger is a no-op implementation of the logging interface
type NopLogger struct{}

// NewNopLogger creates a new no-op logger
func NewNopLogger() Interface {
	return &NopLogger{}
}

// WithField returns itself (no-op)
func (n *NopLogger) WithField(key string, value interface{}) Interface {
	return n
}

// WithError returns itself (no-op)
func (n *NopLogger) WithError(err error) Interface {
	return n
}

// Debug does nothing
func (n *NopLogger) Debug(msg string) {}

// Info does nothing
func (n *NopLogger) Info(msg string) {}

// Warn does nothing
func (n *NopLogger) Warn(msg string) {}

// Error does nothing
func (n *NopLogger) Error(msg string) {}

// Fatal does nothing (note: in production this should actually exit)
func (n *NopLogger) Fatal(msg string) {}

// Debugf does nothing
func (n *NopLogger) Debugf(format string, args ...interface{}) {}

// Infof does nothing
func (n *NopLogger) Infof(format string, args ...interface{}) {}

// Warnf does nothing
func (n *NopLogger) Warnf(format string, args ...interface{}) {}

// Errorf does nothing
func (n *NopLogger) Errorf(format string, args ...interface{}) {}

// Fatalf does nothing (note: in production this should actually exit)
func (n *NopLogger) Fatalf(format string, args ...interface{}) {}
