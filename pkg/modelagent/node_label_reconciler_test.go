package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
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

// TestGetModelLabelKey tests the getModelLabelKey function
func TestGetModelLabelKey(t *testing.T) {
	// Test with BaseModel
	baseModel := createTestBaseModel()
	op := &NodeLabelOp{
		BaseModel:        baseModel,
		ClusterBaseModel: nil,
	}
	labelKey, err := getModelLabelKey(op)
	assert.NoError(t, err)
	assert.Equal(t, constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name), labelKey)

	// Test with ClusterBaseModel
	clusterBaseModel := createTestClusterBaseModel()
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: clusterBaseModel,
	}
	labelKey, err = getModelLabelKey(op)
	assert.NoError(t, err)
	assert.Equal(t, constants.GetClusterBaseModelLabel(clusterBaseModel.Name), labelKey)

	// Test with no model (should return error)
	op = &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: nil,
	}
	_, err = getModelLabelKey(op)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node labeler got empty op without any models")
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

func TestGetNodeLabelMergePatchPayloadBytes(t *testing.T) {
	baseModel := createTestBaseModel()
	clusterBaseModel := createTestClusterBaseModel()

	tests := []struct {
		name          string
		op            *NodeLabelOp
		labelKey      string
		expectedValue interface{}
	}{
		{
			name: "BaseModel Ready",
			op: &NodeLabelOp{
				BaseModel:        baseModel,
				ClusterBaseModel: nil,
				ModelStateOnNode: Ready,
			},
			labelKey:      constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name),
			expectedValue: string(Ready),
		},
		{
			name: "BaseModel Failed",
			op: &NodeLabelOp{
				BaseModel:        baseModel,
				ClusterBaseModel: nil,
				ModelStateOnNode: Failed,
			},
			labelKey:      constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name),
			expectedValue: string(Failed),
		},
		{
			name: "ClusterBaseModel Updating",
			op: &NodeLabelOp{
				BaseModel:        nil,
				ClusterBaseModel: clusterBaseModel,
				ModelStateOnNode: Updating,
			},
			labelKey:      constants.GetClusterBaseModelLabel(clusterBaseModel.Name),
			expectedValue: string(Updating),
		},
		{
			name: "ClusterBaseModel Deleted",
			op: &NodeLabelOp{
				BaseModel:        nil,
				ClusterBaseModel: clusterBaseModel,
				ModelStateOnNode: Deleted,
			},
			labelKey:      constants.GetClusterBaseModelLabel(clusterBaseModel.Name),
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := getNodeLabelMergePatchPayloadBytes(tt.op)
			assert.NoError(t, err)

			var patch map[string]map[string]map[string]interface{}
			err = json.Unmarshal(payload, &patch)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, patch["metadata"]["labels"][tt.labelKey])
			assert.Len(t, patch["metadata"]["labels"], 1)
		})
	}

	_, err := getNodeLabelMergePatchPayloadBytes(&NodeLabelOp{ModelStateOnNode: Ready})
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
		assert.Equal(t, types.MergePatchType, patchAction.GetPatchType())

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

func TestApplyNodeLabelOperationWithMissingLabels(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	kubeClient := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-without-labels",
		},
	})
	reconciler := NewNodeLabelReconciler("node-without-labels", kubeClient, 1, logger)
	baseModel := createTestBaseModel()

	err := reconciler.applyNodeLabelOperation(&NodeLabelOp{
		BaseModel:        baseModel,
		ModelStateOnNode: Failed,
	})
	assert.NoError(t, err)

	node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), "node-without-labels", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, string(Failed), node.Labels[constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name)])
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

// TestIdempotentOperations tests idempotent operation handling
func TestIdempotentOperations(t *testing.T) {
	// Set up the test environment
	reconciler, kubeClient, _ := setupTest(t)

	// Create a test node with existing label for the already-applied test
	labeledNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "labeled-node",
			Labels: map[string]string{
				constants.GetClusterBaseModelLabel("test-cluster-model"): string(Ready),
			},
		},
	}
	_, err := kubeClient.CoreV1().Nodes().Create(context.TODO(), labeledNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Prepare a ClusterBaseModel test case
	clusterBaseModel := createTestClusterBaseModel()

	// Test case 1: Idempotent Ready operation (label already exists with correct value)
	op := &NodeLabelOp{
		ClusterBaseModel: clusterBaseModel,
		ModelStateOnNode: Ready,
	}
	reconciler.nodeName = "labeled-node" // Point to the node with existing label
	err = reconciler.applyNodeLabelOperation(op)
	assert.NoError(t, err)

	// Test case 2: Idempotent Delete operation (label doesn't exist)
	// First create a node without any labels
	unlabeledNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "unlabeled-node",
			Labels: map[string]string{},
		},
	}
	_, err = kubeClient.CoreV1().Nodes().Create(context.TODO(), unlabeledNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Setup delete operation on a node without the label
	op = &NodeLabelOp{
		ClusterBaseModel: clusterBaseModel,
		ModelStateOnNode: Deleted,
	}
	reconciler.nodeName = "unlabeled-node" // Point to node without label
	err = reconciler.applyNodeLabelOperation(op)
	assert.NoError(t, err)

	// Test case 3: invalid model refs are ignored before patching
	invalidModelOp := &NodeLabelOp{
		BaseModel:        nil,
		ClusterBaseModel: nil,
		ModelStateOnNode: Ready,
	}
	reconciler.nodeName = "test-node"
	err = reconciler.applyNodeLabelOperation(invalidModelOp)
	assert.NoError(t, err)
}

// TestNodeLabelErrorHandling tests error handling in applyNodeLabelOperation
func TestNodeLabelErrorHandling(t *testing.T) {
	// Set up the test environment
	reconciler, kubeClient, _ := setupTest(t)

	// Prepare a model for tests
	clusterBaseModel := createTestClusterBaseModel()
	// Setup a delete operation for a label that will fail with a non-retryable request error
	op := &NodeLabelOp{
		ClusterBaseModel: clusterBaseModel,
		ModelStateOnNode: Deleted,
	}

	// Mock the patch to fail with a bad request error
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(ktesting.PatchAction)
		if patchAction.GetName() == "test-node" {
			return true, nil, apierrors.NewBadRequest("patch rejected")
		}
		return false, nil, nil
	})

	// Run the operation - it should handle the error gracefully
	err := reconciler.applyNodeLabelOperation(op)
	assert.NoError(t, err) // Should return nil since we consider this non-retryable

	// Mock a conflict error, which should be retryable
	kubeClient.PrependReactor("patch", "nodes", func(action ktesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(ktesting.PatchAction)
		if patchAction.GetName() == "test-node" {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "nodes"}, "test-node", errors.New("object modified"))
		}
		return false, nil, nil
	})

	// Run the operation - it should return the error for retry
	op.ModelStateOnNode = Ready // Change to an add operation
	err = reconciler.applyNodeLabelOperation(op)
	assert.Error(t, err) // Should return error for conflict to trigger retry
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
