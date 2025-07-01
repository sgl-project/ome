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

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/storage"
	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
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
	configMapMutex       sync.Mutex             // Mutex to coordinate ConfigMap access
	storageFactory       storage.StorageFactory // Storage factory for multi-cloud support
}

const (
	BigFileSizeInMB = 200
)

func NewGopher(
	modelConfigParser *ModelConfigParser,
	configMapReconciler *ConfigMapReconciler,
	hubClient *hub.HubClient,
	kubeClient kubernetes.Interface,
	storageFactory storage.StorageFactory,
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

	if storageFactory == nil {
		return nil, fmt.Errorf("storage factory cannot be nil")
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
		storageFactory:       storageFactory,
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

	storageType, err := utilstorage.GetStorageType(*baseModelSpec.Storage.StorageUri)

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
		case utilstorage.StorageTypeOCI:
			// For OCI and other cloud storage types
			osUri, err := getTargetDirPath(&baseModelSpec)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)
			if err != nil {
				s.logger.Errorf("Failed to get target directory path for model %s: %v", modelInfo, err)
				return err
			}

			// Download using the new multi-cloud storage interface
			err = s.downloadModel(osUri, destPath, task)
			if err != nil {
				s.logger.Errorf("Download failed for model %s: %v", modelInfo, err)

				// Record download failure in metrics
				errorType := "download_error"
				if strings.Contains(err.Error(), "checksum") || strings.Contains(err.Error(), "verification") {
					errorType = "verification_error"
				}
				s.metrics.RecordFailedDownload(modelType, namespace, name, errorType)

				s.markModelOnNodeFailed(task)
				return err
			}

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
		case utilstorage.StorageTypeVendor:
			s.logger.Infof("Skipping download for model %s", modelInfo)
		case utilstorage.StorageTypeHuggingFace:
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
		case utilstorage.StorageTypeOCI:
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
		case utilstorage.StorageTypeVendor:
			s.logger.Infof("Skipping deletion for model %s", modelInfo)
		case utilstorage.StorageTypeHuggingFace:
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
func getTargetDirPath(baseModel *v1beta1.BaseModelSpec) (*storage.ObjectURI, error) {
	storagePath := *baseModel.Storage.StorageUri

	// Parse the storage URI using the storage package parser
	objectURI, err := storage.ParseURI(storagePath)
	if err != nil {
		return nil, err
	}

	// Note: We don't add a trailing slash to the prefix as it might affect OCI API behavior

	return objectURI, nil
}

// createStorageClient creates a storage client based on the model's storage configuration
func (s *Gopher) createStorageClient(ctx context.Context, baseModelSpec v1beta1.BaseModelSpec) (storage.Storage, storage.Provider, *storage.ObjectURI, error) {
	// Parse the storage URI using the storage package's parser
	objectURI, err := storage.ParseURI(*baseModelSpec.Storage.StorageUri)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to parse storage URI: %w", err)
	}

	// Use StorageConfig which properly implements AuthConfigExtractor
	storageConfig := &storage.StorageConfig{
		Provider: objectURI.Provider,
		Region:   objectURI.Region,
		AuthConfig: auth.Config{
			Provider: auth.Provider(objectURI.Provider),
			AuthType: getDefaultAuthType(objectURI.Provider),
			Extra:    make(map[string]interface{}),
		},
		Extra: make(map[string]interface{}),
	}

	// Set provider-specific fields in Extra map
	storageConfig.Extra["region"] = objectURI.Region
	// Note: For OCI, namespace is not the same as compartment ID
	// Compartment ID should come from parameters or use tenancy root compartment

	// Override with parameters if provided
	if baseModelSpec.Storage.Parameters != nil {
		params := *baseModelSpec.Storage.Parameters

		// Update auth type if specified
		if authType, ok := params["auth"]; ok {
			storageConfig.AuthConfig.AuthType = auth.AuthType(authType)
		}

		// Update region if specified
		if region, ok := params["region"]; ok {
			storageConfig.Region = region
			storageConfig.AuthConfig.Region = region
			storageConfig.Extra["region"] = region
		}

		// Copy all parameters to Extra map and auth Extra
		for k, v := range params {
			if k != "auth" {
				storageConfig.Extra[k] = v
			}
			storageConfig.AuthConfig.Extra[k] = v
		}
	}

	// Add storage operation configuration
	storageConfig.Extra["concurrency"] = s.concurrency
	storageConfig.Extra["part_size_mb"] = BigFileSizeInMB
	storageConfig.Extra["retries"] = s.downloadRetry

	// Handle Kubernetes secret if specified
	if baseModelSpec.Storage.StorageKey != nil && *baseModelSpec.Storage.StorageKey != "" {
		storageConfig.AuthConfig.Extra["secret_name"] = *baseModelSpec.Storage.StorageKey
	}

	// Create storage client using StorageConfig
	// The factory will extract auth config from it and pass it to provider factory
	storageClient, err := s.storageFactory.Create(ctx, objectURI.Provider, storageConfig)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return storageClient, objectURI.Provider, objectURI, nil
}

// getDefaultAuthType returns the default auth type for a provider
func getDefaultAuthType(provider storage.Provider) auth.AuthType {
	switch provider {
	case storage.ProviderOCI:
		return auth.OCIInstancePrincipal
	case storage.ProviderAWS:
		return auth.AWSInstanceProfile
	case storage.ProviderGCP:
		return auth.GCPApplicationDefault
	case storage.ProviderAzure:
		return auth.AzureManagedIdentity
	default:
		return ""
	}
}

func (s *Gopher) downloadModel(uri *storage.ObjectURI, destPath string, task *GopherTask) error {
	startTime := time.Now()
	defer func() {
		s.logger.Infof("Download process took %v", time.Since(startTime).Round(time.Millisecond))
	}()

	// Get model type, namespace, and name for metrics
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	// Get the model spec
	var baseModelSpec v1beta1.BaseModelSpec
	if task.BaseModel != nil {
		baseModelSpec = task.BaseModel.Spec
	} else {
		baseModelSpec = task.ClusterBaseModel.Spec
	}

	// Create storage client using new factory
	ctx := context.Background()
	storageClient, provider, _, err := s.createStorageClient(ctx, baseModelSpec)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	// List objects in the bucket
	// Note: The URI already contains the prefix, so we don't need to specify it again in ListOptions
	s.logger.Infof("Listing objects in bucket %s with prefix %s", uri.BucketName, uri.Prefix)
	listOpts := storage.ListOptions{}
	objects, err := storageClient.List(ctx, *uri, listOpts)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return fmt.Errorf("no objects found in bucket %s with prefix %s", uri.BucketName, uri.Prefix)
	}

	s.logger.Infof("Found %d objects in model bucket folder", len(objects))

	// Shape filtering for TensorRTLLM
	if task.TensorRTLLMShapeFilter != nil && task.TensorRTLLMShapeFilter.IsTensorrtLLMModel &&
		task.TensorRTLLMShapeFilter.ModelType == string(constants.ServingBaseModel) {
		s.logger.Infof("TensorRTLLM model detected. Filtering for shape %s", task.TensorRTLLMShapeFilter.ShapeAlias)

		var filteredObjects []storage.ObjectInfo
		for _, obj := range objects {
			if strings.Contains(obj.Name, fmt.Sprintf("/%s/", task.TensorRTLLMShapeFilter.ShapeAlias)) {
				filteredObjects = append(filteredObjects, obj)
			}
		}
		objects = filteredObjects

		if len(objects) == 0 {
			return fmt.Errorf("no suitable objects found for shape %s", task.TensorRTLLMShapeFilter.ShapeAlias)
		}
		s.logger.Infof("Found %d objects applicable for shape %s", len(objects), task.TensorRTLLMShapeFilter.ShapeAlias)
	}

	// Check if provider supports bulk download
	if bulkStorage, ok := storageClient.(storage.BulkStorage); ok {
		s.logger.Info("Using bulk download")

		// Bulk download is handled differently now

		// Prepare bulk download options
		bulkOpts := storage.BulkDownloadOptions{
			Concurrency: s.concurrency,
			DownloadOptions: storage.DownloadOptions{
				Threads:       s.multipartConcurrency,
				ChunkSizeInMB: BigFileSizeInMB,
			},
		}

		// Prepare object URIs for bulk download
		var objectURIs []storage.ObjectURI
		for _, obj := range objects {
			objectURIs = append(objectURIs, storage.ObjectURI{
				Provider:   uri.Provider,
				Namespace:  uri.Namespace,
				BucketName: uri.BucketName,
				ObjectName: obj.Name,
			})
		}

		// Perform bulk download
		results, err := bulkStorage.BulkDownload(ctx, objectURIs, destPath, bulkOpts)
		if err != nil {
			return fmt.Errorf("bulk download failed: %w", err)
		}

		// Check for errors in results
		var errors []error
		for _, result := range results {
			if result.Error != nil {
				errors = append(errors, result.Error)
			}
		}
		if len(errors) > 0 {
			var errMsgs []string
			for _, err := range errors {
				errMsgs = append(errMsgs, err.Error())
			}
			return fmt.Errorf("bulk download failed: %s", strings.Join(errMsgs, "; "))
		}
	} else {
		// Fallback to sequential downloads
		s.logger.Info("Using sequential download (provider does not support bulk download)")

		for _, obj := range objects {
			// Calculate destination path
			relativePath := strings.TrimPrefix(obj.Name, uri.Prefix)
			if strings.HasPrefix(relativePath, "/") {
				relativePath = relativePath[1:]
			}
			localPath := filepath.Join(destPath, relativePath)

			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			// Download the object
			s.logger.Debugf("Downloading %s to %s", obj.Name, localPath)
			objURI := storage.ObjectURI{
				Provider:   uri.Provider,
				Namespace:  uri.Namespace,
				BucketName: uri.BucketName,
				ObjectName: obj.Name,
			}
			err := utils.Retry(s.downloadRetry, 100*time.Millisecond, func() error {
				return storageClient.Download(ctx, objURI, localPath)
			})
			if err != nil {
				return fmt.Errorf("failed to download %s: %w", obj.Name, err)
			}
		}
	}

	// Perform verification
	s.logger.Info("Performing integrity verification of downloaded files...")
	verificationStartTime := time.Now()
	verificationErrors := s.verifyDownloadedFiles(ctx, storageClient, uri, objects, destPath, task)
	verificationDuration := time.Since(verificationStartTime)

	// Record verification duration
	s.metrics.ObserveVerificationDuration(verificationDuration)

	if len(verificationErrors) > 0 {
		s.logger.Errorf("Verification failed for %d files", len(verificationErrors))
		var errMsgs []string
		for file, err := range verificationErrors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", file, err))
			s.logger.Errorf("Verification failed for %s: %v", file, err)
		}
		return fmt.Errorf("integrity verification failed for %d/%d files: %s",
			len(verificationErrors), len(objects), strings.Join(errMsgs, "; "))
	}

	// Calculate and record total bytes transferred
	var totalBytes int64
	for _, obj := range objects {
		totalBytes += obj.Size
	}
	s.metrics.RecordBytesTransferred(modelType, namespace, name, totalBytes)

	s.logger.Infof("Provider %s: Downloaded and verified %d files (%d bytes) in %v",
		provider, len(objects), totalBytes, time.Since(startTime).Round(time.Millisecond))
	return nil
}

func (s *Gopher) verifyDownloadedFiles(ctx context.Context, storageClient storage.Storage, uri *storage.ObjectURI, objects []storage.ObjectInfo, destPath string, task *GopherTask) map[string]error {
	errors := make(map[string]error)

	for _, obj := range objects {
		// Calculate local path
		relativePath := strings.TrimPrefix(obj.Name, uri.Prefix)
		if strings.HasPrefix(relativePath, "/") {
			relativePath = relativePath[1:]
		}
		localPath := filepath.Join(destPath, relativePath)

		// Check if file exists
		fileInfo, err := os.Stat(localPath)
		if err != nil {
			errors[obj.Name] = fmt.Errorf("file not found: %w", err)
			continue
		}

		// Verify size
		if fileInfo.Size() != obj.Size {
			errors[obj.Name] = fmt.Errorf("size mismatch: expected %d, got %d", obj.Size, fileInfo.Size())
			continue
		}

		// Verify checksum if available
		if obj.ETag != "" {
			// Calculate local file checksum
			file, err := os.Open(localPath)
			if err != nil {
				errors[obj.Name] = fmt.Errorf("failed to open file for verification: %w", err)
				continue
			}
			defer file.Close()

			// For now, we'll trust size verification
			// Additional checksum verification can be added if the storage client supports it
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
	hfComponents, err := utilstorage.ParseHuggingFaceStorageURI(*baseModelSpec.Storage.StorageUri)
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
