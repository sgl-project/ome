package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// ModelStateOnNode represents the model state in legacy format
// Maintained for backward compatibility with existing codepaths
type ModelStateOnNode string

// Model state constants (legacy format)
const (
	// Ready indicates the model is ready to use
	Ready ModelStateOnNode = "Ready"
	// Updating indicates the model is being downloaded or updated
	Updating ModelStateOnNode = "Updating"
	// Failed indicates the model failed to download or initialize
	Failed ModelStateOnNode = "Failed"
	// Deleted indicates the model was marked for deletion
	Deleted ModelStateOnNode = "Deleted"
)

// convertModelStateToStatus converts a legacy ModelStateOnNode to the new ModelStatus format
// This is necessary for compatibility during the transition period
func convertModelStateToStatus(state ModelStateOnNode) ModelStatus {
	switch state {
	case Ready:
		return ModelStatusReady
	case Updating:
		return ModelStatusUpdating
	case Failed:
		return ModelStatusFailed
	case Deleted:
		return ModelStatusDeleted
	default:
		return ModelStatusReady // Default to Ready for unknown states
	}
}

type NodeLabelOp struct {
	ModelStateOnNode ModelStateOnNode
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

type NodeLabeler struct {
	opRetry    int
	kubeClient *kubernetes.Clientset
	nodeName   string
	namespace  string
	logger     *zap.SugaredLogger
}

type patchStringValue struct {
	Op    string `json:"op,omitempty"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

func NewNodeLabeler(nodeName string, namespace string, kubeClient *kubernetes.Clientset, opRetry int, logger *zap.SugaredLogger) *NodeLabeler {
	return &NodeLabeler{
		opRetry:    opRetry,
		nodeName:   nodeName,
		kubeClient: kubeClient,
		namespace:  namespace,
		logger:     logger,
	}
}

// LabelNode applies model state changes to both node labels and ConfigMap
// Note: This method still uses retries for handling node label updates,
// but ConfigMap updates are now coordinated by Gopher's mutex
func (n *NodeLabeler) LabelNode(op *NodeLabelOp) error {
	return utils.Retry(n.opRetry, 100*time.Millisecond, func() error {
		return n.ProcessOp(op)
	})
}

// ProcessOp applies model state changes both to the node labels and to the ConfigMap
// This method is exported so that it can be called from Gopher with mutex protection
func (n *NodeLabeler) ProcessOp(op *NodeLabelOp) error {
	modelInfo := getModelOpInfo(op)

	n.logger.Infof("Processing %s operation for %s in state: %s", op.ModelStateOnNode, modelInfo, op.ModelStateOnNode)

	payloadBytes, err := getPatchPayloadBytes(op)
	if err != nil {
		n.logger.Errorf("Failed to get patch payload for %s: %v", modelInfo, err)
		return err
	}
	n.logger.Debugf("Generated patch payload for %s: %s", modelInfo, string(payloadBytes))

	// Patch the node
	_, err = n.kubeClient.CoreV1().Nodes().Patch(context.TODO(), n.nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		n.logger.Errorf("Failed to patch node %s for %s: %v", n.nodeName, modelInfo, err)
		return err
	}
	n.logger.Infof("Successfully patched node %s with %s state for %s", n.nodeName, op.ModelStateOnNode, modelInfo)

	// Get or create the ConfigMap
	configMap, needCreate, err := n.getOrNewConfigMap()
	if err != nil {
		n.logger.Errorf("Failed to get or create ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	n.logger.Debugf("Got ConfigMap (needCreate=%v) for %s: %+v", needCreate, modelInfo, configMap.Name)

	// Update the ConfigMap
	err = n.createOrUpdateConfigMap(configMap, op, needCreate)
	if err != nil {
		n.logger.Errorf("Failed to create/update ConfigMap for %s: %v", modelInfo, err)
		return err
	}
	n.logger.Infof("Successfully updated ConfigMap for %s with state: %s", modelInfo, op.ModelStateOnNode)

	return nil
}

func getModelOpInfo(op *NodeLabelOp) string {
	if op.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", op.BaseModel.Namespace, op.BaseModel.Name)
	} else if op.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", op.ClusterBaseModel.Name)
	}
	return "unknown model"
}

func getPatchPayloadBytes(op *NodeLabelOp) ([]byte, error) {
	var labelKey string
	if op.ClusterBaseModel != nil && len(op.ClusterBaseModel.UID) > 0 {
		labelKey = constants.GetModelsLabelWithUid(op.ClusterBaseModel.UID)
	} else if op.BaseModel != nil && len(op.BaseModel.UID) > 0 {
		labelKey = constants.GetModelsLabelWithUid(op.BaseModel.UID)
	}

	if len(labelKey) == 0 {
		if op.ClusterBaseModel != nil && len(op.ClusterBaseModel.UID) == 0 {
			return []byte{}, fmt.Errorf("node labeler get ClusterBaseModel %s with empty UID", op.ClusterBaseModel.Name)
		}

		if op.BaseModel != nil && len(op.BaseModel.UID) == 0 {
			return []byte{}, fmt.Errorf("node labeler get BaseModel %s in namespace %s with empty UID", op.BaseModel.Name, op.BaseModel.Namespace)
		}

		if op.ClusterBaseModel == nil && op.BaseModel == nil {
			return []byte{}, fmt.Errorf("node labeler get empty op without any models")
		}
		return []byte{}, nil
	}

	var payload []patchStringValue
	switch op.ModelStateOnNode {
	case Ready:
		payload = []patchStringValue{{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1")),
			Value: string(Ready),
		}}
	case Updating:
		payload = []patchStringValue{{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1")),
			Value: string(Updating),
		}}
	case Failed:
		payload = []patchStringValue{{
			Op:    "add",
			Path:  fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1")),
			Value: string(Failed),
		}}
	case Deleted:
		payload = []patchStringValue{{
			Op:   "remove",
			Path: fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1")),
		}}
	default:
		break
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return payloadBytes, nil
}

func (n *NodeLabeler) getOrNewConfigMap() (*corev1.ConfigMap, bool, error) {
	var notFound = false
	existedConfigMap, err := n.kubeClient.CoreV1().ConfigMaps(n.namespace).Get(context.TODO(), n.nodeName, metav1.GetOptions{})
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
				Name:      n.nodeName,
				Namespace: n.namespace,
				Labels:    labels,
			},
			Data: data,
		}, true, nil
	}

	return existedConfigMap, false, nil
}

func (n *NodeLabeler) createOrUpdateConfigMap(configMap *corev1.ConfigMap, op *NodeLabelOp, needCreate bool) error {
	// Get the model name and namespace based on the model type
	var modelName, namespace, modelInfo string
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
	n.logger.Debugf("Using key '%s' for %s", key, modelInfo)

	if configMap.Data == nil {
		n.logger.Debugf("ConfigMap Data is nil, initializing it for %s", modelInfo)
		configMap.Data = make(map[string]string)
	}

	// Check if there's already an entry for this model
	var modelEntry ModelEntry
	if existingData, exists := configMap.Data[key]; exists {
		// If entry exists, try to unmarshal it
		if err := json.Unmarshal([]byte(existingData), &modelEntry); err != nil {
			// If it's not in our format yet, create a new entry with just the status
			modelEntry = ModelEntry{
				Name:   modelName,
				Status: convertModelStateToStatus(op.ModelStateOnNode),
				Config: nil,
			}
		} else {
			// Update just the status, preserving the config
			modelEntry.Status = convertModelStateToStatus(op.ModelStateOnNode)
		}
	} else {
		// No existing entry, create a new one
		modelEntry = ModelEntry{
			Name:   modelName,
			Status: convertModelStateToStatus(op.ModelStateOnNode),
			Config: nil,
		}
	}

	// For 'Deleted' status, we might want to entirely remove the entry
	if op.ModelStateOnNode == Deleted {
		n.logger.Debugf("Deleting ConfigMap data[%s] for %s", key, modelInfo)
		delete(configMap.Data, key)
	} else {
		// Marshal the model entry to JSON
		entryJSON, err := json.Marshal(modelEntry)
		if err != nil {
			n.logger.Errorf("Failed to marshal model entry for %s: %v", modelInfo, err)
			return err
		}
		n.logger.Debugf("Setting ConfigMap data[%s] to %s for %s", key, string(entryJSON), modelInfo)
		configMap.Data[key] = string(entryJSON)
	}

	if needCreate {
		n.logger.Infof("Creating new ConfigMap '%s' in namespace '%s' for %s", configMap.Name, n.namespace, modelInfo)
		_, err := n.kubeClient.CoreV1().ConfigMaps(n.namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			n.logger.Errorf("Failed to create ConfigMap '%s' in namespace '%s' for %s: %v", configMap.Name, n.namespace, modelInfo, err)
			return err
		}
		n.logger.Infof("Successfully created ConfigMap '%s' in namespace '%s' for %s", configMap.Name, n.namespace, modelInfo)
	} else {
		n.logger.Infof("Updating ConfigMap '%s' in namespace '%s' for %s", configMap.Name, n.namespace, modelInfo)
		_, err := n.kubeClient.CoreV1().ConfigMaps(n.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			n.logger.Errorf("Failed to update ConfigMap '%s' in namespace '%s' for %s: %v", configMap.Name, n.namespace, modelInfo, err)
			return err
		}
		n.logger.Infof("Successfully updated ConfigMap '%s' in namespace '%s' for %s", configMap.Name, n.namespace, modelInfo)
	}
	return nil
}
