package isvc

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
)

// =============================================================================
// Helper Functions
// =============================================================================

// createTestScheme creates a runtime scheme with v1beta1 types for testing
func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)
	return scheme
}

// createFakeClient creates a fake client with optional ClusterBaseModel or BaseModel objects
func createFakeClient(t *testing.T, models ...client.Object) client.Client {
	scheme := createTestScheme()
	objects := []client.Object{}
	objects = append(objects, models...)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

// Helper function to create int64 pointers
func int64Ptr(i int64) *int64 {
	return &i
}

// Helper to check if a service has a deprecation warning annotation
func hasDeprecationWarning(isvc *v1beta1.InferenceService) bool {
	if isvc.ObjectMeta.Annotations == nil {
		return false
	}
	_, exists := isvc.ObjectMeta.Annotations[constants.DeprecationWarning]
	return exists
}

// createBasicInferenceService creates a basic InferenceService for testing
func createBasicInferenceService(name, namespace string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.InferenceServiceSpec{},
	}
}

// createInferenceServiceWithPredictor creates an InferenceService with a Predictor
func createInferenceServiceWithPredictor(name, namespace, baseModel string) *v1beta1.InferenceService {
	isvc := createBasicInferenceService(name, namespace)
	isvc.Spec.Predictor = v1beta1.PredictorSpec{
		Model: &v1beta1.ModelSpec{
			BaseModel: stringPtr(baseModel),
		},
	}
	return isvc
}

// =============================================================================
// Main DefaultInferenceService Tests
// =============================================================================

func TestDefaultInferenceService(t *testing.T) {
	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		deployConfig    *controllerconfig.DeployConfig
		wantAnnotations map[string]string
		wantEngine      bool
		wantModel       bool
		wantRuntime     bool
	}{
		{
			name:         "no deployment mode annotation, deployConfig with RawDeployment",
			isvc:         createInferenceServiceWithPredictor("test-isvc", "default", "test-model"),
			deployConfig: &controllerconfig.DeployConfig{DefaultDeploymentMode: string(constants.RawDeployment)},
			wantAnnotations: map[string]string{
				constants.DeploymentMode:     string(constants.RawDeployment),
				constants.DeprecationWarning: DeprecationWarningPredictor,
			},
			wantEngine: true,
			wantModel:  true,
		},
		{
			name: "existing deployment mode annotation should not be overridden",
			isvc: func() *v1beta1.InferenceService {
				isvc := createInferenceServiceWithPredictor("test-isvc", "default", "test-model")
				isvc.ObjectMeta.Annotations = map[string]string{
					constants.DeploymentMode: "serverless",
				}
				return isvc
			}(),
			deployConfig: &controllerconfig.DeployConfig{DefaultDeploymentMode: string(constants.RawDeployment)},
			wantAnnotations: map[string]string{
				constants.DeploymentMode:     "serverless",
				constants.DeprecationWarning: DeprecationWarningPredictor,
			},
			wantEngine: true,
			wantModel:  true,
		},
		{
			name:            "nil deployConfig should not set deployment mode",
			isvc:            createInferenceServiceWithPredictor("test-isvc", "default", "test-model"),
			deployConfig:    nil,
			wantAnnotations: map[string]string{constants.DeprecationWarning: DeprecationWarningPredictor},
			wantEngine:      true,
			wantModel:       true,
		},
		{
			name:         "deployConfig with non-RawDeployment default should not set mode",
			isvc:         createInferenceServiceWithPredictor("test-isvc", "default", "test-model"),
			deployConfig: &controllerconfig.DeployConfig{DefaultDeploymentMode: "serverless"},
			wantAnnotations: map[string]string{
				constants.DeprecationWarning: DeprecationWarningPredictor,
			},
			wantEngine: true,
			wantModel:  true,
		},
		{
			name:            "empty InferenceService should have no annotations",
			isvc:            createBasicInferenceService("test-isvc", "default"),
			deployConfig:    nil,
			wantAnnotations: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment variable
			originalValue := os.Getenv(EnablePredictorMigrationEnvVar)
			defer func(key, value string) {
				err := os.Setenv(key, value)
				if err != nil {
					t.Errorf("Failed to set environment variable %s to %s: %v", key, value, err)
				}
			}(EnablePredictorMigrationEnvVar, originalValue)
			_ = os.Setenv(EnablePredictorMigrationEnvVar, "true") // Enable migration for these tests

			// Create fake client with test model if needed
			var c client.Client
			if tt.wantModel && tt.isvc.Spec.Predictor.Model != nil && tt.isvc.Spec.Predictor.Model.BaseModel != nil {
				modelName := *tt.isvc.Spec.Predictor.Model.BaseModel
				clusterBaseModel := &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: modelName,
					},
				}
				c = createFakeClient(t, clusterBaseModel)
			} else {
				c = createFakeClient(t)
			}

			ctx := context.Background()
			err := DefaultInferenceService(ctx, c, tt.isvc, tt.deployConfig)
			require.NoError(t, err)

			// Check annotations
			if tt.wantAnnotations == nil {
				if tt.isvc.Annotations != nil {
					assert.Empty(t, tt.isvc.Annotations, "Expected no annotations")
				}
			} else {
				require.NotNil(t, tt.isvc.Annotations, "Expected annotations to exist")
				for key, expectedVal := range tt.wantAnnotations {
					actualVal, exists := tt.isvc.Annotations[key]
					assert.True(t, exists, "Expected annotation key %s to exist", key)
					assert.Equal(t, expectedVal, actualVal, "Expected annotation value to match for key %s", key)
				}
			}

			// Check migration results
			assert.Equal(t, tt.wantEngine, tt.isvc.Spec.Engine != nil, "Engine presence mismatch")
			assert.Equal(t, tt.wantModel, tt.isvc.Spec.Model != nil, "Model presence mismatch")
			assert.Equal(t, tt.wantRuntime, tt.isvc.Spec.Runtime != nil, "Runtime presence mismatch")
		})
	}
}

// =============================================================================
// Deployment Mode Detection Tests
// =============================================================================

func TestDeploymentModeDetection(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		deployConfig *controllerconfig.DeployConfig
		expectedMode string
	}{
		{
			name: "engine and decoder should set PDDisaggregated",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine:  &v1beta1.EngineSpec{},
					Decoder: &v1beta1.DecoderSpec{},
				},
			},
			deployConfig: nil,
			expectedMode: string(constants.PDDisaggregated),
		},
		{
			name: "engine with leader and worker should set MultiNode",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Leader: &v1beta1.LeaderSpec{},
						Worker: &v1beta1.WorkerSpec{Size: intPtr(2)},
					},
				},
			},
			deployConfig: nil,
			expectedMode: string(constants.MultiNode),
		},
		{
			name: "engine without leader/worker should default to RawDeployment",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			deployConfig: &controllerconfig.DeployConfig{DefaultDeploymentMode: string(constants.RawDeployment)},
			expectedMode: string(constants.RawDeployment),
		},
		{
			name: "engine with worker size zero should default to RawDeployment",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Leader: &v1beta1.LeaderSpec{},
						Worker: &v1beta1.WorkerSpec{Size: intPtr(0)},
					},
				},
			},
			deployConfig: &controllerconfig.DeployConfig{DefaultDeploymentMode: string(constants.RawDeployment)},
			expectedMode: string(constants.RawDeployment),
		},
		{
			name: "existing deployment mode should not be overridden",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.DeploymentMode: string(constants.Serverless),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine:  &v1beta1.EngineSpec{},
					Decoder: &v1beta1.DecoderSpec{},
				},
			},
			deployConfig: nil,
			expectedMode: string(constants.Serverless),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := createFakeClient(t)
			ctx := context.Background()
			err := DefaultInferenceService(ctx, c, tt.isvc, tt.deployConfig)
			require.NoError(t, err)

			require.NotNil(t, tt.isvc.ObjectMeta.Annotations, "Annotations should exist")
			mode, exists := tt.isvc.ObjectMeta.Annotations[constants.DeploymentMode]
			assert.True(t, exists, "Deployment mode annotation should exist")
			assert.Equal(t, tt.expectedMode, mode, "Expected deployment mode should match")
		})
	}
}

// =============================================================================
// Predictor Usage Detection Tests
// =============================================================================

func TestIsPredictorUsed(t *testing.T) {
	tests := []struct {
		name string
		isvc *v1beta1.InferenceService
		want bool
	}{
		{
			name: "predictor with model",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with minReplicas",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: intPtr(1)},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with containers",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{
							Containers: []v1.Container{{Name: "test-container", Image: "test-image"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with service account",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{ServiceAccountName: "test-sa"},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with volumes",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{
							Volumes: []v1.Volume{{Name: "test-volume"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with node selector",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{
							NodeSelector: map[string]string{"key": "value"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with tolerations",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{
							Tolerations: []v1.Toleration{{Key: "test"}},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "predictor with affinity",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{
							Affinity: &v1.Affinity{},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "empty predictor",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPredictorUsed(tt.isvc)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Predictor Migration Tests
// =============================================================================

func TestMigrateFromPredictorToNewArchitecture(t *testing.T) {
	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		setupClient     func(t *testing.T, isvc *v1beta1.InferenceService) client.Client
		wantEngine      bool
		wantModel       bool
		wantModelKind   *string // Expected model kind (ClusterBaseModel or BaseModel)
		wantRuntime     bool
		wantMinReplicas *int
		wantMaxReplicas int
		wantRunner      bool
		wantContainers  int
		expectError     bool
		errorMsg        string
	}{
		{
			name: "basic predictor with model - ClusterBaseModel",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: intPtr(2),
							MaxReplicas: 5,
						},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				modelName := *isvc.Spec.Predictor.Model.BaseModel
				clusterBaseModel := &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: modelName,
					},
				}
				return createFakeClient(t, clusterBaseModel)
			},
			wantEngine:      true,
			wantModel:       true,
			wantModelKind:   stringPtr("ClusterBaseModel"),
			wantRuntime:     false,
			wantMinReplicas: intPtr(2),
			wantMaxReplicas: 5,
		},
		{
			name: "basic predictor with model - BaseModel (namespace-scoped)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-basemodel")},
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: intPtr(1),
							MaxReplicas: 3,
						},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				modelName := *isvc.Spec.Predictor.Model.BaseModel
				baseModel := &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      modelName,
						Namespace: isvc.Namespace,
					},
				}
				return createFakeClient(t, baseModel)
			},
			wantEngine:      true,
			wantModel:       true,
			wantModelKind:   stringPtr("BaseModel"),
			wantRuntime:     false,
			wantMinReplicas: intPtr(1),
			wantMaxReplicas: 3,
		},
		{
			name: "predictor with model not found - should return error",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("non-existent-model")},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				// No model objects in the client
				return createFakeClient(t)
			},
			wantEngine:  false,
			wantModel:   false,
			expectError: true,
			errorMsg:    "neither ClusterBaseModel nor BaseModel found with name non-existent-model",
		},
		{
			name: "predictor with model and runtime",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
							Runtime:   stringPtr("test-runtime"),
						},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				modelName := *isvc.Spec.Predictor.Model.BaseModel
				clusterBaseModel := &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: modelName,
					},
				}
				return createFakeClient(t, clusterBaseModel)
			},
			wantEngine:    true,
			wantModel:     true,
			wantModelKind: stringPtr("ClusterBaseModel"),
			wantRuntime:   true,
		},
		{
			name: "predictor with no model but other fields",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{ServiceAccountName: "test-sa"},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				return createFakeClient(t)
			},
			wantEngine:  true,
			wantModel:   false,
			wantRuntime: false,
		},
		{
			name: "engine already configured - should skip migration",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
					},
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: intPtr(3)},
					},
				},
			},
			setupClient: func(t *testing.T, isvc *v1beta1.InferenceService) client.Client {
				return createFakeClient(t)
			},
			wantEngine:      true,
			wantModel:       false,
			wantRuntime:     false,
			wantMinReplicas: intPtr(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deep copy to avoid modifying the original
			isvc := tt.isvc.DeepCopy()

			// Create fake client using setupClient function if provided, otherwise use default logic
			var c client.Client
			if tt.setupClient != nil {
				c = tt.setupClient(t, isvc)
			} else {
				// Default: Create fake client with test model if needed
				if tt.wantModel && isvc.Spec.Predictor.Model != nil && isvc.Spec.Predictor.Model.BaseModel != nil {
					modelName := *isvc.Spec.Predictor.Model.BaseModel
					clusterBaseModel := &v1beta1.ClusterBaseModel{
						ObjectMeta: metav1.ObjectMeta{
							Name: modelName,
						},
					}
					c = createFakeClient(t, clusterBaseModel)
				} else {
					c = createFakeClient(t)
				}
			}

			ctx := context.Background()
			// Migrate the predictor
			err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)

			// Check for expected errors
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}
			require.NoError(t, err)

			// Verify engine presence
			assert.Equal(t, tt.wantEngine, isvc.Spec.Engine != nil)

			// Verify model presence
			assert.Equal(t, tt.wantModel, isvc.Spec.Model != nil)

			// Verify model kind if specified
			if tt.wantModel && tt.wantModelKind != nil {
				require.NotNil(t, isvc.Spec.Model)
				require.NotNil(t, isvc.Spec.Model.Kind)
				assert.Equal(t, *tt.wantModelKind, *isvc.Spec.Model.Kind, "Model kind should match")
			}

			// Verify runtime presence
			assert.Equal(t, tt.wantRuntime, isvc.Spec.Runtime != nil)

			// Check engine configuration if it was created
			if tt.wantEngine && isvc.Spec.Engine != nil {
				if tt.wantMinReplicas != nil {
					require.NotNil(t, isvc.Spec.Engine.MinReplicas)
					assert.Equal(t, *tt.wantMinReplicas, *isvc.Spec.Engine.MinReplicas)
				}
				if tt.wantMaxReplicas != 0 {
					assert.Equal(t, tt.wantMaxReplicas, isvc.Spec.Engine.MaxReplicas)
				}
			}
		})
	}
}

func TestMigrateFromPredictorToNewArchitectureContainerHandling(t *testing.T) {
	tests := []struct {
		name               string
		containers         []v1.Container
		wantRunnerName     string
		wantContainerCount int
	}{
		{
			name: "ome-container should become runner",
			containers: []v1.Container{
				{Name: "ome-container", Image: "test-image"},
				{Name: "sidecar", Image: "sidecar-image"},
			},
			wantRunnerName:     "ome-container",
			wantContainerCount: 1,
		},
		{
			name: "container with ome in name should become runner",
			containers: []v1.Container{
				{Name: "some-ome-runner", Image: "some-image:latest"},
				{Name: "sidecar", Image: "sidecar:latest"},
			},
			wantRunnerName:     "some-ome-runner",
			wantContainerCount: 1,
		},
		{
			name: "single regular container should become runner",
			containers: []v1.Container{
				{Name: "regular-container", Image: "some-image:latest"},
			},
			wantRunnerName:     "regular-container",
			wantContainerCount: 0,
		},
		{
			name: "multiple regular containers - first becomes runner",
			containers: []v1.Container{
				{Name: "first-container", Image: "first-image:latest"},
				{Name: "second-container", Image: "second-image:latest"},
				{Name: "third-container", Image: "third-image:latest"},
			},
			wantRunnerName:     "first-container",
			wantContainerCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						PodSpec: v1beta1.PodSpec{Containers: tt.containers},
					},
				},
			}

			c := createFakeClient(t)
			ctx := context.Background()
			err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
			require.NoError(t, err)

			require.NotNil(t, isvc.Spec.Engine, "Engine should be created")
			require.NotNil(t, isvc.Spec.Engine.Runner, "Runner should be created")
			assert.Equal(t, tt.wantRunnerName, isvc.Spec.Engine.Runner.Name)

			if tt.wantContainerCount == 0 {
				assert.Nil(t, isvc.Spec.Engine.Containers)
			} else {
				require.NotNil(t, isvc.Spec.Engine.Containers)
				assert.Len(t, isvc.Spec.Engine.Containers, tt.wantContainerCount)
			}
		})
	}
}

func TestMigrateFromPredictorToNewArchitectureWithWorker(t *testing.T) {
	workerSize := 3
	isvc := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{{Name: "main-container", Image: "some-image:latest"}},
				},
				Worker: &v1beta1.WorkerSpec{Size: &workerSize},
			},
		},
	}

	c := createFakeClient(t)
	ctx := context.Background()
	err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
	require.NoError(t, err)

	require.NotNil(t, isvc.Spec.Engine, "Engine should be created")
	require.NotNil(t, isvc.Spec.Engine.Worker, "Worker should be migrated")
	assert.Equal(t, workerSize, *isvc.Spec.Engine.Worker.Size)
}

func TestMigrateFromPredictorToNewArchitectureEdgeCases(t *testing.T) {
	t.Run("predictor with model but no BaseModel", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{
						// BaseModel is nil, but Model exists
						Runtime: stringPtr("test-runtime"),
					},
				},
			},
		}

		c := createFakeClient(t)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.Nil(t, isvc.Spec.Model, "Model should not be created when BaseModel is nil")
		assert.NotNil(t, isvc.Spec.Runtime, "Runtime should be created")
	})

	t.Run("predictor with existing top-level model", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{BaseModel: stringPtr("predictor-model")},
				},
				Model: &v1beta1.ModelRef{Name: "existing-model"}, // Already exists
			},
		}

		c := createFakeClient(t)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.Equal(t, "existing-model", isvc.Spec.Model.Name, "Existing model should be preserved")
	})

	t.Run("predictor with existing top-level runtime", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{
						BaseModel: stringPtr("test-model"),
						Runtime:   stringPtr("predictor-runtime"),
					},
				},
				Runtime: &v1beta1.ServingRuntimeRef{Name: "existing-runtime"}, // Already exists
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.Equal(t, "existing-runtime", isvc.Spec.Runtime.Name, "Existing runtime should be preserved")
	})

	t.Run("predictor with model but no runtime", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{
						BaseModel: stringPtr("test-model"),
						// Runtime is nil
					},
				},
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.NotNil(t, isvc.Spec.Model, "Model should be created")
		assert.Nil(t, isvc.Spec.Runtime, "Runtime should not be created when not specified")
	})

	t.Run("predictor with no containers", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					PodSpec: v1beta1.PodSpec{
						ServiceAccountName: "test-sa",
						// No containers
					},
				},
			},
		}

		c := createFakeClient(t)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.Nil(t, isvc.Spec.Engine.Runner, "Runner should not be created when no containers")
		assert.Nil(t, isvc.Spec.Engine.Containers, "Containers should be nil")
	})

	t.Run("predictor with fine-tuned weights", func(t *testing.T) {
		weights := []string{"weight1", "weight2", "weight3"}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{
						BaseModel:        stringPtr("test-model"),
						FineTunedWeights: weights,
					},
				},
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.NotNil(t, isvc.Spec.Model, "Model should be created")
		assert.Equal(t, weights, isvc.Spec.Model.FineTunedWeights, "Fine-tuned weights should be migrated")
	})

	t.Run("predictor with empty fine-tuned weights", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{
						BaseModel:        stringPtr("test-model"),
						FineTunedWeights: []string{}, // Empty slice
					},
				},
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := migrateFromPredictorToNewArchitecture(ctx, c, isvc)
		require.NoError(t, err)

		assert.NotNil(t, isvc.Spec.Engine, "Engine should be created")
		assert.NotNil(t, isvc.Spec.Model, "Model should be created")
		assert.Empty(t, isvc.Spec.Model.FineTunedWeights, "Empty fine-tuned weights should not be copied")
	})
}

// =============================================================================
// Component Default Value Tests
// =============================================================================

func TestDefaultComponents(t *testing.T) {
	t.Run("defaultEngine", func(t *testing.T) {
		tests := []struct {
			name            string
			engine          *v1beta1.EngineSpec
			wantMinReplicas int
			wantMaxReplicas int
		}{
			{
				name:            "nil MinReplicas should be set to 1",
				engine:          &v1beta1.EngineSpec{},
				wantMinReplicas: 1,
				wantMaxReplicas: 3,
			},
			{
				name: "existing values should be preserved",
				engine: &v1beta1.EngineSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						MinReplicas: intPtr(2),
						MaxReplicas: 5,
					},
				},
				wantMinReplicas: 2,
				wantMaxReplicas: 5,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				defaultEngine(tt.engine)
				require.NotNil(t, tt.engine.MinReplicas)
				assert.Equal(t, tt.wantMinReplicas, *tt.engine.MinReplicas)
				assert.Equal(t, tt.wantMaxReplicas, tt.engine.MaxReplicas)
			})
		}
	})

	t.Run("defaultDecoder", func(t *testing.T) {
		tests := []struct {
			name            string
			decoder         *v1beta1.DecoderSpec
			wantMinReplicas int
			wantMaxReplicas int
		}{
			{
				name:            "nil MinReplicas should be set to 1",
				decoder:         &v1beta1.DecoderSpec{},
				wantMinReplicas: 1,
				wantMaxReplicas: 3,
			},
			{
				name: "existing values should be preserved",
				decoder: &v1beta1.DecoderSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						MinReplicas: intPtr(2),
						MaxReplicas: 5,
					},
				},
				wantMinReplicas: 2,
				wantMaxReplicas: 5,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				defaultDecoder(tt.decoder)
				require.NotNil(t, tt.decoder.MinReplicas)
				assert.Equal(t, tt.wantMinReplicas, *tt.decoder.MinReplicas)
				assert.Equal(t, tt.wantMaxReplicas, tt.decoder.MaxReplicas)
			})
		}
	})

	t.Run("defaultRouter", func(t *testing.T) {
		tests := []struct {
			name            string
			router          *v1beta1.RouterSpec
			wantMinReplicas int
			wantMaxReplicas int
		}{
			{
				name:            "nil MinReplicas should be set to 1",
				router:          &v1beta1.RouterSpec{},
				wantMinReplicas: 1,
				wantMaxReplicas: 2,
			},
			{
				name: "existing values should be preserved",
				router: &v1beta1.RouterSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						MinReplicas: intPtr(2),
						MaxReplicas: 5,
					},
				},
				wantMinReplicas: 2,
				wantMaxReplicas: 5,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				defaultRouter(tt.router)
				require.NotNil(t, tt.router.MinReplicas)
				assert.Equal(t, tt.wantMinReplicas, *tt.router.MinReplicas)
				assert.Equal(t, tt.wantMaxReplicas, tt.router.MaxReplicas)
			})
		}
	})
}

// =============================================================================
// Deprecation Warning Tests
// =============================================================================

func TestDeprecationWarning(t *testing.T) {
	tests := []struct {
		name        string
		isvc        *v1beta1.InferenceService
		wantWarning bool
	}{
		{
			name: "predictor used should have warning",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
					},
				},
			},
			wantWarning: true,
		},
		{
			name: "engine used should not have warning",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: intPtr(1)},
					},
					Model: &v1beta1.ModelRef{Name: "test-model"},
				},
			},
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment variable
			originalValue := os.Getenv(EnablePredictorMigrationEnvVar)
			defer func(key, value string) {
				_ = os.Setenv(key, value)
			}(EnablePredictorMigrationEnvVar, originalValue)
			_ = os.Setenv(EnablePredictorMigrationEnvVar, "true") // Enable migration for these tests

			// Create fake client with test model if predictor has a model
			var c client.Client
			if tt.isvc.Spec.Predictor.Model != nil && tt.isvc.Spec.Predictor.Model.BaseModel != nil {
				modelName := *tt.isvc.Spec.Predictor.Model.BaseModel
				clusterBaseModel := &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: modelName,
					},
				}
				c = createFakeClient(t, clusterBaseModel)
			} else {
				c = createFakeClient(t)
			}

			ctx := context.Background()
			err := DefaultInferenceService(ctx, c, tt.isvc, nil)
			require.NoError(t, err)

			hasWarning := hasDeprecationWarning(tt.isvc)
			assert.Equal(t, tt.wantWarning, hasWarning)

			if tt.wantWarning {
				val, exists := tt.isvc.ObjectMeta.Annotations[constants.DeprecationWarning]
				assert.True(t, exists, "Deprecation warning annotation should exist")
				assert.Equal(t, DeprecationWarningPredictor, val)
			}
		})
	}
}

func TestDeprecationWarningEdgeCases(t *testing.T) {
	t.Run("existing deprecation warning should not be overwritten", func(t *testing.T) {
		customWarning := "Custom deprecation warning"
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.DeprecationWarning: customWarning,
				},
			},
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
				},
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := DefaultInferenceService(ctx, c, isvc, nil)
		require.NoError(t, err)

		val, exists := isvc.ObjectMeta.Annotations[constants.DeprecationWarning]
		assert.True(t, exists, "Deprecation warning annotation should exist")
		assert.Equal(t, customWarning, val, "Existing warning should be preserved")
	})

	t.Run("predictor with nil annotations should create annotations map", func(t *testing.T) {
		// Save and restore environment variable
		originalValue := os.Getenv(EnablePredictorMigrationEnvVar)
		defer func(key, value string) {
			_ = os.Setenv(key, value)
		}(EnablePredictorMigrationEnvVar, originalValue)
		_ = os.Setenv(EnablePredictorMigrationEnvVar, "true") // Enable migration for these tests

		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: nil, // Explicitly nil
			},
			Spec: v1beta1.InferenceServiceSpec{
				Predictor: v1beta1.PredictorSpec{
					Model: &v1beta1.ModelSpec{BaseModel: stringPtr("test-model")},
				},
			},
		}

		// Create fake client with test model
		clusterBaseModel := &v1beta1.ClusterBaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-model",
			},
		}
		c := createFakeClient(t, clusterBaseModel)
		ctx := context.Background()
		err := DefaultInferenceService(ctx, c, isvc, nil)
		require.NoError(t, err)

		assert.NotNil(t, isvc.ObjectMeta.Annotations, "Annotations map should be created")
		val, exists := isvc.ObjectMeta.Annotations[constants.DeprecationWarning]
		assert.True(t, exists, "Deprecation warning annotation should exist")
		assert.Equal(t, DeprecationWarningPredictor, val)
	})
}

// =============================================================================
// Environment Variable Control Tests
// =============================================================================

func TestPredictorMigrationWithEnvironmentVariable(t *testing.T) {
	// Save original env var value to restore after test
	originalValue := os.Getenv(EnablePredictorMigrationEnvVar)
	defer func(key, value string) {
		err := os.Setenv(key, value)
		if err != nil {
			t.Errorf("Failed to restore environment variable %s: %v", key, err)
		}
	}(EnablePredictorMigrationEnvVar, originalValue)

	tests := []struct {
		name            string
		envValue        string
		expectMigration bool
	}{
		{
			name:            "env var not set - migration should happen",
			envValue:        "",
			expectMigration: true,
		},
		{
			name:            "env var set to 'true' - migration should happen",
			envValue:        "true",
			expectMigration: true,
		},
		{
			name:            "env var set to 'yes' - migration should happen",
			envValue:        "yes",
			expectMigration: true,
		},
		{
			name:            "env var set to 'false' - migration should not happen",
			envValue:        "false",
			expectMigration: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test case
			if err := os.Setenv(EnablePredictorMigrationEnvVar, tt.envValue); err != nil {
				return
			}

			// Create a test InferenceService with Predictor
			isvc := createInferenceServiceWithPredictor("test-isvc", "default", "test-model")

			// Create fake client with test model if migration is expected
			var c client.Client
			if tt.expectMigration {
				clusterBaseModel := &v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
					},
				}
				c = createFakeClient(t, clusterBaseModel)
			} else {
				c = createFakeClient(t)
			}

			ctx := context.Background()
			// Apply defaulting
			err := DefaultInferenceService(ctx, c, isvc, nil)
			if tt.expectMigration {
				require.NoError(t, err)
			} else {
				// When migration is disabled, we don't need the model to exist
				require.NoError(t, err)
			}

			// Check if migration happened (by checking if Engine was populated)
			if tt.expectMigration {
				assert.NotNil(t, isvc.Spec.Engine, "Expected Engine to be populated when migration is enabled")
				assert.NotNil(t, isvc.Spec.Model, "Expected Model to be populated when migration is enabled")
				assert.Equal(t, "test-model", isvc.Spec.Model.Name, "Expected Model name to match Predictor model")
			} else {
				assert.Nil(t, isvc.Spec.Engine, "Expected Engine to be nil when migration is disabled")
				assert.Nil(t, isvc.Spec.Model, "Expected Model to be nil when migration is disabled")
			}

			// Verify the deprecation warning is still added regardless of migration setting
			assert.True(t, hasDeprecationWarning(isvc), "Deprecation warning should be added even when migration is disabled")
		})
	}
}

func TestShouldEnableMigration(t *testing.T) {
	// Save original env var value to restore after test
	originalValue := os.Getenv(EnablePredictorMigrationEnvVar)
	defer func(key, value string) {
		err := os.Setenv(key, value)
		if err != nil {
			return
		}
	}(EnablePredictorMigrationEnvVar, originalValue)

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "env var not set",
			envValue: "",
			expected: true,
		},
		{
			name:     "env var set to 'true'",
			envValue: "true",
			expected: true,
		},
		{
			name:     "env var set to 'false'",
			envValue: "false",
			expected: false,
		},
		{
			name:     "env var set to some other value",
			envValue: "somevalue",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test case
			err := os.Setenv(EnablePredictorMigrationEnvVar, tt.envValue)
			if err != nil {
				return
			}

			// Test the function
			result := shouldEnableMigration()
			assert.Equal(t, tt.expected, result, "shouldEnableMigration returned unexpected result")
		})
	}
}

// =============================================================================
// Utility Function Tests
// =============================================================================

func TestMigrateSpecViaJSON(t *testing.T) {
	t.Run("successful migration", func(t *testing.T) {
		// Create a struct with ComponentExtensionSpec embedded
		source := struct {
			v1beta1.ComponentExtensionSpec
			ServiceAccountName *string
		}{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				MinReplicas:          intPtr(1),
				MaxReplicas:          3,
				ContainerConcurrency: int64Ptr(2),
			},
			ServiceAccountName: stringPtr("test-sa"),
		}

		target := &v1beta1.EngineSpec{}
		err := migrateSpecViaJSON(&source, target)

		assert.NoError(t, err)
		assert.Equal(t, 1, *target.MinReplicas)
		assert.Equal(t, int32(3), int32(target.MaxReplicas))
		assert.Equal(t, int64(2), *target.ContainerConcurrency)
		assert.Equal(t, "test-sa", target.ServiceAccountName)
	})

	t.Run("error during marshal", func(t *testing.T) {
		// Creating a circular reference to cause marshal error
		type CircularStruct struct {
			Self *CircularStruct
		}
		circular := CircularStruct{}
		circular.Self = &circular

		target := &v1beta1.EngineSpec{}
		err := migrateSpecViaJSON(&circular, target)

		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "json") ||
			strings.Contains(err.Error(), "circular") ||
			strings.Contains(err.Error(), "marshal"))
	})

	t.Run("error during unmarshal", func(t *testing.T) {
		// Create a source that will marshal fine but unmarshal poorly
		source := map[string]interface{}{
			"minReplicas": "not-a-number", // This will cause unmarshal error
		}

		target := &v1beta1.EngineSpec{}
		err := migrateSpecViaJSON(source, target)

		assert.Error(t, err)
	})
}

// =============================================================================
// Webhook Integration Tests
// =============================================================================

func TestDefault(t *testing.T) {
	t.Run("conversion error", func(t *testing.T) {
		// Create an object that cannot be converted to InferenceService
		invalidObj := &v1.Pod{}
		defaulter := &InferenceServiceDefaulter{
			Client: createFakeClient(t),
		}

		err := defaulter.Default(context.Background(), invalidObj)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected an InferenceService object but got")
	})
}
