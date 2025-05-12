package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type ModelStateOnNode string

const (
	Ready    ModelStateOnNode = "Ready"
	Updating ModelStateOnNode = "Updating"
	Failed   ModelStateOnNode = "Failed"
	Deleted  ModelStateOnNode = "Deleted"
)

type NodeLabelOp struct {
	ModelStateOnNode ModelStateOnNode
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

type NodeLabeler struct {
	mu         sync.Mutex
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

func (n *NodeLabeler) LabelNode(op *NodeLabelOp) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return utils.Retry(n.opRetry, 100*time.Millisecond, func() error {
		return n.processOp(op)
	})
}

func (n *NodeLabeler) processOp(op *NodeLabelOp) error {
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
	var key, modelInfo string
	if op.BaseModel != nil {
		key = fmt.Sprintf("%s_%s", op.BaseModel.Namespace, op.BaseModel.Name)
		modelInfo = fmt.Sprintf("BaseModel %s/%s", op.BaseModel.Namespace, op.BaseModel.Name)
		n.logger.Debugf("Using key '%s' for %s", key, modelInfo)
	} else {
		key = op.ClusterBaseModel.Name
		modelInfo = fmt.Sprintf("ClusterBaseModel %s", op.ClusterBaseModel.Name)
		n.logger.Debugf("Using key '%s' for %s", key, modelInfo)
	}

	if configMap.Data == nil {
		n.logger.Debugf("ConfigMap Data is nil, initializing it for %s", modelInfo)
		configMap.Data = make(map[string]string)
	}

	switch op.ModelStateOnNode {
	case Ready:
		n.logger.Debugf("Setting ConfigMap data[%s] = Ready for %s", key, modelInfo)
		configMap.Data[key] = string(Ready)
	case Updating:
		n.logger.Debugf("Setting ConfigMap data[%s] = Updating for %s", key, modelInfo)
		configMap.Data[key] = string(Updating)
	case Failed:
		n.logger.Debugf("Setting ConfigMap data[%s] = Failed for %s", key, modelInfo)
		configMap.Data[key] = string(Failed)
	case Deleted:
		n.logger.Debugf("Deleting ConfigMap data[%s] for %s", key, modelInfo)
		delete(configMap.Data, key)
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
