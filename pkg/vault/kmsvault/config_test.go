package kmsvault

import (
	"testing"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty config",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "config with logger",
			options: []Option{
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "config with nil logger - should not error during creation",
			options: []Option{
				WithAnotherLogger(nil),
			},
			expectError: false, // WithAnotherLogger doesn't validate nil during creation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestConfig_Apply(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name:        "apply empty options",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "apply valid options",
			options: []Option{
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply nil option",
			options: []Option{
				nil,
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			err := config.Apply(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithAnotherLogger(t *testing.T) {
	tests := []struct {
		name        string
		logger      interface{}
		expectError bool
	}{
		{
			name:        "valid logger",
			logger:      testingPkg.SetupMockLogger(),
			expectError: false,
		},
		{
			name:        "nil logger - should not error during option creation",
			logger:      nil,
			expectError: false, // WithAnotherLogger doesn't validate nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			var option Option

			if tt.logger != nil {
				option = WithAnotherLogger(tt.logger.(*testingPkg.MockLogger))
			} else {
				option = WithAnotherLogger(nil)
			}

			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.logger != nil {
					assert.NotNil(t, config.AnotherLogger)
				}
			}
		})
	}
}

func TestWithViper(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectError bool
	}{
		{
			name: "valid viper config",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("region_override", "us-ashburn-1")
				v.Set("enable_obo_token", false)
				return v
			},
			expectError: false,
		},
		{
			name: "viper config with OBO token",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("enable_obo_token", true)
				v.Set("obo_token", "test-obo-token")
				return v
			},
			expectError: false,
		},
		{
			name: "empty viper config",
			setupViper: func() *viper.Viper {
				return viper.New()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()
			config := &Config{}

			option := WithViper(v)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config without OBO token",
			config: &Config{
				AnotherLogger:  testingPkg.SetupMockLogger(),
				AuthType:       &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				EnableOboToken: false,
			},
			expectError: false,
		},
		{
			name: "valid config with OBO token",
			config: &Config{
				AnotherLogger:  testingPkg.SetupMockLogger(),
				AuthType:       &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				EnableOboToken: true,
				OboToken:       "test-obo-token",
			},
			expectError: false,
		},
		{
			name: "missing auth type",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
			},
			expectError: true,
		},
		{
			name: "OBO token enabled but missing token",
			config: &Config{
				AnotherLogger:  testingPkg.SetupMockLogger(),
				AuthType:       &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				EnableOboToken: true,
				OboToken:       "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_OboTokenHandling(t *testing.T) {
	// Test OBO token configuration scenarios
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	t.Run("OBO token disabled", func(t *testing.T) {
		config := &Config{
			AnotherLogger:  mockLogger,
			AuthType:       &authType,
			EnableOboToken: false,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("OBO token enabled with valid token", func(t *testing.T) {
		config := &Config{
			AnotherLogger:  mockLogger,
			AuthType:       &authType,
			EnableOboToken: true,
			OboToken:       "valid-obo-token",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("OBO token enabled with empty token", func(t *testing.T) {
		config := &Config{
			AnotherLogger:  mockLogger,
			AuthType:       &authType,
			EnableOboToken: true,
			OboToken:       "",
		}

		err := config.Validate()
		assert.Error(t, err)
	})
}

func TestConfig_Integration(t *testing.T) {
	// Test complete configuration flow
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Test with viper
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("region_override", "us-ashburn-1")
	v.Set("enable_obo_token", false)

	config, err := NewConfig(
		WithViper(v),
		WithAnotherLogger(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.AnotherLogger)
	assert.Equal(t, authType, *config.AuthType)
	assert.False(t, config.EnableOboToken)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}
