package model_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
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
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func NewNodeLabler(nodeName string, namespace string, kubeClient *kubernetes.Clientset, opRetry int) *NodeLabeler {
	return &NodeLabeler{
		opRetry:    opRetry,
		nodeName:   nodeName,
		kubeClient: kubeClient,
		namespace:  namespace,
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
	payloadBytes, err := getPatchPayloadBytes(op)
	if err != nil {
		return err
	}

	_, err = n.kubeClient.CoreV1().Nodes().Patch(context.TODO(), n.nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		return err
	}

	configMap, needCreate, err := n.getOrNewConfigMap()
	if err != nil {
		return err
	}

	err = n.createOrUpdateConfigMap(configMap, op, needCreate)
	if err != nil {
		return err
	}

	return nil
}

func (n *NodeLabeler) getOrNewConfigMap() (*corev1.ConfigMap, bool, error) {
	var notFound bool = false
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
			return []byte{}, fmt.Errorf("node labeler get BaseModel %s in namespace %s with empty UID", op.ClusterBaseModel.Name, op.ClusterBaseModel.Namespace)
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

func (n *NodeLabeler) createOrUpdateConfigMap(configMap *corev1.ConfigMap, op *NodeLabelOp, needCreate bool) error {
	var key string
	if op.BaseModel != nil {
		// '_' is not allowed in object namespace and name, so we can use it as a seperater
		key = fmt.Sprintf("%s_%s", op.BaseModel.Namespace, op.BaseModel.Name)
	} else {
		key = op.ClusterBaseModel.Name
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	switch op.ModelStateOnNode {
	case Ready:
		configMap.Data[key] = string(Ready)
	case Updating:
		configMap.Data[key] = string(Updating)
	case Failed:
		configMap.Data[key] = string(Failed)
	case Deleted:
		delete(configMap.Data, key)
	}

	if needCreate {
		_, err := n.kubeClient.CoreV1().ConfigMaps(n.namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := n.kubeClient.CoreV1().ConfigMaps(n.namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
