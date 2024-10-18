package serving_init

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/aes_cipher"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/otiai10/copy"
)

const exportMetadataFile = ".exports.metadata"

// ServingInit represents an Example application
type ServingInit struct {
	anotherLogger logging.Interface

	Config Config
}

// NewApplication constructs a new server from the given configuration.
func NewApplication(config *Config) (*ServingInit, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("app configuration invalid: %v", err)
	}

	return &ServingInit{anotherLogger: config.AnotherLogger, Config: *config}, nil
}

// Start starts the application
func (a *ServingInit) Start() {
	a.anotherLogger.Infof("Starting serving init for model %s", a.Config.ModelName)

	// 0. Validate downloaded model in model store
	var modelStorePath string
	if a.Config.IsTensorrtModel {
		tensorRTSubPath := filepath.Join(
			a.Config.TensorrtConfig.TensorrtLlmVersion,
			a.Config.TensorrtConfig.NodeShapeShort,
			a.Config.TensorrtConfig.NumOfGpu+"Gpu")
		modelStorePath = filepath.Join(a.Config.ModelStoreDirectory, a.Config.ModelName, tensorRTSubPath)
	} else {
		modelStorePath = filepath.Join(a.Config.ModelStoreDirectory, a.Config.ModelName)
	}

	err := validateModelExistence(modelStorePath)
	if err != nil {
		panic(err)
	}
	a.anotherLogger.Infof("Verified model %s exists at storage path %s", a.Config.ModelName, modelStorePath)

	// 1. Copy model weights from model store to local store
	localStorePath := filepath.Join(a.Config.LocalStoreDirectory, a.Config.ModelName)
	err = copy.Copy(modelStorePath, localStorePath)
	if err != nil {
		panic(err)
	}

	if a.Config.DisableModelDecryption {
		a.anotherLogger.Info("Not necessary for model copying and decryption")
		return
	}

	a.anotherLogger.Infof("Done with copying model to local path %s", localStorePath)
	a.decryptModel(localStorePath)
	a.anotherLogger.Infof("Done with serving init for model %s", a.Config.ModelName)
}

func (a *ServingInit) decryptModel(localStorePath string) {
	// 2. Fetch MEK, DEK from service tenancy
	genaiMasterKeys, err := a.Config.KmsManagement.GetKeys(*a.Config.KeyConfig)
	if err != nil {
		panic(err)
	}
	genaiMasterKeyId := genaiMasterKeys[0].Id

	var genaiCipherDataKey *string
	// First fetch by secret id; when there is an error, then fetch by vault id and secret name
	if genaiCipherDataKey, err = a.Config.SecretRetrieval.GetSecretBundleContentBySecretId(*a.Config.SecretConfig); err != nil {
		genaiCipherDataKey, err = a.Config.SecretRetrieval.GetSecretBundleContentByNameAndVaultId(*a.Config.SecretConfig)
		if err != nil {
			panic(err)
		}
	}
	// 3. Decrypt DEK using MEK
	genaiPlainDataKey, err := a.Config.KmsCrypto.Decrypt(*genaiCipherDataKey, true, *genaiMasterKeyId, keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm)
	if err != nil {
		panic(err)
	}
	a.anotherLogger.Info("Done with preparing model's DEK for decryption")

	// 4. Decrypt model weights
	err = filepath.Walk(localStorePath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			cipherModelWeight, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}

			var outputModelWeight []byte
			if !strings.Contains(info.Name(), exportMetadataFile) {
				outputModelWeight, err = aes_cipher.GCMDecryptWithoutCopy(cipherModelWeight, genaiPlainDataKey)
				if err != nil {
					panic(err)
				}
			} else {
				// Skip the metadata file to avoid decryption for it
				outputModelWeight = cipherModelWeight
				a.anotherLogger.Infof("Skipping decryption for metadata file %s", info.Name())
			}
			err = os.WriteFile(path, outputModelWeight, 0666)
			if err != nil {
				panic(err)
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func validateModelExistence(modelDirPath string) error {
	// TODO: Checksum validation
	dir, err := os.Open(modelDirPath)
	if err != nil {
		return fmt.Errorf("model directory validation failed: model directory %s does not exist: %w", modelDirPath, err)
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)

	if err == io.EOF { // model directory is empty
		return fmt.Errorf("model directory validation failed: model directory %s is empty", modelDirPath)
	}
	if err != nil {
		return fmt.Errorf("model directory validation failed: %s", err)
	}
	return nil
}
