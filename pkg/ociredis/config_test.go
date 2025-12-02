package ociredis

import (
	"fmt"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
)

type fakeLogger struct {
	fields map[string]interface{}
	err    error
	msgs   []string
}

func (f *fakeLogger) WithField(key string, value interface{}) logging.Interface {
	if f.fields == nil {
		f.fields = map[string]interface{}{}
	}
	f.fields[key] = value
	return f
}
func (f *fakeLogger) WithError(err error) logging.Interface { f.err = err; return f }

func (f *fakeLogger) Debug(msg string) { f.msgs = append(f.msgs, "D:"+msg) }
func (f *fakeLogger) Info(msg string)  { f.msgs = append(f.msgs, "I:"+msg) }
func (f *fakeLogger) Warn(msg string)  { f.msgs = append(f.msgs, "W:"+msg) }
func (f *fakeLogger) Error(msg string) { f.msgs = append(f.msgs, "E:"+msg) }
func (f *fakeLogger) Fatal(msg string) { f.msgs = append(f.msgs, "F:"+msg) }

func (f *fakeLogger) Debugf(format string, args ...interface{}) { f.Debug(sprintf(format, args...)) }
func (f *fakeLogger) Infof(format string, args ...interface{})  { f.Info(sprintf(format, args...)) }
func (f *fakeLogger) Warnf(format string, args ...interface{})  { f.Warn(sprintf(format, args...)) }
func (f *fakeLogger) Errorf(format string, args ...interface{}) { f.Error(sprintf(format, args...)) }
func (f *fakeLogger) Fatalf(format string, args ...interface{}) { f.Fatal(sprintf(format, args...)) }

func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmtSprintf(format, args...)
}

// keep stdlib fmt isolated to avoid import noise at top
func fmtSprintf(format string, a ...interface{}) string {
	return (func() string {
		return fmtSprintfImpl(format, a...)
	})()
}

// tiny shim to actually call fmt.Sprintf without a named import at top
var fmtSprintfImpl = func(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}

//
// ---- tests ----
//

func TestNewConfig_Defaults(t *testing.T) {
	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("NewConfig() returned nil cfg")
	}
	if cfg.AuthType != nil {
		t.Fatalf("expected AuthType=nil by default, got %v", *cfg.AuthType)
	}
	if cfg.AnotherLogger != nil {
		t.Fatalf("expected AnotherLogger=nil by default")
	}
}

func TestNewConfig_WithLogger_SetsField(t *testing.T) {
	l := &fakeLogger{}
	cfg, err := NewConfig(WithAnotherLog(l))
	if err != nil {
		t.Fatalf("NewConfig(WithAnotherLog) error: %v", err)
	}
	if cfg.AnotherLogger != l {
		t.Fatalf("AnotherLogger not set correctly; got %T want %T", cfg.AnotherLogger, l)
	}
}

func TestNewConfig_WithNilLogger_ReturnsError(t *testing.T) {
	_, err := NewConfig(WithAnotherLog(nil))
	if err == nil {
		t.Fatalf("expected error for nil logger, got nil")
	}
	want := "nil another logger"
	if err.Error() != want {
		t.Fatalf("unexpected error: got %q want %q", err.Error(), want)
	}
}

func TestConfig_Apply_MultipleOptions_AndNilAreHandled(t *testing.T) {
	cfg := &Config{}

	// Option to set AuthType
	setAuth := func(a principals.AuthenticationType) Option {
		return func(c *Config) error {
			c.AuthType = &a
			return nil
		}
	}

	// Apply with a nil in the middle
	err := cfg.Apply(
		setAuth(principals.InstancePrincipal),
		nil, // should be ignored
	)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
}
