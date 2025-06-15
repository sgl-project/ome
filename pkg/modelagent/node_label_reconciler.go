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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// NodeLabelOp represents an operation on node labels
// This is used to pass model references to the NodeLabelReconciler
type NodeLabelOp struct {
	ModelStateOnNode ModelStateOnNode
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

// NodeLabelReconciler handles updating node labels œwith model status information
// It provides a clean separation from ConfigMap operations
type NodeLabelReconciler struct {
	opRetry    int                  // Number of retries for operations
	kubeClient kubernetes.Interface // Kubernetes client for node operations
	nodeName   string               // The name of the node
	logger     *zap.SugaredLogger   // Logger for recording operations
}

// patchStringValue represents a JSON patch operation for node labels
type patchStringValue struct {
	Op    string `json:"op,omitempty"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

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

// NewNodeLabelReconciler creates a new NodeLabelReconciler instance
func NewNodeLabelReconciler(nodeName string, kubeClient kubernetes.Interface, opRetry int, logger *zap.SugaredLogger) *NodeLabelReconciler {
	return &NodeLabelReconciler{
		opRetry:    opRetry,
		nodeName:   nodeName,
		kubeClient: kubeClient,
		logger:     logger,
	}
}

// ReconcileNodeLabels applies model state changes to node labels with retries
func (n *NodeLabelReconciler) ReconcileNodeLabels(op *NodeLabelOp) error {
	return utils.Retry(n.opRetry, 100*time.Millisecond, func() error {
		return n.applyNodeLabelOperation(op)
	})
}

// applyNodeLabelOperation applies model state changes to the node labels
func (n *NodeLabelReconciler) applyNodeLabelOperation(op *NodeLabelOp) error {
	modelInfo := getNodeLabelModelInfo(op)
	n.logger.Infof("Processing node label %s operation for %s in state: %s", op.ModelStateOnNode, modelInfo, op.ModelStateOnNode)

	// First check if the node exists to avoid unnecessary work and clearer error handling
	_, err := n.kubeClient.CoreV1().Nodes().Get(context.TODO(), n.nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Node doesn't exist, log warning and return nil to avoid retries
			n.logger.Warnf("Node %s not found, skipping node labeling for %s: %v", n.nodeName, modelInfo, err)
			return nil // Return nil to avoid retries for non-existent nodes
		}
		// For other errors, log and return error for possible retry
		n.logger.Errorf("Error checking node %s existence for %s: %v", n.nodeName, modelInfo, err)
		return err
	}

	payloadBytes, err := getNodeLabelPatchPayloadBytes(op)
	if err != nil {
		n.logger.Errorf("Failed to get node label patch payload for %s: %v", modelInfo, err)
		return err
	}
	n.logger.Debugf("Generated node label patch payload for %s: %s", modelInfo, string(payloadBytes))

	// Patch the node
	_, err = n.kubeClient.CoreV1().Nodes().Patch(
		context.TODO(),
		n.nodeName,
		types.JSONPatchType,
		payloadBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		// Check for specific error types and handle them gracefully
		if errors.IsNotFound(err) {
			// Node disappeared after our initial check
			n.logger.Warnf("Node %s not found during patch operation for %s, skipping", n.nodeName, modelInfo)
			return nil // Don't retry for non-existent nodes
		} else if errors.IsInvalid(err) || errors.IsBadRequest(err) {
			// Invalid request, could be malformed patch, log but don't retry
			n.logger.Errorf("Invalid patch request for node %s and model %s: %v", n.nodeName, modelInfo, err)
			return nil // Don't retry for bad requests
		}

		// For other errors, log and return for retry
		n.logger.Errorf("Failed to patch node %s for %s: %v", n.nodeName, modelInfo, err)
		return err
	}
	n.logger.Infof("Successfully patched node %s with %s state for %s", n.nodeName, op.ModelStateOnNode, modelInfo)

	return nil
}

// getNodeLabelModelInfo returns a string identifying a model for logging
func getNodeLabelModelInfo(op *NodeLabelOp) string {
	if op.BaseModel != nil {
		return fmt.Sprintf("BaseModel %s/%s", op.BaseModel.Namespace, op.BaseModel.Name)
	} else if op.ClusterBaseModel != nil {
		return fmt.Sprintf("ClusterBaseModel %s", op.ClusterBaseModel.Name)
	}
	return "unknown model"
}

// getNodeLabelPatchPayloadBytes generates the JSON patch for node labels
func getNodeLabelPatchPayloadBytes(op *NodeLabelOp) ([]byte, error) {
	var labelKey string

	// Use the deterministic labeling system
	if op.ClusterBaseModel != nil {
		labelKey = constants.GetClusterBaseModelLabel(op.ClusterBaseModel.Name)
	} else if op.BaseModel != nil {
		labelKey = constants.GetBaseModelLabel(op.BaseModel.Namespace, op.BaseModel.Name)
	}

	if len(labelKey) == 0 {
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
