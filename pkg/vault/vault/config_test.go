package oci_vault

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/principals"
	testingPkg "github.com/sgl-project/sgl-ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretInVaultConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name:        "empty config",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "config with logger",
			options: []Option{
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewSecretInVaultConfig(tt.options...)

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
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply nil option",
			options: []Option{
				nil,
				WithAnotherLog(testingPkg.SetupMockLogger()),
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
				v.Set("name", "test-vault")
				v.Set("region_override", "us-ashburn-1")
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

func TestWithAnotherLog(t *testing.T) {
	tests := []struct {
		name        string
		logger      interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid logger",
			logger:      testingPkg.SetupMockLogger(),
			expectError: false,
		},
		{
			name:        "nil logger",
			logger:      nil,
			expectError: true,
			errorMsg:    "nil another logger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			var option Option

			if tt.logger != nil {
				option = WithAnotherLog(tt.logger.(*testingPkg.MockLogger))
			} else {
				option = WithAnotherLog(nil)
			}

			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config.AnotherLogger)
			}
		})
	}
}

func TestWithAppParams(t *testing.T) {
	tests := []struct {
		name        string
		setupParams func() vaultParams
		expectError bool
	}{
		{
			name: "valid params",
			setupParams: func() vaultParams {
				mockLogger := testingPkg.SetupMockLogger()

				return vaultParams{
					AnotherLogger: mockLogger,
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.setupParams()
			config := &Config{}

			option := WithAppParams(params)
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
	}{
		{
			name: "valid config",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-vault",
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "missing auth type",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-vault",
				Region:        "us-ashburn-1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Integration(t *testing.T) {
	// Test complete configuration flow
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Test with viper
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("name", "test-vault")
	v.Set("region_override", "us-ashburn-1")

	config, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.AnotherLogger)
	assert.Equal(t, "test-vault", config.Name)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "us-ashburn-1", config.Region)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}

func TestConfig_Fields(t *testing.T) {
	// Test Config structure
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.InstancePrincipal

	config := &Config{
		AnotherLogger: mockLogger,
		Name:          "production-vault",
		Region:        "eu-frankfurt-1",
		AuthType:      &authType,
	}

	assert.Equal(t, mockLogger, config.AnotherLogger)
	assert.Equal(t, "production-vault", config.Name)
	assert.Equal(t, "eu-frankfurt-1", config.Region)
	assert.Equal(t, principals.InstancePrincipal, *config.AuthType)
}

func TestVaultParams_Fields(t *testing.T) {
	// Test vaultParams structure
	mockLogger := testingPkg.SetupMockLogger()

	params := vaultParams{
		AnotherLogger: mockLogger,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.Equal(t, mockLogger, params.AnotherLogger)
}
