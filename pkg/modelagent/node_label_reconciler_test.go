package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// setupTest prepares a test environment with fake clients and test models
func setupTest(t *testing.T) (*NodeLabelReconciler, *fake.Clientset, *zap.SugaredLogger) {
	// Create a test logger
	logger := zaptest.NewLogger(t).Sugar()

	// Create a fake Kubernetes client
	kubeClient := fake.NewSimpleClientset()

	// Create a test node in the fake client
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-node",
			Labels: map[string]string{},
		},
	}
	_, err := kubeClient.CoreV1().Nodes().Create(context.TODO(), testNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Add a default successful reactor for patch operations
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		// Successfully patch the node and return it
		patchAction := action.(ktesting.PatchAction)
		// Only handle our test node
		if patchAction.GetName() == "test-node" {
			// Return the patched node
			return true, testNode, nil
		}
		// Let other patch operations fall through to default handlers
		return false, nil, nil
	})

	// Create the reconciler
	reconciler := NewNodeLabelReconciler("test-node", kubeClient, 3, logger)

	return reconciler, kubeClient, logger
}

// createTestBaseModel creates a test BaseModel for tests
func createTestBaseModel() *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
	}
}

// createTestClusterBaseModel creates a test ClusterBaseModel for tests
func createTestClusterBaseModel() *v1beta1.ClusterBaseModel {
	return &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-model",
		},
	}
}

// TestNewNodeLabelReconciler tests the constructor
func TestNewNodeLabelReconciler(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	kubeClient := fake.NewSimpleClientset()

	reconciler := NewNodeLabelReconciler("test-node", kubeClient, 3, logger)

	assert.NotNil(t, reconciler)
	assert.Equal(t, "test-node", reconciler.nodeName)
	assert.Equal(t, 3, reconciler.opRetry)
	assert.NotNil(t, reconciler.kubeClient)
	assert.NotNil(t, reconciler.logger)
}

// TestGetNodeLabelModelInfo tests the getNodeLabelModelInfo function
func TestGetNodeLabelModelInfo(t *testing.T) {
	// Test with BaseModel
	baseModel := createTestBaseModel()
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ClusterBaseModel: nil,
		ModelStateOnNode: Ready,
	}

	info := getNodeLabelModelInfo(op)
	assert.Equal(t, "BaseModel default/test-model", info)

	// Test with ClusterBaseModel
	clusterBaseModel := createTestClusterBaseModel()
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: clusterBaseModel,
		ModelStateOnNode: Ready,
	}

	info = getNodeLabelModelInfo(op)
	assert.Equal(t, "ClusterBaseModel test-cluster-model", info)

	// Test with no model
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: nil,
		ModelStateOnNode: Ready,
	}

	info = getNodeLabelModelInfo(op)
	assert.Equal(t, "unknown model", info)
}

// TestGetNodeLabelPatchPayloadBytes tests the getNodeLabelPatchPayloadBytes function
func TestGetNodeLabelPatchPayloadBytes(t *testing.T) {
	// Test with BaseModel and Ready state
	baseModel := createTestBaseModel()
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ClusterBaseModel: nil,
		ModelStateOnNode: Ready,
	}

	payload, err := getNodeLabelPatchPayloadBytes(op)
	assert.NoError(t, err)

	// Verify JSON patch structure
	var patches []patchStringValue
	err = json.Unmarshal(payload, &patches)
	assert.NoError(t, err)
	assert.Len(t, patches, 1)

	labelKey := constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name)
	expectedPath := fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1"))

	assert.Equal(t, "add", patches[0].Op)
	assert.Equal(t, expectedPath, patches[0].Path)
	assert.Equal(t, "Ready", patches[0].Value)

	// Test with ClusterBaseModel and Updating state
	clusterBaseModel := createTestClusterBaseModel()
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: clusterBaseModel,
		ModelStateOnNode: Updating,
	}

	payload, err = getNodeLabelPatchPayloadBytes(op)
	assert.NoError(t, err)

	err = json.Unmarshal(payload, &patches)
	assert.NoError(t, err)
	assert.Len(t, patches, 1)

	labelKey = constants.GetClusterBaseModelLabel(clusterBaseModel.Name)
	expectedPath = fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(labelKey, "/", "~1"))

	assert.Equal(t, "add", patches[0].Op)
	assert.Equal(t, expectedPath, patches[0].Path)
	assert.Equal(t, "Updating", patches[0].Value)

	// Test with Failed state
	op.ModelStateOnNode = Failed
	payload, err = getNodeLabelPatchPayloadBytes(op)
	assert.NoError(t, err)

	err = json.Unmarshal(payload, &patches)
	assert.NoError(t, err)
	assert.Len(t, patches, 1)
	assert.Equal(t, "add", patches[0].Op)
	// The Failed enum is converted to a string, so we need to compare with "Failed"
	assert.Equal(t, "Failed", patches[0].Value)

	// Test with Deleted state (should be "remove" operation)
	op.ModelStateOnNode = Deleted
	payload, err = getNodeLabelPatchPayloadBytes(op)
	assert.NoError(t, err)

	// For a remove operation, let's verify the raw JSON doesn't contain a value field
	var jsonMap []map[string]interface{}
	err = json.Unmarshal(payload, &jsonMap)
	assert.NoError(t, err)
	assert.Len(t, jsonMap, 1)
	assert.Equal(t, "remove", jsonMap[0]["op"])
	// In a remove operation, the value field should not exist in the JSON at all
	_, valueExists := jsonMap[0]["value"]
	assert.False(t, valueExists, "value field should not exist in remove operation")

	// Test with no model (should return error)
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: nil,
		ModelStateOnNode: Ready,
	}

	_, err = getNodeLabelPatchPayloadBytes(op)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty op without any models")
}

// TestApplyNodeLabelOperation tests the applyNodeLabelOperation method
func TestApplyNodeLabelOperation(t *testing.T) {
	reconciler, kubeClient, _ := setupTest(t)
	baseModel := createTestBaseModel()

	// Add a tracker to capture patch operations
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		// Let the default reactor handle the action, but capture the patch for verification
		patchAction := action.(ktesting.PatchAction)
		assert.Equal(t, "test-node", patchAction.GetName())
		assert.Equal(t, types.JSONPatchType, patchAction.GetPatchType())

		// Return default reactor response
		return false, nil, nil
	})

	// Test successful operation
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ModelStateOnNode: Ready,
	}

	err := reconciler.applyNodeLabelOperation(op)
	assert.NoError(t, err)

	// Verify that the node was patched with correct labels
	_, err = kubeClient.CoreV1().Nodes().Get(context.TODO(), "test-node", metav1.GetOptions{})
	assert.NoError(t, err)

	// Test error in patching
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("test error")
	})

	err = reconciler.applyNodeLabelOperation(op)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

// TestReconcileNodeLabels tests the ReconcileNodeLabels method
func TestReconcileNodeLabels(t *testing.T) {
	// Test successful patching
	reconciler, kubeClient, _ := setupTest(t)
	baseModel := createTestBaseModel()

	// Test successful reconciliation
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ModelStateOnNode: Ready,
	}

	err := reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)

	// Test retry logic with temporary errors - replace existing reactor with our test one
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		return false, nil, nil // Clear previous reactors first
	})

	var attempts int32
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		// Only handle test-node
		patchAction := action.(ktesting.PatchAction)
		if patchAction.GetName() != "test-node" {
			return false, nil, nil
		}

		if attempts < 2 {
			attempts++
			return true, nil, errors.New("temporary error")
		}
		// After two failures, succeed
		return true, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}, nil
	})

	// Reset attempts counter
	attempts = 0
	// This should succeed after retries
	err = reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)

	// Test permanent error - replace all previous reactors
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		return false, nil, nil // Clear previous reactors
	})

	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		// Only handle test-node
		patchAction := action.(ktesting.PatchAction)
		if patchAction.GetName() == "test-node" {
			return true, nil, errors.New("permanent error")
		}
		return false, nil, nil
	})

	// This should fail after all retries
	err = reconciler.ReconcileNodeLabels(op)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permanent error")
}

// TestDifferentModelStates tests applying different model states
func TestDifferentModelStates(t *testing.T) {
	reconciler, kubeClient, _ := setupTest(t)
	baseModel := createTestBaseModel()

	// Add a reactor to handle node patch operations successfully
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		// Return a successful response with the test node
		return true, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}, nil
	})

	// Test Ready state
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ModelStateOnNode: Ready,
	}

	err := reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)

	// Test Updating state
	op.ModelStateOnNode = Updating
	err = reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)

	// Test Failed state
	op.ModelStateOnNode = Failed
	err = reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)

	// Test Deleted state
	op.ModelStateOnNode = Deleted
	err = reconciler.ReconcileNodeLabels(op)
	assert.NoError(t, err)
}
