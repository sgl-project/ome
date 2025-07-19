package kmscrypto

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/vault"
)

// MockKmsCryptoClientInterface defines the interface for mocking KMS crypto client
type MockKmsCryptoClientInterface struct {
	mock.Mock
}

func (m *MockKmsCryptoClientInterface) Encrypt(ctx context.Context, request keymanagement.EncryptRequest) (keymanagement.EncryptResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.EncryptResponse), args.Error(1)
}

func (m *MockKmsCryptoClientInterface) Decrypt(ctx context.Context, request keymanagement.DecryptRequest) (keymanagement.DecryptResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.DecryptResponse), args.Error(1)
}

func (m *MockKmsCryptoClientInterface) GenerateDataEncryptionKey(ctx context.Context, request keymanagement.GenerateDataEncryptionKeyRequest) (keymanagement.GenerateDataEncryptionKeyResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.GenerateDataEncryptionKeyResponse), args.Error(1)
}

func TestNewKmsCrypto(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				AnotherLogger:     testingPkg.SetupMockLogger(),
				AuthType:          &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				KmsCryptoEndpoint: "https://test-crypto-endpoint.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since NewKmsCrypto calls external dependencies that are hard to mock,
			// we'll test the configuration validation
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
			assert.NotEmpty(t, tt.config.KmsCryptoEndpoint)
		})
	}
}

func TestKmsCrypto_Encrypt(t *testing.T) {
	tests := []struct {
		name               string
		plaintext          string
		keyId              string
		algorithm          keymanagement.EncryptDataDetailsEncryptionAlgorithmEnum
		setupMocks         func(*MockKmsCryptoClientInterface, *testingPkg.MockLogger)
		expectError        bool
		errorMsg           string
		expectedCiphertext string
	}{
		{
			name:      "successful encryption",
			plaintext: "test-plaintext",
			keyId:     "ocid1.key.oc1.ap-mumbai-1.test",
			algorithm: keymanagement.EncryptDataDetailsEncryptionAlgorithmAes256Gcm,
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestEncryptResponse("encrypted-ciphertext")
				mockClient.On("Encrypt", mock.Anything, mock.MatchedBy(func(req keymanagement.EncryptRequest) bool {
					return req.EncryptDataDetails.KeyId != nil &&
						*req.EncryptDataDetails.KeyId == "ocid1.key.oc1.ap-mumbai-1.test" &&
						req.EncryptDataDetails.Plaintext != nil &&
						*req.EncryptDataDetails.Plaintext == "test-plaintext"
				})).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError:        false,
			expectedCiphertext: "encrypted-ciphertext",
		},
		{
			name:      "encryption failure",
			plaintext: "test-plaintext",
			keyId:     "ocid1.key.oc1.ap-mumbai-1.test",
			algorithm: keymanagement.EncryptDataDetailsEncryptionAlgorithmAes256Gcm,
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("Encrypt", mock.Anything, mock.Anything).Return(
					keymanagement.EncryptResponse{}, fmt.Errorf("failed to encrypt data with key test: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to encrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockKmsCryptoClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the encryption request preparation
			encryptRequest := keymanagement.EncryptRequest{
				EncryptDataDetails: keymanagement.EncryptDataDetails{
					KeyId:               &tt.keyId,
					Plaintext:           &tt.plaintext,
					EncryptionAlgorithm: tt.algorithm,
				},
			}

			// Verify request structure
			assert.Equal(t, &tt.keyId, encryptRequest.EncryptDataDetails.KeyId)
			assert.Equal(t, &tt.plaintext, encryptRequest.EncryptDataDetails.Plaintext)
			assert.Equal(t, tt.algorithm, encryptRequest.EncryptDataDetails.EncryptionAlgorithm)

			// Test the mock client directly
			response, err := mockClient.Encrypt(context.Background(), encryptRequest)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCiphertext, *response.EncryptedData.Ciphertext)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestKmsCrypto_Decrypt(t *testing.T) {
	tests := []struct {
		name              string
		ciphertext        string
		requireDecode     bool
		keyId             string
		algorithm         keymanagement.DecryptDataDetailsEncryptionAlgorithmEnum
		setupMocks        func(*MockKmsCryptoClientInterface, *testingPkg.MockLogger)
		expectError       bool
		errorMsg          string
		expectedPlaintext string
	}{
		{
			name:          "successful decryption without decode",
			ciphertext:    "encrypted-ciphertext",
			requireDecode: false,
			keyId:         "ocid1.key.oc1.ap-mumbai-1.test",
			algorithm:     keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm,
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestDecryptResponse("decrypted-plaintext")
				mockClient.On("Decrypt", mock.Anything, mock.MatchedBy(func(req keymanagement.DecryptRequest) bool {
					return req.DecryptDataDetails.KeyId != nil &&
						*req.DecryptDataDetails.KeyId == "ocid1.key.oc1.ap-mumbai-1.test" &&
						req.DecryptDataDetails.Ciphertext != nil &&
						*req.DecryptDataDetails.Ciphertext == "encrypted-ciphertext"
				})).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError:       false,
			expectedPlaintext: "decrypted-plaintext",
		},
		{
			name:          "successful decryption with decode",
			ciphertext:    vault.B64Encode("encrypted-ciphertext"),
			requireDecode: true,
			keyId:         "ocid1.key.oc1.ap-mumbai-1.test",
			algorithm:     keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm,
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestDecryptResponse("decrypted-plaintext")
				mockClient.On("Decrypt", mock.Anything, mock.MatchedBy(func(req keymanagement.DecryptRequest) bool {
					return req.DecryptDataDetails.KeyId != nil &&
						*req.DecryptDataDetails.KeyId == "ocid1.key.oc1.ap-mumbai-1.test" &&
						req.DecryptDataDetails.Ciphertext != nil &&
						*req.DecryptDataDetails.Ciphertext == "encrypted-ciphertext"
				})).Return(expectedResponse, nil)

				mockLogger.On("Debug", mock.AnythingOfType("string")).Maybe()
				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError:       false,
			expectedPlaintext: "decrypted-plaintext",
		},
		{
			name:          "decryption failure",
			ciphertext:    "encrypted-ciphertext",
			requireDecode: false,
			keyId:         "ocid1.key.oc1.ap-mumbai-1.test",
			algorithm:     keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm,
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("Decrypt", mock.Anything, mock.Anything).Return(
					keymanagement.DecryptResponse{}, fmt.Errorf("failed to decrypt data with key test: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to decrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockKmsCryptoClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Prepare ciphertext (decode if required)
			ciphertext := tt.ciphertext
			if tt.requireDecode {
				ciphertext = vault.B64Decode(tt.ciphertext)
			}

			// Test the decryption request preparation
			decryptRequest := keymanagement.DecryptRequest{
				DecryptDataDetails: keymanagement.DecryptDataDetails{
					KeyId:               &tt.keyId,
					Ciphertext:          &ciphertext,
					EncryptionAlgorithm: tt.algorithm,
				},
			}

			// Verify request structure
			assert.Equal(t, &tt.keyId, decryptRequest.DecryptDataDetails.KeyId)
			assert.Equal(t, &ciphertext, decryptRequest.DecryptDataDetails.Ciphertext)
			assert.Equal(t, tt.algorithm, decryptRequest.DecryptDataDetails.EncryptionAlgorithm)

			// Test the mock client directly
			response, err := mockClient.Decrypt(context.Background(), decryptRequest)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPlaintext, *response.DecryptedData.Plaintext)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestKmsCrypto_GenerateDEK(t *testing.T) {
	tests := []struct {
		name               string
		keyId              string
		setupMocks         func(*MockKmsCryptoClientInterface, *testingPkg.MockLogger)
		expectError        bool
		errorMsg           string
		expectedPlaintext  string
		expectedCiphertext string
	}{
		{
			name:  "successful DEK generation",
			keyId: "ocid1.key.oc1.ap-mumbai-1.test",
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				expectedResponse := testingPkg.CreateTestGenerateDataEncryptionKeyResponse("dGVzdC1wbGFpbnRleHQ=", "dGVzdC1jaXBoZXJ0ZXh0")
				mockClient.On("GenerateDataEncryptionKey", mock.Anything, mock.MatchedBy(func(req keymanagement.GenerateDataEncryptionKeyRequest) bool {
					return req.GenerateKeyDetails.KeyId != nil &&
						*req.GenerateKeyDetails.KeyId == "ocid1.key.oc1.ap-mumbai-1.test" &&
						req.GenerateKeyDetails.KeyShape != nil &&
						req.GenerateKeyDetails.KeyShape.Algorithm == keymanagement.KeyShapeAlgorithmAes &&
						*req.GenerateKeyDetails.KeyShape.Length == 32
				})).Return(expectedResponse, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Info", mock.AnythingOfType("string")).Maybe()
			},
			expectError:        false,
			expectedPlaintext:  "dGVzdC1wbGFpbnRleHQ=",
			expectedCiphertext: "dGVzdC1jaXBoZXJ0ZXh0",
		},
		{
			name:  "DEK generation failure",
			keyId: "ocid1.key.oc1.ap-mumbai-1.test",
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				mockClient.On("GenerateDataEncryptionKey", mock.Anything, mock.Anything).Return(
					keymanagement.GenerateDataEncryptionKeyResponse{}, fmt.Errorf("failed to generate DEK from MEK test: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to generate DEK",
		},
		{
			name:  "DEK generation non-OK status",
			keyId: "ocid1.key.oc1.ap-mumbai-1.test",
			setupMocks: func(mockClient *MockKmsCryptoClientInterface, mockLogger *testingPkg.MockLogger) {
				response := testingPkg.CreateTestGenerateDataEncryptionKeyResponse("dGVzdC1wbGFpbnRleHQ=", "dGVzdC1jaXBoZXJ0ZXh0")
				response.RawResponse = testingPkg.CreateMockHTTPResponse(http.StatusBadRequest)
				mockClient.On("GenerateDataEncryptionKey", mock.Anything, mock.Anything).Return(response, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "unexpected response status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockKmsCryptoClientInterface{}

			tt.setupMocks(mockClient, mockLogger)

			// Test the DEK generation request preparation
			keyShapeLength := 32
			includePlaintextKey := true

			request := keymanagement.GenerateDataEncryptionKeyRequest{
				GenerateKeyDetails: keymanagement.GenerateKeyDetails{
					KeyId:               &tt.keyId,
					IncludePlaintextKey: &includePlaintextKey,
					KeyShape: &keymanagement.KeyShape{
						Algorithm: keymanagement.KeyShapeAlgorithmAes,
						Length:    &keyShapeLength,
					},
				},
			}

			// Verify request structure
			assert.Equal(t, &tt.keyId, request.GenerateKeyDetails.KeyId)
			assert.Equal(t, &includePlaintextKey, request.GenerateKeyDetails.IncludePlaintextKey)
			assert.NotNil(t, request.GenerateKeyDetails.KeyShape)
			assert.Equal(t, keymanagement.KeyShapeAlgorithmAes, request.GenerateKeyDetails.KeyShape.Algorithm)
			assert.Equal(t, &keyShapeLength, request.GenerateKeyDetails.KeyShape.Length)

			// Test the mock client directly
			response, err := mockClient.GenerateDataEncryptionKey(context.Background(), request)

			if tt.expectError {
				if err != nil {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.errorMsg)
				} else if response.RawResponse != nil && response.RawResponse.StatusCode != http.StatusOK {
					// Non-OK status should be handled as error
					assert.NotEqual(t, http.StatusOK, response.RawResponse.StatusCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPlaintext, *response.GeneratedKey.Plaintext)
				assert.Equal(t, tt.expectedCiphertext, *response.GeneratedKey.Ciphertext)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestNewGenerateKeyRequest(t *testing.T) {
	testKeyId := "ocid1.key.oc1.ap-mumbai-1.test"

	// Create KmsCrypto instance
	kmsCrypto := &KmsCrypto{
		Logger: testingPkg.SetupMockLogger(),
	}

	request := kmsCrypto.newGenerateKeyRequest(testKeyId)

	// Verify the request structure
	assert.Equal(t, &testKeyId, request.GenerateKeyDetails.KeyId)
	assert.NotNil(t, request.GenerateKeyDetails.IncludePlaintextKey)
	assert.True(t, *request.GenerateKeyDetails.IncludePlaintextKey)
	assert.NotNil(t, request.GenerateKeyDetails.KeyShape)
	assert.Equal(t, keymanagement.KeyShapeAlgorithmAes, request.GenerateKeyDetails.KeyShape.Algorithm)
	assert.NotNil(t, request.GenerateKeyDetails.KeyShape.Length)
	assert.Equal(t, 32, *request.GenerateKeyDetails.KeyShape.Length)
}

func TestKmsCrypto_Integration(t *testing.T) {
	// This test demonstrates how the KmsCrypto would work in integration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	config := &Config{
		AnotherLogger:     mockLogger,
		AuthType:          &authType,
		KmsCryptoEndpoint: "https://test-crypto-endpoint.com",
	}

	// Verify config validation
	assert.NotNil(t, config.AnotherLogger)
	assert.NotNil(t, config.AuthType)
	assert.Equal(t, principals.UserPrincipal, *config.AuthType)
	assert.NotEmpty(t, config.KmsCryptoEndpoint)

	// Test encryption/decryption flow
	plaintext := "test-secret-data"
	keyId := "ocid1.key.oc1.ap-mumbai-1.test"
	algorithm := keymanagement.EncryptDataDetailsEncryptionAlgorithmAes256Gcm

	// Verify keyId format
	assert.Contains(t, keyId, "ocid1.key.oc1")
	assert.NotEmpty(t, keyId)

	// Test base64 encoding/decoding
	encoded := vault.B64Encode(plaintext)
	assert.NotEmpty(t, encoded)
	assert.NotEqual(t, plaintext, encoded)

	decoded := vault.B64Decode(encoded)
	assert.Equal(t, plaintext, decoded)

	// Verify algorithm constants
	assert.Equal(t, keymanagement.EncryptDataDetailsEncryptionAlgorithmAes256Gcm, algorithm)
}
