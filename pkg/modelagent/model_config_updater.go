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

// ModelConfigOp represents an operation on a model configuration
// This is used to pass model metadata and model references to the ModelConfigUpdater
type ModelConfigOp struct {
	// The extracted metadata from the model
	ModelMetadata ModelMetadata

	// One of these should be non-nil based on the model type
	BaseModel        *v1beta1.BaseModel        // For namespace-scoped models
	ClusterBaseModel *v1beta1.ClusterBaseModel // For cluster-scoped models
}

// ModelConfigUpdater handles updating ConfigMaps with model configuration data
// It provides a consistent way to store model metadata in ConfigMaps, making it
// accessible to both the model agent and controller components.
// Note: Thread-safety is now handled by the Gopher's mutex rather than internally.
type ModelConfigUpdater struct {
	kubeClient *kubernetes.Clientset // Kubernetes client for ConfigMap CRUD operations
	nodeName   string                // The name of the node (used as ConfigMap name)
	namespace  string                // The namespace to store the ConfigMap in
	logger     *zap.SugaredLogger    // Logger for recording operations
}

// NewModelConfigUpdater creates a new ModelConfigUpdater instance
func NewModelConfigUpdater(nodeName string, namespace string, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger) *ModelConfigUpdater {
	return &ModelConfigUpdater{
		nodeName:   nodeName,
		kubeClient: kubeClient,
		namespace:  namespace,
		logger:     logger,
	}
}

// UpdateModelConfig updates the ConfigMap with model configuration data
// Note: Thread-safety is now handled by the Gopher's mutex
func (m *ModelConfigUpdater) UpdateModelConfig(op *ModelConfigOp) error {

	modelInfo := getModelInfo(op)
	m.logger.Infof("Updating model configuration for %s", modelInfo)

	// Get or create the ConfigMap
	configMap, needCreate, err := m.getOrNewConfigMap()
	if err != nil {
		m.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	m.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap
	err = m.createOrUpdateConfigMap(configMap, op, needCreate)
	if err != nil {
		m.logger.Errorf("Failed to create/update ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	m.logger.Infof("Successfully updated ConfigMap for %s with model configuration", modelInfo)

	return nil
}

// getModelInfo returns a string representing the model
func getModelInfo(op *ModelConfigOp) string {
	if op.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", op.BaseModel.Namespace, op.BaseModel.Name)
	} else if op.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", op.ClusterBaseModel.Name)
	}
	return "unknown model"
}

// getOrNewConfigMap gets an existing ConfigMap or creates a new one if it doesn't exist
func (m *ModelConfigUpdater) getOrNewConfigMap() (*corev1.ConfigMap, bool, error) {
	var notFound = false
	// Use the same ConfigMap name as NodeLabeler (just the node name)
	existingConfigMap, err := m.kubeClient.CoreV1().ConfigMaps(m.namespace).Get(context.TODO(), m.nodeName, metav1.GetOptions{})
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
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      m.nodeName,
				Namespace: m.namespace,
				Labels:    labels,
			},
			Data: data,
		}, true, nil
	}

	return existingConfigMap, false, nil
}

// createOrUpdateConfigMap creates or updates the ConfigMap with model configuration data
func (m *ModelConfigUpdater) createOrUpdateConfigMap(configMap *corev1.ConfigMap, op *ModelConfigOp, needCreate bool) error {
	var modelName, namespace, modelInfo string

	// Get the model name and namespace based on the model type
	if op.BaseModel != nil {
		modelName = op.BaseModel.Name
		namespace = op.BaseModel.Namespace
		modelInfo = fmt.Sprintf("BaseModel %s/%s", namespace, modelName)
	} else {
		modelName = op.ClusterBaseModel.Name
		namespace = ""
		modelInfo = fmt.Sprintf("ClusterBaseModel %s", modelName)
	}

	// Get the unique key for this model
	key := GetModelKey(namespace, modelName)
	m.logger.Debugf("Using key '%s' for %s", key, modelInfo)

	if configMap.Data == nil {
		m.logger.Debugf("ConfigMap Data is nil, initializing it for %s", modelInfo)
		configMap.Data = make(map[string]string)
	}

	// First, check if there's already an entry for this model
	var modelEntry ModelEntry
	if existingData, exists := configMap.Data[key]; exists {
		// If entry exists, try to unmarshal it
		if err := json.Unmarshal([]byte(existingData), &modelEntry); err != nil {
			// If it's not in our format yet, create a new entry
			// This handles the transition from old format to new format
			if existingData == string(ModelStatusReady) ||
				existingData == string(ModelStatusUpdating) ||
				existingData == string(ModelStatusFailed) {
				// This is from the old NodeLabeler format
				modelEntry = ModelEntry{
					Name:   modelName,
					Status: ModelStatus(existingData),
					Config: nil,
				}
			} else {
				// Cannot parse the existing data, log a warning and create a new entry
				m.logger.Warnf("Could not unmarshal existing entry for %s: %v", modelInfo, err)
				modelEntry = ModelEntry{
					Name:   modelName,
					Status: ModelStatusReady, // Assume Ready if we're adding config
					Config: nil,
				}
			}
		}
	} else {
		// No existing entry, create a new one
		modelEntry = ModelEntry{
			Name:   modelName,
			Status: ModelStatusReady, // Assume Ready if we're adding config
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
		m.logger.Errorf("Failed to marshal model entry for %s: %v", modelInfo, err)
		return err
	}

	// Store the model entry in the ConfigMap
	configMap.Data[key] = string(entryJSON)
	m.logger.Debugf("Setting ConfigMap data[%s] = %s for %s", key, string(entryJSON), modelInfo)

	// Create or update the ConfigMap
	if needCreate {
		m.logger.Infof("Creating new ConfigMap '%s' in namespace '%s' for %s", m.nodeName, m.namespace, modelInfo)
		_, err := m.kubeClient.CoreV1().ConfigMaps(m.namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			m.logger.Errorf("Failed to create ConfigMap '%s' in namespace '%s' for %s: %v", m.nodeName, m.namespace, modelInfo, err)
			return err
		}
		m.logger.Infof("Successfully created ConfigMap '%s' in namespace '%s' for %s", m.nodeName, m.namespace, modelInfo)
	} else {
		m.logger.Infof("Updating ConfigMap '%s' in namespace '%s' for %s", m.nodeName, m.namespace, modelInfo)
		_, err := m.kubeClient.CoreV1().ConfigMaps(m.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			m.logger.Errorf("Failed to update ConfigMap '%s' in namespace '%s' for %s: %v", m.nodeName, m.namespace, modelInfo, err)
			return err
		}
		m.logger.Infof("Successfully updated ConfigMap '%s' in namespace '%s' for %s", m.nodeName, m.namespace, modelInfo)
	}
	return nil
}

// DeleteModelConfig removes model configuration from the ConfigMap
// Note: Thread-safety is now handled by the Gopher's mutex
func (m *ModelConfigUpdater) DeleteModelConfig(op *ModelConfigOp) error {

	modelInfo := getModelInfo(op)
	m.logger.Infof("Deleting model configuration for %s", modelInfo)

	// Get the ConfigMap
	existingConfigMap, err := m.kubeClient.CoreV1().ConfigMaps(m.namespace).Get(context.TODO(), m.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, nothing to delete
			return nil
		}
		return err
	}

	// Get the model name and namespace based on the model type
	var modelName, namespace string
	if op.BaseModel != nil {
		modelName = op.BaseModel.Name
		namespace = op.BaseModel.Namespace
	} else {
		modelName = op.ClusterBaseModel.Name
		namespace = ""
	}

	// Get the unique key for this model
	key := GetModelKey(namespace, modelName)

	// Check if the key exists in the ConfigMap
	if existingData, exists := existingConfigMap.Data[key]; exists {
		// If the entry exists in the new format, keep the status but remove the config
		var modelEntry ModelEntry
		if err := json.Unmarshal([]byte(existingData), &modelEntry); err == nil {
			// Entry is in the new format, just remove the config
			modelEntry.Config = nil

			// If status is not set, mark as Deleted
			if modelEntry.Status == "" {
				modelEntry.Status = ModelStatusDeleted
			}

			// Marshal and update
			updatedJSON, err := json.Marshal(modelEntry)
			if err != nil {
				m.logger.Errorf("Failed to marshal updated model entry for %s: %v", modelInfo, err)
				return err
			}

			existingConfigMap.Data[key] = string(updatedJSON)
		} else {
			// Old format or unrecognized, just delete the entry
			delete(existingConfigMap.Data, key)
		}

		m.logger.Debugf("Updated/deleted ConfigMap data[%s] for %s", key, modelInfo)
	} else {
		// Key doesn't exist, nothing to do
		m.logger.Debugf("No entry found for %s, nothing to delete", modelInfo)
		return nil
	}

	// Update the ConfigMap
	_, err = m.kubeClient.CoreV1().ConfigMaps(m.namespace).Update(context.TODO(), existingConfigMap, metav1.UpdateOptions{})
	if err != nil {
		m.logger.Errorf("Failed to update ConfigMap '%s' in namespace '%s' after deleting %s: %v", m.nodeName, m.namespace, modelInfo, err)
		return err
	}
	m.logger.Infof("Successfully updated ConfigMap '%s' in namespace '%s' after deleting %s", m.nodeName, m.namespace, modelInfo)

	return nil
}
