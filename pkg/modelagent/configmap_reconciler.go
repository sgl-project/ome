package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CacheEntry represents an entry in the model cache for ConfigMap reconciliation.
type CacheEntry struct {
	ModelName     string         // Name of the model
	ModelStatus   ModelStatus    // Current status of the model
	ModelMetadata *ModelMetadata // Model metadata if available
}

// ConfigMapReconciler handles all ConfigMap operations for storing model state and metadata.
// It provides self-healing capabilities through periodic reconciliation to recover from
// manual ConfigMap deletions or modifications without requiring agent restarts.
type ConfigMapReconciler struct {
	kubeClient        kubernetes.Interface   // Kubernetes client for ConfigMap CRUD operations
	nodeName          string                 // The name of the node (used as ConfigMap name)
	namespace         string                 // The namespace to store the ConfigMap in
	logger            *zap.SugaredLogger     // Logger for recording operations
	modelCache        map[string]*CacheEntry // In-memory cache of model information
	cacheMutex        sync.RWMutex           // Mutex to protect concurrent access to the cache
	reconcileInterval time.Duration          // Interval for periodic reconciliation
	isReconciling     bool                   // Flag to prevent concurrent reconciliations
	stopCh            chan struct{}          // Channel to signal reconciliation goroutine to stop
}

// ConfigMapStatusOp represents an operation to update model status in ConfigMap.
// It contains the necessary information to identify the model and its new status.
type ConfigMapStatusOp struct {
	ModelStatus      ModelStatus               // The updated status of the model
	BaseModel        *v1beta1.BaseModel        // Reference to a namespace-scoped BaseModel (nil if using ClusterBaseModel)
	ClusterBaseModel *v1beta1.ClusterBaseModel // Reference to a cluster-scoped BaseModel (nil if using BaseModel)
}

// ConfigMapMetadataOp represents an operation to update model metadata in ConfigMap.
// It contains the necessary information to identify the model and its metadata.
type ConfigMapMetadataOp struct {
	ModelMetadata    ModelMetadata             // The metadata to be stored for the model
	BaseModel        *v1beta1.BaseModel        // Reference to a namespace-scoped BaseModel (nil if using ClusterBaseModel)
	ClusterBaseModel *v1beta1.ClusterBaseModel // Reference to a cluster-scoped BaseModel (nil if using BaseModel)
}

// NewConfigMapReconciler creates a new ConfigMapReconciler with the given parameters.
// It initializes the in-memory model cache and sets up the reconciliation interval.
//
// Parameters:
//   - nodeName: Name of the node, used as the ConfigMap name
//   - namespace: Kubernetes namespace where the ConfigMap will be stored
//   - kubeClient: Interface to the Kubernetes API
//   - logger: Structured logger for operation recording
//
// Returns:
//   - A configured ConfigMapReconciler ready to use
func NewConfigMapReconciler(nodeName string, namespace string, kubeClient kubernetes.Interface, logger *zap.SugaredLogger) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		kubeClient:        kubeClient,
		nodeName:          nodeName,
		namespace:         namespace,
		logger:            logger,
		modelCache:        make(map[string]*CacheEntry),
		reconcileInterval: 5 * time.Minute, // Perform reconciliation every 5 minutes by default
		stopCh:            make(chan struct{}),
	}
}

// StartReconciliation begins the periodic reconciliation of ConfigMaps.
// This launches a background goroutine that checks for ConfigMap consistency
// at regular intervals and repairs any detected issues without requiring agent restarts.
// The interval is configurable through the reconcileInterval field (default: 5 minutes).
//
// This method should be called once during component initialization,
// typically from the model agent's main startup sequence.
func (c *ConfigMapReconciler) StartReconciliation() {
	c.logger.Infof("Starting ConfigMap reconciliation with interval %v", c.reconcileInterval)
	go func() {
		ticker := time.NewTicker(c.reconcileInterval)
		defer ticker.Stop()

		// Perform initial reconciliation immediately
		c.reconcileConfigMaps()

		for {
			select {
			case <-ticker.C:
				c.reconcileConfigMaps()
			case <-c.stopCh:
				c.logger.Info("Stopping ConfigMap reconciliation")
				return
			}
		}
	}()
}

// StopReconciliation safely stops the periodic reconciliation process.
// This should be called during graceful shutdown of the component to ensure
// that background goroutines are properly terminated.
// This method is idempotent - calling it multiple times has no additional effect.
func (c *ConfigMapReconciler) StopReconciliation() {
	select {
	case <-c.stopCh:
		// Channel already closed, no action needed
		return
	default:
		close(c.stopCh)
		c.logger.Debug("ConfigMap reconciliation stopped")
	}
}

// reconcileConfigMaps performs the reconciliation between the in-memory cache and the actual ConfigMaps.
// It detects and repairs two types of issues:
//  1. Missing ConfigMap: If the ConfigMap is completely missing, it recreates it with all cached models.
//  2. Missing model entries: If the ConfigMap exists but some model entries are missing, it restores just those entries.
//
// This method is thread-safe and prevents concurrent reconciliations to avoid resource contention.
// It is called periodically by the reconciliation goroutine started in StartReconciliation.
func (c *ConfigMapReconciler) reconcileConfigMaps() {
	// Create a new context for this reconciliation
	ctx := context.Background()
	// Prevent concurrent reconciliations
	if c.isReconciling {
		c.logger.Debug("Reconciliation already in progress, skipping")
		return
	}

	c.isReconciling = true
	defer func() { c.isReconciling = false }()

	c.logger.Debug("Starting ConfigMap reconciliation")

	// Get the current ConfigMap
	cm, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.logger.Warn("ConfigMap not found during reconciliation, will recreate it")
			// ConfigMap doesn't exist, recreate it from scratch
			c.recreateConfigMap(ctx)
			return
		}
		c.logger.Errorf("Failed to get ConfigMap during reconciliation: %v", err)
		return
	}

	// Check if all models in the cache are present in the ConfigMap
	c.cacheMutex.RLock()
	for modelID, cacheEntry := range c.modelCache {
		// Check if model exists in ConfigMap
		if _, exists := cm.Data[modelID]; !exists {
			c.logger.Warnf("Model %s missing from ConfigMap, will restore it", modelID)
			c.restoreModelInConfigMap(modelID, cacheEntry)
		}
	}
	c.cacheMutex.RUnlock()

	c.logger.Debug("ConfigMap reconciliation completed successfully")
}

// recreateConfigMap creates a new ConfigMap from the in-memory model cache.
// This is called when the ConfigMap is completely missing (e.g., manually deleted),
// and needs to be reconstructed from the cached model data.
//
// The method handles the following tasks:
// 1. Creates a new ConfigMap with the correct name and namespace
// 2. Populates it with all model entries from the cache
// 3. Correctly maps cached metadata to ModelConfig entries
// 4. Creates the ConfigMap in the Kubernetes API
//
// Thread safety is ensured through the read lock on the cache mutex.
func (c *ConfigMapReconciler) recreateConfigMap(ctx context.Context) {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	// Skip if cache is empty
	if len(c.modelCache) == 0 {
		c.logger.Info("No models in cache to recreate ConfigMap")
		return
	}

	// Create a new ConfigMap with all models from the cache
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.nodeName,
			Namespace: c.namespace,
		},
		Data: make(map[string]string),
	}

	// Add all models from cache to the ConfigMap
	for modelID, cacheEntry := range c.modelCache {
		// Create model entry from cache data
		modelEntry := &ModelEntry{
			Name:   cacheEntry.ModelName,
			Status: cacheEntry.ModelStatus,
		}

		// Convert metadata to ModelConfig if available
		if cacheEntry.ModelMetadata != nil {
			config := &ModelConfig{}
			// Copy metadata fields to config
			config.ModelType = cacheEntry.ModelMetadata.ModelType
			config.ModelArchitecture = cacheEntry.ModelMetadata.ModelArchitecture
			config.ModelCapabilities = cacheEntry.ModelMetadata.ModelCapabilities
			config.ModelParameterSize = cacheEntry.ModelMetadata.ModelParameterSize
			config.MaxTokens = cacheEntry.ModelMetadata.MaxTokens
			config.Quantization = string(cacheEntry.ModelMetadata.Quantization)

			modelEntry.Config = config
		}

		// Serialize the model entry to JSON
		modelEntryJSON, err := json.Marshal(modelEntry)
		if err != nil {
			c.logger.Errorf("Failed to marshal model entry for %s: %v", modelID, err)
			continue
		}

		cm.Data[modelID] = string(modelEntryJSON)
	}

	// Create the ConfigMap
	_, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		c.logger.Errorf("Failed to recreate ConfigMap: %v", err)
		return
	}

	c.logger.Info("Successfully recreated ConfigMap from cache")
}

// restoreModelInConfigMap adds or updates a specific model in the ConfigMap.
// This is called when an individual model entry is missing from an existing ConfigMap.
// Unlike recreateConfigMap, this method only updates a single model entry while preserving
// the rest of the ConfigMap content.
//
// The method:
// 1. Retrieves the current ConfigMap from the API
// 2. Constructs a ModelEntry from the cached model data
// 3. Serializes the entry to JSON and adds it to the ConfigMap
// 4. Updates the ConfigMap through the Kubernetes API
//
// If the ConfigMap is missing entirely, this will trigger a fallback to recreateConfigMap.
func (c *ConfigMapReconciler) restoreModelInConfigMap(modelID string, cacheEntry *CacheEntry) {
	// Construct model entry from cache data
	modelEntry := &ModelEntry{
		Name:   cacheEntry.ModelName,
		Status: cacheEntry.ModelStatus,
	}

	// Convert metadata to ModelConfig if available
	if cacheEntry.ModelMetadata != nil {
		config := &ModelConfig{}
		// Copy metadata fields to config
		config.ModelType = cacheEntry.ModelMetadata.ModelType
		config.ModelArchitecture = cacheEntry.ModelMetadata.ModelArchitecture
		config.ModelCapabilities = cacheEntry.ModelMetadata.ModelCapabilities
		config.ModelParameterSize = cacheEntry.ModelMetadata.ModelParameterSize
		config.MaxTokens = cacheEntry.ModelMetadata.MaxTokens
		config.Quantization = string(cacheEntry.ModelMetadata.Quantization)

		modelEntry.Config = config
	}

	// Serialize the model entry to JSON
	modelEntryJSON, err := json.Marshal(modelEntry)
	if err != nil {
		c.logger.Errorf("Failed to marshal model entry for %s: %v", modelID, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the current ConfigMap
	cm, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist anymore, fallback to full recreation
			c.logger.Warn("ConfigMap not found during model restore, falling back to full recreation")
			c.recreateConfigMap(ctx)
			return
		}
		c.logger.Errorf("Failed to get ConfigMap during model restore: %v", err)
		return
	}

	// Add or update the model entry - initialize Data map if nil
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// Add the restored model entry to the ConfigMap
	cm.Data[modelID] = string(modelEntryJSON)

	// Update the ConfigMap with retry logic (3 attempts)
	for attempts := 0; attempts < 3; attempts++ {
		_, err = c.kubeClient.CoreV1().ConfigMaps(c.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err == nil {
			// Successfully updated
			break
		}

		// Check if we need to retry due to conflict
		if errors.IsConflict(err) && attempts < 2 {
			c.logger.Warnf("Conflict during model restore (attempt %d), retrying: %v", attempts+1, err)
			// Get the latest version of the ConfigMap
			cm, err = c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.nodeName, metav1.GetOptions{})
			if err != nil {
				c.logger.Errorf("Failed to get ConfigMap for conflict resolution: %v", err)
				return
			}
			// Re-apply our changes to the latest version
			if cm.Data == nil {
				cm.Data = make(map[string]string)
			}
			cm.Data[modelID] = string(modelEntryJSON)
			continue
		}

		// Non-conflict error or final attempt
		c.logger.Errorf("Failed to update ConfigMap with restored model %s after %d attempts: %v", modelID, attempts+1, err)
		return
	}

	c.logger.Infof("Successfully restored model %s in ConfigMap", modelID)
}

// ReconcileModelStatus updates the ConfigMap with model status information and synchronizes the in-memory cache.
//
// This method performs two key operations:
// 1. Updates the model status in the Kubernetes ConfigMap, creating it if necessary
// 2. Synchronizes the in-memory model cache with the updated status information
//
// The cache updates are atomic, protected by mutex, ensuring thread safety even with concurrent reconciliation.
// Both operations must succeed for the method to return nil, otherwise an error is returned.
//
// Parameters:
//   - op: ConfigMapStatusOp containing model references and new status
//
// Returns:
//   - error: nil if both ConfigMap and cache updates succeed, error otherwise
func (c *ConfigMapReconciler) ReconcileModelStatus(ctx context.Context, statusOp *ConfigMapStatusOp) error {
	modelInfo := getConfigMapModelInfo(statusOp.BaseModel, statusOp.ClusterBaseModel)
	c.logger.Infof("Reconciling model status in ConfigMap for %s with status: %s", modelInfo, statusOp.ModelStatus)

	// Get or create the ConfigMap
	configMap, needCreate, err := c.getOrCreateConfigMap(ctx)
	if err != nil {
		c.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap with status
	err = c.updateModelStatusInConfigMap(ctx, configMap, statusOp, needCreate)
	if err != nil {
		c.logger.Errorf("Failed to update model status in ConfigMap for %s: %v", modelInfo, err)
		return err
	}

	// Update the in-memory cache
	modelID := getModelID(statusOp.BaseModel, statusOp.ClusterBaseModel)
	c.cacheMutex.Lock()
	if c.modelCache == nil {
		c.modelCache = make(map[string]*CacheEntry)
	}

	// Get existing cache entry or create a new one
	cacheEntry, exists := c.modelCache[modelID]
	if !exists {
		// Extract model name for the cache entry
		modelName := ""
		if statusOp.BaseModel != nil {
			modelName = statusOp.BaseModel.Name
		} else if statusOp.ClusterBaseModel != nil {
			modelName = statusOp.ClusterBaseModel.Name
		}

		cacheEntry = &CacheEntry{
			ModelName:   modelName,
			ModelStatus: statusOp.ModelStatus,
		}
		c.modelCache[modelID] = cacheEntry
	} else {
		// Just update the status in existing entry
		cacheEntry.ModelStatus = statusOp.ModelStatus
	}
	c.cacheMutex.Unlock()

	c.logger.Infof("Successfully updated ConfigMap and cache for %s with status: %s", modelInfo, statusOp.ModelStatus)
	return nil
}

// getModelID generates a unique deterministic identifier string for a model.
// It handles both namespace-scoped BaseModel and cluster-scoped ClusterBaseModel objects.
//
// For BaseModel: The format is "namespace/name".
// For ClusterBaseModel: The format is "cluster/name".
//
// These IDs serve as consistent keys for the in-memory cache and ConfigMap entries,
// ensuring proper reconciliation between cache and ConfigMap state.
//
// Parameters:
//   - baseModel: A namespace-scoped BaseModel object (nil if using ClusterBaseModel)
//   - clusterBaseModel: A cluster-scoped ClusterBaseModel object (nil if using BaseModel)
//
// Returns:
//   - A unique string identifier for the model, or empty string if both inputs are nil
func getModelID(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) string {
	var namespace, modelName string
	var isClusterBaseModel bool

	if baseModel != nil {
		modelName = baseModel.Name
		namespace = baseModel.Namespace
		isClusterBaseModel = false
	} else if clusterBaseModel != nil {
		modelName = clusterBaseModel.Name
		namespace = ""
		isClusterBaseModel = true
	} else {
		return ""
	}

	return constants.GetModelConfigMapKey(namespace, modelName, isClusterBaseModel)
}

// ReconcileModelMetadata updates the ConfigMap with model metadata
func (c *ConfigMapReconciler) ReconcileModelMetadata(ctx context.Context, op *ConfigMapMetadataOp) error {
	modelInfo := getConfigMapModelInfo(op.BaseModel, op.ClusterBaseModel)
	c.logger.Infof("Reconciling model metadata in ConfigMap for %s", modelInfo)

	// Get or create the ConfigMap
	configMap, needCreate, err := c.getOrCreateConfigMap(ctx)
	if err != nil {
		c.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap with metadata
	err = c.updateModelMetadataInConfigMap(ctx, configMap, op, needCreate)
	if err != nil {
		c.logger.Errorf("Failed to update model metadata in ConfigMap for %s: %v", modelInfo, err)
		return err
	}

	// Update the in-memory cache with metadata
	modelID := getModelID(op.BaseModel, op.ClusterBaseModel)
	c.cacheMutex.Lock()
	if c.modelCache == nil {
		c.modelCache = make(map[string]*CacheEntry)
	}

	cacheEntry, exists := c.modelCache[modelID]
	if !exists {
		modelName := ""
		if op.BaseModel != nil {
			modelName = op.BaseModel.Name
		} else if op.ClusterBaseModel != nil {
			modelName = op.ClusterBaseModel.Name
		}

		cacheEntry = &CacheEntry{
			ModelName:     modelName,
			ModelMetadata: &op.ModelMetadata,
		}
		c.modelCache[modelID] = cacheEntry
	} else {
		// Update the metadata
		cacheEntry.ModelMetadata = &op.ModelMetadata
	}
	c.cacheMutex.Unlock()

	c.logger.Infof("Successfully updated ConfigMap and cache for %s with metadata", modelInfo)
	return nil
}

// DeleteModelFromConfigMap removes a model entry from the ConfigMap
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - baseModel: The BaseModel reference
//   - clusterBaseModel: The ClusterBaseModel reference
//
// Returns:
//   - error: nil if deletion succeeds or model doesn't exist, error otherwise
func (c *ConfigMapReconciler) DeleteModelFromConfigMap(ctx context.Context, baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) error {
	modelInfo := getConfigMapModelInfo(baseModel, clusterBaseModel)
	c.logger.Infof("Deleting model from ConfigMap: %s", modelInfo)

	// Get ConfigMap
	configMap, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, nothing to delete
			return nil
		}
		c.logger.Errorf("Failed to get ConfigMap for model deletion: %v", err)
		return err
	}

	// Determine the model ID in the ConfigMap
	modelID := c.getModelConfigMapKey(baseModel, clusterBaseModel)

	// Check if model exists in ConfigMap
	if _, exists := configMap.Data[modelID]; !exists {
		c.logger.Infof("Model %s doesn't exist in ConfigMap, nothing to delete", modelInfo)
		return nil
	}

	// Delete the model from ConfigMap
	delete(configMap.Data, modelID)

	// Update the ConfigMap
	_, err = c.kubeClient.CoreV1().ConfigMaps(c.namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		c.logger.Errorf("Failed to update ConfigMap after model deletion: %v", err)
		return err
	}

	// Also update our in-memory cache to remove the model
	c.cacheMutex.Lock()
	if c.modelCache != nil {
		modelCacheID := getModelID(baseModel, clusterBaseModel)
		delete(c.modelCache, modelCacheID)
		c.logger.Debugf("Removed model %s from cache", modelInfo)
	}
	c.cacheMutex.Unlock()

	c.logger.Infof("Successfully deleted model %s from ConfigMap and cache", modelInfo)
	return nil
}

// getOrCreateConfigMap gets an existing ConfigMap or creates a new one if it doesn't exist
func (c *ConfigMapReconciler) getOrCreateConfigMap(ctx context.Context) (*corev1.ConfigMap, bool, error) {
	var notFound = false
	existingConfigMap, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			notFound = true
		} else {
			return nil, false, err
		}
	}

	if notFound {
		data := make(map[string]string)
		labels := make(map[string]string)
		labels[constants.ModelStatusConfigMapLabel] = "true"

		// Add node name as label for easier querying
		labels["node"] = c.nodeName

		annotations := make(map[string]string)
		// Add annotation to track which node this ConfigMap belongs to
		annotations["models.ome.io/node-name"] = c.nodeName
		annotations["models.ome.io/managed-by"] = "model-agent"

		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        c.nodeName,
				Namespace:   c.namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Data: data,
		}, true, nil
	}

	return existingConfigMap, false, nil
}

// getModelConfigMapKey gets the deterministic key for a model in the ConfigMap
func (c *ConfigMapReconciler) getModelConfigMapKey(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) string {
	var modelName, namespace string
	var isClusterBaseModel bool

	if baseModel != nil {
		modelName = baseModel.Name
		namespace = baseModel.Namespace
		isClusterBaseModel = false
	} else {
		modelName = clusterBaseModel.Name
		namespace = ""
		isClusterBaseModel = true
	}

	return constants.GetModelConfigMapKey(namespace, modelName, isClusterBaseModel)
}

// updateModelStatusInConfigMap updates the model status in the ConfigMap
func (c *ConfigMapReconciler) updateModelStatusInConfigMap(ctx context.Context, configMap *corev1.ConfigMap, op *ConfigMapStatusOp, needCreate bool) error {
	// Get model information and key
	key := c.getModelConfigMapKey(op.BaseModel, op.ClusterBaseModel)
	modelInfo := getConfigMapModelInfo(op.BaseModel, op.ClusterBaseModel)
	c.logger.Debugf("Using key '%s' for model %s", key, modelInfo)

	if configMap.Data == nil {
		c.logger.Debugf("ConfigMap Data is nil, initializing it for %s", modelInfo)
		configMap.Data = make(map[string]string)
	}

	// Get the existing model entry or create a new one
	var modelEntry ModelEntry
	var modelName string
	if op.BaseModel != nil {
		modelName = op.BaseModel.Name
	} else {
		modelName = op.ClusterBaseModel.Name
	}

	// Check if there's already an entry for this model
	if existingData, exists := configMap.Data[key]; exists {
		// If entry exists, try to unmarshal it
		if err := json.Unmarshal([]byte(existingData), &modelEntry); err != nil {
			// If it's not in our format yet, create a new entry with just the status
			modelEntry = ModelEntry{
				Name:   modelName,
				Status: op.ModelStatus,
				Config: nil,
			}
		} else {
			// Update just the status, preserving the config
			modelEntry.Status = op.ModelStatus
		}
	} else {
		// No existing entry, create a new one
		modelEntry = ModelEntry{
			Name:   modelName,
			Status: op.ModelStatus,
			Config: nil,
		}
	}

	// For 'ModelStatusDeleted' status, we might want to entirely remove the entry
	if op.ModelStatus == ModelStatusDeleted {
		c.logger.Debugf("Deleting ConfigMap data[%s] for %s", key, modelInfo)
		delete(configMap.Data, key)
	} else {
		// Marshal the model entry to JSON
		entryJSON, err := json.Marshal(modelEntry)
		if err != nil {
			c.logger.Errorf("Failed to marshal model entry for %s: %v", modelInfo, err)
			return err
		}
		c.logger.Debugf("Setting ConfigMap data[%s] to %s for %s", key, string(entryJSON), modelInfo)
		configMap.Data[key] = string(entryJSON)
	}

	return c.saveConfigMap(ctx, configMap, modelInfo, needCreate)
}

// updateModelMetadataInConfigMap updates the model metadata in the ConfigMap
func (c *ConfigMapReconciler) updateModelMetadataInConfigMap(ctx context.Context, configMap *corev1.ConfigMap, op *ConfigMapMetadataOp, needCreate bool) error {
	// Get model information and key
	key := c.getModelConfigMapKey(op.BaseModel, op.ClusterBaseModel)
	modelInfo := getConfigMapModelInfo(op.BaseModel, op.ClusterBaseModel)
	c.logger.Debugf("Using key '%s' for model %s", key, modelInfo)

	if configMap.Data == nil {
		c.logger.Debugf("ConfigMap Data is nil, initializing it for %s", modelInfo)
		configMap.Data = make(map[string]string)
	}

	// Get the existing model entry or create a new one
	var modelEntry ModelEntry
	var modelName string
	if op.BaseModel != nil {
		modelName = op.BaseModel.Name
	} else {
		modelName = op.ClusterBaseModel.Name
	}

	// Check if there's already an entry for this model
	if existingData, exists := configMap.Data[key]; exists {
		// If entry exists, try to unmarshal it
		if err := json.Unmarshal([]byte(existingData), &modelEntry); err != nil {
			// If it's not in our format yet, create a new entry
			modelEntry = ModelEntry{
				Name:   modelName,
				Status: ModelStatusReady, // Default to ready when adding metadata
				Config: nil,
			}
		}
		// Keep existing status
	} else {
		// No existing entry, create a new one
		modelEntry = ModelEntry{
			Name:   modelName,
			Status: ModelStatusReady, // Default to ready when adding metadata
			Config: nil,
		}
	}

	// Create model config from metadata
	modelConfig := ConvertMetadataToModelConfig(op.ModelMetadata)

	// Update the config in the model entry
	modelEntry.Config = modelConfig

	// Marshal the model entry back to JSON
	entryJSON, err := json.Marshal(modelEntry)
	if err != nil {
		c.logger.Errorf("Failed to marshal model entry for %s: %v", modelInfo, err)
		return err
	}

	// Store the model entry in the ConfigMap
	configMap.Data[key] = string(entryJSON)
	c.logger.Debugf("Setting ConfigMap data[%s] = %s for %s", key, string(entryJSON), modelInfo)

	return c.saveConfigMap(ctx, configMap, modelInfo, needCreate)
}

// saveConfigMap creates or updates the ConfigMap in Kubernetes
func (c *ConfigMapReconciler) saveConfigMap(ctx context.Context, configMap *corev1.ConfigMap, modelInfo string, needCreate bool) error {
	// Create or update the ConfigMap
	if needCreate {
		c.logger.Infof("Creating new ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
		_, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Create(ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			c.logger.Errorf("Failed to create ConfigMap '%s' in namespace '%s' for %s: %v", c.nodeName, c.namespace, modelInfo, err)
			return err
		}
		c.logger.Infof("Successfully created ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
	} else {
		c.logger.Infof("Updating ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
		_, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Update(ctx, configMap, metav1.UpdateOptions{})
		if err != nil {
			c.logger.Errorf("Failed to update ConfigMap '%s' in namespace '%s' for %s: %v", c.nodeName, c.namespace, modelInfo, err)
			return err
		}
		c.logger.Infof("Successfully updated ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
	}
	return nil
}

// Helper function to get a string representation of the model for logging
func getConfigMapModelInfo(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) string {
	if baseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", baseModel.Namespace, baseModel.Name)
	} else if clusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", clusterBaseModel.Name)
	}
	return "unknown model"
}
