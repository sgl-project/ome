package key_management

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
)

type CryptoClient struct {
	logger          logging.Interface
	KmsCryptoClient *keymanagement.KmsCryptoClient
	Config          *KmsConfig
}

func NewCryptoClient(config *KmsConfig, e *env.Environment) (*CryptoClient, error) {
	if config == nil {
		return nil, fmt.Errorf("KmsConfig is nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("KmsConfig is invalid: %+v", err)
	}

	configProvider, err := getConfigProvider(config, e)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %+v", err)
	}

	client, err := NewKmsCryptoClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS client: %v", err)
	}

	return &CryptoClient{
		Config:          config,
		KmsCryptoClient: client,
	}, nil
}

// Encrypt encrypts the given plaintext using the specified key ID and algorithm.
func (kc *CryptoClient) Encrypt(plaintext, keyId string, algorithm keymanagement.EncryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	encryptRequest := keymanagement.EncryptRequest{
		EncryptDataDetails: keymanagement.EncryptDataDetails{
			KeyId:               &keyId,
			Plaintext:           &plaintext,
			EncryptionAlgorithm: algorithm,
		},
	}

	response, err := kc.KmsCryptoClient.Encrypt(context.Background(), encryptRequest)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt data: %v", err)
	}
	return *response.Ciphertext, nil
}

// Decrypt decrypts the given ciphertext using the specified key ID and algorithm. Optionally decodes the ciphertext.
func (kc *CryptoClient) Decrypt(ciphertext string, requireDecode bool, keyId string, algorithm keymanagement.DecryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	if requireDecode {
		ciphertext = secrets.B64Decode(ciphertext)
	}

	decryptRequest := keymanagement.DecryptRequest{
		DecryptDataDetails: keymanagement.DecryptDataDetails{
			KeyId:               &keyId,
			Ciphertext:          &ciphertext,
			EncryptionAlgorithm: algorithm,
		},
	}

	response, err := kc.KmsCryptoClient.Decrypt(context.Background(), decryptRequest)
	if err != nil {
		return "", fmt.Errorf("cannot decrypt data: %v", err)
	}
	return *response.Plaintext, nil
}

// GenerateDEK generates a Data Encryption Key (DEK) using the provided Master Encryption Key (MEK) ID.
func (kc *CryptoClient) GenerateDEK(keyId string) (*keymanagement.GeneratedKey, error) {
	generateKeyRequest := kc.newGenerateKeyRequest(keyId)

	response, err := kc.KmsCryptoClient.GenerateDataEncryptionKey(context.Background(), generateKeyRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot generate DEK from MEK %s: %v", keyId, err)
	}

	return &response.GeneratedKey, nil
}

// Helper function to create a GenerateDataEncryptionKeyRequest with AES-256 algorithm.
func (kc *CryptoClient) newGenerateKeyRequest(keyId string) keymanagement.GenerateDataEncryptionKeyRequest {
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
