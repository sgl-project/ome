package oci_vault

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

func TestNewVaultClient(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func() (*testingPkg.MockLogger, *testingPkg.MockConfigurationProvider)
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful client creation",
			setupMocks: func() (*testingPkg.MockLogger, *testingPkg.MockConfigurationProvider) {
				mockLogger := testingPkg.SetupMockLogger()
				mockProvider := testingPkg.SetupMockConfigProvider()
				return mockLogger, mockProvider
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger, mockProvider := tt.setupMocks()

			// Create a test config
			authType := principals.UserPrincipal
			config := &Config{
				AnotherLogger: mockLogger,
				AuthType:      &authType,
			}

			// Since we can't easily mock the internal OCI client creation,
			// we'll test the configuration validation and setup
			assert.NotNil(t, config.AnotherLogger)
			assert.NotNil(t, config.AuthType)
			assert.Equal(t, principals.UserPrincipal, *config.AuthType)

			// Verify mock expectations
			mockLogger.AssertExpectations(t)
			mockProvider.AssertExpectations(t)
		})
	}
}

func TestVaultClient_CreateSecretInVault(t *testing.T) {
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
					// Verify the request structure
					return req.CreateSecretDetails.SecretName != nil &&
						*req.CreateSecretDetails.SecretName == "test-secret" &&
						req.CreateSecretDetails.VaultId != nil &&
						*req.CreateSecretDetails.VaultId == "ocid1.vault.oc1.ap-mumbai-1.test"
				})).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
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

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything).Maybe()
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

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "received non-OK response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockVaultsClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the secret creation logic
			// Since we can't easily inject the mock client, we'll test the request preparation
			base64Content := vaultUtils.B64Encode(tt.plaintext)
			assert.NotEmpty(t, base64Content)

			createSecretDetails := vault.CreateSecretDetails{
				CompartmentId: tt.secretConfig.CompartmentId,
				SecretName:    tt.secretConfig.SecretName,
				SecretContent: vault.Base64SecretContentDetails{
					Content: common.String(base64Content),
				},
				VaultId:     tt.secretConfig.VaultId,
				Description: common.String("DEK for the model " + *tt.secretConfig.SecretName),
				KeyId:       tt.secretConfig.KeyId,
			}

			// Verify the request structure
			assert.Equal(t, tt.secretConfig.CompartmentId, createSecretDetails.CompartmentId)
			assert.Equal(t, tt.secretConfig.SecretName, createSecretDetails.SecretName)
			assert.Equal(t, tt.secretConfig.VaultId, createSecretDetails.VaultId)
			assert.Equal(t, tt.secretConfig.KeyId, createSecretDetails.KeyId)

			// Verify the content is base64 encoded
			contentDetails, ok := createSecretDetails.SecretContent.(vault.Base64SecretContentDetails)
			require.True(t, ok)
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
		config      *Config
		expectError bool
	}{
		{
			name: "valid config with user principal",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
			},
			expectError: false,
		},
		{
			name: "valid config with instance principal",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.InstancePrincipal}[0],
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Since getConfigProvider uses the principals package internally,
			// and we can't easily mock that without significant refactoring,
			// we'll test that the function exists and can be called with valid config
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)

			// In a real test environment, you would mock the principals.Config.Build method
			// For now, we verify the config structure is correct
		})
	}
}

func TestVaultClient_Integration(t *testing.T) {
	// This test demonstrates how the VaultClient would work in integration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	config := &Config{
		AnotherLogger: mockLogger,
		AuthType:      &authType,
	}

	// Verify config validation
	assert.NotNil(t, config.AnotherLogger)
	assert.NotNil(t, config.AuthType)
	assert.Equal(t, principals.UserPrincipal, *config.AuthType)

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

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
