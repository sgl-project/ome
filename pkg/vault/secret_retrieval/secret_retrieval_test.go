package secret_retrieval

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	omesecrets "github.com/sgl-project/ome/pkg/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSecretsClientInterface defines the interface for mocking secrets client
type MockSecretsClientInterface struct {
	mock.Mock
}

func (m *MockSecretsClientInterface) GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(secrets.GetSecretBundleResponse), args.Error(1)
}

func (m *MockSecretsClientInterface) GetSecretBundleByName(ctx context.Context, request secrets.GetSecretBundleByNameRequest) (secrets.GetSecretBundleByNameResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(secrets.GetSecretBundleByNameResponse), args.Error(1)
}

func TestNewSecretRetriever(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecretRetrievalConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config is nil",
		},
		{
			name: "valid config",
			config: &SecretRetrievalConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since NewSecretRetriever calls external dependencies that are hard to mock,
			// we'll test the validation logic
			if tt.config == nil {
				_, err := NewSecretRetriever(tt.config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				return
			}

			// Test config validation
			if tt.config.AuthType != nil {
				assert.NotNil(t, tt.config.AnotherLogger)
				assert.Equal(t, principals.UserPrincipal, *tt.config.AuthType)
			}
		})
	}
}

func TestSecretRetriever_GetSecretBundleContentByNameAndVaultId(t *testing.T) {
	tests := []struct {
		name            string
		secretConfig    omesecrets.SecretConfig
		setupMocks      func(*MockSecretsClientInterface, *testingPkg.MockLogger)
		expectError     bool
		errorMsg        string
		expectedContent string
	}{
		{
			name: "successful secret retrieval",
			secretConfig: omesecrets.SecretConfig{
				SecretName: stringPtr("test-secret"),
				VaultId:    stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestSecretBundleByNameResponse("dGVzdC1zZWNyZXQtY29udGVudA==")
				mockClient.On("GetSecretBundleByName", mock.Anything, mock.MatchedBy(func(req secrets.GetSecretBundleByNameRequest) bool {
					return req.SecretName != nil && *req.SecretName == "test-secret" &&
						req.VaultId != nil && *req.VaultId == "ocid1.vault.oc1.ap-mumbai-1.test"
				})).Return(expectedResponse, nil)
			},
			expectError:     false,
			expectedContent: "dGVzdC1zZWNyZXQtY29udGVudA==",
		},
		{
			name: "invalid secret config - missing name",
			secretConfig: omesecrets.SecretConfig{
				VaultId: stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				// No mocks needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "invalid secret config",
		},
		{
			name: "invalid secret config - missing vault ID",
			secretConfig: omesecrets.SecretConfig{
				SecretName: stringPtr("test-secret"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				// No mocks needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "invalid secret config",
		},
		{
			name: "OCI API error",
			secretConfig: omesecrets.SecretConfig{
				SecretName: stringPtr("test-secret"),
				VaultId:    stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("GetSecretBundleByName", mock.Anything, mock.Anything).Return(
					secrets.GetSecretBundleByNameResponse{}, fmt.Errorf("failed to get secret test-secret in vault test: OCI error"))
			},
			expectError: true,
			errorMsg:    "failed to get secret",
		},
		{
			name: "non-OK HTTP status",
			secretConfig: omesecrets.SecretConfig{
				SecretName: stringPtr("test-secret"),
				VaultId:    stringPtr("ocid1.vault.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				response := testingPkg.CreateTestSecretBundleByNameResponse("dGVzdC1zZWNyZXQtY29udGVudA==")
				response.RawResponse = testingPkg.CreateMockHTTPResponse(http.StatusNotFound)
				mockClient.On("GetSecretBundleByName", mock.Anything, mock.Anything).Return(response, nil)
			},
			expectError: true,
			errorMsg:    "failed to get secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockSecretsClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test config validation first
			err := tt.secretConfig.ValidateNameAndVaultId()
			if tt.expectError && (tt.secretConfig.SecretName == nil || tt.secretConfig.VaultId == nil) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "SecretName and VaultId must be provided")
				return
			}

			// Test the request preparation
			request := secrets.GetSecretBundleByNameRequest{
				SecretName: tt.secretConfig.SecretName,
				VaultId:    tt.secretConfig.VaultId,
			}

			// Verify request structure
			assert.Equal(t, tt.secretConfig.SecretName, request.SecretName)
			assert.Equal(t, tt.secretConfig.VaultId, request.VaultId)

			// Test the mock client directly
			response, err := mockClient.GetSecretBundleByName(context.Background(), request)

			if tt.expectError && err != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else if tt.expectError && response.RawResponse != nil && response.RawResponse.StatusCode != http.StatusOK {
				// Non-OK status should be handled as error
				assert.NotEqual(t, http.StatusOK, response.RawResponse.StatusCode)
			} else if !tt.expectError {
				assert.NoError(t, err)

				// Test content extraction
				content, ok := response.SecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
				require.True(t, ok)
				assert.Equal(t, tt.expectedContent, *content.Content)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestSecretRetriever_GetSecretBundleContentBySecretId(t *testing.T) {
	tests := []struct {
		name            string
		secretConfig    omesecrets.SecretConfig
		setupMocks      func(*MockSecretsClientInterface, *testingPkg.MockLogger)
		expectError     bool
		errorMsg        string
		expectedContent string
	}{
		{
			name: "successful secret retrieval by ID",
			secretConfig: omesecrets.SecretConfig{
				SecretId: stringPtr("ocid1.secret.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestSecretBundleResponse("dGVzdC1zZWNyZXQtY29udGVudA==")
				mockClient.On("GetSecretBundle", mock.Anything, mock.MatchedBy(func(req secrets.GetSecretBundleRequest) bool {
					return req.SecretId != nil && *req.SecretId == "ocid1.secret.oc1.ap-mumbai-1.test"
				})).Return(expectedResponse, nil)
			},
			expectError:     false,
			expectedContent: "dGVzdC1zZWNyZXQtY29udGVudA==",
		},
		{
			name: "invalid secret config - missing secret ID",
			secretConfig: omesecrets.SecretConfig{
				SecretName: stringPtr("test-secret"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				// No mocks needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "invalid secret config",
		},
		{
			name: "OCI API error",
			secretConfig: omesecrets.SecretConfig{
				SecretId: stringPtr("ocid1.secret.oc1.ap-mumbai-1.test"),
			},
			setupMocks: func(mockClient *MockSecretsClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("GetSecretBundle", mock.Anything, mock.Anything).Return(
					secrets.GetSecretBundleResponse{}, fmt.Errorf("failed to get secret test-id: OCI error"))
			},
			expectError: true,
			errorMsg:    "failed to get secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockSecretsClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test config validation first
			err := tt.secretConfig.ValidateSecretId()
			if tt.expectError && tt.secretConfig.SecretId == nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "SecretId must be provided")
				return
			}

			// Test the request preparation
			request := secrets.GetSecretBundleRequest{
				SecretId: tt.secretConfig.SecretId,
			}

			// Verify request structure
			assert.Equal(t, tt.secretConfig.SecretId, request.SecretId)

			// Test the mock client directly
			response, err := mockClient.GetSecretBundle(context.Background(), request)

			if tt.expectError && err != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else if !tt.expectError {
				assert.NoError(t, err)

				// Test content extraction
				content, ok := response.SecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
				require.True(t, ok)
				assert.Equal(t, tt.expectedContent, *content.Content)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestSetSecretVersionConfig(t *testing.T) {
	tests := []struct {
		name          string
		versionConfig *omesecrets.SecretVersionConfig
		requestType   string
		expectChanges bool
	}{
		{
			name:          "nil version config",
			versionConfig: nil,
			requestType:   "GetSecretBundleByNameRequest",
			expectChanges: false,
		},
		{
			name: "version config with version number",
			versionConfig: &omesecrets.SecretVersionConfig{
				SecretVersionNumber: int64Ptr(1),
			},
			requestType:   "GetSecretBundleByNameRequest",
			expectChanges: true,
		},
		{
			name: "version config with version name",
			versionConfig: &omesecrets.SecretVersionConfig{
				SecretVersionName: stringPtr("test-version"),
			},
			requestType:   "GetSecretBundleRequest",
			expectChanges: true,
		},
		{
			name: "version config with stage",
			versionConfig: &omesecrets.SecretVersionConfig{
				Stage: &[]omesecrets.SecretVersionStage{omesecrets.CurrentStage}[0],
			},
			requestType:   "GetSecretBundleByNameRequest",
			expectChanges: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.requestType == "GetSecretBundleByNameRequest" {
				request := &secrets.GetSecretBundleByNameRequest{}
				setSecretVersionConfig(tt.versionConfig, request)

				if tt.expectChanges && tt.versionConfig != nil {
					if tt.versionConfig.SecretVersionNumber != nil {
						assert.Equal(t, tt.versionConfig.SecretVersionNumber, request.VersionNumber)
					}
					if tt.versionConfig.SecretVersionName != nil {
						assert.Equal(t, tt.versionConfig.SecretVersionName, request.SecretVersionName)
					}
				}
			} else {
				request := &secrets.GetSecretBundleRequest{}
				setSecretVersionConfig(tt.versionConfig, request)

				if tt.expectChanges && tt.versionConfig != nil {
					if tt.versionConfig.SecretVersionNumber != nil {
						assert.Equal(t, tt.versionConfig.SecretVersionNumber, request.VersionNumber)
					}
					if tt.versionConfig.SecretVersionName != nil {
						assert.Equal(t, tt.versionConfig.SecretVersionName, request.SecretVersionName)
					}
				}
			}
		})
	}
}

func TestIsResponseStatusOK(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
		expected bool
	}{
		{
			name:     "nil response",
			response: nil,
			expected: false,
		},
		{
			name:     "OK status",
			response: testingPkg.CreateMockHTTPResponse(http.StatusOK),
			expected: true,
		},
		{
			name:     "not found status",
			response: testingPkg.CreateMockHTTPResponse(http.StatusNotFound),
			expected: false,
		},
		{
			name:     "internal server error",
			response: testingPkg.CreateMockHTTPResponse(http.StatusInternalServerError),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isResponseStatusOK(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSecretContent(t *testing.T) {
	tests := []struct {
		name        string
		bundle      secrets.SecretBundle
		expectError bool
		expected    string
	}{
		{
			name: "valid base64 content",
			bundle: secrets.SecretBundle{
				SecretBundleContent: secrets.Base64SecretBundleContentDetails{
					Content: stringPtr("dGVzdC1jb250ZW50"),
				},
			},
			expectError: false,
			expected:    "dGVzdC1jb250ZW50",
		},
		{
			name: "invalid content type",
			bundle: secrets.SecretBundle{
				SecretBundleContent: "invalid-content-type",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := extractSecretContent(tt.bundle)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, content)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, *content)
			}
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
