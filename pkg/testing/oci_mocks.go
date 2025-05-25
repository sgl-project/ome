package testing

import (
	"context"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/oracle/oci-go-sdk/v65/vault"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/principals"
	"github.com/stretchr/testify/mock"
)

// MockConfigurationProvider implements common.ConfigurationProvider for testing
type MockConfigurationProvider struct {
	mock.Mock
}

func (m *MockConfigurationProvider) TenancyOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) UserOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) KeyFingerprint() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) Region() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) PrivateRSAKey() (interface{}, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}

func (m *MockConfigurationProvider) KeyID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// MockLogger implements logging.Interface for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Info(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warn(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Error(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatal(msg string) {
	m.Called(msg)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) WithField(key string, value interface{}) logging.Interface {
	args := m.Called(key, value)
	return args.Get(0).(logging.Interface)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) logging.Interface {
	args := m.Called(fields)
	return args.Get(0).(logging.Interface)
}

func (m *MockLogger) WithError(err error) logging.Interface {
	args := m.Called(err)
	return args.Get(0).(logging.Interface)
}

// MockVaultsClient mocks the OCI Vault client
type MockVaultsClient struct {
	mock.Mock
}

func (m *MockVaultsClient) CreateSecret(ctx context.Context, request vault.CreateSecretRequest) (vault.CreateSecretResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(vault.CreateSecretResponse), args.Error(1)
}

func (m *MockVaultsClient) GetVault(ctx context.Context, request keymanagement.GetVaultRequest) (keymanagement.GetVaultResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.GetVaultResponse), args.Error(1)
}

// MockSecretsClient mocks the OCI Secrets client
type MockSecretsClient struct {
	mock.Mock
}

func (m *MockSecretsClient) GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(secrets.GetSecretBundleResponse), args.Error(1)
}

func (m *MockSecretsClient) GetSecretBundleByName(ctx context.Context, request secrets.GetSecretBundleByNameRequest) (secrets.GetSecretBundleByNameResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(secrets.GetSecretBundleByNameResponse), args.Error(1)
}

// MockKmsVaultClient mocks the OCI KMS Vault client
type MockKmsVaultClient struct {
	mock.Mock
}

func (m *MockKmsVaultClient) GetVault(ctx context.Context, request keymanagement.GetVaultRequest) (keymanagement.GetVaultResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.GetVaultResponse), args.Error(1)
}

// MockKmsManagementClient mocks the OCI KMS Management client
type MockKmsManagementClient struct {
	mock.Mock
}

func (m *MockKmsManagementClient) ListKeys(ctx context.Context, request keymanagement.ListKeysRequest) (keymanagement.ListKeysResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.ListKeysResponse), args.Error(1)
}

// MockKmsCryptoClient mocks the OCI KMS Crypto client
type MockKmsCryptoClient struct {
	mock.Mock
}

func (m *MockKmsCryptoClient) Encrypt(ctx context.Context, request keymanagement.EncryptRequest) (keymanagement.EncryptResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.EncryptResponse), args.Error(1)
}

func (m *MockKmsCryptoClient) Decrypt(ctx context.Context, request keymanagement.DecryptRequest) (keymanagement.DecryptResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.DecryptResponse), args.Error(1)
}

func (m *MockKmsCryptoClient) GenerateDataEncryptionKey(ctx context.Context, request keymanagement.GenerateDataEncryptionKeyRequest) (keymanagement.GenerateDataEncryptionKeyResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(keymanagement.GenerateDataEncryptionKeyResponse), args.Error(1)
}

// MockPrincipalFactory implements principals.Factory for testing
type MockPrincipalFactory struct {
	mock.Mock
}

func (m *MockPrincipalFactory) InstancePrincipal(opts principals.Opts, config principals.InstancePrincipalConfig) (common.ConfigurationProvider, error) {
	args := m.Called(opts, config)
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

func (m *MockPrincipalFactory) ResourcePrincipal(opts principals.Opts, config principals.ResourcePrincipalConfig) (common.ConfigurationProvider, error) {
	args := m.Called(opts, config)
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

func (m *MockPrincipalFactory) UserPrincipal(opts principals.Opts, config principals.UserPrincipalConfig) (common.ConfigurationProvider, error) {
	args := m.Called(opts, config)
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

func (m *MockPrincipalFactory) OkeWorkloadIdentity(opts principals.Opts, config principals.OkeWorkloadIdentityConfig) (common.ConfigurationProvider, error) {
	args := m.Called(opts, config)
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

// Helper functions for creating test data

// CreateMockHTTPResponse creates a mock HTTP response for testing
func CreateMockHTTPResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
	}
}

// CreateTestSecretConfig creates a test SecretConfig for testing
func CreateTestSecretConfig() map[string]interface{} {
	return map[string]interface{}{
		"compartment_id": "ocid1.compartment.oc1..test",
		"secret_name":    "test-secret",
		"vault_id":       "ocid1.vault.oc1.ap-mumbai-1.test",
		"key_id":         "ocid1.key.oc1.ap-mumbai-1.test",
	}
}

// CreateTestCreateSecretResponse creates a mock CreateSecretResponse for testing
func CreateTestCreateSecretResponse() vault.CreateSecretResponse {
	secretId := "ocid1.secret.oc1.ap-mumbai-1.test"

	return vault.CreateSecretResponse{
		Secret: vault.Secret{
			Id: &secretId,
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestKeySummary creates a mock KeySummary for testing
func CreateTestKeySummary(id, name string, state keymanagement.KeySummaryLifecycleStateEnum) keymanagement.KeySummary {
	return keymanagement.KeySummary{
		Id:             &id,
		DisplayName:    &name,
		LifecycleState: state,
	}
}

// CreateTestListKeysResponse creates a mock ListKeysResponse for testing
func CreateTestListKeysResponse(keys []keymanagement.KeySummary) keymanagement.ListKeysResponse {
	return keymanagement.ListKeysResponse{
		Items:       keys,
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestSecretBundleByNameResponse creates a mock GetSecretBundleByNameResponse for testing
func CreateTestSecretBundleByNameResponse(content string) secrets.GetSecretBundleByNameResponse {
	return secrets.GetSecretBundleByNameResponse{
		SecretBundle: secrets.SecretBundle{
			SecretBundleContent: secrets.Base64SecretBundleContentDetails{
				Content: &content,
			},
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestSecretBundleResponse creates a mock GetSecretBundleResponse for testing
func CreateTestSecretBundleResponse(content string) secrets.GetSecretBundleResponse {
	return secrets.GetSecretBundleResponse{
		SecretBundle: secrets.SecretBundle{
			SecretBundleContent: secrets.Base64SecretBundleContentDetails{
				Content: &content,
			},
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestVaultResponse creates a mock GetVaultResponse for testing
func CreateTestVaultResponse(vaultId, cryptoEndpoint, managementEndpoint string) keymanagement.GetVaultResponse {
	return keymanagement.GetVaultResponse{
		Vault: keymanagement.Vault{
			Id:                 &vaultId,
			CryptoEndpoint:     &cryptoEndpoint,
			ManagementEndpoint: &managementEndpoint,
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestGenerateDataEncryptionKeyResponse creates a mock GenerateDataEncryptionKeyResponse for testing
func CreateTestGenerateDataEncryptionKeyResponse(plaintext, ciphertext string) keymanagement.GenerateDataEncryptionKeyResponse {
	return keymanagement.GenerateDataEncryptionKeyResponse{
		GeneratedKey: keymanagement.GeneratedKey{
			Plaintext:  &plaintext,
			Ciphertext: &ciphertext,
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestEncryptResponse creates a mock EncryptResponse for testing
func CreateTestEncryptResponse(ciphertext string) keymanagement.EncryptResponse {
	return keymanagement.EncryptResponse{
		EncryptedData: keymanagement.EncryptedData{
			Ciphertext: &ciphertext,
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// CreateTestDecryptResponse creates a mock DecryptResponse for testing
func CreateTestDecryptResponse(plaintext string) keymanagement.DecryptResponse {
	return keymanagement.DecryptResponse{
		DecryptedData: keymanagement.DecryptedData{
			Plaintext: &plaintext,
		},
		RawResponse: CreateMockHTTPResponse(http.StatusOK),
	}
}

// SetupMockLogger creates a mock logger that returns itself for chaining methods
func SetupMockLogger() *MockLogger {
	mockLogger := &MockLogger{}

	// Setup common expectations for logger chaining
	mockLogger.On("WithField", mock.Anything, mock.Anything).Return(mockLogger).Maybe()
	mockLogger.On("WithFields", mock.Anything).Return(mockLogger).Maybe()
	mockLogger.On("WithError", mock.Anything).Return(mockLogger).Maybe()

	// Setup common logging calls to not fail tests
	mockLogger.On("Debug", mock.Anything).Maybe()
	mockLogger.On("Debugf", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Info", mock.Anything).Maybe()
	mockLogger.On("Infof", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Warn", mock.Anything).Maybe()
	mockLogger.On("Warnf", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Error", mock.Anything).Maybe()
	mockLogger.On("Errorf", mock.Anything, mock.Anything).Maybe()

	return mockLogger
}

// SetupMockConfigProvider creates a mock configuration provider with common expectations
func SetupMockConfigProvider() *MockConfigurationProvider {
	mockProvider := &MockConfigurationProvider{}

	// Setup common expectations
	mockProvider.On("TenancyOCID").Return("ocid1.tenancy.oc1..test", nil).Maybe()
	mockProvider.On("UserOCID").Return("ocid1.user.oc1..test", nil).Maybe()
	mockProvider.On("KeyFingerprint").Return("test:fingerprint", nil).Maybe()
	mockProvider.On("Region").Return("us-ashburn-1", nil).Maybe()
	mockProvider.On("KeyID").Return("ocid1.tenancy.oc1..test/ocid1.user.oc1..test/test:fingerprint", nil).Maybe()

	return mockProvider
}
