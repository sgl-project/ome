package modelagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	omev1beta1lister "github.com/sgl-project/ome/pkg/client/listers/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/distributor"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/utils"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"github.com/sgl-project/ome/pkg/xet"
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
	modelConfigParser      *ModelConfigParser
	configMapReconciler    *ConfigMapReconciler
	downloadRetry          int
	concurrency            int
	multipartConcurrency   int
	modelRootDir           string
	xetConfig              *xet.Config
	kubeClient             kubernetes.Interface
	gopherChan             <-chan *GopherTask
	deleteChan             <-chan *GopherTask // Dedicated channel for delete tasks
	nodeLabelReconciler    *NodeLabelReconciler
	metrics                *Metrics
	logger                 *zap.SugaredLogger
	configMapMutex         sync.Mutex // Mutex to coordinate ConfigMap access
	baseModelLister        omev1beta1lister.BaseModelLister
	clusterBaseModelLister omev1beta1lister.ClusterBaseModelLister

	// Track active downloads for cancellation
	activeDownloads      map[string]context.CancelFunc // key: model UID
	activeDownloadsMutex sync.RWMutex

	// P2P distribution components
	p2pEnabled      bool
	p2pDistributor  *distributor.ModelDistributor
	p2pLeaseManager *P2PLeaseManager
	p2pTimeout      time.Duration
}

const (
	BigFileSizeInMB = 200
)

func NewGopher(
	modelConfigParser *ModelConfigParser,
	configMapReconciler *ConfigMapReconciler,
	xetConfig *xet.Config,
	kubeClient kubernetes.Interface,
	concurrency int,
	multipartConcurrency int,
	downloadRetry int,
	modelRootDir string,
	gopherChan <-chan *GopherTask,
	deleteChan <-chan *GopherTask,
	nodeLabelReconciler *NodeLabelReconciler,
	metrics *Metrics,
	logger *zap.SugaredLogger,
	baseModelLister omev1beta1lister.BaseModelLister,
	clusterBaseModelLister omev1beta1lister.ClusterBaseModelLister) (*Gopher, error) {

	if xetConfig == nil {
		return nil, fmt.Errorf("xet hugging face config cannot be nil")
	}

	return &Gopher{
		modelConfigParser:      modelConfigParser,
		configMapReconciler:    configMapReconciler,
		downloadRetry:          downloadRetry,
		concurrency:            concurrency,
		multipartConcurrency:   multipartConcurrency,
		modelRootDir:           modelRootDir,
		xetConfig:              xetConfig,
		kubeClient:             kubeClient,
		gopherChan:             gopherChan,
		deleteChan:             deleteChan,
		nodeLabelReconciler:    nodeLabelReconciler,
		metrics:                metrics,
		logger:                 logger,
		activeDownloads:        make(map[string]context.CancelFunc),
		baseModelLister:        baseModelLister,
		clusterBaseModelLister: clusterBaseModelLister,
		p2pTimeout:             time.Duration(constants.P2PDefaultP2PTimeoutSeconds) * time.Second,
	}, nil
}

// EnableP2P configures P2P distribution for the Gopher.
// This must be called before Run() if P2P is desired.
func (s *Gopher) EnableP2P(dist *distributor.ModelDistributor, leaseManager *P2PLeaseManager) {
	s.p2pEnabled = true
	s.p2pDistributor = dist
	s.p2pLeaseManager = leaseManager
	s.logger.Info("P2P distribution enabled")
}

// SetP2PTimeout sets the timeout for P2P download attempts.
func (s *Gopher) SetP2PTimeout(timeout time.Duration) {
	s.p2pTimeout = timeout
}

// computeModelHash generates a hash for the model to use in P2P coordination.
// The hash is based on the HuggingFace model ID and revision.
func computeModelHash(modelID, revision string) string {
	input := modelID
	if revision != "" {
		input = modelID + "@" + revision
	}
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func (s *Gopher) Run(stopCh <-chan struct{}, numWorker int) {
	// Start the ConfigMap reconciliation service
	s.configMapReconciler.StartReconciliation()
	s.logger.Info("Started ConfigMap reconciliation service")

	// Start dedicated delete worker - runs separately from download workers
	// to ensure deletions are never blocked by downloads
	go s.runDeleteWorker()
	s.logger.Info("Started dedicated delete worker")

	// Start download worker goroutines
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

// runDeleteWorker is a dedicated worker for processing delete tasks.
// This worker runs separately from download workers to ensure deletions
// are never blocked by downloads (even with 100 concurrent downloads).
// Cancellation of active downloads is handled inside processTask.
func (s *Gopher) runDeleteWorker() {
	for {
		select {
		case task, ok := <-s.deleteChan:
			if ok {
				err := s.processTask(task)
				if err != nil {
					s.logger.Errorf("Delete task failed with error: %s", err.Error())
				}
			} else {
				s.logger.Info("delete channel closed, delete worker exits.")
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
	// Use timeout context to prevent indefinite blocking on K8s API calls
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
func (s *Gopher) safeParseAndUpdateModelConfig(modelPath string, baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel, sha string) error {
	// Use timeout context to prevent indefinite blocking on K8s API calls
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s.configMapMutex.Lock()
	defer s.configMapMutex.Unlock()

	// First parse the configuration without updating the ConfigMap
	// This call will return model metadata
	metadata, err := s.modelConfigParser.ParseModelConfig(modelPath, baseModel, clusterBaseModel)
	if err != nil {
		return err
	}

	// add artifact info
	if sha != "" {
		metadata = s.modelConfigParser.populateArtifactAttribute(sha, modelPath, *metadata)
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
	modelUID := getModelUID(task)
	s.logger.Debugf("Processing gopher task: %s, type: %s", modelInfo, task.TaskType)

	// Get model type, namespace, and name for metrics
	modelType, namespace, name := GetModelTypeNamespaceAndName(task)

	var baseModelSpec v1beta1.BaseModelSpec
	if task.BaseModel != nil {
		baseModelSpec = task.BaseModel.Spec
	} else {
		baseModelSpec = task.ClusterBaseModel.Spec
	}

	// Create context - will be cancellable for downloads
	ctx := context.Background()
	var cancel context.CancelFunc

	// For Download and DownloadOverride tasks, set the node label to "Updating"
	if task.TaskType == Download || task.TaskType == DownloadOverride {
		s.logger.Debugf("Setting model %s status to Updating before download", modelInfo)
		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Updating,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}

		if err := s.safeNodeLabelReconciliation(nodeLabelOp); err != nil {
			s.logger.Errorf("Failed to set model %s status to Updating: %v", modelInfo, err)
			// Continue with download anyway
		}

		// Create a cancellable context for this download
		ctx, cancel = context.WithCancel(context.Background())

		// Check if there's already an active download for this model
		s.activeDownloadsMutex.Lock()
		if existingCancel, exists := s.activeDownloads[modelUID]; exists {
			// For DownloadOverride, don't cancel an existing download - let it finish
			// This prevents the cancel-restart cycle when seeder marks model Ready
			if task.TaskType == DownloadOverride {
				s.activeDownloadsMutex.Unlock()
				s.logger.Debugf("Download already in progress for model %s, skipping DownloadOverride", modelInfo)
				cancel() // Clean up the context we just created
				return nil
			}
			// For regular Download tasks, cancel the existing one
			s.logger.Debugf("Cancelling previous download for model %s due to new download task", modelInfo)
			existingCancel()
		}
		s.activeDownloads[modelUID] = cancel
		s.activeDownloadsMutex.Unlock()

		// Ensure cleanup on completion
		defer func() {
			s.activeDownloadsMutex.Lock()
			delete(s.activeDownloads, modelUID)
			s.activeDownloadsMutex.Unlock()
			cancel() // Ensure context is cancelled
		}()
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
		s.logger.Debugf("Starting download for model %s", modelInfo)

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
				downloadErr := s.downloadModel(ctx, osUri, destPath, task)
				if downloadErr != nil {
					// Check if context was cancelled
					if ctx.Err() != nil {
						s.logger.Debugf("Download cancelled for model %s: %v", modelInfo, ctx.Err())
						return ctx.Err()
					}
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

			// Check if context was cancelled (delete requested) - abort before config parsing
			if ctx.Err() != nil {
				s.logger.Debugf("Download cancelled for OCI model %s before config parsing: %v", modelInfo, ctx.Err())
				return ctx.Err()
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

			if err := s.safeParseAndUpdateModelConfig(destPath, baseModel, clusterBaseModel, ""); err != nil {
				s.logger.Errorf("Failed to parse and update model config: %v", err)
			}
		case storage.StorageTypeVendor:
			s.logger.Debugf("Skipping download for model %s", modelInfo)
		case storage.StorageTypeHuggingFace:
			s.logger.Debugf("Starting Hugging Face download for model %s", modelInfo)

			// Handle Hugging Face model download
			if err := s.processHuggingFaceModel(ctx, task, baseModelSpec, modelInfo, modelType, namespace, name); err != nil {
				// Error is already logged and metrics recorded in the method
				return err
			}
		case storage.StorageTypePVC:
			s.logger.Debugf("Skipping PVC storage type for model %s (handled by BaseModel controller)", modelInfo)
			// PVC storage is handled entirely by the BaseModel controller
			// Model agent doesn't need to do anything for PVC storage
			return nil
		case storage.StorageTypeLocal:
			s.logger.Debugf("Processing local storage type for model %s", modelInfo)
			// For local storage, we just need to validate the path exists and parse model config
			if err := s.processLocalStorageModel(ctx, task, baseModelSpec, modelInfo, modelType, namespace, name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown storage type %s", storageType)
		}
		// Calculate download duration
		downloadDuration := time.Since(downloadStartTime)

		// Check if context was cancelled (delete requested) - abort before marking as Ready
		// This is critical to prevent racing with the delete handler
		if ctx.Err() != nil {
			s.logger.Debugf("Download cancelled for model %s after download completed: %v", modelInfo, ctx.Err())
			return ctx.Err()
		}

		// Also check if model is being deleted - additional safety check
		if s.isModelBeingDeleted(task) {
			s.logger.Infof("Model %s is being deleted, aborting post-download processing", modelInfo)
			return fmt.Errorf("model %s is being deleted", modelInfo)
		}

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
		// First, cancel any ongoing download for this model
		s.activeDownloadsMutex.RLock()
		if cancelFunc, exists := s.activeDownloads[modelUID]; exists {
			s.logger.Debugf("Cancelling ongoing download for model %s", modelInfo)
			cancelFunc() // This will cancel the download context
		}
		s.activeDownloadsMutex.RUnlock()

		// Wait a bit for download to stop
		time.Sleep(2 * time.Second)

		// Now proceed with deletion
		switch storageType {
		case storage.StorageTypeOCI:
			s.logger.Debugf("Starting deletion for model %s", modelInfo)
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)

			// Double-check if the path is still referenced by other models
			isReferenced, err := s.isPathReferencedByOtherModels(destPath, task.BaseModel, task.ClusterBaseModel)
			if err != nil {
				// Cannot determine if the path is referenced; skip deletion to be safe
				s.logger.Errorf("Failed to check if path %s is referenced by other models, skip the path deletion: %v", destPath, err)
			} else if isReferenced {
				s.logger.Debugf("Skipping deletion of path %s for model %s as it is still referenced by other models", destPath, modelInfo)
			} else {
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
			}
		case storage.StorageTypeVendor:
			s.logger.Debugf("Skipping deletion for model %s", modelInfo)
		case storage.StorageTypeHuggingFace:
			s.logger.Infof("Removing Hugging Face model %s", modelInfo)
			// Use getDestPath to get the same path used during download
			destPath := getDestPath(&baseModelSpec, s.modelRootDir)

			// Double-check if the path is still referenced by other models
			isReferenced, err := s.isPathReferencedByOtherModels(destPath, task.BaseModel, task.ClusterBaseModel)
			if err != nil {
				// Cannot determine if the path is referenced; skip deletion to be safe
				s.logger.Errorf("Failed to check if path %s is referenced by other models, skip the path deletion: %v", destPath, err)
			} else if isReferenced {
				s.logger.Debugf("Skipping deletion of path %s for model %s as it is still referenced by other models", destPath, modelInfo)
			} else {
				err = s.deleteModel(destPath, task)
				if err != nil {
					s.logger.Errorf("Failed to delete Hugging Face model %s: %v", modelInfo, err)
					return err
				}
				s.logger.Infof("Successfully deleted Hugging Face model %s", modelInfo)
			}
		case storage.StorageTypeLocal:
			s.logger.Debugf("Skipping deletion for local storage model %s (local files should not be deleted)", modelInfo)
			// For local storage, we should NOT delete the actual files
			// Just update the node labels and ConfigMap to reflect removal
		case storage.StorageTypePVC:
			s.logger.Debugf("Skipping deletion for PVC storage model %s (handled by BaseModel controller)", modelInfo)
			// PVC storage is handled entirely by the BaseModel controller
			// Model agent doesn't delete PVC volumes
		default:
			s.logger.Warnf("Unsupported storage type %s for deletion of model %s", storageType, modelInfo)
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

		// Clean up the active downloads map
		s.activeDownloadsMutex.Lock()
		delete(s.activeDownloads, modelUID)
		s.activeDownloadsMutex.Unlock()
	}

	return nil
}

// isPathReferencedByOtherModels checks if the given path is still referenced by other BaseModel or ClusterBaseModel resources
// excluding the model being deleted
func (s *Gopher) isPathReferencedByOtherModels(targetPath string, excludeBaseModel *v1beta1.BaseModel, excludeClusterBaseModel *v1beta1.ClusterBaseModel) (bool, error) {
	// Check BaseModels
	baseModels, err := s.baseModelLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("failed to list BaseModels: %w", err)
	}

	for _, baseModel := range baseModels {
		// Skip the model being deleted
		if excludeBaseModel != nil && baseModel.Namespace == excludeBaseModel.Namespace && baseModel.Name == excludeBaseModel.Name {
			continue
		}

		// Check if this BaseModel references the same path
		if baseModel.Spec.Storage.Path != nil && *baseModel.Spec.Storage.Path == targetPath {
			s.logger.Infof("Path %s is still referenced by BaseModel %s/%s", targetPath, baseModel.Namespace, baseModel.Name)
			return true, nil
		}
	}

	// Check ClusterBaseModels
	clusterBaseModels, err := s.clusterBaseModelLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("failed to list ClusterBaseModels: %w", err)
	}

	for _, clusterBaseModel := range clusterBaseModels {
		// Skip the model being deleted
		if excludeClusterBaseModel != nil && clusterBaseModel.Name == excludeClusterBaseModel.Name {
			continue
		}

		// Check if this ClusterBaseModel references the same path
		if clusterBaseModel.Spec.Storage.Path != nil && *clusterBaseModel.Spec.Storage.Path == targetPath {
			s.logger.Infof("Path %s is still referenced by ClusterBaseModel %s", targetPath, clusterBaseModel.Name)
			return true, nil
		}
	}

	return false, nil
}

func getModelInfoForLogging(task *GopherTask) string {
	if task.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", task.BaseModel.Namespace, task.BaseModel.Name)
	} else if task.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", task.ClusterBaseModel.Name)
	}
	return "unknown model"
}

// getModelUID returns the unique identifier for a model
func getModelUID(task *GopherTask) string {
	if task.BaseModel != nil {
		return string(task.BaseModel.UID)
	} else if task.ClusterBaseModel != nil {
		return string(task.ClusterBaseModel.UID)
	}
	return ""
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
		// ClusterBaseModels look for secrets in the ome namespace by default
		namespace = "ome"
	}

	// Try to get token from storage key first (Kubernetes secret)
	if baseModelSpec.Storage.StorageKey != nil && *baseModelSpec.Storage.StorageKey != "" {
		// Get the token from the referenced Kubernetes secret
		if s.kubeClient != nil {
			s.logger.Debugf("Fetching Hugging Face token from secret %s in namespace %s for model %s", *baseModelSpec.Storage.StorageKey, namespace, modelInfo)

			secret, err := s.kubeClient.CoreV1().Secrets(namespace).Get(context.Background(), *baseModelSpec.Storage.StorageKey, metav1.GetOptions{})
			if err != nil {
				s.logger.Warnf("Failed to retrieve secret %s in namespace %s for Hugging Face token: %v", *baseModelSpec.Storage.StorageKey, namespace, err)
			} else {
				// Check if a custom secret key name is specified in parameters
				secretKeyName := "token" // default key name
				if baseModelSpec.Storage.Parameters != nil {
					if customKey, exists := (*baseModelSpec.Storage.Parameters)["secretKey"]; exists && customKey != "" {
						secretKeyName = customKey
						s.logger.Debugf("Using custom secret key name for model %s", modelInfo)
					}
				}

				// Try to get the token using the determined key name
				if tokenBytes, exists := secret.Data[secretKeyName]; exists {
					hfToken = string(tokenBytes)
					s.logger.Debugf("Successfully retrieved Hugging Face token from secret %s in namespace %s", *baseModelSpec.Storage.StorageKey, namespace)
				} else {
					s.logger.Warnf("Secret %s in namespace %s does not contain the expected token key", *baseModelSpec.Storage.StorageKey, namespace)
				}
			}
		} else {
			s.logger.Warnf("Cannot fetch token: Kubernetes client not initialized")
		}
	}

	// Fallback to parameters if token not found in secret or no secret provided
	if hfToken == "" && baseModelSpec.Storage.Parameters != nil {
		if token, exists := (*baseModelSpec.Storage.Parameters)["token"]; exists {
			hfToken = token
			s.logger.Debugf("Using token from Parameters for model %s", modelInfo)
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
			s.logger.Debugf("Using auth type from model parameters: %s", authType)
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
			s.logger.Debugf("Using region from model parameters: %s", region)
		}
	}

	// Create OCIOSDataStore
	ociOSDS, err := ociobjectstore.NewOCIOSDataStore(osConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ociobjectstore data store: %w", err)
	}

	return ociOSDS, nil
}

func (s *Gopher) downloadModel(ctx context.Context, uri *ociobjectstore.ObjectURI, destPath string, task *GopherTask) error {
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

	// Check context before making expensive operations
	select {
	case <-ctx.Done():
		return fmt.Errorf("download cancelled before listing objects: %w", ctx.Err())
	default:
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

	// Check context before starting bulk download
	select {
	case <-ctx.Done():
		return fmt.Errorf("download cancelled before starting bulk download: %w", ctx.Err())
	default:
	}

	// TODO: BulkDownload doesn't support context cancellation yet
	// This means downloads may continue even after deletion request
	// Future enhancement: modify ociobjectstore to support context
	errs := ociOSDataStore.BulkDownload(objectUris, destPath, s.concurrency,
		ociobjectstore.WithThreads(s.multipartConcurrency),
		ociobjectstore.WithChunkSize(BigFileSizeInMB),
		ociobjectstore.WithSizeThreshold(BigFileSizeInMB),
		ociobjectstore.WithOverrideEnabled(false),
		ociobjectstore.WithStripPrefix(uri.Prefix))
	if errs != nil {
		// Check if we were cancelled during download
		select {
		case <-ctx.Done():
			return fmt.Errorf("download cancelled during bulk download: %w", ctx.Err())
		default:
			return fmt.Errorf("failed to download objects: %v", errs)
		}
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
	if s.isReservingModelArtifact(task) {
		return nil
	}

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

// isReservingModelArtifact determines whether to preserve the model artifact directory during deletion.
// Behavior:
//   - Returns true if either ClusterBaseModel or BaseModel has the label models.ome/reserve-model-artifact
//     (constants.ReserveModelArtifact) set to "true" (case-insensitive).
//   - Returns false when the task is nil, labels are absent, or the label value is not "true".
//
// Precedence:
//   - If both BaseModel and ClusterBaseModel exist and at least one has the reserve label set to "true",
//     the function returns true (i.e., preserve the artifact).
func (s *Gopher) isReservingModelArtifact(task *GopherTask) bool {
	// Guard against nil task or BaseModel; reserve logic applies only to BaseModel labels
	if task == nil {
		s.logger.Infof("Model artifact will be deleted")
		return false
	}
	// for clusterBaseModel
	if task.ClusterBaseModel != nil && task.ClusterBaseModel.Labels != nil {
		if val, exists := task.ClusterBaseModel.Labels[constants.ReserveModelArtifact]; exists && strings.EqualFold(val, "true") {
			s.logger.Infof("Model artifact will be reserved as ClusterBaseModel has matched label")
			return true
		}
	}
	// for baseModel
	if task.BaseModel != nil && task.BaseModel.Labels != nil {
		if val, exists := task.BaseModel.Labels[constants.ReserveModelArtifact]; exists && strings.EqualFold(val, "true") {
			s.logger.Infof("Model artifact will be reserved as BaseModel has matched label")
			return true
		}
	}

	s.logger.Infof("Model artifact will be deleted")
	return false
}

// processHuggingFaceModel handles downloading models from Hugging Face Hub.
// It extracts model information from the URI, configures the download with proper authentication,
// performs the download using the hub client, and updates model configuration.
func (s *Gopher) processHuggingFaceModel(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
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

	// fetch sha value based on model ID from Huggingface model API
	shaStr, isShaAvailable := s.fetchSha(ctx, hfComponents.ModelID)
	isEligible, matchedModelTypeAndModeName, parentPath := s.isEligibleForOptimization(ctx, task, baseModelSpec, modelType, namespace, isShaAvailable, shaStr, hfComponents.ModelID)

	if isEligible {
		// create symbolic link
		err := utils.CreateSymbolicLink(destPath, parentPath)
		if err != nil {
			s.logger.Errorf("failed to create symbolic link from %s to %s for model %s: %s", destPath, parentPath, hfComponents.ModelID, err)
			return err
		}
		s.logger.Infof("successfully create symbolic link from %s to %s for model: %s", destPath, parentPath, modelInfo)
		// add path to childrenPaths in configmap
		err = s.configMapReconciler.updateConfigMapWithUpdatedChildrenPaths(ctx, matchedModelTypeAndModeName, destPath)
		if err != nil {
			s.logger.Errorf("fail to update configmap to add new path to childrenPaths: %s", err)
			return err
		}
		s.logger.Infof("successfully add the new path to childrenPath for model: %s", modelInfo)
	} else {
		// Compute model hash for P2P coordination
		modelHash := computeModelHash(hfComponents.ModelID, hfComponents.Branch)

		// Try P2P-aware download if enabled, otherwise fallback to direct HF download
		if err := s.downloadWithP2P(ctx, task, baseModelSpec, hfComponents, destPath, modelHash, modelInfo, modelType, namespace, name); err != nil {
			return err
		}
	}

	// Check if context was cancelled (delete requested) - abort before config parsing
	if ctx.Err() != nil {
		s.logger.Debugf("Download cancelled for model %s before config parsing: %v", modelInfo, ctx.Err())
		return ctx.Err()
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

	if err := s.safeParseAndUpdateModelConfig(destPath, baseModel, clusterBaseModel, shaStr); err != nil {
		s.logger.Errorf("Failed to parse and update model config: %v", err)
	}
	return nil
}

// downloadFromHuggingFace performs the actual download from HuggingFace Hub.
// This is extracted to allow P2P integration to share the download logic.
func (s *Gopher) downloadFromHuggingFace(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	hfComponents *storage.HuggingFaceStorageComponents, destPath, modelInfo, modelType, namespace, name string) error {

	// Get Hugging Face token from storage key or parameters
	hfToken := s.getHuggingFaceToken(task, baseModelSpec, modelInfo)

	s.logger.Infof("Downloading HuggingFace model %s (revision: %s) to %s",
		hfComponents.ModelID, hfComponents.Branch, destPath)

	// Init xet HF download config
	config := s.xetConfig.ToDownloadConfig()
	config.LocalDir = destPath
	config.RepoID = hfComponents.ModelID

	// Set revision if specified
	if hfComponents.Branch != "" {
		config.Revision = hfComponents.Branch
	}

	// If we have a token, pass it as a download option
	if hfToken != "" {
		s.logger.Debugf("Using authentication token for HuggingFace model %s", modelInfo)
		config.Token = hfToken
	}

	// Create progress handler for tracking download progress
	var lastBytes uint64
	var lastTime = time.Now()
	progressThrottle := 30 * time.Second // Update ConfigMap every 30 seconds

	progressHandler := func(update xet.ProgressUpdate) {
		now := time.Now()

		// Calculate speed (bytes per second)
		var speedBytesPerSec float64
		elapsed := now.Sub(lastTime).Seconds()
		if elapsed > 0 && update.CompletedBytes > lastBytes {
			speedBytesPerSec = float64(update.CompletedBytes-lastBytes) / elapsed
		}
		lastBytes = update.CompletedBytes
		lastTime = now

		// Create progress object
		progress := &DownloadProgress{
			Phase:            update.Phase.String(),
			TotalBytes:       update.TotalBytes,
			CompletedBytes:   update.CompletedBytes,
			TotalFiles:       update.TotalFiles,
			CompletedFiles:   update.CompletedFiles,
			SpeedBytesPerSec: speedBytesPerSec,
			LastUpdated:      now.Format(time.RFC3339),
		}

		// Update ConfigMap with progress (non-blocking)
		// Skip if context is cancelled to avoid unnecessary work
		if ctx.Err() == nil {
			go func() {
				progressOp := &ConfigMapProgressOp{
					Progress:         progress,
					BaseModel:        task.BaseModel,
					ClusterBaseModel: task.ClusterBaseModel,
				}
				if err := s.configMapReconciler.ReconcileModelProgress(ctx, progressOp); err != nil {
					// Only log if not cancelled - cancelled is expected during delete
					if ctx.Err() == nil {
						s.logger.Warnf("Failed to update download progress for %s: %v", modelInfo, err)
					}
				}
			}()
		}
	}

	// Perform snapshot download with progress tracking
	downloadPath, err := xet.SnapshotDownloadWithProgress(ctx, config, progressHandler, progressThrottle)

	// Check if context was cancelled (delete requested) - abort early
	if ctx.Err() != nil {
		s.logger.Debugf("Download cancelled for HuggingFace model %s: %v", modelInfo, ctx.Err())
		return ctx.Err()
	}

	if err != nil {
		// Check error type for better handling
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit") {
			s.logger.Warnf("Rate limited while downloading HuggingFace model %s: %v", modelInfo, err)
			s.metrics.RecordRateLimit(modelType, namespace, name, 30*time.Second)
			s.metrics.RecordFailedDownload(modelType, namespace, name, "rate_limit_error")
		} else {
			s.logger.Errorf("Failed to download HuggingFace model %s: %v", modelInfo, err)
			s.metrics.RecordFailedDownload(modelType, namespace, name, "hf_download_error")
		}

		s.markModelOnNodeFailed(task)
		return err
	}

	s.logger.Infof("Successfully downloaded HuggingFace model %s to %s", modelInfo, downloadPath)
	return nil
}

// waitForP2PAvailability waits for the model to become available via P2P.
// This is used when another node holds the download lease.
// It checks the lease status to determine whether to keep waiting:
// - If lease is complete: P2P should be available, try download
// - If lease exists and not expired: keep waiting (lease holder still downloading)
// - If lease expired or not found: give up (lease holder crashed)
// - If model is being deleted: abort early
func (s *Gopher) waitForP2PAvailability(ctx context.Context, task *GopherTask, modelHash, modelInfo, leaseName, destPath string) error {
	if s.p2pDistributor == nil {
		return fmt.Errorf("P2P distributor not configured")
	}

	// Use constants for configurable wait behavior
	checkInterval := time.Duration(constants.P2PDefaultWaitBaseDelayMs) * time.Millisecond
	maxWaitTime := time.Duration(constants.P2PMaxWaitTimeMinutes) * time.Minute
	startTime := time.Now()

	for {
		elapsed := time.Since(startTime)

		// Check context cancellation first
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check absolute maximum wait time
		if elapsed > maxWaitTime {
			return fmt.Errorf("absolute timeout waiting for P2P availability for model %s after %v", modelInfo, elapsed)
		}

		// Check if the model is being deleted - abort early to allow cleanup
		if s.isModelBeingDeleted(task) {
			s.logger.Infof("Model %s is being deleted, aborting P2P wait", modelInfo)
			return fmt.Errorf("model %s is being deleted, aborting P2P wait", modelInfo)
		}

		// Check lease status to decide whether to keep waiting
		lease, err := s.p2pLeaseManager.Get(ctx, leaseName)
		if err != nil {
			s.logger.Debugf("Failed to get lease %s: %v, will retry", leaseName, err)
		} else if lease != nil {
			// Check if lease is complete (download finished, seeding started)
			if s.p2pLeaseManager.IsComplete(lease) {
				s.logger.Infof("Lease %s is complete, P2P should be available for model %s", leaseName, modelInfo)
				// Give a short delay for seeding to fully start
				time.Sleep(2 * time.Second)
			} else if s.p2pLeaseManager.IsExpired(lease) {
				// Lease expired - holder might have crashed
				s.logger.Warnf("Lease %s expired for model %s, giving up on P2P wait", leaseName, modelInfo)
				return fmt.Errorf("lease expired while waiting for P2P availability for model %s", modelInfo)
			} else {
				// Lease is active but not complete - holder still downloading
				s.logger.Debugf("Lease %s still active (holder: %s) for model %s, waiting... (elapsed: %v)",
					leaseName, *lease.Spec.HolderIdentity, modelInfo, elapsed.Round(time.Second))
			}
		}

		// Check if model is available via P2P
		if s.p2pDistributor.HasPeers(ctx, modelHash) {
			s.logger.Infof("P2P peers now available for model %s, attempting download", modelInfo)
			if err := s.p2pDistributor.TryP2PDownload(ctx, modelHash, destPath, s.p2pTimeout); err == nil {
				s.logger.Infof("Successfully downloaded model %s via P2P after waiting %v", modelInfo, elapsed.Round(time.Second))
				return nil
			} else {
				s.logger.Warnf("P2P download attempt failed for model %s: %v", modelInfo, err)
			}
		}

		// Log progress periodically (every 30 seconds)
		if int(elapsed.Seconds())%30 == 0 && elapsed.Seconds() > 0 {
			s.logger.Infof("Still waiting for P2P availability for model %s (elapsed: %v)", modelInfo, elapsed.Round(time.Second))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
		}
	}
}

// downloadWithP2P orchestrates the model download with P2P support.
// The flow is:
//  1. If already seeding (in-memory state) → skip, we have it
//  2. Try to acquire lease first (determines if we're the HF downloader)
//  3. If lease acquired → download from HF (handles partial/resume) → start seeding
//  4. If lease not acquired → wait for P2P from lease holder
//  5. If P2P wait fails → fallback to HF download
//
// Key insight: The lease holder ALWAYS downloads from HF because HF handles
// partial file recovery and resume. We don't skip HF based on files existing
// on disk because they might be incomplete from an interrupted download.
func (s *Gopher) downloadWithP2P(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	hfComponents *storage.HuggingFaceStorageComponents, destPath, modelHash, modelInfo, modelType, namespace, name string) error {

	// If P2P is not enabled, go directly to HuggingFace
	if !s.p2pEnabled || s.p2pDistributor == nil {
		return s.downloadFromHuggingFace(ctx, task, baseModelSpec, hfComponents, destPath, modelInfo, modelType, namespace, name)
	}

	// Check if we're already seeding this model - means we already have it locally
	// and verified (seeding state is only set after successful download + metainfo creation).
	// This prevents re-downloading a model we just downloaded (e.g., from update events).
	if s.p2pDistributor.IsSeeding(modelHash) {
		s.logger.Infof("Already seeding model %s (hash: %s), skipping download", modelInfo, modelHash[:16])
		return nil
	}

	// Check ConfigMap - if model is marked Ready, files are complete (pod restart recovery).
	// ConfigMap is only updated to Ready AFTER successful download, so this is safe.
	// We still need to start seeding since in-memory state was lost on restart.
	if s.configMapReconciler.IsModelReady(task.BaseModel, task.ClusterBaseModel) {
		if stat, err := os.Stat(destPath); err == nil && stat.IsDir() {
			s.logger.Infof("Model %s marked Ready in ConfigMap (pod restart recovery), starting seeding", modelInfo)
			s.startSeeding(destPath, modelHash, modelInfo)
			return nil
		}
		// ConfigMap says Ready but files don't exist - this is unexpected, proceed with download
		s.logger.Warnf("Model %s marked Ready in ConfigMap but files not found at %s, will re-download", modelInfo, destPath)
	}

	// Check if model already exists on disk - handles cases where ConfigMap cache is not populated
	// (e.g., pod restart before cache was synced). This is a safety net for restart recovery.
	// We check the hash directory path to see if it exists (either as symlink or directory).
	// os.Stat follows symlinks, so IsDir() will be true if hash path is a symlink to a directory.
	hashPath := filepath.Join(s.modelRootDir, modelHash)
	if stat, err := os.Stat(hashPath); err == nil && stat.IsDir() {
		s.logger.Infof("Model %s already exists at hash path %s (disk recovery), starting seeding", modelInfo, hashPath)
		// Use destPath for seeding to maintain consistency with the rest of the codebase
		s.startSeeding(destPath, modelHash, modelInfo)
		return nil
	}

	s.logger.Infof("P2P enabled for model %s (hash: %s), acquiring lease", modelInfo, modelHash[:16])

	// Step 1: Try to acquire lease FIRST
	// The lease determines who downloads from HuggingFace vs who waits for P2P.
	// We acquire lease before checking for peers because:
	// - Lease holder always downloads from HF (handles partial recovery)
	// - Non-lease holders wait for P2P from the lease holder
	if s.p2pLeaseManager == nil {
		// No lease manager, just download directly from HF
		s.logger.Infof("No lease manager configured, downloading directly from HuggingFace for model %s", modelInfo)
		if err := s.downloadFromHuggingFace(ctx, task, baseModelSpec, hfComponents, destPath, modelInfo, modelType, namespace, name); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.startSeeding(destPath, modelHash, modelInfo)
		return nil
	}

	// Use resource UID for lease name (matches what controller creates)
	resourceUID := getModelUID(task)
	leaseName := constants.GetP2PLeaseName(types.UID(resourceUID))
	acquired, err := s.p2pLeaseManager.TryAcquire(ctx, leaseName)
	if err != nil {
		s.logger.Warnf("Failed to acquire P2P lease for model %s: %v, will try P2P or fallback", modelInfo, err)
	}

	// Step 2: If we acquired the lease, we're the designated HF downloader
	if acquired {
		s.logger.Infof("Acquired lease for model %s, downloading from HuggingFace", modelInfo)
		return s.downloadWithLeaseHeld(ctx, task, baseModelSpec, hfComponents, destPath, modelHash, modelInfo, modelType, namespace, name, leaseName)
	}

	// Step 3: Lease held by another node - first check if P2P is already available
	// (the lease holder might have already finished downloading and started seeding)
	if s.p2pDistributor.HasPeers(ctx, modelHash) {
		s.logger.Infof("Peers found for model %s, attempting P2P download", modelInfo)
		if err := s.p2pDistributor.TryP2PDownload(ctx, modelHash, destPath, s.p2pTimeout); err == nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Infof("Successfully downloaded model %s via P2P", modelInfo)
			// Start seeding so we can serve other nodes
			s.startSeeding(destPath, modelHash, modelInfo)
			return nil
		}
		s.logger.Warnf("P2P download failed for model %s: %v, will wait for P2P availability", modelInfo, err)
	}

	// Step 4: Wait for P2P availability from the lease holder
	s.logger.Infof("Lease held by another node for model %s, waiting for P2P availability", modelInfo)
	if err := s.waitForP2PAvailability(ctx, task, modelHash, modelInfo, leaseName, destPath); err == nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.logger.Infof("Model %s downloaded via P2P", modelInfo)
		// Start seeding so we can serve other nodes
		s.startSeeding(destPath, modelHash, modelInfo)
		return nil
	} else {
		s.logger.Warnf("Wait for P2P failed for model %s: %v, falling back to HuggingFace download", modelInfo, err)
	}

	// Step 5: Final fallback - download directly from HuggingFace
	// This happens when:
	// - P2P wait timed out (lease holder took too long or crashed)
	// - Lease expired (holder crashed before completing)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.logger.Infof("Fallback: downloading model %s directly from HuggingFace", modelInfo)
	if err := s.downloadFromHuggingFace(ctx, task, baseModelSpec, hfComponents, destPath, modelInfo, modelType, namespace, name); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.startSeeding(destPath, modelHash, modelInfo)
	return nil
}

// downloadWithLeaseHeld downloads from HuggingFace while holding a lease.
// It ensures proper cleanup of lease resources regardless of success or failure.
func (s *Gopher) downloadWithLeaseHeld(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	hfComponents *storage.HuggingFaceStorageComponents, destPath, modelHash, modelInfo, modelType, namespace, name, leaseName string) error {

	s.logger.Infof("Acquired lease for model %s, downloading from HuggingFace", modelInfo)

	// Start lease renewal in background
	cancelRenewal := s.p2pLeaseManager.StartRenewal(ctx, leaseName)
	defer cancelRenewal()

	// Download from HuggingFace
	if err := s.downloadFromHuggingFace(ctx, task, baseModelSpec, hfComponents, destPath, modelInfo, modelType, namespace, name); err != nil {
		// Release lease on failure so another node can try
		if releaseErr := s.p2pLeaseManager.Release(ctx, leaseName); releaseErr != nil {
			s.logger.Warnf("Failed to release lease after download failure for model %s: %v", modelInfo, releaseErr)
		}
		return err
	}

	// Check if context was cancelled (delete requested) - release lease and abort
	if ctx.Err() != nil {
		s.logger.Debugf("Download cancelled for model %s with lease, releasing lease: %v", modelInfo, ctx.Err())
		if releaseErr := s.p2pLeaseManager.Release(ctx, leaseName); releaseErr != nil {
			s.logger.Warnf("Failed to release lease after cancellation for model %s: %v", modelInfo, releaseErr)
		}
		return ctx.Err()
	}

	// Start seeding the downloaded model
	s.startSeeding(destPath, modelHash, modelInfo)

	// Mark lease as complete so other nodes know P2P is available
	if err := s.p2pLeaseManager.MarkComplete(ctx, leaseName); err != nil {
		s.logger.Warnf("Failed to mark lease complete for model %s: %v", modelInfo, err)
	}

	return nil
}

// isModelBeingDeleted checks if the model resource is being deleted (has deletionTimestamp).
// This is used to abort long-running operations early when the resource is deleted.
func (s *Gopher) isModelBeingDeleted(task *GopherTask) bool {
	if task.BaseModel != nil {
		bm, err := s.baseModelLister.BaseModels(task.BaseModel.Namespace).Get(task.BaseModel.Name)
		if err != nil {
			// If we can't get the resource, it might be deleted
			return true
		}
		return !bm.ObjectMeta.DeletionTimestamp.IsZero()
	}
	if task.ClusterBaseModel != nil {
		cbm, err := s.clusterBaseModelLister.Get(task.ClusterBaseModel.Name)
		if err != nil {
			// If we can't get the resource, it might be deleted
			return true
		}
		return !cbm.ObjectMeta.DeletionTimestamp.IsZero()
	}
	return false
}

// startSeeding begins seeding the model to peers. Errors are logged but not returned
// since seeding failure shouldn't fail the overall download operation.
func (s *Gopher) startSeeding(destPath, modelHash, modelInfo string) {
	if s.p2pDistributor == nil {
		return
	}
	if err := s.p2pDistributor.SeedModel(destPath, modelHash); err != nil {
		s.logger.Warnf("Failed to start seeding model %s: %v", modelInfo, err)
	} else {
		s.logger.Infof("Started seeding model %s", modelInfo)
	}
}

/*
handelReuseArtifactIfNecessary determines whether to reuse an existing model artifact
based on the BaseModel's download policy and artifacts previously recorded in the
node-scoped ConfigMap.

any error thrown in the process of searching for matched parent model, will be ignored, the process will proceed
to download artifact. Will let model cr reconciliation process handel searching for matched model to avoid impact
model creation process

Returns:
  - matchedKey: the matched ConfigMap data key
  - parentPath: the value of config.artifact.parentPath extracted from the matched entry
*/
func (s *Gopher) handelReuseArtifactIfNecessary(ctx context.Context, baseModelSpec v1beta1.BaseModelSpec,
	modelType string, modelId string, namespace string, shaStr string) (string, string) {
	// check whether identical artifact is already existing if model specified with ReuseIfExists
	if baseModelSpec.Storage.DownloadPolicy != nil && *baseModelSpec.Storage.DownloadPolicy == v1beta1.ReuseIfExists {
		var matchedModelTypeAndModelName string
		var parentPath string
		var err error
		// prioritize searching parent path in ClusterBaseModel
		// with hoping different basemodel in different namespaces could be linked to the same parent path to lower the chance of downloading artifact
		if strings.ToLower(modelType) == strings.ToLower(constants.ClusterBaseModel) || strings.ToLower(modelType) == strings.ToLower(constants.BaseModel) {
			matchedModelTypeAndModelName, parentPath, err = s.configMapReconciler.getModelDataByArtifactSha(ctx, shaStr, constants.LowerCaseClusterBaseModel)
			if err != nil {
				s.logger.Warnf("get error when finding matched model in configmap for model : %s: %s", modelId, err)
			}
		}
		if strings.ToLower(modelType) == strings.ToLower(constants.BaseModel) && matchedModelTypeAndModelName == "" {
			// build namespaced model type
			namespacedModelType := fmt.Sprintf("%s.%s", namespace, constants.LowerCaseBaseModel)
			matchedModelTypeAndModelName, parentPath, err = s.configMapReconciler.getModelDataByArtifactSha(ctx, shaStr, namespacedModelType)
			if err != nil {
				s.logger.Warnf("get error when finding matched model in configmap for model : %s: %s", modelId, err)
			}
		}
		return matchedModelTypeAndModelName, parentPath
	}
	return "", ""
}

// processLocalStorageModel handles local filesystem models.
// For local storage:
//   - Download: validates that the path exists and parses model configuration (no actual download)
//   - Delete: no-op for files (they are preserved), only updates node labels and ConfigMap
//
// This allows users to reference pre-existing models without copying or removing them.
func (s *Gopher) processLocalStorageModel(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	modelInfo, modelType, namespace, name string) error {
	// Parse the local storage URI to get the path
	localComponents, err := storage.ParseLocalStorageURI(*baseModelSpec.Storage.StorageUri)
	if err != nil {
		s.logger.Errorf("Failed to parse local storage URI for model %s: %v", modelInfo, err)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "invalid_local_uri")
		s.markModelOnNodeFailed(task)
		return err
	}

	// Determine the actual model path
	// If Path is specified in the CRD, use it; otherwise use the path from the URI
	var modelPath string
	if baseModelSpec.Storage.Path != nil && *baseModelSpec.Storage.Path != "" {
		// Use the explicit Path from the CRD
		modelPath = *baseModelSpec.Storage.Path
		s.logger.Debugf("Using explicit path from CRD for local model %s: %s", modelInfo, modelPath)
	} else {
		// Use the path from the local:// URI
		modelPath = localComponents.Path
		s.logger.Debugf("Using path from URI for local model %s: %s", modelInfo, modelPath)
	}

	// Check if the path exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		s.logger.Errorf("Local model path does not exist for model %s: %s", modelInfo, modelPath)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "local_path_not_found")
		s.markModelOnNodeFailed(task)
		return fmt.Errorf("local model path does not exist: %s", modelPath)
	}

	s.logger.Infof("Local model path exists for model %s: %s", modelInfo, modelPath)

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

	if err := s.safeParseAndUpdateModelConfig(modelPath, baseModel, clusterBaseModel, ""); err != nil {
		s.logger.Errorf("Failed to parse and update model config for local model: %v", err)
		// This is not necessarily a failure - the model might still be usable
	}

	s.logger.Infof("Successfully processed local model %s at path %s", modelInfo, modelPath)
	return nil
}

// for unit test
var fetchAttributeFromHfModelMetaData = FetchAttributeFromHfModelMetaData

// fetchSha retrieves the git commit SHA associated with a Hugging Face model ID.
// It queries the Hugging Face model metadata API for the "sha" attribute and returns:
// - the SHA string if available, along with true
// - an empty string and false if the API call fails, or the attribute is missing/non-string/empty.
func (s *Gopher) fetchSha(ctx context.Context, modelId string) (string, bool) {
	var isShaAvailable = true
	sha, err := fetchAttributeFromHfModelMetaData(ctx, modelId, Sha)
	if err != nil {
		s.logger.Errorf("Failed to retrieve sha from Hugging Face endpoint for model %s: %s", modelId, err)
		isShaAvailable = false
	}
	shaStr, ok := sha.(string)
	if !ok || shaStr == "" {
		s.logger.Warnf("Could not get a valid sha string for model %s, proceeding with download without artifact reuse.", modelId)
		isShaAvailable = false
	}
	if isShaAvailable {
		s.logger.Infof("fetched sha of model %s is %s", modelId, shaStr)
	}
	return shaStr, isShaAvailable
}

/*
isEligibleForOptimization determines whether a Hugging Face model can reuse an existing artifact.

Returns:
  - eligible: true if reuse is possible; false otherwise
  - matchedModelTypeAndModeName: ConfigMap key of the matched entry (empty if no match)
  - parentPath: artifact parent path from the matched entry (empty if no match)
*/
func (s *Gopher) isEligibleForOptimization(ctx context.Context, task *GopherTask, baseModelSpec v1beta1.BaseModelSpec,
	modelType string, namespace string, isShaAvailable bool, shaStr, modelId string) (bool, string, string) {
	if !isShaAvailable {
		return false, "", ""
	}

	currentModelTypeAndNodeName := s.configMapReconciler.getModelConfigMapKey(task.BaseModel, task.ClusterBaseModel)
	matchedModelTypeAndModeName, parentPath := s.handelReuseArtifactIfNecessary(ctx, baseModelSpec, modelType, modelId, namespace, shaStr)
	isEligible := matchedModelTypeAndModeName != "" && strings.ToLower(currentModelTypeAndNodeName) != strings.ToLower(matchedModelTypeAndModeName)
	s.logger.Infof("found matched matchedModelTypeAndModeName %s for model %s, parentPath is %s, isEligible %t", matchedModelTypeAndModeName, modelId, parentPath, isEligible)
	return isEligible, matchedModelTypeAndModeName, parentPath
}
