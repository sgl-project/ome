package partner_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"strings"
	"sync"
)

const (
	DefaultNumOfWorkers = 3
	cohereAppendPrefix  = "cohere"
)

type DownloadAgent struct {
	logger logging.Interface

	Config *Config

	taskChan chan *DownloadAgentTask
}

func NewDownloadAgent(config *Config) (*DownloadAgent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("partner download agent configuration invalid: %+v", err)
	}

	taskChan := make(chan *DownloadAgentTask)

	return &DownloadAgent{
		logger:   config.AnotherLogger,
		Config:   config,
		taskChan: taskChan,
	}, nil
}

// Start starts the application
func (d *DownloadAgent) Start() {
	d.logger.Infof("Starting %s for %s", *d.Config.DownloadAgentMode, d.Config.ModelName)
	// 1. Get decryption key from source if necessary
	var sourceDecryptionKey *string
	if d.Config.SourceModelEncrypted {
		sourceDecryptionKey = common.String(d.getSourceDecryptionKey())
	} else {
		sourceDecryptionKey = nil
	}

	// 2. Get encryption key from target tenancy
	targetEncryptionKey := d.getTargetEncryptionKey()

	// 3. prepare all Download tasks
	var wg sync.WaitGroup
	allTasks := getAllDownloadAgentTasks(d.Config, &wg, sourceDecryptionKey, &targetEncryptionKey)

	// 4. submit all Download tasks
	wg.Add(len(allTasks))
	go func() {
		defer close(d.taskChan)
		for _, task := range allTasks {
			d.taskChan <- task
		}
	}()

	// 5. launch all workers
	for i := 0; i < DefaultNumOfWorkers; i++ {
		worker := NewDownloadAgentWorker(d.Config.AnotherLogger, d.taskChan, d.Config)
		go worker.Start()
	}

	// 6. wait for all tasks completions
	wg.Wait()
	d.logger.Infof("Finished %s for %s", *d.Config.DownloadAgentMode, d.Config.ModelName)
}

func (d *DownloadAgent) getSourceDecryptionKey() string {
	// 1. Fetch MEK from Source tenancy
	sourceMasterKeys, err := d.Config.SourceKmsManagement.GetKeys(*d.Config.SourceKeyConfig)
	if err != nil {
		panic(err)
	}
	sourceMasterKeyId := sourceMasterKeys[0].Id
	d.logger.Infof("Fetched MEK %s in source successfully", *sourceMasterKeyId)

	// 2. Fetch DEK from source tenancy
	var sourceCipherDataKey *string
	// First fetch by secret id; when there is an error, then fetch by vault id and secret name
	if sourceCipherDataKey, err = d.Config.SourceSecretRetrieval.GetSecretBundleContentBySecretId(*d.Config.SourceSecretConfig); err != nil {
		sourceCipherDataKey, err = d.Config.SourceSecretRetrieval.GetSecretBundleContentByNameAndVaultId(*d.Config.SourceSecretConfig)
		if err != nil {
			panic(err)
		}
	}
	d.logger.Info("Fetched DEK in source successfully")

	// 3. Decrypt source DEK using source MEK
	requireDecode := false
	if *d.Config.DownloadAgentMode == ModelReplication {
		requireDecode = true
	}
	sourcePlainDataKey, err := d.Config.SourceKmsCrypto.Decrypt(*sourceCipherDataKey, requireDecode, *sourceMasterKeyId, keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm)
	if err != nil {
		panic(err)
	}
	d.logger.Info("Decrypted DEK in source successfully")

	return sourcePlainDataKey
}

func (d *DownloadAgent) getTargetEncryptionKey() string {
	// 1. Fetch MEK from target tenancy
	targetMasterKeys, err := d.Config.TargetKmsManagement.GetKeys(*d.Config.TargetKeyConfig)
	if err != nil {
		panic(err)
	}
	targetMasterKeyId := targetMasterKeys[0].Id
	d.logger.Infof("Fetched MEK %s in destination successfully", *targetMasterKeyId)

	// 2. Get DEK for target tenancy
	// Check if DEK already exists for current model in target tenancy
	var targetCipherDataKey *string
	if targetCipherDataKey, err = d.Config.TargetSecretRetrieval.GetSecretBundleContentBySecretId(*d.Config.TargetSecretConfig); err != nil {
		targetCipherDataKey, _ = d.Config.TargetSecretRetrieval.GetSecretBundleContentByNameAndVaultId(*d.Config.TargetSecretConfig)
	}
	var targetPlainDataKey string
	if targetCipherDataKey != nil {
		// 2.a. If exists, decrypt it using target MEK
		d.logger.Infof("DEK %s in destination already exists, directly decrypt it for use", *d.Config.TargetSecretConfig.SecretName)
		targetPlainDataKey, err = d.Config.TargetKmsCrypto.Decrypt(*targetCipherDataKey, true, *targetMasterKeyId, keymanagement.DecryptDataDetailsEncryptionAlgorithmAes256Gcm)
		if err != nil {
			panic(err)
		}
	} else {
		// 2.a. If not exists, generate one for use
		d.logger.Infof("DEK %s does not exist in destination, generate one for use", *d.Config.TargetSecretConfig.SecretName)
		targetDataKey, err := d.Config.TargetKmsCrypto.GenerateDEK(*targetMasterKeyId)
		if err != nil {
			panic(err)
		}
		targetPlainDataKey = *targetDataKey.Plaintext
		d.logger.Infof("Done with generating DEK %s in destination", *d.Config.TargetSecretConfig.SecretName)

		// 2.b. Store target DEK into target tenancy as a secret
		d.Config.TargetSecretConfig.KeyId = targetMasterKeyId
		if _, err = d.Config.SecretInVault.CreateSecretInVault(*d.Config.TargetSecretConfig, *targetDataKey.Ciphertext); err != nil {
			panic(err)
		}
		d.logger.Info("Store DEK in destination successfully")
	}

	return targetPlainDataKey
}

func getAllDownloadAgentTasks(config *Config, completion *sync.WaitGroup, sourceDecryptionKey *string, targetEncryptionKey *string) []*DownloadAgentTask {
	allTasks := make([]*DownloadAgentTask, 0)

	if len(config.ModelPathConfigs) == 0 {
		sourceObjectStoreURI := &casper.ObjectURI{
			Namespace:  config.SourceObjectStoreURI.Namespace,
			BucketName: config.SourceObjectStoreURI.BucketName,
			Prefix:     getObjectPrefix(config.SourceObjectStoreURI.Prefix, "", ""),
		}

		var targetObjectStoreURIPrefix string
		if *config.DownloadAgentMode == ModelImporting {
			targetObjectStoreURIPrefix = getObjectPrefix(config.TargetObjectStoreURI.Prefix, cohereAppendPrefix, "")
		} else {
			targetObjectStoreURIPrefix = getObjectPrefix(config.TargetObjectStoreURI.Prefix, "", "")
		}

		targetObjectStoreURI := &casper.ObjectURI{
			Namespace:  config.TargetObjectStoreURI.Namespace,
			BucketName: config.TargetObjectStoreURI.BucketName,
			Prefix:     targetObjectStoreURIPrefix,
		}

		downloadTask := &DownloadAgentTask{
			SourceObjectStoreURI: sourceObjectStoreURI,
			TargetObjectStoreURI: targetObjectStoreURI,
			DownloadAgentMode:    config.DownloadAgentMode,
			TempModelStorePath:   config.TempModelStorePath,
			SourceDecryptionKey:  sourceDecryptionKey,
			TargetEncryptionKey:  targetEncryptionKey,
			completion:           completion,
		}

		allTasks = append(allTasks, downloadTask)
		return allTasks
	}

	for _, sourceModelPathConfig := range config.ModelPathConfigs {
		var sourceObjectStoreURIPrefix string
		if *config.DownloadAgentMode == ModelImporting {
			sourceObjectStoreURIPrefix = getObjectPrefix(sourceModelPathConfig.ModelObjectName, "", "")
		} else {
			sourceObjectStoreURIPrefix = getObjectPrefix(config.SourceObjectStoreURI.Prefix, "", sourceModelPathConfig.ModelPath)
		}

		sourceObjectStoreURI := &casper.ObjectURI{
			Namespace:  config.SourceObjectStoreURI.Namespace,
			BucketName: config.SourceObjectStoreURI.BucketName,
			Prefix:     sourceObjectStoreURIPrefix,
		}

		var targetObjectStoreURIPrefix string
		if *config.DownloadAgentMode == ModelImporting {
			targetObjectStoreURIPrefix = getObjectPrefix(config.TargetObjectStoreURI.Prefix, cohereAppendPrefix, sourceModelPathConfig.ModelPath)
		} else {
			targetObjectStoreURIPrefix = getObjectPrefix(config.TargetObjectStoreURI.Prefix, "", sourceModelPathConfig.ModelPath)
		}

		targetObjectStoreURI := &casper.ObjectURI{
			Namespace:  config.TargetObjectStoreURI.Namespace,
			BucketName: config.TargetObjectStoreURI.BucketName,
			Prefix:     targetObjectStoreURIPrefix,
		}

		downloadTask := &DownloadAgentTask{
			SourceObjectStoreURI: sourceObjectStoreURI,
			TargetObjectStoreURI: targetObjectStoreURI,
			DownloadAgentMode:    config.DownloadAgentMode,
			TempModelStorePath:   config.TempModelStorePath,
			SourceDecryptionKey:  sourceDecryptionKey,
			TargetEncryptionKey:  targetEncryptionKey,
			completion:           completion,
		}

		allTasks = append(allTasks, downloadTask)
	}

	return allTasks
}

func getObjectPrefix(prefix, prefixHead, prefixTail string) string {
	if prefixTail != "" {
		if strings.HasSuffix(prefix, "/") {
			prefix = prefix + prefixTail
		} else {
			prefix = prefix + "/" + prefixTail
		}
	}

	if prefixHead != "" {
		if strings.HasSuffix(prefixHead, "/") {
			prefix = prefixHead + prefix
		} else {
			prefix = prefixHead + "/" + prefix
		}
	}

	if !strings.HasSuffix(prefix, "/") {
		return prefix + "/"
	}

	return prefix
}
