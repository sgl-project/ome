package oci_vault

import (
	"testing"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	// Test that the module can be created without panicking
	assert.NotNil(t, Module)
}

func TestVaultParams(t *testing.T) {
	// Test the vaultParams struct
	mockLogger := testingPkg.SetupMockLogger()

	params := vaultParams{
		AnotherLogger: mockLogger,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.Equal(t, mockLogger, params.AnotherLogger)
}

func TestModuleProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		setupParams func() vaultParams
		expectError bool
	}{
		{
			name: "valid configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("name", "test-vault")
				v.Set("region_override", "us-ashburn-1")
				return v
			},
			setupParams: func() vaultParams {
				mockLogger := testingPkg.SetupMockLogger()

				return vaultParams{
					AnotherLogger: mockLogger,
				}
			},
			expectError: false,
		},
		{
			name: "invalid viper configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Missing required auth_type
				return v
			},
			setupParams: func() vaultParams {
				mockLogger := testingPkg.SetupMockLogger()

				return vaultParams{
					AnotherLogger: mockLogger,
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()
			params := tt.setupParams()

			// Test the provider function logic (without actual fx execution)
			config, err := NewSecretInVaultConfig(
				WithViper(v),
				WithAnotherLog(params.AnotherLogger),
				WithAppParams(params),
			)

			if tt.expectError {
				// For missing auth_type, config creation succeeds but validation fails
				if err == nil {
					err = config.Validate()
				}
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestModuleIntegration(t *testing.T) {
	// Test module integration with valid configuration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Setup viper with valid configuration
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("name", "test-vault")
	v.Set("region_override", "us-ashburn-1")

	// Test configuration creation
	config, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "test-vault", config.Name)
	assert.Equal(t, "us-ashburn-1", config.Region)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}

func TestModuleErrorHandling(t *testing.T) {
	// Test error handling in module provider
	mockLogger := testingPkg.SetupMockLogger()

	// Test with invalid configuration
	v := viper.New()
	// Don't set required fields

	config, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	// Should succeed in creation but fail validation
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Validation should fail due to missing auth_type
	err = config.Validate()
	assert.Error(t, err)
}

func TestModuleConfigurationOptions(t *testing.T) {
	// Test various configuration options
	mockLogger := testingPkg.SetupMockLogger()

	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectValid bool
	}{
		{
			name: "minimal valid config",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				return v
			},
			expectValid: true,
		},
		{
			name: "config with name and region",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "InstancePrincipal")
				v.Set("name", "production-vault")
				v.Set("region_override", "eu-frankfurt-1")
				return v
			},
			expectValid: true,
		},
		{
			name: "config with resource principal",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "ResourcePrincipal")
				v.Set("name", "resource-vault")
				return v
			},
			expectValid: true,
		},
		{
			name: "config missing auth type",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("name", "test-vault")
				// auth_type not set
				return v
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()

			config, err := NewSecretInVaultConfig(
				WithViper(v),
				WithAnotherLog(mockLogger),
			)

			require.NoError(t, err)
			assert.NotNil(t, config)

			err = config.Validate()
			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
