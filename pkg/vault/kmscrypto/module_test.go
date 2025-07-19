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

func TestModule(t *testing.T) {
	// Test that the module can be created without panicking
	assert.NotNil(t, Module)
}

func TestKmsCryptoParams(t *testing.T) {
	// Test the kmsCryptoParams struct
	mockLogger := testingPkg.SetupMockLogger()
	mockVaultClient := &kmsvault.KMSVault{}

	params := kmsCryptoParams{
		AnotherLogger:  mockLogger,
		KmsVaultClient: mockVaultClient,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.NotNil(t, params.KmsVaultClient)
	assert.Equal(t, mockLogger, params.AnotherLogger)
	assert.Equal(t, mockVaultClient, params.KmsVaultClient)
}

func TestModuleProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		setupParams func() kmsCryptoParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("vault_id", "ocid1.vault.oc1.test")
				return v
			},
			setupParams: func() kmsCryptoParams {
				mockLogger := testingPkg.SetupMockLogger()
				mockVaultClient := &kmsvault.KMSVault{}

				return kmsCryptoParams{
					AnotherLogger:  mockLogger,
					KmsVaultClient: mockVaultClient,
				}
			},
			expectError: false, // Test without calling WithAppParams
		},
		{
			name: "invalid viper configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Missing required auth_type
				return v
			},
			setupParams: func() kmsCryptoParams {
				mockLogger := testingPkg.SetupMockLogger()
				mockVaultClient := &kmsvault.KMSVault{}

				return kmsCryptoParams{
					AnotherLogger:  mockLogger,
					KmsVaultClient: mockVaultClient,
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
			// Don't call WithAppParams to avoid the mock vault client issue
			config, err := NewConfig(
				WithViper(v, params.AnotherLogger),
				WithAnotherLog(params.AnotherLogger),
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

				// Test that the params structure is correct
				assert.NotNil(t, params.AnotherLogger)
				assert.NotNil(t, params.KmsVaultClient)
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
	v.Set("vault_id", "ocid1.vault.oc1.test")

	// Test configuration creation
	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "ocid1.vault.oc1.test", config.VaultId)

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

	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAnotherLog(mockLogger),
	)

	// Should succeed in creation but fail validation
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Validation should fail due to missing auth_type
	err = config.Validate()
	assert.Error(t, err)
}
