package partner_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/aes_cipher"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	BigFileSizeInMB              = 200
	DefaultDownloadChunkSizeInMB = 50
	DefaultDownloadThreads       = 32
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 16
)

const (
	configFileName         = "config.pbtxt"
	exportMetadataFileName = ".exports.metadata"
)

type DownloadAgentTask struct {
	SourceObjectStoreURI *casper.ObjectURI
	TargetObjectStoreURI *casper.ObjectURI
	DownloadAgentMode    *DownloadAgentMode
	TempModelStorePath   string
	SourceDecryptionKey  *string
	TargetEncryptionKey  *string
	completion           *sync.WaitGroup
}

type DownloadAgentWorker struct {
	config *Config

	taskChan chan *DownloadAgentTask
	logger   logging.Interface
}

func NewDownloadAgentWorker(logger logging.Interface, taskChan chan *DownloadAgentTask, config *Config) DownloadAgentWorker {
	return DownloadAgentWorker{
		config:   config,
		taskChan: taskChan,
		logger:   logger,
	}
}

func (worker *DownloadAgentWorker) Start() {
	for task := range worker.taskChan {
		worker.ProcessTask(task)
		// send completion signal
		task.completion.Done()
	}

	worker.logger.Info("Task Channel closed. Worker exits.")
}

func (worker *DownloadAgentWorker) ProcessTask(task *DownloadAgentTask) {
	// 1. List all model weights in Partner bucket
	worker.logger.Info("Start to download model weights from source")
	objects, err := worker.config.SourceCasperDataStore.ListObjects(*task.SourceObjectStoreURI)
	if err != nil {
		panic(err)
	}
	worker.logger.Infof("Done with listing all %d model weight objects under prefix %s", len(objects), task.SourceObjectStoreURI.Prefix)

	var encryptedFiles []string
	if task.SourceDecryptionKey != nil {
		if *task.DownloadAgentMode == ModelImporting {
			encryptedFiles = worker.getEncryptedFiles(task)
		}
	}

	// 2. Iterate over each model weight:
	//    1). Get 2). Decrypt using Partner key 3). Encrypt using Service key 4). Push to Service object store 5). Cleanup
	count := 1
	for _, object := range objects {
		if count == 2 {
			worker.logger.Infof("Done with first %s flow of model %s", *task.DownloadAgentMode, task.SourceObjectStoreURI.Prefix)
		}

		if count == len(objects)/2 {
			worker.logger.Infof("Done with half %s flow of model %s", *task.DownloadAgentMode, task.SourceObjectStoreURI.Prefix)
		}

		// 1). download the model
		partnerObjectURI := casper.ObjectURI{
			Namespace:  task.SourceObjectStoreURI.Namespace,
			BucketName: task.SourceObjectStoreURI.BucketName,
			ObjectName: *object.Name,
		}

		err = worker.config.SourceCasperDataStore.MultipartDownload(partnerObjectURI, task.TempModelStorePath, false, &object, DefaultDownloadChunkSizeInMB, DefaultDownloadThreads)
		if err != nil {
			panic(err)
		}

		currentModelFilePath := filepath.Join(task.TempModelStorePath, *object.Name)

		if !strings.Contains(*object.Name, exportMetadataFileName) {
			downloadedModelWeight, err := os.ReadFile(currentModelFilePath)
			if err != nil {
				panic(err)
			}

			var plainModelWeight []byte
			if task.SourceDecryptionKey != nil {
				// 2). Decrypt using Partner key
				if *task.DownloadAgentMode == ModelImporting {
					if contains(encryptedFiles, *object.Name) {
						if strings.Contains(*object.Name, configFileName) {
							worker.logger.Infof("Start decryption for config file %s", configFileName)
						}

						plainModelWeight, err = aes_cipher.GCMDecryptWithoutCopy(downloadedModelWeight, *task.SourceDecryptionKey)
						if err != nil {
							panic(err)
						}
					} else {
						plainModelWeight = downloadedModelWeight
					}

					// Temp step: add/update required parameters in config.pbtxt
					if strings.Contains(*object.Name, configFileName) {
						worker.logger.Infof("Start working on the parameter updates for %s", configFileName)
						plainModelWeight = AddAndUpdateModelParameters(string(plainModelWeight), *object.Name)
					}

				} else {
					plainModelWeight, err = aes_cipher.GCMDecryptWithoutCopy(downloadedModelWeight, *task.SourceDecryptionKey)
					if err != nil {
						panic(err)
					}
				}
			} else {
				plainModelWeight = downloadedModelWeight
			}

			// 3). Encrypt using Service key
			genaiCipheredModelWeight, err := aes_cipher.GCMEncryptWithoutCopy(plainModelWeight, *task.TargetEncryptionKey)
			if err != nil {
				panic(err)
			}

			err = os.WriteFile(currentModelFilePath, genaiCipheredModelWeight, 0666)
			if err != nil {
				panic(err)
			}
		} else {
			worker.logger.Info("Metadata file, skip decryption or encryption")
		}

		objectNameForGenai := strings.Replace(*object.Name, task.SourceObjectStoreURI.Prefix, task.TargetObjectStoreURI.Prefix, 1)
		// 4). Push to Service object store
		genaiObjectURI := casper.ObjectURI{
			Namespace:  task.TargetObjectStoreURI.Namespace,
			BucketName: task.TargetObjectStoreURI.BucketName,
			ObjectName: objectNameForGenai,
		}
		if err = worker.config.TargetCasperDataStore.MultipartFileUpload(currentModelFilePath, genaiObjectURI, DefaultUploadChunkSizeInMB, DefaultUploadThreads); err != nil {
			panic(err)
		}

		// 5). Delete from the current temp storage to not use too much memory
		os.Remove(currentModelFilePath)

		count++
	}
	worker.logger.Infof("%s succeeded for model %s with %d model weight objects", *task.DownloadAgentMode, task.SourceObjectStoreURI.Prefix, count-1)
}

// getEncryptedFiles fetches names of all files which are encrypted by Partner(Cohere) from .exports.metadata
func (worker *DownloadAgentWorker) getEncryptedFiles(task *DownloadAgentTask) []string {
	worker.logger.Info("Reading .exports.metadata file to get a list of encrypted files")

	// Get .exports.metadata file
	metadataFileURI := casper.ObjectURI{
		Namespace:  task.SourceObjectStoreURI.Namespace,
		BucketName: task.SourceObjectStoreURI.BucketName,
		ObjectName: fmt.Sprintf("%s%s", task.SourceObjectStoreURI.Prefix, exportMetadataFileName),
	}

	getMetadataFileResponse, err := worker.config.SourceCasperDataStore.GetObject(metadataFileURI)
	if err != nil {
		panic(err)
	}

	// Read .exports.metadata line by line into a slice
	fileScanner := bufio.NewScanner(getMetadataFileResponse.Content)
	fileScanner.Split(bufio.ScanLines)
	var encryptedFiles []string
	for fileScanner.Scan() {
		encryptedFiles = append(encryptedFiles, fileScanner.Text())
	}
	getMetadataFileResponse.Content.Close()
	return encryptedFiles
}

func contains(s []string, target string) bool {
	for _, ele := range s {
		// using strings.Contains here(instead of 'target == ele') because of in our use case, target passed would not be exactly equals to the element in passed slice
		// e.g. target = command_6b_model/1/config.ini, corresponding element in slice = 1/config.ini
		if strings.Contains(target, ele) {
			return true
		}
	}
	return false
}
