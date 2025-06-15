package modelagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapReconciler handles all ConfigMap operations for storing model state and metadata
type ConfigMapReconciler struct {
	kubeClient kubernetes.Interface // Kubernetes client for ConfigMap CRUD operations
	nodeName   string               // The name of the node (used as ConfigMap name)
	namespace  string               // The namespace to store the ConfigMap in
	logger     *zap.SugaredLogger   // Logger for recording operations
}

// ConfigMapStatusOp represents an operation to update model status in ConfigMap
type ConfigMapStatusOp struct {
	ModelStatus      ModelStatus
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

// ConfigMapMetadataOp represents an operation to update model metadata in ConfigMap
type ConfigMapMetadataOp struct {
	ModelMetadata    ModelMetadata
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

// NewConfigMapReconciler creates a new ConfigMapReconciler instance
func NewConfigMapReconciler(nodeName string, namespace string, kubeClient kubernetes.Interface, logger *zap.SugaredLogger) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		nodeName:   nodeName,
		kubeClient: kubeClient,
		namespace:  namespace,
		logger:     logger,
	}
}

// ReconcileModelStatus updates the ConfigMap with model status information
func (c *ConfigMapReconciler) ReconcileModelStatus(op *ConfigMapStatusOp) error {
	modelInfo := getConfigMapModelInfo(op.BaseModel, op.ClusterBaseModel)
	c.logger.Infof("Reconciling model status in ConfigMap for %s with status: %s", modelInfo, op.ModelStatus)

	// Get or create the ConfigMap
	configMap, needCreate, err := c.getOrCreateConfigMap()
	if err != nil {
		c.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap with status
	err = c.updateModelStatusInConfigMap(configMap, op, needCreate)
	if err != nil {
		c.logger.Errorf("Failed to update model status in ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Infof("Successfully updated ConfigMap for %s with status: %s", modelInfo, op.ModelStatus)

	return nil
}

// ReconcileModelMetadata updates the ConfigMap with model metadata
func (c *ConfigMapReconciler) ReconcileModelMetadata(op *ConfigMapMetadataOp) error {
	modelInfo := getConfigMapModelInfo(op.BaseModel, op.ClusterBaseModel)
	c.logger.Infof("Reconciling model metadata in ConfigMap for %s", modelInfo)

	// Get or create the ConfigMap
	configMap, needCreate, err := c.getOrCreateConfigMap()
	if err != nil {
		c.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap with metadata
	err = c.updateModelMetadataInConfigMap(configMap, op, needCreate)
	if err != nil {
		c.logger.Errorf("Failed to update model metadata in ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	c.logger.Infof("Successfully updated ConfigMap for %s with metadata", modelInfo)

	return nil
}

// DeleteModelFromConfigMap removes a model entry from the ConfigMap
func (c *ConfigMapReconciler) DeleteModelFromConfigMap(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) error {
	modelInfo := getConfigMapModelInfo(baseModel, clusterBaseModel)
	c.logger.Infof("Deleting model from ConfigMap: %s", modelInfo)

	// Get ConfigMap
	configMap, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(context.TODO(), c.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, nothing to delete
			return nil
		}
		return err
	}

	// Get the model name, namespace and type
	key := c.getModelConfigMapKey(baseModel, clusterBaseModel)

	// Check if entry exists
	if existingData, exists := configMap.Data[key]; exists {
		// Delete the entry
		delete(configMap.Data, key)
		c.logger.Debugf("Deleted ConfigMap data[%s] for %s (previous value: %s)", key, modelInfo, existingData)

		// Update the ConfigMap
		_, err = c.kubeClient.CoreV1().ConfigMaps(c.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			c.logger.Errorf("Failed to update ConfigMap '%s' in namespace '%s' after deleting %s: %v", c.nodeName, c.namespace, modelInfo, err)
			return err
		}
		c.logger.Infof("Successfully updated ConfigMap '%s' in namespace '%s' after deleting %s", c.nodeName, c.namespace, modelInfo)
	} else {
		c.logger.Debugf("No entry found for %s in ConfigMap, nothing to delete", modelInfo)
	}

	return nil
}

// getOrCreateConfigMap gets an existing ConfigMap or creates a new one if it doesn't exist
func (c *ConfigMapReconciler) getOrCreateConfigMap() (*corev1.ConfigMap, bool, error) {
	var notFound = false
	existingConfigMap, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Get(context.TODO(), c.nodeName, metav1.GetOptions{})
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
func (c *ConfigMapReconciler) updateModelStatusInConfigMap(configMap *corev1.ConfigMap, op *ConfigMapStatusOp, needCreate bool) error {
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

	return c.saveConfigMap(configMap, modelInfo, needCreate)
}

// updateModelMetadataInConfigMap updates the model metadata in the ConfigMap
func (c *ConfigMapReconciler) updateModelMetadataInConfigMap(configMap *corev1.ConfigMap, op *ConfigMapMetadataOp, needCreate bool) error {
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

	return c.saveConfigMap(configMap, modelInfo, needCreate)
}

// saveConfigMap creates or updates the ConfigMap in Kubernetes
func (c *ConfigMapReconciler) saveConfigMap(configMap *corev1.ConfigMap, modelInfo string, needCreate bool) error {
	// Create or update the ConfigMap
	if needCreate {
		c.logger.Infof("Creating new ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
		_, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			c.logger.Errorf("Failed to create ConfigMap '%s' in namespace '%s' for %s: %v", c.nodeName, c.namespace, modelInfo, err)
			return err
		}
		c.logger.Infof("Successfully created ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
	} else {
		c.logger.Infof("Updating ConfigMap '%s' in namespace '%s' for %s", c.nodeName, c.namespace, modelInfo)
		_, err := c.kubeClient.CoreV1().ConfigMaps(c.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
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
