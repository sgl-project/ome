package enigma

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/aes_cipher"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Enigma struct {
	logger logging.Interface
	Config Config
}

const exportMetadataFile = ".exports.metadata"

func NewApplication(config *Config) (*Enigma, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Enigma{logger: config.AnotherLogger, Config: *config}, nil
}

func (e *Enigma) Start() {
	e.logger.Infof("Starting Enigma for model %s", e.Config.ModelName)
	e.validateModelStore()

	if e.Config.DisableModelDecryption {
		e.logger.Info("Model decryption is disabled")
		return
	}
	e.decryptModelWeights()
}

func (e *Enigma) validateModelStore() {
	e.logger.Info("Validating model existence in model store")
	modelStorePath := e.getModelStorePath()
	if err := validateModelExistence(modelStorePath); err != nil {
		panic(err)
	}
	e.logger.Infof("Model %s exists at storage path %s", e.Config.ModelName, modelStorePath)
}

func (e *Enigma) getModelStorePath() string {
	if e.Config.ModelFramework == TensorRTLLM {
		return filepath.Join(
			e.Config.ModelStoreDirectory, e.Config.ModelName,
			e.Config.TensorrtLLMConfig.TensorrtLlmVersion,
			e.Config.TensorrtLLMConfig.NodeShapeAlias,
			e.Config.TensorrtLLMConfig.NumOfGpu+"Gpu",
		)
	}
	return filepath.Join(e.Config.ModelStoreDirectory, e.Config.ModelName)
}

func (e *Enigma) decryptModelWeights() {
	// Retrieve MEK, DEK and decrypt the data key
	plainDataKey := e.prepareDecryptionKey()

	// Perform model weights decryption
	modelStorePath := e.getModelStorePath()
	err := filepath.Walk(modelStorePath, func(path string, info fs.FileInfo, err error) error {
		return e.decryptFile(path, info, plainDataKey)
	})
	if err != nil {
		panic(err)
	}
	e.logger.Info("Decryption of model weights complete")
}

func (e *Enigma) prepareDecryptionKey() *string {
	masterKeyId := e.getMasterKeyID()
	cipherDataKey := e.getCipherDataKey()

	plainDataKey, err := e.Config.CryptoClient.Decrypt(
		*cipherDataKey, true, *masterKeyId,
		keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm,
	)
	if err != nil {
		panic(fmt.Errorf("failed to decrypt DEK using MEK: %v", err))
	}
	e.logger.Info("Successfully decrypted model's DEK for decryption")
	return &plainDataKey
}

func (e *Enigma) getMasterKeyID() *string {
	keys, err := e.Config.KmsKeyManager.GetKeys(*e.Config.KeyConfig)
	if err != nil || len(keys) == 0 {
		panic(fmt.Errorf("failed to retrieve KMS keys: %v", err))
	}
	return keys[0].Id
}

func (e *Enigma) getCipherDataKey() *string {
	cipherDataKey, err := e.Config.SecretRetriever.GetSecretBundleContentBySecretId(*e.Config.SecretConfig)
	if err == nil {
		return cipherDataKey
	}

	cipherDataKey, err = e.Config.SecretRetriever.GetSecretBundleContentByNameAndVaultId(*e.Config.SecretConfig)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve cipher data key: %v", err))
	}
	return cipherDataKey
}

func (e *Enigma) decryptFile(path string, info fs.FileInfo, genaiPlainDataKey *string) error {
	if info.IsDir() {
		return nil
	}

	cipherModelWeight, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", path, err)
	}

	var outputModelWeight []byte
	if !strings.Contains(info.Name(), exportMetadataFile) {
		outputModelWeight, err = aes_cipher.GCMDecryptWithoutCopy(cipherModelWeight, *genaiPlainDataKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt file %s: %v", path, err)
		}
	} else {
		outputModelWeight = cipherModelWeight
		e.logger.Infof("Skipping decryption for metadata file %s", info.Name())
	}

	return os.WriteFile(path, outputModelWeight, 0666)
}

func validateModelExistence(modelDirPath string) error {
	dir, err := os.Open(modelDirPath)
	if err != nil {
		return fmt.Errorf("model directory validation failed: model directory %s does not exist: %w", modelDirPath, err)
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	if err == io.EOF {
		return fmt.Errorf("model directory validation failed: model directory %s is empty", modelDirPath)
	}
	if err != nil {
		return fmt.Errorf("model directory validation failed: %s", err)
	}
	return nil
}
