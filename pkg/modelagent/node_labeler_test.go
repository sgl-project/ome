package modelagent

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockNodeLabeler provides a testable implementation of NodeLabelerInterface
type MockNodeLabeler struct {
	// Mock fields
	NodeName   string
	Namespace  string
	OpRetry    int
	FakeClient *fake.Clientset

	// Test tracking
	LastOp *NodeLabelOp
}

// Implement the LabelNode interface method
func (m *MockNodeLabeler) LabelNode(op *NodeLabelOp) error {
	m.LastOp = op

	// Get or create configmap
	configMap, needCreate, err := m.getOrNewConfigMap()
	if err != nil {
		return err
	}

	// Update the configmap
	return m.createOrUpdateConfigMap(configMap, op, needCreate)
}

// Implements getOrNewConfigMap for testing
func (m *MockNodeLabeler) getOrNewConfigMap() (*corev1.ConfigMap, bool, error) {
	var notFound = false
	existedConfigMap, err := m.FakeClient.CoreV1().ConfigMaps(m.Namespace).Get(context.TODO(), m.NodeName, metav1.GetOptions{})
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
				Name:      m.NodeName,
				Namespace: m.Namespace,
				Labels:    labels,
			},
			Data: data,
		}, true, nil
	}

	return existedConfigMap, false, nil
}

// Implements createOrUpdateConfigMap for testing
func (m *MockNodeLabeler) createOrUpdateConfigMap(configMap *corev1.ConfigMap, op *NodeLabelOp, needCreate bool) error {
	// Get the model name and namespace based on the model type
	var modelName, namespace string
	var isClusterBaseModel bool

	if op.BaseModel != nil {
		modelName = op.BaseModel.Name
		namespace = op.BaseModel.Namespace
		isClusterBaseModel = false
	} else if op.ClusterBaseModel != nil {
		modelName = op.ClusterBaseModel.Name
		namespace = ""
		isClusterBaseModel = true
	}

	// Get the new deterministic key for this model
	key := constants.GetModelConfigMapKey(namespace, modelName, isClusterBaseModel)

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	switch op.ModelStateOnNode {
	case Ready, Updating, Failed:
		// Create model entry with the new format
		modelEntry := ModelEntry{
			Name:   modelName,
			Status: convertModelStateToStatus(op.ModelStateOnNode),
			Config: nil,
		}
		entryJSON, err := json.Marshal(modelEntry)
		if err != nil {
			return err
		}
		configMap.Data[key] = string(entryJSON)
	case Deleted:
		delete(configMap.Data, key)
	}

	var err error
	if needCreate {
		_, err = m.FakeClient.CoreV1().ConfigMaps(m.Namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	} else {
		_, err = m.FakeClient.CoreV1().ConfigMaps(m.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	}
	return err
}

func TestLabelNode(t *testing.T) {
	// Test cases
	testCases := []struct {
		name                  string
		op                    *NodeLabelOp
		configMapExists       bool
		existingConfigMapData map[string]string
		expectedNodeLabel     string
		expectedNodeValue     string
		expectedConfigMapData map[string]string
		expectError           bool
	}{
		{
			name: "Add Ready label with new ConfigMap",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			configMapExists:       false,
			expectedNodeLabel:     constants.GetClusterBaseModelLabel("test-model"),
			expectedNodeValue:     string(Ready),
			expectedConfigMapData: map[string]string{constants.GetModelConfigMapKey("", "test-model", true): `{"name":"test-model","status":"Ready"}`},
		},
		{
			name: "Add Ready label with existing ConfigMap",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			configMapExists:       true,
			existingConfigMapData: map[string]string{"existing-model": string(Ready)},
			expectedNodeLabel:     constants.GetClusterBaseModelLabel("test-model"),
			expectedNodeValue:     string(Ready),
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				constants.GetModelConfigMapKey("", "test-model", true): `{"name":"test-model","status":"Ready"}`,
			},
		},
		{
			name: "Add Updating label",
			op: &NodeLabelOp{
				ModelStateOnNode: Updating,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "test-namespace",
						UID:       "test-uid",
					},
				},
			},
			configMapExists:       true,
			existingConfigMapData: map[string]string{"existing-model": string(Ready)},
			expectedNodeLabel:     constants.GetBaseModelLabel("test-namespace", "test-model"),
			expectedNodeValue:     string(Updating),
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				constants.GetModelConfigMapKey("test-namespace", "test-model", false): `{"name":"test-model","status":"Updating"}`,
			},
		},
		{
			name: "Add Failed label",
			op: &NodeLabelOp{
				ModelStateOnNode: Failed,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			configMapExists:       true,
			existingConfigMapData: map[string]string{"existing-model": string(Ready)},
			expectedNodeLabel:     constants.GetClusterBaseModelLabel("test-model"),
			expectedNodeValue:     string(Failed),
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				constants.GetModelConfigMapKey("", "test-model", true): `{"name":"test-model","status":"Failed"}`,
			},
		},
		{
			name: "Delete label",
			op: &NodeLabelOp{
				ModelStateOnNode: Deleted,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			configMapExists: true,
			existingConfigMapData: map[string]string{
				"existing-model": string(Ready),
				constants.GetModelConfigMapKey("", "test-model", true): `{"name":"test-model","status":"Ready"}`,
			},
			expectedNodeLabel: constants.GetClusterBaseModelLabel("test-model"),
			// No expected value since label should be removed
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				// model entry should be removed for Deleted state
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fake clientset with a test node
			testNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-node",
					Labels: make(map[string]string),
				},
			}
			kubeClient := fake.NewSimpleClientset(testNode)

			// Add ConfigMap if needed
			if tc.configMapExists {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-node",
						Namespace: "test-namespace",
						Labels: map[string]string{
							constants.ModelStatusConfigMapLabel: "true",
						},
					},
					Data: tc.existingConfigMapData,
				}
				_, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Create(context.TODO(), configMap, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test ConfigMap: %v", err)
				}
			}

			// Create mock labeler
			labeler := &MockNodeLabeler{
				NodeName:   "test-node",
				Namespace:  "test-namespace",
				OpRetry:    3,
				FakeClient: kubeClient,
			}

			// Execute the labeling operation
			err := labeler.LabelNode(tc.op)

			// Check for errors
			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				return
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify the operation was recorded correctly
			if !reflect.DeepEqual(labeler.LastOp, tc.op) {
				t.Errorf("Operation was not correctly recorded: got %+v, want %+v", labeler.LastOp, tc.op)
			}

			// Verify ConfigMap data
			configMap, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Get(context.TODO(), "test-node", metav1.GetOptions{})
			if err != nil {
				if !tc.configMapExists && errors.IsNotFound(err) {
					// This is expected if the config map didn't exist
					return
				}
				t.Fatalf("Failed to get ConfigMap: %v", err)
			}

			if !reflect.DeepEqual(configMap.Data, tc.expectedConfigMapData) {
				t.Errorf("Expected ConfigMap data %v, got %v", tc.expectedConfigMapData, configMap.Data)
			}
		})
	}
}

func TestGetPatchPayloadBytes(t *testing.T) {
	testCases := []struct {
		name           string
		op             *NodeLabelOp
		expectedBytes  string
		expectedErrMsg string
	}{
		{
			name: "Ready state with ClusterBaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome.io~1clusterbasemodel.test-model","value":"Ready"}]`,
		},
		{
			name: "Updating state with BaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Updating,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "test-namespace",
						UID:       "test-uid",
					},
				},
			},
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome.io~1test-namespace.basemodel.test-model","value":"Updating"}]`,
		},
		{
			name: "Failed state with ClusterBaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Failed,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome.io~1clusterbasemodel.test-model","value":"Failed"}]`,
		},
		{
			name: "Deleted state with ClusterBaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Deleted,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						UID:  "test-uid",
					},
				},
			},
			expectedBytes: `[{"op":"remove","path":"/metadata/labels/models.ome.io~1clusterbasemodel.test-model"}]`,
		},
		{
			name: "No models provided",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				// No models
			},
			expectedErrMsg: "node labeler get empty op without any models",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payloadBytes, err := getPatchPayloadBytes(tc.op)

			// Check error
			if tc.expectedErrMsg != "" {
				if err == nil {
					t.Fatalf("Expected error containing '%s', got nil", tc.expectedErrMsg)
				}
				if err.Error() != tc.expectedErrMsg {
					t.Fatalf("Expected error '%s', got '%s'", tc.expectedErrMsg, err.Error())
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Fatalf("Expected no error, got '%s'", err.Error())
			}

			// Check payload bytes
			actualBytes := string(payloadBytes)
			if actualBytes != tc.expectedBytes {
				t.Fatalf("Expected bytes '%s', got '%s'", tc.expectedBytes, actualBytes)
			}
		})
	}
}
