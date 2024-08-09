package key_management

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/secrets"
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"net/http"
)

type KmsCrypto struct {
	logger          logging.Interface
	KmsCryptoClient *keymanagement.KmsCryptoClient
	KmsCryptoConfig *KmsConfig
}

func NewKmsCrypto(config *KmsConfig, e *env.Environment) (*KmsCrypto, error) {
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
		return nil, err
	}

	return &KmsCrypto{
		KmsCryptoConfig: config,
		KmsCryptoClient: client,
	}, nil
}

func (kc *KmsCrypto) Encrypt(plaintext string, keyId string, algorithm keymanagement.EncryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	encryptDataDetails := keymanagement.EncryptDataDetails{
		KeyId:               &keyId,
		Plaintext:           &plaintext,
		EncryptionAlgorithm: algorithm,
	}

	encryptRequest := keymanagement.EncryptRequest{
		EncryptDataDetails: encryptDataDetails,
	}

	response, err := kc.KmsCryptoClient.Encrypt(context.Background(), encryptRequest)
	if err != nil {
		return "", fmt.Errorf("fail to encrypt data: %v", err)
	}
	return *response.Ciphertext, nil
}

func (kc *KmsCrypto) Decrypt(ciphertext string, requireDecode bool, keyId string, algorithm keymanagement.DecryptDataDetailsEncryptionAlgorithmEnum) (string, error) {
	decodedCipherText := ciphertext
	if requireDecode {
		decodedCipherText = secrets.B64Decode(ciphertext)
	}

	decryptDataDetails := keymanagement.DecryptDataDetails{
		KeyId:               &keyId,
		Ciphertext:          &decodedCipherText,
		EncryptionAlgorithm: algorithm,
	}

	decryptRequest := keymanagement.DecryptRequest{
		DecryptDataDetails: decryptDataDetails,
	}
	response, err := kc.KmsCryptoClient.Decrypt(context.Background(), decryptRequest)
	if err != nil {
		return "", fmt.Errorf("cannot decrypt data: %v", err)
	}

	return *response.Plaintext, nil
}

func (kc *KmsCrypto) GenerateDEK(keyId string) (*keymanagement.GeneratedKey, error) {
	keyShapeLength := 32
	flagForIncludePlaintextKey := true
	generateKeyDetails := keymanagement.GenerateKeyDetails{
		KeyId:               &keyId,
		IncludePlaintextKey: &flagForIncludePlaintextKey,
		KeyShape: &keymanagement.KeyShape{
			Algorithm: keymanagement.KeyShapeAlgorithmAes,
			Length:    &keyShapeLength,
		},
	}
	generateKeyRequest := keymanagement.GenerateDataEncryptionKeyRequest{
		GenerateKeyDetails: generateKeyDetails,
	}

	response, err := kc.KmsCryptoClient.GenerateDataEncryptionKey(context.Background(), generateKeyRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot generate DEK from MEK %s: %v", keyId, err)
	}

	return &response.GeneratedKey, nil
}
