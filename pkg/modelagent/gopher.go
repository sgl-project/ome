package modelagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgl-project/ome/pkg/utils"

	"github.com/sgl-project/ome/pkg/constants"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	configMapReconciler  *ConfigMapReconciler
	downloadRetry        int
	concurrency          int
	multipartConcurrency int
	modelRootDir         string
	hubClient            *hub.HubClient
	kubeClient           kubernetes.Interface
	gopherChan           <-chan *GopherTask
	nodeLabelReconciler  *NodeLabelReconciler
	metrics              *Metrics
	logger               *zap.SugaredLogger
	configMapMutex       sync.Mutex // Mutex to coordinate ConfigMap access
}

const (
	BigFileSizeInMB = 200
)

func NewGopher(
	modelConfigParser *ModelConfigParser,
	configMapReconciler *ConfigMapReconciler,
	hubClient *hub.HubClient,
	kubeClient kubernetes.Interface,
	concurrency int,
	multipartConcurrency int,
	downloadRetry int,
	modelRootDir string,
	gopherChan <-chan *GopherTask,
	nodeLabelReconciler *NodeLabelReconciler,
	metrics *Metrics,
	logger *zap.SugaredLogger) (*Gopher, error) {

	if hubClient == nil {
		return nil, fmt.Errorf("hugging face hub client cannot be nil")
	}

	return &Gopher{
		modelConfigParser:    modelConfigParser,
		configMapReconciler:  configMapReconciler,
		downloadRetry:        downloadRetry,
		concurrency:          concurrency,
		multipartConcurrency: multipartConcurrency,
		modelRootDir:         modelRootDir,
		hubClient:            hubClient,
		kubeClient:           kubeClient,
		gopherChan:           gopherChan,
		nodeLabelReconciler:  nodeLabelReconciler,
		metrics:              metrics,
		logger:               logger,
	}, nil
}

func (s *Gopher) Run(stopCh <-chan struct{}, numWorker int) {
	// Start the ConfigMap reconciliation service
	s.configMapReconciler.StartReconciliation()
	s.logger.Info("Started ConfigMap reconciliation service")

	// Start worker goroutines
	for i := 0; i < numWorker; i++ {
		go s.runWorker()
	}

	// Wait for stop signal
	<-stopCh

	// Stop the ConfigMap reconciliation service
	s.configMapReconciler.StopReconciliation()
	s.logger.Info("Stopped ConfigMap reconciliation service")

	s.logger.Info("Received stop signal, shutting down Gopher workers...")
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

// safeNodeLabelReconciliation executes the NodeLabelReconciler's ReconcileNodeLabels method with mutex protection
// to ensure thread-safe ConfigMap updates
func (s *Gopher) safeNodeLabelReconciliation(op *NodeLabelOp) error {
	ctx := context.Background()
	s.configMapMutex.Lock()
	defer s.configMapMutex.Unlock()

	// Mark the node label
	err := s.nodeLabelReconciler.ReconcileNodeLabels(op)
	if err != nil {
		return err
	}

	// Also update the ConfigMap with model status
	if op.BaseModel != nil || op.ClusterBaseModel != nil {
		// Convert ModelStateOnNode to ModelStatus
		var status ModelStatus
		switch op.ModelStateOnNode {
		case Ready:
			status = ModelStatusReady
		case Updating:
			status = ModelStatusUpdating
		case Failed:
			status = ModelStatusFailed
		case Deleted:
			// For deletion, use the DeleteModelFromConfigMap method instead
			return s.configMapReconciler.DeleteModelFromConfigMap(ctx, op.BaseModel, op.ClusterBaseModel)
		}

		// Create StatusOp for ConfigMap update
		statusOp := &ConfigMapStatusOp{
			ModelStatus:      status,
			BaseModel:        op.BaseModel,
			ClusterBaseModel: op.ClusterBaseModel,
		}

		// Update the ConfigMap with model status
		return s.configMapReconciler.ReconcileModelStatus(ctx, statusOp)
	}

	return nil
}

// safeParseAndUpdateModelConfig executes the ModelConfigParser's ParseAndUpdateModelConfig method with mutex protection
// to ensure thread-safe ConfigMap updates
func (s *Gopher) safeParseAndUpdateModelConfig(modelPath string, baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) error {
	ctx := context.Background()
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
		op := &ConfigMapMetadataOp{
			ModelMetadata:    *metadata,
			BaseModel:        baseModel,
			ClusterBaseModel: clusterBaseModel,
		}

		// Update the ConfigMap with model configuration
		// Since we're holding the lock, we can call the ReconcileModelMetadata method directly
		return s.configMapReconciler.ReconcileModelMetadata(ctx, op)
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

	// For Download and DownloadOverride tasks, set the node label to "Updating"
	if task.TaskType == Download || task.TaskType == DownloadOverride {
		s.logger.Infof("Setting model %s status to Updating before download", modelInfo)
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Updating,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		if err := s.safeNodeLabelReconciliation(nodeLabelOp); err != nil {
			s.logger.Errorf("Failed to set model %s status to Updating: %v", modelInfo, err)
			// Continue with download anyway
		}
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
			osUri, err := getTargetDirPath(&baseModelSpec)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)
			if err != nil {
				s.logger.Errorf("Failed to get target directory path for model %s: %v", modelInfo, err)
				return err
			}
			err = utils.Retry(s.downloadRetry, 100*time.Millisecond, func() error {
				downloadErr := s.downloadModel(osUri, destPath, task)
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
		case storage.StorageTypeHuggingFace:
			s.logger.Infof("Starting Hugging Face download for model %s", modelInfo)

			// Handle Hugging Face model download
			if err := s.processHuggingFaceModel(task, baseModelSpec, modelInfo, modelType, namespace, name); err != nil {
				// Error is already logged and metrics recorded in the method
				return err
			}
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

		// mark the model as Ready on both node labels and ConfigMap
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Ready,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		// This will update both the node label and ConfigMap status
		err = s.safeNodeLabelReconciliation(nodeLabelOp)
		if err != nil {
			s.logger.Errorf("Failed to mark model %s as Ready: %v", modelInfo, err)
			return err
		}
	case Delete:
		switch storageType {
		case storage.StorageTypeOCI:
			s.logger.Infof("Starting deletion for model %s", modelInfo)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)
			err = s.deleteModel(destPath, task)
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
		case storage.StorageTypeHuggingFace:
			s.logger.Infof("Removing Hugging Face model %s", modelInfo)
			modelRepo := strings.TrimPrefix(*baseModelSpec.Storage.StorageUri, "hf://")
			destPath := filepath.Join(s.modelRootDir, modelRepo)
			err = s.deleteModel(destPath, task)
			if err != nil {
				s.logger.Errorf("Failed to delete Hugging Face model %s: %v", modelInfo, err)
				return err
			}
			s.logger.Infof("Successfully deleted Hugging Face model %s", modelInfo)
		}

		// Mark the model as deleted in the node labels and remove from ConfigMap
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Deleted,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		err = s.safeNodeLabelReconciliation(nodeLabelOp)
		if err != nil {
			s.logger.Errorf("Failed to mark model %s as deleted: %v", modelInfo, err)
			return err
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

	// This will update both node label and ConfigMap status
	err := s.safeNodeLabelReconciliation(nodeLabelOp)
	if err != nil {
		s.logger.Errorf("Failed to mark model %s as Failed on node: %v", modelInfo, err)
	} else {
		s.logger.Infof("Successfully marked model %s as Failed on node", modelInfo)
	}
}

// getHuggingFaceToken retrieves authentication token for Hugging Face models.
// It attempts to get the token from either a Kubernetes secret or direct parameters.
func (s *Gopher) getHuggingFaceToken(task *GopherTask, baseModelSpec v1beta1.BaseModelSpec, modelInfo string) string {
	var hfToken string
	var namespace string

	// Get namespace depending on model type
	if task.BaseModel != nil {
		namespace = task.BaseModel.Namespace
	} else if task.ClusterBaseModel != nil {
		namespace = "" // ClusterBaseModels use the default namespace for secrets
	}

	// Try to get token from storage key first (Kubernetes secret)
	if baseModelSpec.Storage.StorageKey != nil && *baseModelSpec.Storage.StorageKey != "" {
		// Get the token from the referenced Kubernetes secret
		if s.kubeClient != nil {
			s.logger.Infof("Fetching Hugging Face token from secret %s for model %s", *baseModelSpec.Storage.StorageKey, modelInfo)

			secret, err := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), *baseModelSpec.Storage.StorageKey, metav1.GetOptions{})
			if err != nil {
				s.logger.Warnf("Failed to retrieve secret %s for Hugging Face token: %v", *baseModelSpec.Storage.StorageKey, err)
			} else if tokenBytes, exists := secret.Data["token"]; exists {
				hfToken = string(tokenBytes)
				s.logger.Infof("Successfully retrieved Hugging Face token from secret %s", *baseModelSpec.Storage.StorageKey)
			} else {
				s.logger.Warnf("Secret %s does not contain 'token' key", *baseModelSpec.Storage.StorageKey)
			}
		} else {
			s.logger.Warnf("Cannot fetch token: Kubernetes client not initialized")
		}
	}

	// Fallback to parameters if token not found in secret or no secret provided
	if hfToken == "" && baseModelSpec.Storage.Parameters != nil {
		if token, exists := (*baseModelSpec.Storage.Parameters)["token"]; exists {
			hfToken = token
			s.logger.Infof("Using token from Parameters for model %s", modelInfo)
		}
	}

	return hfToken
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
func getTargetDirPath(baseModel *v1beta1.BaseModelSpec) (*ociobjectstore.ObjectURI, error) {

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

// createOCIOSDataStore creates an OCIOSDataStore client based on storage parameters in the model spec
func (s *Gopher) createOCIOSDataStore(baseModelSpec v1beta1.BaseModelSpec) (*ociobjectstore.OCIOSDataStore, error) {
	// Default auth type is InstancePrincipal if not specified
	authType := principals.InstancePrincipal

	// Check if auth type is specified in the storage parameters
	if baseModelSpec.Storage.Parameters != nil {
		if authTypeStr, ok := (*baseModelSpec.Storage.Parameters)["auth"]; ok && authTypeStr != "" {
			// Convert string to AuthenticationType
			authType = principals.AuthenticationType(authTypeStr)
			s.logger.Infof("Using auth type from model parameters: %s", authType)
		}
	}

	// Create OCI Object Store config with a proper logger adapter
	osConfig, err := ociobjectstore.NewConfig(
		ociobjectstore.WithAnotherLog(logging.ForZap(s.logger.Desugar())),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ociobjectstore config: %w", err)
	}

	// Set auth type
	osConfig.AuthType = &authType

	// Check for additional parameters like region
	if baseModelSpec.Storage.Parameters != nil {
		if region, ok := (*baseModelSpec.Storage.Parameters)["region"]; ok && region != "" {
			osConfig.Region = region
			s.logger.Infof("Using region from model parameters: %s", region)
		}
	}

	// Create OCIOSDataStore
	ociOSDS, err := ociobjectstore.NewOCIOSDataStore(osConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ociobjectstore data store: %w", err)
	}

	return ociOSDS, nil
}

func (s *Gopher) downloadModel(uri *ociobjectstore.ObjectURI, destPath string, task *GopherTask) error {
	startTime := time.Now()
	defer func() {
		s.logger.Infof("Download process took %v", time.Since(startTime).Round(time.Millisecond))
	}()

	// Get model type, namespace, and name for metrics outside the defer to use within function
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	// Get the model spec
	var baseModelSpec v1beta1.BaseModelSpec
	if task.BaseModel != nil {
		baseModelSpec = task.BaseModel.Spec
	} else {
		baseModelSpec = task.ClusterBaseModel.Spec
	}

	// Create oci object storage data store client for this task
	ociOSDataStore, err := s.createOCIOSDataStore(baseModelSpec)
	if err != nil {
		return fmt.Errorf("failed to create object storage client: %w", err)
	}

	s.logger.Infof("Making call to object storage with endpoint %s", ociOSDataStore.Client.Endpoint())
	objects, err := ociOSDataStore.ListObjects(*uri)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	s.logger.Infof("Done with list all %d objects in model bucket folder", len(objects))

	// Shape filtering for TensorRTLLM
	if task.TensorRTLLMShapeFilter != nil && task.TensorRTLLMShapeFilter.IsTensorrtLLMModel && task.TensorRTLLMShapeFilter.ModelType == string(constants.ServingBaseModel) {
		s.logger.Infof("TensorRTLLM Serving model detected. Start filtering model files that doesn't belong to the node shape %s in model bucket folder", task.TensorRTLLMShapeFilter.ShapeAlias)
		shapeFilteredObjects := make([]objectstorage.ObjectSummary, 0)
		for _, object := range objects {
			if object.Name != nil {
				if strings.Contains(*object.Name, fmt.Sprintf("/%s/", task.TensorRTLLMShapeFilter.ShapeAlias)) {
					shapeFilteredObjects = append(shapeFilteredObjects, object)
				}
			}
		}
		objects = shapeFilteredObjects

		if len(objects) == 0 {
			return fmt.Errorf("no suitable objects found for shape %s", task.TensorRTLLMShapeFilter.ShapeAlias)
		}
		s.logger.Infof("Found %d objects applicable for shape %s", len(objects), task.TensorRTLLMShapeFilter.ShapeAlias)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found under namespace %s, bucket %s, object prefix %s", uri.Namespace, uri.BucketName, uri.Prefix)
	}

	var objectUris []ociobjectstore.ObjectURI
	for _, obj := range objects {
		if obj.Name == nil {
			continue
		}
		objectUris = append(objectUris, ociobjectstore.ObjectURI{
			Namespace:  uri.Namespace,
			BucketName: uri.BucketName,
			ObjectName: *obj.Name,
			Prefix:     uri.Prefix,
		})
	}

	errs := ociOSDataStore.BulkDownload(objectUris, destPath, s.concurrency,
		ociobjectstore.WithThreads(s.multipartConcurrency),
		ociobjectstore.WithChunkSize(BigFileSizeInMB),
		ociobjectstore.WithSizeThreshold(BigFileSizeInMB),
		ociobjectstore.WithOverrideEnabled(false),
		ociobjectstore.WithStripPrefix(uri.Prefix))
	if errs != nil {
		return fmt.Errorf("failed to download objects: %v", errs)
	}

	// Perform final verification of all downloaded files
	s.logger.Info("Performing final integrity verification of all downloaded files...")
	verificationStartTime := time.Now()
	verificationErrors := s.verifyDownloadedFiles(ociOSDataStore, objectUris, destPath, task)
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

	// Calculate and record total bytes transferred
	var totalBytes int64
	for _, obj := range objects {
		if obj.Size != nil {
			totalBytes += *obj.Size
		}
	}
	s.metrics.RecordBytesTransferred(modelType, namespace, name, totalBytes)

	s.logger.Infof("All files downloaded and verified successfully (%d files, %d bytes, verification took %v)",
		len(objects), totalBytes, verificationDuration.Round(time.Millisecond))
	return nil
}

func (s *Gopher) verifyDownloadedFiles(ociOSDataStore *ociobjectstore.OCIOSDataStore, uris []ociobjectstore.ObjectURI, destPath string, task *GopherTask) map[string]error {
	errors := make(map[string]error)
	for _, obj := range uris {
		relativeName := filepath.Join(destPath, ociobjectstore.TrimObjectPrefix(obj.ObjectName, obj.Prefix))
		// Fallback: if relativeName is empty, use the object name directly
		if relativeName == "" {
			relativeName = obj.ObjectName
		}

		valid, err := ociOSDataStore.IsLocalCopyValid(obj, relativeName)
		if err != nil {
			errors[obj.ObjectName] = err
			continue
		}
		if !valid {
			errors[obj.ObjectName] = fmt.Errorf("MD5 or size mismatch for %s", obj.ObjectName)
		}
	}

	// Record verification result in metrics
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)
	s.metrics.RecordVerification(modelType, namespace, name, len(errors) == 0)

	return errors
}

func (s *Gopher) deleteModel(destPath string, task *GopherTask) error {
	startTime := time.Now()

	err := os.RemoveAll(destPath)

	// Log deletion time regardless of success or failure
	deleteTime := time.Since(startTime)
	s.logger.Infof("Model deletion from %s took %v", destPath, deleteTime.Round(time.Millisecond))

	// Record deletion in metrics if task is provided
	if task != nil {
		modelType, namespace, name := GetModelTypeNamespaceAndName(task)
		// We could add a dedicated deletion metric in the future
		// For now just log with context
		s.logger.Infof("Completed deletion of %s model %s/%s in %v",
			modelType, namespace, name, deleteTime.Round(time.Millisecond))
	}

	return err
}

// processHuggingFaceModel handles downloading models from Hugging Face Hub.
// It extracts model information from the URI, configures the download with proper authentication,
// performs the download using the hub client, and updates model configuration.
func (s *Gopher) processHuggingFaceModel(task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	modelInfo, modelType, namespace, name string) error {
	// Parse the Hugging Face URI to get modelID and branch
	hfComponents, err := storage.ParseHuggingFaceStorageURI(*baseModelSpec.Storage.StorageUri)
	if err != nil {
		s.logger.Errorf("Failed to parse Hugging Face URI for model %s: %v", modelInfo, err)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "invalid_hf_uri")
		s.markModelOnNodeFailed(task)
		return err
	}

	// Create destination path
	destPath := getDestPath(&baseModelSpec, s.modelRootDir)

	// Get Hugging Face token from storage key or parameters
	hfToken := s.getHuggingFaceToken(task, baseModelSpec, modelInfo)

	s.logger.Infof("Downloading HuggingFace model %s (revision: %s) to %s",
		hfComponents.ModelID, hfComponents.Branch, destPath)

	// Build download options for the hub client
	var downloadOptions []hub.DownloadOption

	// Set revision if specified
	if hfComponents.Branch != "" {
		downloadOptions = append(downloadOptions, hub.WithRevision(hfComponents.Branch))
	}

	// Set repository type (always model for HuggingFace)
	downloadOptions = append(downloadOptions, hub.WithRepoType(hub.RepoTypeModel))

	// Use the hub client to download the entire model repository
	ctx := context.Background()

	// If we have a token, we need to set it in the hub config
	// For now, we'll assume the token is already configured in the hub client
	// In a future enhancement, we could create a new client with the specific token
	if hfToken != "" {
		s.logger.Infof("Using authentication token for HuggingFace model %s", modelInfo)
	}

	// Perform snapshot download - the hub client already has built-in retry logic
	// with exponential backoff and proper 429 handling
	downloadPath, err := s.hubClient.SnapshotDownload(
		ctx,
		hfComponents.ModelID,
		destPath,
		downloadOptions...,
	)
	if err != nil {
		// Check error type for better handling
		var rateLimitErr *hub.RateLimitError
		var httpErr *hub.HTTPError

		switch {
		case errors.As(err, &rateLimitErr):
			// Proper rate limit error with retry-after information
			s.logger.Warnf("Rate limited while downloading HuggingFace model %s: %v", modelInfo, err)
			if rateLimitErr.RetryAfter > 0 {
				s.metrics.RecordRateLimit(modelType, namespace, name, rateLimitErr.RetryAfter)
			} else {
				s.metrics.RecordRateLimit(modelType, namespace, name, 30*time.Second) // Default estimate
			}
			s.metrics.RecordFailedDownload(modelType, namespace, name, "rate_limit_error")

		case errors.As(err, &httpErr) && httpErr.StatusCode == 429:
			// HTTP 429 without proper RateLimitError type
			s.logger.Warnf("Rate limited while downloading HuggingFace model %s: %v", modelInfo, err)
			s.metrics.RecordRateLimit(modelType, namespace, name, 30*time.Second) // Estimate
			s.metrics.RecordFailedDownload(modelType, namespace, name, "rate_limit_error")

		case strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit"):
			// Fallback string matching for backwards compatibility
			s.logger.Warnf("Rate limited while downloading HuggingFace model %s: %v", modelInfo, err)
			s.metrics.RecordRateLimit(modelType, namespace, name, 30*time.Second) // Estimate
			s.metrics.RecordFailedDownload(modelType, namespace, name, "rate_limit_error")

		default:
			s.logger.Errorf("Failed to download HuggingFace model %s: %v", modelInfo, err)
			s.metrics.RecordFailedDownload(modelType, namespace, name, "hf_download_error")
		}

		s.markModelOnNodeFailed(task)
		return err
	}

	s.logger.Infof("Successfully downloaded HuggingFace model %s to %s",
		modelInfo, downloadPath)

	// Parse model config and update ConfigMap
	var baseModel *v1beta1.BaseModel
	var clusterBaseModel *v1beta1.ClusterBaseModel

	if task.BaseModel != nil {
		baseModel = task.BaseModel
		s.logger.Debugf("Using BaseModel %s/%s for config parsing", baseModel.Namespace, baseModel.Name)
	} else if task.ClusterBaseModel != nil {
		clusterBaseModel = task.ClusterBaseModel
		s.logger.Debugf("Using ClusterBaseModel %s for config parsing", clusterBaseModel.Name)
	}

	_ = s.safeParseAndUpdateModelConfig(destPath, baseModel, clusterBaseModel)
	return nil
}
