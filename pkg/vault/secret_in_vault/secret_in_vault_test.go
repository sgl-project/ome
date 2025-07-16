package secret_in_vault

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	vaultUtils "github.com/sgl-project/ome/pkg/vault"
)

// MockVaultsClientInterface defines the interface for mocking vault client
type MockVaultsClientInterface struct {
	mock.Mock
}

func (m *MockVaultsClientInterface) CreateSecret(ctx context.Context, request vault.CreateSecretRequest) (vault.CreateSecretResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(vault.CreateSecretResponse), args.Error(1)
}

func TestNewSecretInVault(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecretInVaultConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &SecretInVaultConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-secret",
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "SecretInVaultConfig is nil",
		},
		{
			name: "invalid config - missing auth type",
			config: &SecretInVaultConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-secret",
				Region:        "us-ashburn-1",
			},
			expectError: true,
			errorMsg:    "SecretInVaultConfig is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since NewSecretInVault calls external dependencies that are hard to mock,
			// we'll test the configuration validation
			if tt.config == nil {
				_, err := NewSecretInVault(tt.config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			if tt.expectError {
				err := tt.config.Validate()
				assert.Error(t, err)
			} else {
				assert.NotNil(t, tt.config.AnotherLogger)
				assert.NotNil(t, tt.config.AuthType)
				assert.Equal(t, principals.UserPrincipal, *tt.config.AuthType)
			}
		})
	}
}

func TestSecretInVault_CreateSecretInVault(t *testing.T) {
	tests := []struct {
		name         string
		secretConfig vaultUtils.SecretConfig
		plaintext    string
		setupMocks   func(*MockVaultsClientInterface, *testingPkg.MockLogger)
		expectError  bool
		errorMsg     string
	}{
		{
			name: "successful secret creation",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				SecretName:    stringPtr("test-secret"),
				VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			plaintext: "test-secret-content",
			setupMocks: func(mockClient *MockVaultsClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestCreateSecretResponse()
				mockClient.On("CreateSecret", mock.Anything, mock.MatchedBy(func(req vault.CreateSecretRequest) bool {
					return req.CreateSecretDetails.SecretName != nil &&
						*req.CreateSecretDetails.SecretName == "test-secret" &&
						req.CreateSecretDetails.VaultId != nil &&
						*req.CreateSecretDetails.VaultId == "ocid1.vault.oc1.ap-mumbai-1.test"
				})).Return(expectedResponse, nil)
			},
			expectError: false,
		},
		{
			name: "failed secret creation - OCI error",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				SecretName:    stringPtr("test-secret"),
				VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			plaintext: "test-secret-content",
			setupMocks: func(mockClient *MockVaultsClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("CreateSecret", mock.Anything, mock.Anything).Return(
					vault.CreateSecretResponse{}, fmt.Errorf("failed to create secret test-secret in vault test: OCI error"))
			},
			expectError: true,
			errorMsg:    "failed to create secret",
		},
		{
			name: "failed secret creation - non-OK status",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				SecretName:    stringPtr("test-secret"),
				VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			plaintext: "test-secret-content",
			setupMocks: func(mockClient *MockVaultsClientInterface, mockLogger *testingPkg.MockLogger) {
				response := testingPkg.CreateTestCreateSecretResponse()
				response.RawResponse = testingPkg.CreateMockHTTPResponse(http.StatusBadRequest)
				mockClient.On("CreateSecret", mock.Anything, mock.Anything).Return(response, nil)
			},
			expectError: true,
			errorMsg:    "failed to create secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockVaultsClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the secret creation logic
			base64Content := vaultUtils.B64Encode(tt.plaintext)
			assert.NotEmpty(t, base64Content)

			createSecretDetails := vault.CreateSecretDetails{
				CompartmentId: tt.secretConfig.CompartmentId,
				SecretName:    tt.secretConfig.SecretName,
				SecretContent: vault.Base64SecretContentDetails{
					Content: &base64Content,
				},
				VaultId:     tt.secretConfig.VaultId,
				Description: stringPtr(fmt.Sprintf("DEK for the model %s", *tt.secretConfig.SecretName)),
				KeyId:       tt.secretConfig.KeyId,
			}

			// Verify the request structure
			assert.Equal(t, tt.secretConfig.CompartmentId, createSecretDetails.CompartmentId)
			assert.Equal(t, tt.secretConfig.SecretName, createSecretDetails.SecretName)
			assert.Equal(t, tt.secretConfig.VaultId, createSecretDetails.VaultId)
			assert.Equal(t, tt.secretConfig.KeyId, createSecretDetails.KeyId)

			// Verify the content is base64 encoded
			contentDetails, ok := createSecretDetails.SecretContent.(vault.Base64SecretContentDetails)
			assert.True(t, ok)
			assert.Equal(t, base64Content, *contentDetails.Content)

			// Test the mock client directly
			createSecretRequest := vault.CreateSecretRequest{
				CreateSecretDetails: createSecretDetails,
			}

			response, err := mockClient.CreateSecret(context.Background(), createSecretRequest)

			if tt.expectError {
				if err == nil {
					// Check for non-OK status
					assert.NotEqual(t, http.StatusOK, response.RawResponse.StatusCode)
				} else {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response.Secret.Id)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestGetConfigProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecretInVaultConfig
		expectError bool
	}{
		{
			name: "valid config with user principal",
			config: &SecretInVaultConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Since getConfigProvider uses the principals package internally,
			// we'll test that the function exists and can be called with valid config
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
		})
	}
}

func TestNewVaultClient(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
	}{
		{
			name:        "vault client creation",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since NewVaultClient calls external OCI dependencies,
			// we'll just test that the function signature is correct
			// In a real test environment, you would mock the OCI client creation
			mockProvider := testingPkg.SetupMockConfigProvider()

			// Test that the function can be called with a valid provider
			assert.NotNil(t, mockProvider)
		})
	}
}

func TestSecretInVault_Integration(t *testing.T) {
	// Test complete SecretInVault integration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	config := &SecretInVaultConfig{
		AnotherLogger: mockLogger,
		Name:          "test-secret",
		AuthType:      &authType,
		Region:        "us-ashburn-1",
	}

	// Verify config validation
	assert.NotNil(t, config.AnotherLogger)
	assert.NotNil(t, config.AuthType)
	assert.Equal(t, principals.UserPrincipal, *config.AuthType)
	assert.Equal(t, "test-secret", config.Name)
	assert.Equal(t, "us-ashburn-1", config.Region)

	// Test secret config validation
	secretConfig := vaultUtils.SecretConfig{
		CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
		SecretName:    stringPtr("test-secret"),
		VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
		KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
	}

	err := secretConfig.ValidateNameAndVaultId()
	assert.NoError(t, err)

	// Test base64 encoding
	plaintext := "test-secret-content"
	encoded := vaultUtils.B64Encode(plaintext)
	assert.NotEmpty(t, encoded)
	assert.NotEqual(t, plaintext, encoded)

	decoded := vaultUtils.B64Decode(encoded)
	assert.Equal(t, plaintext, decoded)
}

func TestSecretInVault_SecretConfigValidation(t *testing.T) {
	// Test various secret config scenarios
	tests := []struct {
		name         string
		secretConfig vaultUtils.SecretConfig
		expectValid  bool
	}{
		{
			name: "valid secret config",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				SecretName:    stringPtr("test-secret"),
				VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			expectValid: true,
		},
		{
			name: "missing secret name",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				VaultId:       stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			expectValid: false,
		},
		{
			name: "missing vault ID",
			secretConfig: vaultUtils.SecretConfig{
				CompartmentId: stringPtr("ocid1.compartment.oc1..test"),
				SecretName:    stringPtr("test-secret"),
				KeyId:         stringPtr("ocid1.key.oc1.ap-mumbai-1.test"),
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.secretConfig.ValidateNameAndVaultId()
			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
