package enigma

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sgl-project/ome/pkg/constants"

	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/otiai10/copy"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/vault"
)

type Enigma struct {
	logger logging.Interface
	Config Config
}

const exportMetadataFile = ".exports.metadata"

// ignoredFiles defines a set of files to skip during processing
var ignoredFiles = map[string]struct{}{
	".DS_Store": {},
	".gitkeep":  {},
}

// NewApplication initializes an Enigma instance after validating the config
func NewApplication(config *Config) (*Enigma, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return &Enigma{logger: config.AnotherLogger, Config: *config}, nil
}

// Start begins the Enigma process, handling model validation, copying, and decryption
func (e *Enigma) Start() error {
	e.logger.Infof("Starting Enigma for model %s", e.Config.ModelName)

	if err := e.validateModelStore(); err != nil {
		return fmt.Errorf("model store validation failed: %w", err)
	}

	if e.Config.DisableModelDecryption {
		e.logger.Info("Model decryption is disabled by configuration")
		return nil
	}

	e.logger.Infof("Copying model weights %s to temporary path %s", e.Config.ModelName, e.Config.TempPath)
	if err := e.copyModelWeights(); err != nil {
		return fmt.Errorf("failed to copy model weights: %w", err)
	}

	if err := e.decryptModelWeights(); err != nil {
		return fmt.Errorf("error during model weights decryption: %w", err)
	}

	e.logger.Info("Enigma process completed successfully")
	return nil
}

// validateModelStore checks if the model exists in the specified storage path
func (e *Enigma) validateModelStore() error {
	e.logger.Info("Validating model existence in model store")
	modelStorePath := e.getModelStorePath()

	if err := validateModelExistence(modelStorePath); err != nil {
		return fmt.Errorf("model validation failed: %w", err)
	}

	e.logger.Infof("Model %s exists at storage path %s", e.Config.ModelName, modelStorePath)
	return nil
}

// copyModelWeights copies model weights from the local path to a temporary path
func (e *Enigma) copyModelWeights() error {
	modelStorePath := e.getModelStorePath()
	if err := copy.Copy(modelStorePath, e.Config.TempPath); err != nil {
		return fmt.Errorf("failed to copy model weights to temporary path: %w", err)
	}
	return nil
}

// decryptModelWeights decrypts the model weights in the specified temporary path
func (e *Enigma) decryptModelWeights() error {
	plainDataKey, err := e.prepareDecryptionKey()
	if err != nil {
		return fmt.Errorf("failed to prepare decryption key: %w", err)
	}

	modelStorePath := e.getModelTempPath()
	err = filepath.Walk(modelStorePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if isIgnoredFile(info.Name()) {
			e.logger.Debugf("Skipping ignored file %s", info.Name())
			return nil
		}

		e.logger.Infof("Decrypting file %s", path)
		if err := e.decryptFile(path, info, plainDataKey); err != nil {
			e.logger.Errorf("Error decrypting file %s: %v", path, err)
			return err // Return the error to halt further processing
		}
		e.logger.Infof("File %s decrypted successfully", path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error occurred during model weights decryption: %w", err)
	}

	e.logger.Info("Decryption of model weights complete")
	return nil
}

// prepareDecryptionKey retrieves and decrypts the data encryption key (DEK) using the master encryption key (MEK)
func (e *Enigma) prepareDecryptionKey() (string, error) {
	masterKeyID, err := e.getMasterKeyID()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve master key ID: %w", err)
	}
	e.logger.Infof("Master key ID retrieved: %s", *masterKeyID)

	cipherDataKey, err := e.getCipherDataKey()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve cipher data key: %w", err)
	}

	plainDataKey, err := e.Config.KmsCryptoClient.Decrypt(
		*cipherDataKey, true, *masterKeyID,
		keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm,
	)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt DEK using MEK: %w", err)
	}

	e.logger.Info("Successfully decrypted model's DEK for decryption")
	return plainDataKey, nil
}

// decryptFile decrypts an individual file if it is not marked as metadata
func (e *Enigma) decryptFile(path string, info fs.FileInfo, plainDataKey string) error {
	if info.IsDir() {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var decryptedData []byte
	if strings.Contains(info.Name(), exportMetadataFile) {
		e.logger.Infof("Skipping decryption for metadata file %s", info.Name())
		decryptedData = data
	} else {
		decryptedData, err = vault.GCMDecryptWithoutCopy(data, plainDataKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt file %s: %w", path, err)
		}
	}

	if err := os.WriteFile(path, decryptedData, 0666); err != nil {
		return fmt.Errorf("failed to write decrypted data for file %s: %w", path, err)
	}
	return nil
}

// getModelStorePath constructs the path where the model is stored
func (e *Enigma) getModelStorePath() string {
	if e.Config.ModelFramework == TensorRTLLM && e.Config.ModelType == constants.ServingBaseModel {
		return filepath.Join(
			e.Config.LocalPath,
			e.Config.TensorrtLLMConfig.TensorrtLlmVersion,
			e.Config.TensorrtLLMConfig.NodeShapeAlias,
			e.Config.TensorrtLLMConfig.NumOfGpu+"Gpu",
		)
	}
	return e.Config.LocalPath
}

// getModelTempPath constructs the path to the temporary model location for decryption
func (e *Enigma) getModelTempPath() string {
	return e.Config.TempPath
}

// getMasterKeyID retrieves the master encryption key ID from KMS
func (e *Enigma) getMasterKeyID() (*string, error) {
	keys, err := e.Config.KmsManagement.GetKeys(*e.Config.KeyMetadata)
	if err != nil || len(keys) == 0 {
		return nil, fmt.Errorf("failed to retrieve KMS keys: %w", err)
	}
	return keys[0].Id, nil
}

// getCipherDataKey retrieves the encrypted data key (DEK) from OCI Vault
func (e *Enigma) getCipherDataKey() (*string, error) {
	cipherDataKey, err := e.Config.OCISecret.GetSecretBundleContentByNameAndVaultId(e.Config.SecretName, e.Config.VaultId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cipher data key: %w", err)
	}
	return cipherDataKey, nil
}

// validateModelExistence checks if the model directory exists and is not empty
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

// isIgnoredFile checks if a file should be ignored during processing
func isIgnoredFile(fileName string) bool {
	_, ignored := ignoredFiles[fileName]
	return ignored
}
