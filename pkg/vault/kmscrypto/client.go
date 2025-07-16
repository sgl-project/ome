package kmscrypto

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/vault"
)

type KmsCrypto struct {
	Logger logging.Interface
	Config *Config
	Client *keymanagement.KmsCryptoClient
}

// NewKmsCrypto initializes a new KmsCrypto instance with the given configuration and environment.
func NewKmsCrypto(config *Config) (*KmsCrypto, error) {
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %w", err)
	}

	client, err := newKmsCryptoClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS client: %w", err)
	}

	return &KmsCrypto{
		Logger: config.AnotherLogger,
		Config: config,
		Client: client,
	}, nil
}

// getConfigProvider sets up the configuration provider based on the given environment and configuration.
func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	provider, err := principalConfig.Build(principalOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration provider: %w", err)
	}
	return provider, nil
}

// newKmsCryptoClient creates a new KMS Crypto client with the specified configuration provider.
func newKmsCryptoClient(configProvider common.ConfigurationProvider, config *Config) (*keymanagement.KmsCryptoClient, error) {
	client, err := keymanagement.NewKmsCryptoClientWithConfigurationProvider(configProvider, config.KmsCryptoEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS crypto client: %w", err)
	}
	return &client, nil
}

// Encrypt encrypts the provided plaintext using the specified key ID and algorithm.
func (kc *KmsCrypto) Encrypt(plaintext, keyId string, algorithm keymanagement.EncryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	kc.Logger.Infof("Starting encryption for key ID: %s", keyId)

	encryptRequest := keymanagement.EncryptRequest{
		EncryptDataDetails: keymanagement.EncryptDataDetails{
			KeyId:               &keyId,
			Plaintext:           &plaintext,
			EncryptionAlgorithm: algorithm,
		},
	}

	response, err := kc.Client.Encrypt(context.Background(), encryptRequest)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt data with key %s: %w", keyId, err)
	}

	kc.Logger.Infof("Encryption successful for key ID: %s", keyId)
	return *response.Ciphertext, nil
}

// Decrypt decrypts the provided ciphertext using the specified key ID and algorithm.
// Optionally decodes the ciphertext if required.
func (kc *KmsCrypto) Decrypt(ciphertext string, requireDecode bool, keyId string, algorithm keymanagement.DecryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	kc.Logger.Infof("Starting decryption for key ID: %s", keyId)

	if requireDecode {
		kc.Logger.Debug("Decoding ciphertext")
		ciphertext = vault.B64Decode(ciphertext)
	}

	decryptRequest := keymanagement.DecryptRequest{
		DecryptDataDetails: keymanagement.DecryptDataDetails{
			KeyId:               &keyId,
			Ciphertext:          &ciphertext,
			EncryptionAlgorithm: algorithm,
		},
	}

	response, err := kc.Client.Decrypt(context.Background(), decryptRequest)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data with key %s: %w", keyId, err)
	}

	kc.Logger.Infof("Decryption successful for key ID: %s", keyId)
	return *response.Plaintext, nil
}

// GenerateDEK generates a Data Encryption Key (DEK) using the specified Master Encryption Key (MEK) ID.
func (kc *KmsCrypto) GenerateDEK(keyId string) (*keymanagement.GeneratedKey, error) {
	kc.Logger.Infof("Generating DEK for MEK ID: %s", keyId)

	request := kc.newGenerateKeyRequest(keyId)
	response, err := kc.Client.GenerateDataEncryptionKey(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DEK from MEK %s: %w", keyId, err)
	}
	if response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status for DEK generation: %d", response.RawResponse.StatusCode)
	}

	kc.Logger.Info("DEK generation successful")
	return &response.GeneratedKey, nil
}

// newGenerateKeyRequest constructs a request to generate a DEK with AES-256 algorithm.
func (kc *KmsCrypto) newGenerateKeyRequest(keyId string) keymanagement.GenerateDataEncryptionKeyRequest {
	keyShapeLength := 32
	includePlaintextKey := true

	return keymanagement.GenerateDataEncryptionKeyRequest{
		GenerateKeyDetails: keymanagement.GenerateKeyDetails{
			KeyId:               &keyId,
			IncludePlaintextKey: &includePlaintextKey,
			KeyShape: &keymanagement.KeyShape{
				Algorithm: keymanagement.KeyShapeAlgorithmAes,
				Length:    &keyShapeLength,
			},
		},
	}
}
