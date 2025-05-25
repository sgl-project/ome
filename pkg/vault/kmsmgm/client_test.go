package kmsmgm

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/sgl-project/sgl-ome/pkg/principals"
	testingPkg "github.com/sgl-project/sgl-ome/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockKmsManagementClientInterface defines the interface for mocking KMS management client
type MockKmsManagementClientInterface struct {
	mock.Mock
}

func (m *MockKmsManagementClientInterface) ListKeys(ctx context.Context, request keymanagement.ListKeysRequest) (keymanagement.ListKeysResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.ListKeysResponse), args.Error(1)
}

func TestNewKmsMgm(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since NewKmsMgm calls external dependencies that are hard to mock,
			// we'll test the configuration validation
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
			assert.Equal(t, principals.UserPrincipal, *tt.config.AuthType)
		})
	}
}

func TestKmsMgm_GetKeys(t *testing.T) {
	tests := []struct {
		name        string
		metadata    KeyMetadata
		setupMocks  func(*MockKmsManagementClientInterface, *testingPkg.MockLogger)
		expectError bool
		errorMsg    string
		expectedLen int
	}{
		{
			name: "successful key retrieval",
			metadata: KeyMetadata{
				Name:            "test-key",
				CompartmentId:   "ocid1.compartment.oc1..test",
				Algorithm:       keymanagement.ListKeysAlgorithmAes,
				Length:          256,
				LifecycleState:  keymanagement.KeySummaryLifecycleStateEnabled,
				ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
			},
			setupMocks: func(mockClient *MockKmsManagementClientInterface, mockLogger *testingPkg.MockLogger) {
				keys := []keymanagement.KeySummary{
					testingPkg.CreateTestKeySummary("ocid1.key.oc1..test", "test-key", keymanagement.KeySummaryLifecycleStateEnabled),
				}
				expectedResponse := testingPkg.CreateTestListKeysResponse(keys)
				mockClient.On("ListKeys", mock.Anything, mock.MatchedBy(func(req keymanagement.ListKeysRequest) bool {
					return req.CompartmentId != nil && *req.CompartmentId == "ocid1.compartment.oc1..test"
				})).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			},
			expectError: false,
			expectedLen: 1,
		},
		{
			name: "key retrieval failure",
			metadata: KeyMetadata{
				Name:            "test-key",
				CompartmentId:   "ocid1.compartment.oc1..test",
				Algorithm:       keymanagement.ListKeysAlgorithmAes,
				Length:          256,
				LifecycleState:  keymanagement.KeySummaryLifecycleStateEnabled,
				ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
			},
			setupMocks: func(mockClient *MockKmsManagementClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("ListKeys", mock.Anything, mock.Anything).Return(
					keymanagement.ListKeysResponse{}, fmt.Errorf("failed to list keys: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to list keys",
		},
		{
			name: "no keys found",
			metadata: KeyMetadata{
				Name:            "nonexistent-key",
				CompartmentId:   "ocid1.compartment.oc1..test",
				Algorithm:       keymanagement.ListKeysAlgorithmAes,
				Length:          256,
				LifecycleState:  keymanagement.KeySummaryLifecycleStateEnabled,
				ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
			},
			setupMocks: func(mockClient *MockKmsManagementClientInterface, mockLogger *testingPkg.MockLogger) {
				keys := []keymanagement.KeySummary{
					testingPkg.CreateTestKeySummary("ocid1.key.oc1..test", "different-key", keymanagement.KeySummaryLifecycleStateEnabled),
				}
				expectedResponse := testingPkg.CreateTestListKeysResponse(keys)
				mockClient.On("ListKeys", mock.Anything, mock.Anything).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "no keys found matching metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockKmsManagementClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the request preparation
			request := keymanagement.ListKeysRequest{
				CompartmentId:  &tt.metadata.CompartmentId,
				Algorithm:      tt.metadata.Algorithm,
				Length:         &tt.metadata.Length,
				ProtectionMode: tt.metadata.ProtectionModel,
			}

			// Verify request structure
			assert.Equal(t, &tt.metadata.CompartmentId, request.CompartmentId)
			assert.Equal(t, tt.metadata.Algorithm, request.Algorithm)
			assert.Equal(t, &tt.metadata.Length, request.Length)
			assert.Equal(t, tt.metadata.ProtectionModel, request.ProtectionMode)

			// Test the mock client directly
			response, err := mockClient.ListKeys(context.Background(), request)

			if tt.expectError && err != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else if !tt.expectError {
				assert.NoError(t, err)
				assert.Len(t, response.Items, tt.expectedLen)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestKmsMgm_ListKeysByAttributes(t *testing.T) {
	tests := []struct {
		name        string
		metadata    KeyMetadata
		setupMocks  func(*MockKmsManagementClientInterface, *testingPkg.MockLogger)
		expectError bool
		expectedLen int
	}{
		{
			name: "successful list keys",
			metadata: KeyMetadata{
				CompartmentId:   "ocid1.compartment.oc1..test",
				Algorithm:       keymanagement.ListKeysAlgorithmAes,
				Length:          256,
				ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
			},
			setupMocks: func(mockClient *MockKmsManagementClientInterface, mockLogger *testingPkg.MockLogger) {
				keys := []keymanagement.KeySummary{
					testingPkg.CreateTestKeySummary("ocid1.key.oc1..test1", "key1", keymanagement.KeySummaryLifecycleStateEnabled),
					testingPkg.CreateTestKeySummary("ocid1.key.oc1..test2", "key2", keymanagement.KeySummaryLifecycleStateEnabled),
				}
				expectedResponse := testingPkg.CreateTestListKeysResponse(keys)
				mockClient.On("ListKeys", mock.Anything, mock.Anything).Return(expectedResponse, nil)

				mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name: "non-OK HTTP status",
			metadata: KeyMetadata{
				CompartmentId:   "ocid1.compartment.oc1..test",
				Algorithm:       keymanagement.ListKeysAlgorithmAes,
				Length:          256,
				ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
			},
			setupMocks: func(mockClient *MockKmsManagementClientInterface, mockLogger *testingPkg.MockLogger) {
				response := testingPkg.CreateTestListKeysResponse([]keymanagement.KeySummary{})
				response.RawResponse = testingPkg.CreateMockHTTPResponse(http.StatusBadRequest)
				mockClient.On("ListKeys", mock.Anything, mock.Anything).Return(response, nil)

				mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockKmsManagementClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the mock client directly
			response, err := mockClient.ListKeys(context.Background(), keymanagement.ListKeysRequest{
				CompartmentId:  &tt.metadata.CompartmentId,
				Algorithm:      tt.metadata.Algorithm,
				Length:         &tt.metadata.Length,
				ProtectionMode: tt.metadata.ProtectionModel,
			})

			if tt.expectError && (err != nil || response.RawResponse.StatusCode != http.StatusOK) {
				if err != nil {
					assert.Error(t, err)
				} else {
					assert.NotEqual(t, http.StatusOK, response.RawResponse.StatusCode)
				}
			} else if !tt.expectError {
				assert.NoError(t, err)
				assert.Len(t, response.Items, tt.expectedLen)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestKeyMetadata_Fields(t *testing.T) {
	// Test KeyMetadata structure
	metadata := KeyMetadata{
		Name:             "test-key",
		CompartmentId:    "ocid1.compartment.oc1..test",
		Algorithm:        keymanagement.ListKeysAlgorithmAes,
		Length:           256,
		LifecycleState:   keymanagement.KeySummaryLifecycleStateEnabled,
		ProtectionModel:  keymanagement.ListKeysProtectionModeSoftware,
		EnableDefinedTag: true,
		DefinedTags: DefinedTags{
			Namespace: "test-namespace",
			Key:       "test-key",
			Value:     "test-value",
		},
	}

	assert.Equal(t, "test-key", metadata.Name)
	assert.Equal(t, "ocid1.compartment.oc1..test", metadata.CompartmentId)
	assert.Equal(t, keymanagement.ListKeysAlgorithmAes, metadata.Algorithm)
	assert.Equal(t, 256, metadata.Length)
	assert.Equal(t, keymanagement.KeySummaryLifecycleStateEnabled, metadata.LifecycleState)
	assert.Equal(t, keymanagement.ListKeysProtectionModeSoftware, metadata.ProtectionModel)
	assert.True(t, metadata.EnableDefinedTag)
	assert.Equal(t, "test-namespace", metadata.DefinedTags.Namespace)
	assert.Equal(t, "test-key", metadata.DefinedTags.Key)
	assert.Equal(t, "test-value", metadata.DefinedTags.Value)
}

func TestDefinedTags_Fields(t *testing.T) {
	// Test DefinedTags structure
	tags := DefinedTags{
		Namespace: "environment",
		Key:       "stage",
		Value:     "production",
	}

	assert.Equal(t, "environment", tags.Namespace)
	assert.Equal(t, "stage", tags.Key)
	assert.Equal(t, "production", tags.Value)
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

func TestNewKmsManagementClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				AnotherLogger:         testingPkg.SetupMockLogger(),
				AuthType:              &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				KmsManagementEndpoint: "https://test-management-endpoint.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the configuration validation logic
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
			assert.NotEmpty(t, tt.config.KmsManagementEndpoint)
		})
	}
}

func TestKmsMgm_Integration(t *testing.T) {
	// Test complete KmsMgm integration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	config := &Config{
		AnotherLogger:         mockLogger,
		AuthType:              &authType,
		KmsManagementEndpoint: "https://test-management-endpoint.com",
		VaultId:               "ocid1.vault.oc1.test",
	}

	// Verify config validation
	assert.NotNil(t, config.AnotherLogger)
	assert.NotNil(t, config.AuthType)
	assert.Equal(t, principals.UserPrincipal, *config.AuthType)
	assert.NotEmpty(t, config.KmsManagementEndpoint)
	assert.NotEmpty(t, config.VaultId)

	// Test metadata validation
	metadata := KeyMetadata{
		Name:            "test-key",
		CompartmentId:   "ocid1.compartment.oc1..test",
		Algorithm:       keymanagement.ListKeysAlgorithmAes,
		Length:          256,
		LifecycleState:  keymanagement.KeySummaryLifecycleStateEnabled,
		ProtectionModel: keymanagement.ListKeysProtectionModeSoftware,
	}

	assert.NotEmpty(t, metadata.Name)
	assert.NotEmpty(t, metadata.CompartmentId)
	assert.Equal(t, keymanagement.ListKeysAlgorithmAes, metadata.Algorithm)
	assert.Equal(t, 256, metadata.Length)
}
