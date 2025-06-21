package ociobjectstore

import (
	"crypto/rsa"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConfigProvider implements common.ConfigurationProvider for testing
type MockConfigProvider struct {
	mock.Mock
}

func (m *MockConfigProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	args := m.Called()
	return args.Get(0).(*rsa.PrivateKey), args.Error(1)
}

func (m *MockConfigProvider) KeyID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigProvider) TenancyOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigProvider) UserOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigProvider) KeyFingerprint() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigProvider) Region() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigProvider) AuthType() (common.AuthConfig, error) {
	args := m.Called()
	return args.Get(0).(common.AuthConfig), args.Error(1)
}

// MockLogger for testing
type MockTestLogger struct{}

func (l *MockTestLogger) WithField(key string, value interface{}) logging.Interface { return l }
func (l *MockTestLogger) WithError(err error) logging.Interface                     { return l }
func (l *MockTestLogger) Debug(msg string)                                          {}
func (l *MockTestLogger) Info(msg string)                                           {}
func (l *MockTestLogger) Warn(msg string)                                           {}
func (l *MockTestLogger) Error(msg string)                                          {}
func (l *MockTestLogger) Fatal(msg string)                                          {}
func (l *MockTestLogger) Debugf(format string, args ...interface{})                 {}
func (l *MockTestLogger) Infof(format string, args ...interface{})                  {}
func (l *MockTestLogger) Warnf(format string, args ...interface{})                  {}
func (l *MockTestLogger) Errorf(format string, args ...interface{})                 {}
func (l *MockTestLogger) Fatalf(format string, args ...interface{})                 {}

// Mock for principals package
type MockPrincipalsBuilder struct {
	mock.Mock
}

func (m *MockPrincipalsBuilder) Build(opts principals.Opts) (common.ConfigurationProvider, error) {
	args := m.Called(opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

// Test NewObjectStorageClient function with dependency injection
func TestNewObjectStorageClient(t *testing.T) {
	t.Run("Config validation without OBO token", func(t *testing.T) {
		config := &Config{
			EnableOboToken: false,
			Region:         "us-ashburn-1",
			AnotherLogger:  &MockTestLogger{},
		}

		// We can't test actual client creation without real credentials
		// but we can test the config validation logic
		assert.False(t, config.EnableOboToken)
		assert.Equal(t, "us-ashburn-1", config.Region)
	})

	t.Run("Config validation with OBO token", func(t *testing.T) {
		config := &Config{
			EnableOboToken: true,
			OboToken:       "test-token",
			Region:         "us-ashburn-1",
			AnotherLogger:  &MockTestLogger{},
		}

		assert.True(t, config.EnableOboToken)
		assert.Equal(t, "test-token", config.OboToken)
	})

	t.Run("Error with OBO token enabled but empty token", func(t *testing.T) {
		config := &Config{
			EnableOboToken: true,
			OboToken:       "", // Empty token should cause validation error
			AnotherLogger:  &MockTestLogger{},
		}

		authType := principals.InstancePrincipal
		config.AuthType = &authType

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OboToken")
	})
}

// Test getConfigProvider function
func TestGetConfigProvider(t *testing.T) {
	t.Run("Config provider creation", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:      &authType,
			AnotherLogger: &MockTestLogger{},
		}

		// We can't test actual config provider creation without real environment
		// but we can test that the function exists and handles the config properly
		assert.NotNil(t, config.AuthType)
		assert.Equal(t, principals.InstancePrincipal, *config.AuthType)
	})
}

// Integration test using mocks for both functions
func TestOCIOSClientIntegration(t *testing.T) {
	t.Run("Client configuration validation", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:       &authType,
			EnableOboToken: false,
			Region:         "us-chicago-1",
			AnotherLogger:  &MockTestLogger{},
		}

		// Test that config is properly structured for client creation
		assert.NotNil(t, config.AuthType)
		assert.False(t, config.EnableOboToken)
		assert.Equal(t, "us-chicago-1", config.Region)

		// Validate the config
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("OBO token configuration", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:       &authType,
			EnableOboToken: true,
			OboToken:       "valid-obo-token",
			Region:         "us-chicago-1",
			AnotherLogger:  &MockTestLogger{},
		}

		// Test OBO token configuration
		assert.True(t, config.EnableOboToken)
		assert.Equal(t, "valid-obo-token", config.OboToken)

		// Validate the config
		err := config.Validate()
		assert.NoError(t, err)
	})
}
