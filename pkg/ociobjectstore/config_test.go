package ociobjectstore

import (
	"fmt"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLogger is a mock implementation of logging.Interface for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) WithField(key string, value interface{}) logging.Interface {
	args := m.Called(key, value)
	return args.Get(0).(logging.Interface)
}

func (m *MockLogger) WithError(err error) logging.Interface {
	args := m.Called(err)
	return args.Get(0).(logging.Interface)
}

func (m *MockLogger) Debug(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Info(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warn(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Error(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatal(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) With(args ...interface{}) logging.Interface {
	m.Called(args)
	return m
}

func (m *MockLogger) Named(name string) logging.Interface {
	m.Called(name)
	return m
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// Create a helper to create a valid config for testing
func createValidConfig() *Config {
	authType := principals.AuthenticationType("InstancePrincipal")
	mockLogger := new(MockLogger)
	// Set up necessary mock expectations
	mockLogger.On("WithField", mock.Anything, mock.Anything).Return(mockLogger)
	mockLogger.On("WithError", mock.Anything).Return(mockLogger)

	return &Config{
		AnotherLogger:  mockLogger,
		Name:           "test-config",
		AuthType:       &authType,
		CompartmentId:  common.String("ocid1.compartment.oc1..example"),
		Region:         "us-ashburn-1",
		EnableOboToken: false,
		OboToken:       "",
	}
}

// TestConfig_Apply tests the Apply method with various options
func TestConfig_Apply(t *testing.T) {
	tests := []struct {
		name          string
		initialConfig *Config
		options       []Option
		expectError   bool
		errorContains string
		validateFunc  func(*testing.T, *Config)
	}{
		{
			name:          "Apply empty options",
			initialConfig: &Config{},
			options:       []Option{},
			expectError:   false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.NotNil(t, c)
				assert.Empty(t, c.Name)
				assert.Nil(t, c.AuthType)
			},
		},
		{
			name:          "Apply nil option",
			initialConfig: &Config{},
			options:       []Option{nil},
			expectError:   false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.NotNil(t, c)
				assert.Empty(t, c.Name)
				assert.Nil(t, c.AuthType)
			},
		},
		{
			name:          "Apply option with error",
			initialConfig: &Config{},
			options: []Option{
				func(c *Config) error {
					return fmt.Errorf("test error")
				},
			},
			expectError:   true,
			errorContains: "test error",
		},
		{
			name:          "Apply multiple successful options",
			initialConfig: &Config{},
			options: []Option{
				func(c *Config) error {
					c.Name = "option1"
					return nil
				},
				func(c *Config) error {
					c.Name = c.Name + "-option2"
					authType := principals.AuthenticationType("InstancePrincipal")
					c.AuthType = &authType
					return nil
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "option1-option2", c.Name)
				require.NotNil(t, c.AuthType)
				assert.Equal(t, principals.AuthenticationType("InstancePrincipal"), *c.AuthType)
			},
		},
		{
			name:          "Apply stops at first error",
			initialConfig: &Config{},
			options: []Option{
				func(c *Config) error {
					c.Name = "option1"
					return nil
				},
				func(c *Config) error {
					return fmt.Errorf("error in second option")
				},
				func(c *Config) error {
					c.Name = "option3" // This should not be applied
					return nil
				},
			},
			expectError:   true,
			errorContains: "error in second option",
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "option1", c.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.initialConfig
			err := c.Apply(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, c)
			}
		})
	}
}

// TestNewConfig tests the NewConfig function
func TestNewConfig(t *testing.T) {
	tests := []struct {
		name          string
		options       []Option
		expectError   bool
		errorContains string
		validateFunc  func(*testing.T, *Config)
	}{
		{
			name:        "Empty config",
			options:     []Option{},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.NotNil(t, c)
				assert.Empty(t, c.Name)
				assert.Nil(t, c.AuthType)
			},
		},
		{
			name: "Config with options",
			options: []Option{
				func(c *Config) error {
					c.Name = "test-config"
					authType := principals.AuthenticationType("InstancePrincipal")
					c.AuthType = &authType
					return nil
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "test-config", c.Name)
				require.NotNil(t, c.AuthType)
				assert.Equal(t, principals.AuthenticationType("InstancePrincipal"), *c.AuthType)
			},
		},
		{
			name: "Config with error in options",
			options: []Option{
				func(c *Config) error {
					return fmt.Errorf("option error")
				},
			},
			expectError:   true,
			errorContains: "option error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, c)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c)
				if tt.validateFunc != nil {
					tt.validateFunc(t, c)
				}
			}
		})
	}
}

// TestWithViper tests the WithViper option
func TestWithViper(t *testing.T) {
	tests := []struct {
		name          string
		viperSetup    func(*viper.Viper)
		expectError   bool
		errorContains string
		validateFunc  func(*testing.T, *Config)
	}{
		{
			name: "Basic viper config",
			viperSetup: func(v *viper.Viper) {
				v.Set(NameViperKeyName, "viper-config")
				v.Set(CompartmentIdViperKeyName, "ocid1.compartment.oc1..viper")
				v.Set(RegionViperKeyName, "us-phoenix-1")
				v.Set(EnableOboTokenViperKeyName, false)
				v.Set(OboTokenViperKeyName, "")
				v.Set(AuthTypeViperKeyName, "ResourcePrincipal")
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "viper-config", c.Name)
				assert.Equal(t, "ocid1.compartment.oc1..viper", *c.CompartmentId)
				assert.Equal(t, "us-phoenix-1", c.Region)
				assert.False(t, c.EnableOboToken)
				assert.Empty(t, c.OboToken)
				require.NotNil(t, c.AuthType)
				assert.Equal(t, principals.AuthenticationType("ResourcePrincipal"), *c.AuthType)
			},
		},
		{
			name: "Viper with OBO token enabled",
			viperSetup: func(v *viper.Viper) {
				v.Set(NameViperKeyName, "viper-obo-config")
				v.Set(CompartmentIdViperKeyName, "ocid1.compartment.oc1..viper")
				v.Set(RegionViperKeyName, "us-phoenix-1")
				v.Set(EnableOboTokenViperKeyName, true)
				v.Set(OboTokenViperKeyName, "test-obo-token")
				v.Set(AuthTypeViperKeyName, "ResourcePrincipal")
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "viper-obo-config", c.Name)
				assert.True(t, c.EnableOboToken)
				assert.Equal(t, "test-obo-token", c.OboToken)
			},
		},
		{
			name: "Viper with invalid auth type",
			viperSetup: func(v *viper.Viper) {
				v.Set(NameViperKeyName, "viper-invalid-auth")
				v.Set(CompartmentIdViperKeyName, "ocid1.compartment.oc1..viper")
				v.Set(RegionViperKeyName, "us-phoenix-1")
				// Set an invalid auth type as a complex value that should fail to unmarshal
				v.Set(AuthTypeViperKeyName, map[string]interface{}{"invalid": "structure"})
			},
			expectError:   true,
			errorContains: "error occurred when unmarshalling auth_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			if tt.viperSetup != nil {
				tt.viperSetup(v)
			}

			c := &Config{}
			option := WithViper(v)
			err := option(c)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, c)
				}
			}
		})
	}
}

// TestWithAnotherLog tests the WithAnotherLog option
func TestWithAnotherLog(t *testing.T) {
	tests := []struct {
		name          string
		logger        logging.Interface
		expectError   bool
		errorContains string
	}{
		{
			name:          "Nil logger",
			logger:        nil,
			expectError:   true,
			errorContains: "nil another logger",
		},
		{
			name:        "Valid logger",
			logger:      new(MockLogger),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			option := WithAnotherLog(tt.logger)
			err := option(c)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.logger, c.AnotherLogger)
			}
		})
	}
}

// TestWithName tests the WithName option
func TestWithName(t *testing.T) {
	tests := []struct {
		testName      string
		name          string
		expectError   bool
		errorContains string
	}{
		{
			testName:      "empty name",
			name:          "",
			expectError:   true,
			errorContains: "name cannot be empty",
		},
		{
			testName:    "valid name",
			name:        "Valid name",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			c := &Config{}
			option := WithName(tt.name)
			err := option(c)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.name, c.Name)
			}
		})
	}
}

// TestConfig_Validate tests the Validate method
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectError   bool
		errorContains string
	}{
		{
			name:        "Valid config",
			config:      createValidConfig(),
			expectError: false,
		},
		{
			name: "Missing required auth type",
			config: &Config{
				Name:           "missing-auth",
				AuthType:       nil,
				CompartmentId:  common.String("ocid1.compartment.oc1..example"),
				Region:         "us-ashburn-1",
				EnableOboToken: false,
				OboToken:       "",
			},
			expectError:   true,
			errorContains: "required",
		},
		{
			name: "Missing OBO token when enabled",
			config: &Config{
				Name: "missing-obo-token",
				AuthType: func() *principals.AuthenticationType {
					a := principals.AuthenticationType("InstancePrincipal")
					return &a
				}(),
				CompartmentId:  common.String("ocid1.compartment.oc1..example"),
				Region:         "us-ashburn-1",
				EnableOboToken: true,
				OboToken:       "", // Empty but required when EnableOboToken is true
			},
			expectError:   true,
			errorContains: "required_if",
		},
		{
			name: "Valid config with OBO token",
			config: &Config{
				Name: "valid-obo",
				AuthType: func() *principals.AuthenticationType {
					a := principals.AuthenticationType("InstancePrincipal")
					return &a
				}(),
				CompartmentId:  common.String("ocid1.compartment.oc1..example"),
				Region:         "us-ashburn-1",
				EnableOboToken: true,
				OboToken:       "valid-token",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfigIntegration tests the integration of all config components
func TestConfigIntegration(t *testing.T) {
	// Create a mock logger without expectations - we just need a valid logger instance
	mockLogger := new(MockLogger)

	// Create a viper instance
	v := viper.New()
	v.Set(NameViperKeyName, "integration-config")
	v.Set(CompartmentIdViperKeyName, "ocid1.compartment.oc1..integration")
	v.Set(RegionViperKeyName, "us-ashburn-1")
	v.Set(EnableOboTokenViperKeyName, false)
	v.Set(OboTokenViperKeyName, "")
	v.Set(AuthTypeViperKeyName, "InstancePrincipal")

	// Create config with all options
	c, err := NewConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	// Verify the integrated config
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "integration-config", c.Name)
	assert.Equal(t, "ocid1.compartment.oc1..integration", *c.CompartmentId)
	assert.Equal(t, "us-ashburn-1", c.Region)
	assert.False(t, c.EnableOboToken)
	require.NotNil(t, c.AuthType)
	assert.Equal(t, principals.AuthenticationType("InstancePrincipal"), *c.AuthType)
	assert.Equal(t, mockLogger, c.AnotherLogger)

	// Verify it passes validation
	err = c.Validate()
	assert.NoError(t, err)
}
