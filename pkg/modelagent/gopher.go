package modelagent

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils/storage"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

type GopherTaskType string

const (
	Download         GopherTaskType = "Download"
	DownloadOverride GopherTaskType = "DownloadOverride"
	Delete           GopherTaskType = "Delete"
)

type GopherTask struct {
	TaskType               GopherTaskType
	BaseModel              *v1beta1.BaseModel
	ClusterBaseModel       *v1beta1.ClusterBaseModel
	TensorRTLLMShapeFilter *TensorRTLLMShapeFilter
}

type Gopher struct {
	modelConfigParser    *ModelConfigParser
	modelConfigUpdater   *ModelConfigUpdater
	downloadRetry        int
	concurrency          int
	multipartConcurrency int
	modelRootDir         string
	casperDataStore      *casper.CasperDataStore
	gopherChan           <-chan *GopherTask
	nodeLabeler          *NodeLabeler
	metrics              *Metrics
	logger               *zap.SugaredLogger
	configMapMutex       sync.Mutex // Mutex to coordinate ConfigMap access between nodeLabeler and modelConfigUpdater
}

const (
	BigFileSizeInMB = 200
)

func NewGopher(
	modelConfigParser *ModelConfigParser,
	modelConfigUpdater *ModelConfigUpdater,
	casperDataStore *casper.CasperDataStore,
	concurrency int,
	multipartConcurrency int,
	downloadRetry int,
	modelRootDir string,
	gopherChan <-chan *GopherTask,
	nodeLabeler *NodeLabeler,
	metrics *Metrics,
	logger *zap.SugaredLogger) (*Gopher, error) {

	if casperDataStore == nil {
		return nil, fmt.Errorf("casper data store cannot be nil")
	}

	return &Gopher{
		modelConfigParser:    modelConfigParser,
		modelConfigUpdater:   modelConfigUpdater,
		downloadRetry:        downloadRetry,
		concurrency:          concurrency,
		multipartConcurrency: multipartConcurrency,
		modelRootDir:         modelRootDir,
		casperDataStore:      casperDataStore,
		gopherChan:           gopherChan,
		nodeLabeler:          nodeLabeler,
		metrics:              metrics,
		logger:               logger,
	}, nil
}

func (s *Gopher) Run(stopCh <-chan struct{}, numWorker int) {
	s.logger.Info("Starting gopher workers")

	for i := 0; i < numWorker; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	s.logger.Info("Started gopher workers")
	<-stopCh
	s.logger.Info("Shutting down gopher workers")
}

func (s *Gopher) runWorker() {
	for {
		select {
		case task, ok := <-s.gopherChan:
			if ok {
				err := s.processTask(task)
				if err != nil {
					s.logger.Errorf("Gopher task failed with error: %s", err.Error())
				}
			} else {
				s.logger.Info("gopher channel closed, worker exits.")
				return
			}
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// safeNodeLabelerProcessOp executes the NodeLabeler's ProcessOp method with mutex protection
// to ensure thread-safe ConfigMap updates
func (s *Gopher) safeNodeLabelerProcessOp(op *NodeLabelOp) error {
	s.configMapMutex.Lock()
	defer s.configMapMutex.Unlock()

	return s.nodeLabeler.ProcessOp(op)
}

// safeParseAndUpdateModelConfig executes the ModelConfigParser's ParseAndUpdateModelConfig method with mutex protection
// to ensure thread-safe ConfigMap updates
func (s *Gopher) safeParseAndUpdateModelConfig(modelPath string, baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) error {
	s.configMapMutex.Lock()
	defer s.configMapMutex.Unlock()

	// First parse the configuration without updating the ConfigMap
	// This call will return model metadata
	metadata, err := s.modelConfigParser.ParseModelConfig(modelPath, baseModel, clusterBaseModel)
	if err != nil {
		return err
	}

	// If valid metadata was found, update the ConfigMap while still holding the lock
	if metadata != nil {
		op := &ModelConfigOp{
			ModelMetadata:    *metadata,
			BaseModel:        baseModel,
			ClusterBaseModel: clusterBaseModel,
		}

		// Update the ConfigMap with model configuration
		// Since we're holding the lock, we can call the UpdateModelConfig method directly
		return s.modelConfigUpdater.UpdateModelConfig(op)
	}

	return nil
}

func (s *Gopher) processTask(task *GopherTask) error {
	if task.BaseModel == nil && task.ClusterBaseModel == nil {
		return fmt.Errorf("gopher got empty task")
	}

	// Get model info for logging
	modelInfo := getModelInfoForLogging(task)
	s.logger.Infof("Processing gopher task: %s, type: %s", modelInfo, task.TaskType)

	// Get model type, namespace, and name for metrics
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	var baseModelSpec v1beta1.BaseModelSpec
	if task.BaseModel != nil {
		baseModelSpec = task.BaseModel.Spec
	} else {
		baseModelSpec = task.ClusterBaseModel.Spec
	}

	storageType, err := storage.GetStorageType(*baseModelSpec.Storage.StorageUri)

	if err != nil {
		s.logger.Errorf("Failed to get target directory path for model %s: %v", modelInfo, err)

		// Record failed download in metrics
		if task.TaskType == Download || task.TaskType == DownloadOverride {
			s.metrics.RecordFailedDownload(modelType, namespace, name, "target_path_error")
		}

		s.markModelOnNodeFailed(task)
		return err
	}

	switch task.TaskType {
	case Download:
		// we might implement a "delete/cleanup and then download" logic to update a model in the future
		// use a single download function for now
		fallthrough
	case DownloadOverride:
		s.logger.Infof("Starting download for model %s", modelInfo)

		// Record time for metrics
		downloadStartTime := time.Now()
		switch storageType {
		case storage.StorageTypeOCI:
			casperUri, err := getTargetDirPath(&baseModelSpec)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)
			if err != nil {
				s.logger.Errorf("Failed to get target directory path for model %s: %v", modelInfo, err)
				return err
			}
			err = utils.Retry(s.downloadRetry, 100*time.Millisecond, func() error {
				downloadErr := s.downloadModel(casperUri, destPath, task.TensorRTLLMShapeFilter, task)
				if downloadErr != nil {
					s.logger.Errorf("Failed to download model %s (attempt %d/%d): %v",
						modelInfo, s.downloadRetry, s.downloadRetry, downloadErr)
				}
				return downloadErr
			})
			if err != nil {
				s.logger.Errorf("All download attempts failed for model %s: %v", modelInfo, err)

				// Record download failure in metrics
				errorType := "download_error"
				if strings.Contains(err.Error(), "MD5") {
					errorType = "md5_verification_error"
				}
				s.metrics.RecordFailedDownload(modelType, namespace, name, errorType)

				s.markModelOnNodeFailed(task)
				return err
			}
			// Parse model config and update ConfigMap
			// We can pass either BaseModel or ClusterBaseModel based on the task's model type
			var baseModel *v1beta1.BaseModel
			var clusterBaseModel *v1beta1.ClusterBaseModel

			// Check the actual model type from the task
			if task.BaseModel != nil {
				baseModel = task.BaseModel
				s.logger.Debugf("Using BaseModel %s/%s for config parsing", baseModel.Namespace, baseModel.Name)
			} else if task.ClusterBaseModel != nil {
				clusterBaseModel = task.ClusterBaseModel
				s.logger.Debugf("Using ClusterBaseModel %s for config parsing", clusterBaseModel.Name)
			} else {
				s.logger.Warnf("No model object found in task, skipping config parsing")
			}

			_ = s.safeParseAndUpdateModelConfig(destPath, baseModel, clusterBaseModel)
		case storage.StorageTypeVendor:
			s.logger.Infof("Skipping download for model %s", modelInfo)
		default:
			return fmt.Errorf("unknown storage type %s", storageType)
		}
		// Calculate download duration
		downloadDuration := time.Since(downloadStartTime)

		// Record successful download in metrics
		s.metrics.RecordSuccessfulDownload(modelType, namespace, name)
		s.metrics.ObserveDownloadDuration(modelType, namespace, name, downloadDuration)

		if task.BaseModel != nil {
			s.logger.Infof("Successfully downloaded BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
		} else {
			s.logger.Infof("Successfully downloaded ClusterBaseModel %s", task.ClusterBaseModel.Name)
		}

		// mark the model as Ready
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Ready,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		err = s.safeNodeLabelerProcessOp(nodeLabelOp)
		if err != nil {
			s.logger.Errorf("Failed to mark model %s as Ready: %v", modelInfo, err)
			return err
		}
	case Delete:
		switch storageType {
		case storage.StorageTypeOCI:
			s.logger.Infof("Starting deletion for model %s", modelInfo)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)
			err = s.deleteModel(destPath)
			if err != nil {
				s.logger.Errorf("Failed to delete model %s: %v", modelInfo, err)
				return err
			}
			if task.BaseModel != nil {
				s.logger.Infof("Successfully deleted the BaseModel %s in namespace %s", task.BaseModel.Name, task.BaseModel.Namespace)
			} else {
				s.logger.Infof("Successfully deleted the ClusterBaseModel %s", task.ClusterBaseModel.Name)
			}
		case storage.StorageTypeVendor:
			s.logger.Infof("Skipping deletion for model %s", modelInfo)
		}
	}

	return nil
}

func getModelInfoForLogging(task *GopherTask) string {
	if task.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", task.BaseModel.Namespace, task.BaseModel.Name)
	} else if task.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", task.ClusterBaseModel.Name)
	}
	return "unknown model"
}

func (s *Gopher) markModelOnNodeFailed(task *GopherTask) {
	modelInfo := getModelInfoForLogging(task)
	s.logger.Infof("Marking model %s as Failed on node", modelInfo)

	nodeLabelOp := &NodeLabelOp{
		ModelStateOnNode: Failed,
		BaseModel:        task.BaseModel,
		ClusterBaseModel: task.ClusterBaseModel,
	}

	err := s.safeNodeLabelerProcessOp(nodeLabelOp)
	if err != nil {
		s.logger.Errorf("Failed to mark model %s as Failed on node: %v", modelInfo, err)
	} else {
		s.logger.Infof("Successfully marked model %s as Failed on node", modelInfo)
	}
}

func getDestPath(baseModel *v1beta1.BaseModelSpec, modelRootDir string) string {

	storagePath := *baseModel.Storage.StorageUri
	destPath := *baseModel.Storage.Path

	if len(destPath) == 0 {
		if strings.HasSuffix(modelRootDir, "/") {
			return modelRootDir + storagePath
		} else {
			return modelRootDir + "/" + storagePath
		}
	}

	return destPath
}

// getTargetDirPath determines the target directory path for a model based on its storage configuration
func getTargetDirPath(baseModel *v1beta1.BaseModelSpec) (*casper.ObjectURI, error) {

	storagePath := *baseModel.Storage.StorageUri

	osUri, err := storage.NewObjectURI(storagePath)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(osUri.Prefix, "/") {
		osUri.Prefix = osUri.Prefix + "/"
	}

	return osUri, nil

}

func (s *Gopher) downloadModel(uri *casper.ObjectURI, destPath string, shapeFilter *TensorRTLLMShapeFilter, task *GopherTask) error {
	startTime := time.Now()
	defer func() {
		s.logger.Infof("Download process took %v", time.Since(startTime).Round(time.Millisecond))
	}()

	s.logger.Infof("Making call to object storage with endpoint %s", s.casperDataStore.Client.Endpoint())
	objects, err := s.casperDataStore.ListObjects(*uri)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	s.logger.Infof("Done with list all %d objects in model bucket folder", len(objects))

	// Shape filtering for TensorRTLLM
	if shapeFilter != nil && shapeFilter.IsTensorrtLLMModel && shapeFilter.ModelType == string(constants.ServingBaseModel) {
		s.logger.Infof("TensorRTLLM Serving model detected. Start filtering model files that doesn't belong to the node shape %s in model bucket folder", shapeFilter.ShapeAlias)
		shapeFilteredObjects := make([]objectstorage.ObjectSummary, 0)
		for _, object := range objects {
			if object.Name != nil {
				if strings.Contains(*object.Name, fmt.Sprintf("/%s/", shapeFilter.ShapeAlias)) {
					shapeFilteredObjects = append(shapeFilteredObjects, object)
				}
			}
		}
		objects = shapeFilteredObjects

		if len(objects) == 0 {
			return fmt.Errorf("no suitable objects found for shape %s", shapeFilter.ShapeAlias)
		}
		s.logger.Infof("Found %d objects applicable for shape %s", len(objects), shapeFilter.ShapeAlias)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	downloadOpts := casper.DownloadOptions{
		Threads:             s.multipartConcurrency,
		ChunkSizeInMB:       BigFileSizeInMB,
		SizeThresholdInMB:   BigFileSizeInMB,
		DisableOverride:     true,
		JoinWithTailOverlap: true,
	}

	var objectUris []casper.ObjectURI
	for _, obj := range objects {
		if obj.Name == nil {
			continue
		}
		objectUris = append(objectUris, casper.ObjectURI{
			Namespace:  uri.Namespace,
			BucketName: uri.BucketName,
			ObjectName: *obj.Name,
			Prefix:     uri.Prefix,
		})
	}

	errs := s.casperDataStore.BulkDownload(objectUris, destPath, downloadOpts, s.concurrency)
	if errs != nil {
		return fmt.Errorf("failed to download objects: %v", errs)
	}

	// Perform final verification of all downloaded files
	s.logger.Info("Performing final integrity verification of all downloaded files...")
	verificationStartTime := time.Now()
	verificationErrors := s.verifyDownloadedFiles(objectUris, destPath)
	verificationDuration := time.Since(verificationStartTime)

	// Record verification duration
	s.metrics.ObserveVerificationDuration(verificationDuration)

	if len(verificationErrors) > 0 {
		s.logger.Errorf("Final verification failed for %d files", len(verificationErrors))
		errMsgs := make([]string, 0, len(verificationErrors))
		for file, err := range verificationErrors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", file, err))
			s.logger.Errorf("Verification failed for %s: %v", file, err)
		}
		return fmt.Errorf("integrity verification failed for %d/%d files: %s", len(verificationErrors), len(objects), strings.Join(errMsgs, "; "))
	}

	s.logger.Infof("All files downloaded and verified successfully (%d files, verification took %v)", len(objects), verificationDuration.Round(time.Millisecond))
	return nil
}

func (s *Gopher) verifyDownloadedFiles(uris []casper.ObjectURI, destPath string) map[string]error {
	errors := make(map[string]error)
	for _, obj := range uris {
		relativeName := casper.JoinWithTailOverlap(destPath, obj.ObjectName)
		// Fallback: if relativeName is empty, use the object name directly
		if relativeName == "" {
			relativeName = obj.ObjectName
		}

		valid, err := s.casperDataStore.IsLocalCopyValid(obj, relativeName)
		if err != nil {
			errors[obj.ObjectName] = err
			continue
		}
		if !valid {
			errors[obj.ObjectName] = fmt.Errorf("MD5 or size mismatch for %s", obj.ObjectName)
		}
	}
	return errors
}

func (s *Gopher) deleteModel(destPath string) error {
	return os.RemoveAll(destPath)
}
