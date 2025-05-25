package modelagent

import (
	"context"
	"fmt"
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
	var key string
	if op.BaseModel != nil {
		// '_' is not allowed in object namespace and name, so we can use it as a separator
		key = fmt.Sprintf("%s_%s", op.BaseModel.Namespace, op.BaseModel.Name)
	} else if op.ClusterBaseModel != nil {
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
			expectedNodeLabel:     constants.GetModelsLabelWithUid("test-uid"),
			expectedNodeValue:     string(Ready),
			expectedConfigMapData: map[string]string{"test-model": string(Ready)},
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
			expectedNodeLabel:     constants.GetModelsLabelWithUid("test-uid"),
			expectedNodeValue:     string(Ready),
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				"test-model":     string(Ready),
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
			expectedNodeLabel:     constants.GetModelsLabelWithUid("test-uid"),
			expectedNodeValue:     string(Updating),
			expectedConfigMapData: map[string]string{
				"existing-model":            string(Ready),
				"test-namespace_test-model": string(Updating),
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
			expectedNodeLabel:     constants.GetModelsLabelWithUid("test-uid"),
			expectedNodeValue:     string(Failed),
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				"test-model":     string(Failed),
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
				"test-model":     string(Ready),
			},
			expectedNodeLabel: constants.GetModelsLabelWithUid("test-uid"),
			// No expected value since label should be removed
			expectedConfigMapData: map[string]string{
				"existing-model": string(Ready),
				// "test-model" key should be removed
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
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome~1test-uid","value":"Ready"}]`,
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
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome~1test-uid","value":"Updating"}]`,
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
			expectedBytes: `[{"op":"add","path":"/metadata/labels/models.ome~1test-uid","value":"Failed"}]`,
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
			expectedBytes: `[{"op":"remove","path":"/metadata/labels/models.ome~1test-uid"}]`,
		},
		{
			name: "Empty UID in ClusterBaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				ClusterBaseModel: &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
						// Empty UID
					},
				},
			},
			expectedErrMsg: "node labeler get ClusterBaseModel test-model with empty UID",
		},
		{
			name: "Empty UID in BaseModel",
			op: &NodeLabelOp{
				ModelStateOnNode: Ready,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "test-namespace",
						// Empty UID
					},
				},
			},
			expectedErrMsg: "node labeler get BaseModel test-model in namespace test-namespace with empty UID",
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
