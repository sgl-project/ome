package isvc

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "k8s.io/api/core/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// =============================================================================
// VALIDATOR INTERFACE TESTS
// =============================================================================

func TestInferenceServiceValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid inference service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name format",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Test-ISVC", // Invalid name format
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateCreate(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

func TestInferenceServiceValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		oldIsvc *v1beta1.InferenceService
		newIsvc *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid update",
			oldIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			newIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("updated-model"),
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateUpdate(context.Background(), tt.oldIsvc, tt.newIsvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

func TestInferenceServiceValidator_ValidateDelete(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid inference service",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			warnings, err := v.ValidateDelete(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, warnings)
			}
		})
	}
}

// Test error paths in ValidateCreate, ValidateUpdate, ValidateDelete
func TestValidatorErrorPaths(t *testing.T) {
	validator := &InferenceServiceValidator{}

	t.Run("ValidateCreate with invalid object type", func(t *testing.T) {
		invalidObj := &v1.Pod{} // Wrong type
		warnings, err := validator.ValidateCreate(context.Background(), invalidObj)
		assert.Error(t, err)
		assert.Nil(t, warnings)
		assert.Contains(t, err.Error(), "expected an InferenceService object")
	})

	t.Run("ValidateUpdate with invalid object type", func(t *testing.T) {
		validIsvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		invalidObj := &v1.Pod{} // Wrong type
		warnings, err := validator.ValidateUpdate(context.Background(), validIsvc, invalidObj)
		assert.Error(t, err)
		assert.Nil(t, warnings)
		assert.Contains(t, err.Error(), "expected an InferenceService object")
	})

	t.Run("ValidateDelete with invalid object type", func(t *testing.T) {
		invalidObj := &v1.Pod{} // Wrong type
		warnings, err := validator.ValidateDelete(context.Background(), invalidObj)
		assert.Error(t, err)
		assert.Nil(t, warnings)
		assert.Contains(t, err.Error(), "expected an InferenceService object")
	})
}

// =============================================================================
// NAME VALIDATION TESTS
// =============================================================================

func TestInferenceService_NameValidation(t *testing.T) {
	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid name",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-name",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid name with uppercase",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Invalid-Name",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid name with special characters",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid@name",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInferenceServiceName(tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// AUTOSCALER VALIDATION TESTS
// =============================================================================

func TestInferenceService_AutoscalerValidation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "no autoscaler class",
			annotations: nil,
			wantErr:     false,
		},
		{
			name:        "missing annotations map entirely",
			annotations: nil,
			wantErr:     false,
		},
		{
			name: "valid HPA autoscaler class",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassHPA),
			},
			wantErr: false,
		},
		{
			name: "valid external autoscaler class",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassExternal),
			},
			wantErr: false,
		},
		{
			name: "invalid autoscaler class",
			annotations: map[string]string{
				constants.AutoscalerClass: "invalid-class",
			},
			wantErr: true,
			errMsg:  "is not a supported autoscaler class type",
		},
		{
			name: "HPA autoscaler with valid CPU metric",
			annotations: map[string]string{
				constants.AutoscalerClass:   string(constants.AutoscalerClassHPA),
				constants.AutoscalerMetrics: string(constants.AutoScalerMetricsCPU),
			},
			wantErr: false,
		},
		{
			name: "HPA autoscaler with valid Memory metric",
			annotations: map[string]string{
				constants.AutoscalerClass:   string(constants.AutoscalerClassHPA),
				constants.AutoscalerMetrics: string(constants.AutoScalerMetricsMemory),
			},
			wantErr: false,
		},
		{
			name: "HPA autoscaler with invalid metrics",
			annotations: map[string]string{
				constants.AutoscalerClass:   string(constants.AutoscalerClassHPA),
				constants.AutoscalerMetrics: "invalid-metric",
			},
			wantErr: true,
			errMsg:  "is not a supported metric",
		},
		{
			name: "KEDA autoscaler class",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
			},
			wantErr: true, // KEDA is in allowed list but not handled in switch statement
			errMsg:  "unknown autoscaler class [keda]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-isvc",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validateInferenceServiceAutoscaler(isvc)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInferenceService_TargetUtilizationValidation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
	}{
		{
			name:        "no target utilization percentage",
			annotations: nil,
			wantErr:     false,
		},
		{
			name: "valid target utilization percentage",
			annotations: map[string]string{
				constants.TargetUtilizationPercentage: "50",
			},
			wantErr: false,
		},
		{
			name: "invalid target utilization percentage (too low)",
			annotations: map[string]string{
				constants.TargetUtilizationPercentage: "0",
			},
			wantErr: true,
		},
		{
			name: "invalid target utilization percentage (too high)",
			annotations: map[string]string{
				constants.TargetUtilizationPercentage: "150",
			},
			wantErr: true,
		},
		{
			name: "invalid target utilization percentage (not a number)",
			annotations: map[string]string{
				constants.TargetUtilizationPercentage: "not-a-number",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-isvc",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validateAutoscalerTargetUtilizationPercentage(isvc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test missing branches in validateHPAMetrics
func TestValidateHPAMetrics_AllMetrics(t *testing.T) {
	validMetrics := []v1beta1.ScaleMetric{
		v1beta1.ScaleMetric(constants.AutoScalerMetricsCPU),
		v1beta1.ScaleMetric(constants.AutoScalerMetricsMemory),
	}

	for _, metric := range validMetrics {
		t.Run(string(metric), func(t *testing.T) {
			err := validateHPAMetrics(metric)
			assert.NoError(t, err)
		})
	}

	t.Run("invalid metric", func(t *testing.T) {
		err := validateHPAMetrics("invalid-metric")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not a supported metric")
	})
}

// =============================================================================
// ENGINE/DECODER VALIDATION TESTS
// =============================================================================

func TestInferenceService_EngineDecoderValidation(t *testing.T) {
	tests := []struct {
		name       string
		hasEngine  bool
		hasDecoder bool
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "no decoder, no engine - should pass",
			hasEngine:  false,
			hasDecoder: false,
			wantErr:    false,
		},
		{
			name:       "has engine, has decoder - should pass",
			hasEngine:  true,
			hasDecoder: true,
			wantErr:    false,
		},
		{
			name:       "no engine, has decoder - should fail",
			hasEngine:  false,
			hasDecoder: true,
			wantErr:    true,
			errMsg:     "decoder cannot be specified without engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{},
			}

			// Add engine if needed
			if tt.hasEngine {
				isvc.Spec.Engine = &v1beta1.EngineSpec{}
			}

			// Add decoder if needed
			if tt.hasDecoder {
				isvc.Spec.Decoder = &v1beta1.DecoderSpec{}
			}

			err := validateEngineDecoderConfiguration(isvc)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHasFullRunnerConfig(t *testing.T) {
	tests := []struct {
		name     string
		engine   *v1beta1.EngineSpec
		expected bool
	}{
		{
			name:     "nil engine",
			engine:   nil,
			expected: false,
		},
		{
			name:     "empty engine",
			engine:   &v1beta1.EngineSpec{},
			expected: false,
		},
		{
			name: "engine with runner but no image",
			engine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{},
			},
			expected: false,
		},
		{
			name: "engine with runner and image",
			engine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Image: "test-image:latest",
					},
				},
			},
			expected: true,
		},
		{
			name: "engine with leader and worker with images",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Image: "leader-image:latest",
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Image: "worker-image:latest",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "engine with leader but no worker",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Image: "leader-image:latest",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "has worker but no leader",
			engine: &v1beta1.EngineSpec{
				Worker: &v1beta1.WorkerSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Image: "worker-image:latest",
						},
					},
				},
				// No leader
			},
			expected: false,
		},
		{
			name: "has leader and worker but leader has no image",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							// No image
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Runner: &v1beta1.RunnerSpec{
						Container: v1.Container{
							Image: "worker-image:latest",
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFullRunnerConfig(tt.engine)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// RUNTIME RESOLUTION TESTS
// =============================================================================

func TestInferenceService_RuntimeResolution(t *testing.T) {
	// Create test models with different configurations
	enabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "enabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelType:          stringPtr("text-generation"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1"),
			},
		},
	}

	disabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "disabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelType:          stringPtr("text-generation"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1"),
			},
			ModelExtensionSpec: v1beta1.ModelExtensionSpec{
				Disabled: boolPtr(true),
			},
		},
	}

	explicitlyEnabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "explicitly-enabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelType:          stringPtr("text-generation"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1"),
			},
			ModelExtensionSpec: v1beta1.ModelExtensionSpec{
				Disabled: boolPtr(false), // Explicitly enabled
			},
		},
	}

	modelWithEmptyFormat := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "model-empty-format",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelType:          stringPtr("text-generation"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name: "", // Empty name
			},
		},
	}

	// Create test runtimes
	testRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					Name:    "llama",
					Version: stringPtr("1"),
				},
			},
		},
	}

	runtimeWithWrongVersion := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wrong-version-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					Name:    "llama",
					Version: stringPtr("2"), // Wrong version
				},
			},
		},
	}

	runtimeWithNoNameMatch := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-name-match-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					Name:    "gpt", // Different name
					Version: stringPtr("1"),
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name    string
		objects []client.Object
		isvc    *v1beta1.InferenceService
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no engine - should skip validation",
			objects: []client.Object{enabledModel, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: stringPtr("enabled-model"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "engine with runtime specified - should pass",
			objects: []client.Object{enabledModel, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name: "test-runtime",
					},
					Engine: &v1beta1.EngineSpec{
						Runner: &v1beta1.RunnerSpec{
							Container: v1.Container{
								Image: "test-image:latest",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "engine with complete runner config - should pass",
			objects: []client.Object{enabledModel, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Runner: &v1beta1.RunnerSpec{
							Container: v1.Container{
								Image: "test-image:latest",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "engine without runtime and no model - should fail",
			objects: []client.Object{enabledModel, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						// No runner, incomplete config
					},
				},
			},
			wantErr: true,
			errMsg:  "model reference is required when runtime is not specified and engine does not have complete runner configuration",
		},
		{
			name:    "model not found",
			objects: []client.Object{}, // Empty - no model
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "nonexistent-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "failed to resolve model nonexistent-model",
		},
		{
			name:    "disabled model",
			objects: []client.Object{disabledModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "disabled-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "model disabled-model is disabled",
		},
		{
			name:    "explicitly enabled model",
			objects: []client.Object{explicitlyEnabledModel, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "explicitly-enabled-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true, // Still fails because runtime matching is complex
			errMsg:  "no supporting runtime found",
		},
		{
			name:    "enabled model with no supporting runtimes",
			objects: []client.Object{enabledModel}, // No runtime
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "enabled-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found for model enabled-model",
		},
		{
			name:    "model with empty format name",
			objects: []client.Object{modelWithEmptyFormat, testRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "model-empty-format",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found",
		},
		{
			name:    "runtime with wrong version",
			objects: []client.Object{enabledModel, runtimeWithWrongVersion},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "enabled-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found",
		},
		{
			name:    "runtime with no name match",
			objects: []client.Object{enabledModel, runtimeWithNoNameMatch},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "enabled-model",
					},
					Engine: &v1beta1.EngineSpec{
						// Incomplete config to trigger resolution
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := &InferenceServiceValidator{
				Client: fakeClient,
			}

			_, err := validator.validateRuntimeAndModelResolution(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// COMPREHENSIVE VALIDATION TESTS
// =============================================================================

func TestValidateInferenceService_ComprehensiveErrorPaths(t *testing.T) {
	// Create fake client with test data
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	validator := &InferenceServiceValidator{
		Client: fakeClient,
	}

	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
		errMsg  string
	}{
		{
			name: "invalid name should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "Invalid-Name", // Invalid format
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "invalid InferenceService name",
		},
		{
			name: "invalid autoscaler should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: "invalid-class",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "is not a supported autoscaler class type",
		},
		{
			name: "invalid target utilization should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.TargetUtilizationPercentage: "150", // Invalid
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "target utilization percentage should be a [1-100] integer",
		},
		{
			name: "decoder without engine should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Decoder: &v1beta1.DecoderSpec{}, // Decoder without engine
				},
			},
			wantErr: true,
			errMsg:  "decoder cannot be specified without engine",
		},
		{
			name: "engine without runtime and model should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						// Incomplete config, no runtime, no model
					},
				},
			},
			wantErr: true,
			errMsg:  "model reference is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := validator.validateInferenceService(context.Background(), tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Empty(t, warnings)
			}
		})
	}
}

// =============================================================================
// COMPREHENSIVE RESOLVEMODELANDRUNTIME TESTS
// =============================================================================

func TestResolveModelAndRuntime_Comprehensive(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	// Simple model that will generate label "mt:llama:1:llama"
	simpleModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
		},
	}

	// Disabled model
	disabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "disabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
			ModelExtensionSpec: v1beta1.ModelExtensionSpec{
				Disabled: boolPtr(true),
			},
		},
	}

	// Runtime that matches the simple model with AutoSelect=true
	matchingRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "matching-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1), // Optional weight
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true), // Critical for matching
				},
			},
		},
	}

	// Runtime without AutoSelect (won't match)
	nonAutoSelectRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-autoselect-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1),
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(false), // Won't match
				},
			},
		},
	}

	tests := []struct {
		name         string
		objects      []client.Object
		isvc         *v1beta1.InferenceService
		wantErr      bool
		errMsg       string
		wantWarnings int
	}{
		{
			name:    "model not found",
			objects: []client.Object{},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "nonexistent-model",
					},
				},
			},
			wantErr: true,
			errMsg:  "failed to resolve model nonexistent-model",
		},
		{
			name:    "model disabled",
			objects: []client.Object{disabledModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "disabled-model",
					},
				},
			},
			wantErr: true,
			errMsg:  "model disabled-model is disabled",
		},
		{
			name:    "successful runtime resolution",
			objects: []client.Object{simpleModel, matchingRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "simple-model",
					},
				},
			},
			wantErr:      false,
			wantWarnings: 1,
		},
		{
			name:    "no supporting runtime found",
			objects: []client.Object{simpleModel, nonAutoSelectRuntime},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "simple-model",
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found for model simple-model",
		},
		{
			name:    "no runtimes at all",
			objects: []client.Object{simpleModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "simple-model",
					},
				},
			},
			wantErr: true,
			errMsg:  "no supporting runtime found for model simple-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := &InferenceServiceValidator{
				Client: fakeClient,
			}

			warnings, err := validator.resolveModelAndRuntime(context.Background(), tt.isvc, admission.Warnings{})

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantWarnings > 0 {
					assert.Len(t, warnings, tt.wantWarnings)
					assert.Contains(t, warnings[0], "will be auto-selected for model")
				}
			}
		})
	}
}

// Test edge cases and warning scenarios
func TestResolveModelAndRuntime_EdgeCases(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	// Model with no disabled field (should be treated as enabled)
	enabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "enabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
			// No Disabled field - should be treated as enabled
		},
	}

	// Model explicitly enabled
	explicitlyEnabledModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "explicitly-enabled-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
			ModelExtensionSpec: v1beta1.ModelExtensionSpec{
				Disabled: boolPtr(false), // Explicitly enabled
			},
		},
	}

	matchingRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "matching-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1), // default value is 1
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	tests := []struct {
		name         string
		objects      []client.Object
		modelName    string
		wantErr      bool
		wantWarnings int
	}{
		{
			name:         "model with no disabled field",
			objects:      []client.Object{enabledModel, matchingRuntime},
			modelName:    "enabled-model",
			wantErr:      false,
			wantWarnings: 1,
		},
		{
			name:         "explicitly enabled model",
			objects:      []client.Object{explicitlyEnabledModel, matchingRuntime},
			modelName:    "explicitly-enabled-model",
			wantErr:      false,
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := &InferenceServiceValidator{
				Client: fakeClient,
			}

			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: tt.modelName,
					},
				},
			}

			warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantWarnings > 0 {
					assert.Len(t, warnings, tt.wantWarnings)
				}
			}
		})
	}
}

// Test warning preservation and multiple runtimes
func TestResolveModelAndRuntime_WarningHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
		},
	}

	runtime1 := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "runtime-1",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1),
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	runtime2 := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "runtime-2",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1),
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(model, runtime1, runtime2).
		Build()

	validator := &InferenceServiceValidator{
		Client: fakeClient,
	}

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-isvc",
			Namespace: "default",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Model: &v1beta1.ModelRef{
				Name: "test-model",
			},
		},
	}

	t.Run("multiple runtimes found", func(t *testing.T) {
		warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})
		assert.NoError(t, err)
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "will be auto-selected for model test-model")
	})

	t.Run("existing warnings preserved", func(t *testing.T) {
		initialWarnings := admission.Warnings{"existing warning"}
		warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, initialWarnings)
		assert.NoError(t, err)
		assert.Len(t, warnings, 2)
		assert.Equal(t, "existing warning", warnings[0])
		assert.Contains(t, warnings[1], "will be auto-selected")
	})
}

// Test namespace vs cluster model precedence
func TestResolveModelAndRuntime_NamespacePrecedence(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	// Namespace-scoped model
	namespacedModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "test-namespace",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
		},
	}

	// Cluster-scoped model with same name
	clusterModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("different"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "different",
				Version: stringPtr("1.0.0"),
			},
		},
	}

	runtime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-runtime",
		},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:    "llama",
						Version: stringPtr("1.0.0"),
						Weight:  int64(1),
					},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespacedModel, clusterModel, runtime).
		Build()

	validator := &InferenceServiceValidator{
		Client: fakeClient,
	}

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-isvc",
			Namespace: "test-namespace",
		},
		Spec: v1beta1.InferenceServiceSpec{
			Model: &v1beta1.ModelRef{
				Name: "test-model",
			},
		},
	}

	warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})
	assert.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "will be auto-selected for model test-model")
}

// =============================================================================
// UTILITY TESTS
// =============================================================================

// Test GetIntReference function (0% coverage)
func TestGetIntReference(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{
			name:     "positive number",
			input:    42,
			expected: 42,
		},
		{
			name:     "zero",
			input:    0,
			expected: 0,
		},
		{
			name:     "negative number",
			input:    -10,
			expected: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntReference(tt.input)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}

// Test error cases in convertToInferenceService
func TestConvertToInferenceService_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid InferenceService",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
			wantErr: false,
		},
		{
			name:    "Pod instead of InferenceService",
			obj:     &v1.Pod{},
			wantErr: true,
			errMsg:  "expected an InferenceService object but got *v1.Pod",
		},
		{
			name:    "ConfigMap instead of InferenceService",
			obj:     &v1.ConfigMap{},
			wantErr: true,
			errMsg:  "expected an InferenceService object but got *v1.ConfigMap",
		},
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
			errMsg:  "expected an InferenceService object but got <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToInferenceService(tt.obj)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func boolPtr(b bool) *bool {
	return &b
}
