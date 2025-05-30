package components

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/status"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewPredictorV2(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		deploymentMode constants.DeploymentModeType
		verify         func(t *testing.T, p *PredictorV2)
	}{
		{
			name:           "Create predictor with raw deployment mode",
			deploymentMode: constants.RawDeployment,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.NotNil(t, p)
				assert.Equal(t, constants.RawDeployment, p.DeploymentMode)
				assert.NotNil(t, p.deploymentReconciler)
				assert.NotNil(t, p.podSpecReconciler)
				assert.NotNil(t, p.StatusManager)
				assert.NotNil(t, p.Log)
			},
		},
		{
			name:           "Create predictor with serverless deployment mode",
			deploymentMode: constants.Serverless,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.NotNil(t, p)
				assert.Equal(t, constants.Serverless, p.DeploymentMode)
			},
		},
		{
			name:           "Create predictor with multi-node deployment mode",
			deploymentMode: constants.MultiNode,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.NotNil(t, p)
				assert.Equal(t, constants.MultiNode, p.DeploymentMode)
			},
		},
		{
			name:           "Create predictor with multi-node ray vllm deployment mode",
			deploymentMode: constants.MultiNodeRayVLLM,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.NotNil(t, p)
				assert.Equal(t, constants.MultiNodeRayVLLM, p.DeploymentMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			clientset := clientsetfake.NewSimpleClientset()
			config := &controllerconfig.InferenceServicesConfig{}

			predictor := NewPredictorV2(client, clientset, scheme, config, tt.deploymentMode)
			require.NotNil(t, predictor)

			p, ok := predictor.(*PredictorV2)
			require.True(t, ok)

			tt.verify(t, p)
		})
	}
}

func TestPredictorV2_ReconcileBaseModel(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		isvc          *v1beta1.InferenceService
		baseModel     *v1beta1.BaseModel
		expectedError bool
		errorMessage  string
		verify        func(t *testing.T, p *PredictorV2)
	}{
		{
			name: "No base model specified",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{},
					},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.Nil(t, p.BaseModel)
				assert.Nil(t, p.BaseModelMeta)
			},
		},
		{
			name: "Base model found and enabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "pytorch",
					},
					Storage: &v1beta1.StorageSpec{
						Path: stringPtr("/models/test"),
					},
					ModelExtensionSpec: v1beta1.ModelExtensionSpec{
						Disabled: boolPtr(false),
					},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.NotNil(t, p.BaseModel)
				assert.NotNil(t, p.BaseModelMeta)
				assert.Equal(t, "pytorch", p.BaseModel.ModelFormat.Name)
			},
		},
		{
			name: "Base model is disabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("disabled-model"),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "disabled-model",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "pytorch",
					},
					Storage: &v1beta1.StorageSpec{
						Path: stringPtr("/models/test"),
					},
					ModelExtensionSpec: v1beta1.ModelExtensionSpec{
						Disabled: boolPtr(true),
					},
				},
			},
			expectedError: true,
			errorMessage:  "specified base model disabled-model is disabled",
		},
		{
			name: "Base model not found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("non-existent-model"),
						},
					},
				},
			},
			expectedError: true,
			errorMessage:  "No BaseModel or ClusterBaseModel with the name: non-existent-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.baseModel != nil {
				builder = builder.WithObjects(tt.baseModel)
			}

			client := builder.Build()
			p := &PredictorV2{
				BaseComponentFields: BaseComponentFields{
					Client:        client,
					StatusManager: status.NewStatusReconciler(),
					Log:           ctrl.Log.WithName("test"),
				},
			}

			err := p.reconcileBaseModel(tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, p)
			}
		})
	}
}

func TestPredictorV2_ReconcileFineTunedWeights(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name             string
		isvc             *v1beta1.InferenceService
		fineTunedWeights []*v1beta1.FineTunedWeight
		expectedError    bool
		errorMessage     string
		verify           func(t *testing.T, p *PredictorV2)
	}{
		{
			name: "No fine-tuned weights",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							FineTunedWeights: []string{},
						},
					},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.False(t, p.FineTunedServing)
				assert.Empty(t, p.FineTunedWeights)
			},
		},
		{
			name: "Single fine-tuned weight with merged weights",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							FineTunedWeights: []string{"ft-weight-1"},
						},
					},
				},
			},
			fineTunedWeights: []*v1beta1.FineTunedWeight{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ft-weight-1"},
					Spec: v1beta1.FineTunedWeightSpec{
						Configuration: runtime.RawExtension{
							Raw: []byte(`{"merged_weights": true}`),
						},
						HyperParameters: runtime.RawExtension{
							Raw: []byte(`{"strategy": "lora"}`),
						},
					},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, p *PredictorV2) {
				assert.True(t, p.FineTunedServing)
				assert.True(t, p.FineTunedServingWithMergedWeights)
				assert.Len(t, p.FineTunedWeights, 1)
			},
		},
		{
			name: "Multiple fine-tuned weights (not supported)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							FineTunedWeights: []string{"ft-weight-1", "ft-weight-2"},
						},
					},
				},
			},
			expectedError: true,
			errorMessage:  "stacked fine-tuned serving is not supported yet",
		},
		{
			name: "Fine-tuned weight not found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							FineTunedWeights: []string{"non-existent-weight"},
						},
					},
				},
			},
			expectedError: true,
			errorMessage:  "No FineTunedWeight with the name: non-existent-weight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			for _, ftw := range tt.fineTunedWeights {
				builder = builder.WithObjects(ftw)
			}

			client := builder.Build()
			p := &PredictorV2{
				BaseComponentFields: BaseComponentFields{
					Client:        client,
					StatusManager: status.NewStatusReconciler(),
					Log:           ctrl.Log.WithName("test"),
				},
			}

			err := p.reconcileFineTunedWeights(tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, p)
			}
		})
	}
}

func TestPredictorV2_GetRuntime(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		isvc          *v1beta1.InferenceService
		baseModel     *v1beta1.BaseModelSpec
		runtimes      []runtime.Object
		expectedError bool
		errorMessage  string
		verify        func(t *testing.T, runtime v1beta1.ServingRuntimeSpec, runtimeName string)
	}{
		{
			name: "Specified runtime exists and enabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							Runtime: stringPtr("test-runtime"),
						},
					},
				},
			},
			runtimes: []runtime.Object{
				&v1beta1.ServingRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-runtime",
						Namespace: "default",
					},
					Spec: v1beta1.ServingRuntimeSpec{
						Disabled: boolPtr(false),
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								Name: "pytorch",
								ModelFormat: &v1beta1.ModelFormat{
									Name: "pytorch",
								},
								AutoSelect: boolPtr(true),
							},
						},
						ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
							Containers: []v1.Container{
								{
									Name:  constants.MainContainerName,
									Image: "test-image:latest",
								},
							},
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			expectedError: false,
			verify: func(t *testing.T, runtime v1beta1.ServingRuntimeSpec, runtimeName string) {
				assert.Equal(t, "test-runtime", runtimeName)
				assert.Len(t, runtime.Containers, 1)
			},
		},
		{
			name: "Specified runtime is disabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							Runtime: stringPtr("disabled-runtime"),
						},
					},
				},
			},
			runtimes: []runtime.Object{
				&v1beta1.ServingRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "disabled-runtime",
						Namespace: "default",
					},
					Spec: v1beta1.ServingRuntimeSpec{
						Disabled: boolPtr(true),
					},
				},
			},
			expectedError: true,
			errorMessage:  "specified runtime disabled-runtime is disabled",
		},
		{
			name: "Auto-select runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			runtimes: []runtime.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelFormat: v1beta1.ModelFormat{
							Name: "pytorch",
						},
						ModelParameterSize: stringPtr("7B"),
					},
				},
				&v1beta1.ServingRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "auto-runtime",
						Namespace: "default",
					},
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name: "pytorch",
								},
								AutoSelect: boolPtr(true),
							},
						},
						ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
							Containers: []v1.Container{
								{
									Name:  constants.MainContainerName,
									Image: "auto-image:latest",
								},
							},
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			expectedError: false,
			verify: func(t *testing.T, runtime v1beta1.ServingRuntimeSpec, runtimeName string) {
				assert.Equal(t, "auto-runtime", runtimeName)
			},
		},
		{
			name: "No supporting runtime found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			runtimes: []runtime.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelFormat: v1beta1.ModelFormat{
							Name: "pytorch",
						},
						ModelParameterSize: stringPtr("7B"),
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: stringPtr("7B"),
			},
			expectedError: true,
			errorMessage:  "no runtime found to support specified predictor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.runtimes...).Build()
			p := &PredictorV2{
				BaseComponentFields: BaseComponentFields{
					Client:        client,
					BaseModel:     tt.baseModel,
					StatusManager: status.NewStatusReconciler(),
					Log:           ctrl.Log.WithName("test"),
				},
			}

			runtime, runtimeName, err := p.getRuntime(tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, runtime, runtimeName)
			}
		})
	}
}

func TestPredictorV2_GetWorkerSize(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		runtime      v1beta1.ServingRuntimeSpec
		expectedSize int
	}{
		{
			name: "Size from predictor worker",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Worker: &v1beta1.WorkerSpec{
							Size: intPtr(3),
						},
					},
				},
			},
			runtime:      v1beta1.ServingRuntimeSpec{},
			expectedSize: 3,
		},
		{
			name: "Size from runtime worker pod spec",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			runtime: v1beta1.ServingRuntimeSpec{
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPtr(2),
				},
			},
			expectedSize: 2,
		},
		{
			name: "Default size when not specified",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			runtime:      v1beta1.ServingRuntimeSpec{},
			expectedSize: 0,
		},
		{
			name: "Predictor worker takes precedence over runtime",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Worker: &v1beta1.WorkerSpec{
							Size: intPtr(4),
						},
					},
				},
			},
			runtime: v1beta1.ServingRuntimeSpec{
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPtr(2),
				},
			},
			expectedSize: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PredictorV2{}
			size := p.getWorkerSize(tt.isvc, tt.runtime)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

func TestPredictorV2_ProcessAnnotations(t *testing.T) {
	tests := []struct {
		name                  string
		isvc                  *v1beta1.InferenceService
		runtime               *v1beta1.ServingRuntimeSpec
		baseComponentFields   BaseComponentFields
		expectedAnnotations   map[string]string
		unexpectedAnnotations []string
	}{
		{
			name: "Basic annotations with runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						"custom-annotation": "value",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							Annotations: map[string]string{
								"predictor-annotation": "predictor-value",
							},
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Annotations: map[string]string{
						"runtime-annotation": "runtime-value",
					},
				},
			},
			baseComponentFields: BaseComponentFields{
				RuntimeName: "test-runtime",
				BaseModel: &v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name:    "pytorch",
						Version: stringPtr("1.0"),
					},
					ModelExtensionSpec: v1beta1.ModelExtensionSpec{
						Vendor: stringPtr("meta"),
					},
				},
				BaseModelMeta: &metav1.ObjectMeta{
					Name: "test-model",
				},
			},
			expectedAnnotations: map[string]string{
				"custom-annotation":                    "value",
				"runtime-annotation":                   "runtime-value",
				"predictor-annotation":                 "predictor-value",
				constants.BaseModelName:                "test-model",
				constants.BaseModelVendorAnnotationKey: "meta",
				constants.BaseModelFormat:              "pytorch",
				constants.BaseModelFormatVersion:       "1.0",
				constants.ServingRuntimeKeyName:        "test-runtime",
			},
			unexpectedAnnotations: []string{"proxy.istio.io/config"},
		},
		{
			name: "Fine-tuned serving annotations",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			baseComponentFields: BaseComponentFields{
				FineTunedServing: true,
				FineTunedWeights: []*v1beta1.FineTunedWeight{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "ft-weight-1"},
						Spec: v1beta1.FineTunedWeightSpec{
							HyperParameters: runtime.RawExtension{
								Raw: []byte(`{"strategy": "lora"}`),
							},
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: "ft-weight-1",
				constants.FineTunedWeightFTStrategyKey: "lora",
			},
		},
		{
			name: "Fine-tuned serving with merged weights",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Annotations: map[string]string{
						constants.ModelInitInjectionKey: "true",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			baseComponentFields: BaseComponentFields{
				FineTunedServing:                  true,
				FineTunedServingWithMergedWeights: true,
			},
			expectedAnnotations: map[string]string{
				constants.FTServingWithMergedWeightsAnnotationKey: "true",
			},
			unexpectedAnnotations: []string{constants.ModelInitInjectionKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PredictorV2{
				BaseComponentFields: tt.baseComponentFields,
			}
			p.Runtime = tt.runtime
			p.Log = ctrl.Log.WithName("test")

			annotations, err := p.processAnnotations(tt.isvc)
			require.NoError(t, err)

			// Check expected annotations
			for key, value := range tt.expectedAnnotations {
				assert.Equal(t, value, annotations[key], "Expected annotation %s", key)
			}

			// Check unexpected annotations
			for _, key := range tt.unexpectedAnnotations {
				assert.NotContains(t, annotations, key, "Unexpected annotation %s", key)
			}
		})
	}
}

func TestPredictorV2_ProcessLabels(t *testing.T) {
	tests := []struct {
		name                string
		isvc                *v1beta1.InferenceService
		runtime             *v1beta1.ServingRuntimeSpec
		baseComponentFields BaseComponentFields
		expectedLabels      map[string]string
	}{
		{
			name: "Basic labels with runtime",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
					Labels: map[string]string{
						"custom-label": "value",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							Labels: map[string]string{
								"predictor-label": "predictor-value",
							},
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Labels: map[string]string{
						"runtime-label": "runtime-value",
					},
				},
			},
			baseComponentFields: BaseComponentFields{
				RuntimeName: "test-runtime",
				BaseModel: &v1beta1.BaseModelSpec{
					ModelExtensionSpec: v1beta1.ModelExtensionSpec{
						Vendor: stringPtr("meta"),
					},
				},
				BaseModelMeta: &metav1.ObjectMeta{
					Name: "test-model",
					Annotations: map[string]string{
						constants.ModelCategoryAnnotation: "LARGE",
					},
				},
			},
			expectedLabels: map[string]string{
				"custom-label":                                  "value",
				"runtime-label":                                 "runtime-value",
				"predictor-label":                               "predictor-value",
				constants.InferenceServicePodLabelKey:           "test-isvc",
				constants.KServiceComponentLabel:                "predictor",
				constants.FTServingLabelKey:                     "false",
				constants.InferenceServiceBaseModelNameLabelKey: "test-model",
				constants.InferenceServiceBaseModelSizeLabelKey: "LARGE",
				constants.BaseModelTypeLabelKey:                 string(constants.ServingBaseModel),
				constants.BaseModelVendorLabelKey:               "meta",
				constants.ServingRuntimeLabelKey:                "test-runtime",
			},
		},
		{
			name: "Fine-tuned serving labels",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			baseComponentFields: BaseComponentFields{
				FineTunedServing:                  true,
				FineTunedServingWithMergedWeights: true,
				FineTunedWeights: []*v1beta1.FineTunedWeight{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "ft-weight-1"},
						Spec: v1beta1.FineTunedWeightSpec{
							HyperParameters: runtime.RawExtension{
								Raw: []byte(`{"strategy": "lora"}`),
							},
						},
					},
				},
			},
			expectedLabels: map[string]string{
				constants.InferenceServicePodLabelKey:        "test-isvc",
				constants.KServiceComponentLabel:             "predictor",
				constants.FTServingLabelKey:                  "true",
				constants.FTServingWithMergedWeightsLabelKey: "true",
				constants.FineTunedWeightFTStrategyLabelKey:  "lora",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PredictorV2{
				BaseComponentFields: tt.baseComponentFields,
			}
			p.Runtime = tt.runtime
			p.Log = ctrl.Log.WithName("test")

			labels := p.processLabels(tt.isvc)

			// Check expected labels
			for key, value := range tt.expectedLabels {
				assert.Equal(t, value, labels[key], "Expected label %s", key)
			}
		})
	}
}

func TestPredictorV2_DeterminePredictorName(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = knservingv1.AddToScheme(scheme)

	tests := []struct {
		name           string
		isvc           *v1beta1.InferenceService
		deploymentMode constants.DeploymentModeType
		existingObjs   []runtime.Object
		expectedName   string
	}{
		{
			name: "Raw deployment with existing default service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			deploymentMode: constants.RawDeployment,
			existingObjs: []runtime.Object{
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-predictor-default",
						Namespace: "default",
					},
				},
			},
			expectedName: "test-isvc-predictor-default",
		},
		{
			name: "Raw deployment without existing service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			deploymentMode: constants.RawDeployment,
			existingObjs:   []runtime.Object{},
			expectedName:   "test-isvc",
		},
		{
			name: "Serverless deployment with existing ksvc",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			deploymentMode: constants.Serverless,
			existingObjs: []runtime.Object{
				&knservingv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-predictor-default",
						Namespace: "default",
					},
				},
			},
			expectedName: "test-isvc-predictor-default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.existingObjs...).Build()
			p := &PredictorV2{
				BaseComponentFields: BaseComponentFields{
					Client:         client,
					DeploymentMode: tt.deploymentMode,
				},
			}

			name, err := p.determinePredictorName(tt.isvc)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}
