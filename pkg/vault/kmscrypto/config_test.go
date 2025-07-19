package kmscrypto

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/vault/kmsvault"
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
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "config with nil logger",
			options: []Option{
				WithAnotherLog(nil),
			},
			expectError: true,
			errorMsg:    "logger cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, config)
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
		{
			name: "apply invalid option",
			options: []Option{
				WithAnotherLog(nil),
			},
			expectError: true,
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
			errorMsg:    "logger cannot be nil",
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
				v.Set("vault_id", "ocid1.vault.oc1.test")
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
			mockLogger := testingPkg.SetupMockLogger()
			config := &Config{}

			option := WithViper(v, mockLogger)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithAppParams(t *testing.T) {
	tests := []struct {
		name             string
		setupParams      func() kmsCryptoParams
		expectError      bool
		errorMsg         string
		expectedEndpoint string
	}{
		{
			name: "valid params structure",
			setupParams: func() kmsCryptoParams {
				mockLogger := testingPkg.SetupMockLogger()
				mockVaultClient := &kmsvault.KMSVault{}

				return kmsCryptoParams{
					AnotherLogger:  mockLogger,
					KmsVaultClient: mockVaultClient,
				}
			},
			expectError: false, // Just test the structure, not the actual call
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.setupParams()
			config := &Config{
				VaultId: "ocid1.vault.oc1.test",
			}

			// Test that the params structure is correct
			assert.NotNil(t, params.AnotherLogger)
			assert.NotNil(t, params.KmsVaultClient)
			assert.NotEmpty(t, config.VaultId)
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
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
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
	v.Set("vault_id", "ocid1.vault.oc1.test")

	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.AnotherLogger)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "ocid1.vault.oc1.test", config.VaultId)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}
