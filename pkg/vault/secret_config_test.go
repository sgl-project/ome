package vault

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretConfig_ValidateNameAndVaultId(t *testing.T) {
	tests := []struct {
		name         string
		secretConfig SecretConfig
		expectError  bool
		errorMsg     string
	}{
		{
			name: "valid config with name and vault ID",
			secretConfig: SecretConfig{
				SecretName: stringPtr("test-secret"),
				VaultId:    stringPtr("ocid1.vault.oc1.test"),
			},
			expectError: false,
		},
		{
			name: "missing secret name",
			secretConfig: SecretConfig{
				VaultId: stringPtr("ocid1.vault.oc1.test"),
			},
			expectError: true,
			errorMsg:    "SecretName and VaultId must be provided",
		},
		{
			name: "missing vault ID",
			secretConfig: SecretConfig{
				SecretName: stringPtr("test-secret"),
			},
			expectError: true,
			errorMsg:    "SecretName and VaultId must be provided",
		},
		{
			name: "empty secret name",
			secretConfig: SecretConfig{
				SecretName: stringPtr(""),
				VaultId:    stringPtr("ocid1.vault.oc1.test"),
			},
			expectError: true,
			errorMsg:    "SecretName and VaultId must be provided",
		},
		{
			name: "empty vault ID",
			secretConfig: SecretConfig{
				SecretName: stringPtr("test-secret"),
				VaultId:    stringPtr(""),
			},
			expectError: true,
			errorMsg:    "SecretName and VaultId must be provided",
		},
		{
			name:         "both missing",
			secretConfig: SecretConfig{},
			expectError:  true,
			errorMsg:     "SecretName and VaultId must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.secretConfig.ValidateNameAndVaultId()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecretConfig_ValidateSecretId(t *testing.T) {
	tests := []struct {
		name         string
		secretConfig SecretConfig
		expectError  bool
		errorMsg     string
	}{
		{
			name: "valid config with secret ID",
			secretConfig: SecretConfig{
				SecretId: stringPtr("ocid1.secret.oc1.test"),
			},
			expectError: false,
		},
		{
			name:         "missing secret ID",
			secretConfig: SecretConfig{},
			expectError:  true,
			errorMsg:     "SecretId must be provided",
		},
		{
			name: "empty secret ID",
			secretConfig: SecretConfig{
				SecretId: stringPtr(""),
			},
			expectError: true,
			errorMsg:    "SecretId must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.secretConfig.ValidateSecretId()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecretVersionStage_Constants(t *testing.T) {
	// Test that the constants are defined correctly
	assert.Equal(t, SecretVersionStage("CURRENT"), CurrentStage)
	assert.Equal(t, SecretVersionStage("PENDING"), PendingStage)
	assert.Equal(t, SecretVersionStage("LATEST"), LatestStage)
	assert.Equal(t, SecretVersionStage("PREVIOUS"), PreviousStage)
	assert.Equal(t, SecretVersionStage("DEPRECATED"), DeprecatedStage)
}

func TestSecretVersionConfig_Fields(t *testing.T) {
	versionNumber := int64(1)
	versionName := "test-version"
	stage := CurrentStage

	config := SecretVersionConfig{
		Stage:               &stage,
		SecretVersionNumber: &versionNumber,
		SecretVersionName:   &versionName,
	}

	assert.Equal(t, &stage, config.Stage)
	assert.Equal(t, &versionNumber, config.SecretVersionNumber)
	assert.Equal(t, &versionName, config.SecretVersionName)
}

func TestSecretConfig_CompleteConfig(t *testing.T) {
	compartmentId := "ocid1.compartment.oc1.test"
	secretId := "ocid1.secret.oc1.test"
	secretName := "test-secret"
	vaultId := "ocid1.vault.oc1.test"
	keyId := "ocid1.key.oc1.test"

	versionNumber := int64(1)
	versionName := "test-version"
	stage := CurrentStage

	config := SecretConfig{
		CompartmentId: &compartmentId,
		SecretId:      &secretId,
		SecretName:    &secretName,
		VaultId:       &vaultId,
		KeyId:         &keyId,
		SecretVersionConfig: &SecretVersionConfig{
			Stage:               &stage,
			SecretVersionNumber: &versionNumber,
			SecretVersionName:   &versionName,
		},
	}

	// Test that all fields are set correctly
	assert.Equal(t, &compartmentId, config.CompartmentId)
	assert.Equal(t, &secretId, config.SecretId)
	assert.Equal(t, &secretName, config.SecretName)
	assert.Equal(t, &vaultId, config.VaultId)
	assert.Equal(t, &keyId, config.KeyId)
	assert.NotNil(t, config.SecretVersionConfig)
	assert.Equal(t, &stage, config.SecretVersionConfig.Stage)
	assert.Equal(t, &versionNumber, config.SecretVersionConfig.SecretVersionNumber)
	assert.Equal(t, &versionName, config.SecretVersionConfig.SecretVersionName)

	// Test validations pass
	assert.NoError(t, config.ValidateNameAndVaultId())
	assert.NoError(t, config.ValidateSecretId())
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
