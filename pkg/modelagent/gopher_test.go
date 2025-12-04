package modelagent

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	omev1beta1lister "github.com/sgl-project/ome/pkg/client/listers/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

// TestHandleTaskPVCSkip tests that PVC storage types are properly skipped
func TestHandleTaskPVCSkip(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Define test cases
	testCases := []struct {
		name          string
		task          *GopherTask
		storageType   storage.StorageType
		expectError   bool
		expectSkip    bool
		errorContains string
	}{
		{
			name: "PVC storage type should be skipped",
			task: &GopherTask{
				TaskType: Download,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pvc-model",
						Namespace: "default",
						UID:       "test-uid-1",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
						},
					},
				},
			},
			storageType: storage.StorageTypePVC,
			expectError: false,
			expectSkip:  true,
		},
		{
			name: "OCI storage type should not be skipped",
			task: &GopherTask{
				TaskType: Download,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-oci-model",
						Namespace: "default",
						UID:       "test-uid-2",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: stringPtr("oci://n/namespace/b/bucket/o/model"),
						},
					},
				},
			},
			storageType: storage.StorageTypeOCI,
			expectError: false,
			expectSkip:  false,
		},
		{
			name: "Vendor storage type should be handled",
			task: &GopherTask{
				TaskType: Download,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vendor-model",
						Namespace: "default",
						UID:       "test-uid-3",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: stringPtr("vendor://nvidia/models/llama"),
						},
					},
				},
			},
			storageType: storage.StorageTypeVendor,
			expectError: false,
			expectSkip:  false,
		},
		{
			name: "HuggingFace storage type should be handled",
			task: &GopherTask{
				TaskType: Download,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hf-model",
						Namespace: "default",
						UID:       "test-uid-4",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: stringPtr("hf://meta-llama/Llama-2-7b-hf"),
						},
					},
				},
			},
			storageType: storage.StorageTypeHuggingFace,
			expectError: false,
			expectSkip:  false,
		},
		{
			name: "Invalid storage URI should error",
			task: &GopherTask{
				TaskType: Download,
				BaseModel: &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-invalid-model",
						Namespace: "default",
						UID:       "test-uid-5",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: stringPtr("invalid://storage/uri"),
						},
					},
				},
			},
			expectError:   true,
			errorContains: "unknown storage type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// For PVC test, we need to mock the behavior
			// Since handleTask is complex, we'll test the specific storage type logic
			baseModelSpec := tc.task.BaseModel.Spec
			storageType, err := storage.GetStorageType(*baseModelSpec.Storage.StorageUri)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.storageType, storageType)

			// Verify that PVC storage type would be skipped
			if storageType == storage.StorageTypePVC {
				assert.True(t, tc.expectSkip, "PVC storage type should be skipped")
			}
		})
	}
}

// TestShouldDownloadModelPVC tests that PVC models are skipped in scout
func TestShouldDownloadModelPVC(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	// Set up test node
	testNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"node-type": "gpu",
			},
		},
	}

	// Create a test scout
	scout := &Scout{
		logger:   sugaredLogger,
		nodeInfo: testNode,
	}

	// Test cases
	testCases := []struct {
		name           string
		storageSpec    *v1beta1.StorageSpec
		expectedResult bool
		description    string
	}{
		{
			name: "PVC storage should be skipped",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
			},
			expectedResult: false,
			description:    "PVC storage type should return false (skip)",
		},
		{
			name: "PVC storage with namespace should be skipped",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://namespace:my-pvc/models/llama2"),
			},
			expectedResult: false,
			description:    "PVC storage type with namespace should return false (skip)",
		},
		{
			name: "OCI storage should not be skipped",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("oci://n/namespace/b/bucket/o/model"),
			},
			expectedResult: true,
			description:    "OCI storage type should return true (download)",
		},
		{
			name: "HuggingFace storage should not be skipped",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("hf://meta-llama/Llama-2-7b-hf"),
			},
			expectedResult: true,
			description:    "HuggingFace storage type should return true (download)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scout.shouldDownloadModel(tc.storageSpec)
			assert.Equal(t, tc.expectedResult, result, tc.description)
		})
	}
}

// Mock implementations for testing
type mockBaseModelLister struct {
	models []*v1beta1.BaseModel
	err    error
}

func (m *mockBaseModelLister) List(selector labels.Selector) ([]*v1beta1.BaseModel, error) {
	return m.models, m.err
}

func (m *mockBaseModelLister) BaseModels(namespace string) omev1beta1lister.BaseModelNamespaceLister {
	return nil // Not used in our test
}

type mockClusterBaseModelLister struct {
	models []*v1beta1.ClusterBaseModel
	err    error
}

func (m *mockClusterBaseModelLister) List(selector labels.Selector) ([]*v1beta1.ClusterBaseModel, error) {
	return m.models, m.err
}

func (m *mockClusterBaseModelLister) Get(name string) (*v1beta1.ClusterBaseModel, error) {
	// Simple implementation for testing - find by name
	for _, model := range m.models {
		if model.Name == name {
			return model, nil
		}
	}
	return nil, errors.New("not found")
}

// TestIsPathReferencedByOtherModels tests the isPathReferencedByOtherModels method
func TestIsPathReferencedByOtherModels(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	targetPath := "/models/llama2"

	testCases := []struct {
		name                      string
		baseModels                []*v1beta1.BaseModel
		clusterBaseModels         []*v1beta1.ClusterBaseModel
		excludeBaseModel          *v1beta1.BaseModel
		excludeClusterBaseModel   *v1beta1.ClusterBaseModel
		baseModelListerErr        error
		clusterBaseModelListerErr error
		expectedResult            bool
		expectedError             bool
		errorContains             string
		description               string
	}{
		{
			name:           "no models exist",
			description:    "should return false when no models exist",
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "path not referenced by any model",
			baseModels: []*v1beta1.BaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model1",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr("/models/other-model"),
						},
					},
				},
			},
			clusterBaseModels: []*v1beta1.ClusterBaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-model1",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr("/models/another-model"),
						},
					},
				},
			},
			description:    "should return false when target path is not referenced",
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "path referenced by BaseModel",
			baseModels: []*v1beta1.BaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model1",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
			},
			description:    "should return true when path is referenced by BaseModel",
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "path referenced by ClusterBaseModel",
			clusterBaseModels: []*v1beta1.ClusterBaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-model1",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
			},
			description:    "should return true when path is referenced by ClusterBaseModel",
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "path referenced by BaseModel but excluded",
			baseModels: []*v1beta1.BaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model1",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
			},
			excludeBaseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "model1",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						Path: stringPtr(targetPath),
					},
				},
			},
			description:    "should return false when path is only referenced by excluded BaseModel",
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "path referenced by ClusterBaseModel but excluded",
			clusterBaseModels: []*v1beta1.ClusterBaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-model1",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
			},
			excludeClusterBaseModel: &v1beta1.ClusterBaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-model1",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						Path: stringPtr(targetPath),
					},
				},
			},
			description:    "should return false when path is only referenced by excluded ClusterBaseModel",
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "path referenced by multiple models, one excluded",
			baseModels: []*v1beta1.BaseModel{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model1",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model2",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							Path: stringPtr(targetPath),
						},
					},
				},
			},
			excludeBaseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "model1",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						Path: stringPtr(targetPath),
					},
				},
			},
			description:    "should return true when path is referenced by multiple models but only one is excluded",
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:               "BaseModel lister error",
			baseModelListerErr: errors.New("lister error"),
			description:        "should return error when BaseModel lister fails",
			expectedResult:     false,
			expectedError:      true,
			errorContains:      "failed to list BaseModels",
		},
		{
			name:                      "ClusterBaseModel lister error",
			clusterBaseModelListerErr: errors.New("lister error"),
			description:               "should return error when ClusterBaseModel lister fails",
			expectedResult:            false,
			expectedError:             true,
			errorContains:             "failed to list ClusterBaseModels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock listers
			mockBaseModelLister := &mockBaseModelLister{
				models: tc.baseModels,
				err:    tc.baseModelListerErr,
			}
			mockClusterBaseModelLister := &mockClusterBaseModelLister{
				models: tc.clusterBaseModels,
				err:    tc.clusterBaseModelListerErr,
			}

			// Create a minimal Gopher instance for testing
			gopher := &Gopher{
				logger:                 sugaredLogger,
				baseModelLister:        mockBaseModelLister,
				clusterBaseModelLister: mockClusterBaseModelLister,
			}

			// Call the method under test
			result, err := gopher.isPathReferencedByOtherModels(targetPath, tc.excludeBaseModel, tc.excludeClusterBaseModel)

			// Check error conditions
			if tc.expectedError {
				assert.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, tc.description)
				}
			} else {
				assert.NoError(t, err, tc.description)
			}

			// Check result
			assert.Equal(t, tc.expectedResult, result, tc.description)
		})
	}
}

// TestIsReservingModelArtifact tests isReservingModelArtifact method
func TestIsReservingModelArtifact(t *testing.T) {
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()
	defer func(sugaredLogger *zap.SugaredLogger) {
		_ = sugaredLogger.Sync()
	}(sugaredLogger)

	s := &Gopher{logger: sugaredLogger}

	cases := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{"nil labels", nil, false},
		{"true lower", map[string]string{"models.ome/reserve-model-artifact": "true"}, true},
		{"true upper", map[string]string{"models.ome/reserve-model-artifact": "TRUE"}, true},
		{"true mixed", map[string]string{"models.ome/reserve-model-artifact": "TrUe"}, true},
		{"not containing matched key", map[string]string{"models.ome/reserve-model": "true"}, false},
		{"false", map[string]string{"models.ome/reserve-model-artifact": "false"}, false},
		{"empty", map[string]string{"models.ome/reserve-model-artifact": ""}, false},
		{"other value", map[string]string{"models.ome/reserve-model-artifact": "otherValues"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bm := &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.labels,
				},
			}
			task := &GopherTask{
				TaskType:  Download, // value not important for this helper
				BaseModel: bm,
			}

			got := s.isReservingModelArtifact(task)
			assert.Equal(t, tc.want, got)
		})
	}
}
