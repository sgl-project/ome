package isvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeselector"
	"sigs.k8s.io/ome/pkg/validation"
)

// =============================================================================
// VALIDATOR INTERFACE TESTS
// =============================================================================

func TestValidateClusterBaseModelPVCNamespace(t *testing.T) {
	pvc := func(uri string) *v1beta1.BaseModelSpec {
		return &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{StorageUri: stringPtr(uri)}}
	}
	clusterMeta := &metav1.ObjectMeta{Name: "shared-llama"}
	namespacedMeta := &metav1.ObjectMeta{Name: "llama", Namespace: "models"}
	isvc := func(ns string) *v1beta1.InferenceService {
		return &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: ns},
			Spec:       v1beta1.InferenceServiceSpec{Model: &v1beta1.ModelRef{Name: "shared-llama"}},
		}
	}

	tests := []struct {
		name    string
		spec    *v1beta1.BaseModelSpec
		meta    *metav1.ObjectMeta
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{name: "namespaced BaseModel: skip", spec: pvc("pvc://my-pvc/p"), meta: namespacedMeta, isvc: isvc("models")},
		{name: "ClusterBaseModel non-PVC: skip", spec: pvc("oci://n/ns/b/bucket/o/path"), meta: clusterMeta, isvc: isvc("team-foo")},
		{name: "ClusterBaseModel PVC same ns: ok", spec: pvc("pvc://team-foo:my-pvc/p"), meta: clusterMeta, isvc: isvc("team-foo")},
		{name: "ClusterBaseModel PVC missing ns prefix: defer to BaseModel webhook", spec: pvc("pvc://my-pvc/p"), meta: clusterMeta, isvc: isvc("team-foo")},
		{name: "ClusterBaseModel PVC cross-ns: reject", spec: pvc("pvc://shared:my-pvc/p"), meta: clusterMeta, isvc: isvc("team-foo"), wantErr: true},
		{name: "spec nil: skip", spec: nil, meta: clusterMeta, isvc: isvc("team-foo")},
		{name: "spec storage nil: skip", spec: &v1beta1.BaseModelSpec{}, meta: clusterMeta, isvc: isvc("team-foo")},
		{name: "spec storage uri nil: skip", spec: &v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{}}, meta: clusterMeta, isvc: isvc("team-foo")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClusterBaseModelPVCNamespace(tc.spec, tc.meta, tc.isvc)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

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
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
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
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
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
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
				},
			},
			newIsvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
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
				Spec: v1beta1.InferenceServiceSpec{},
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
			err := validation.ValidateInferenceServiceName(tt.isvc.Name)

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
			name: "valid KEDA autoscaler class",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
			},
			wantErr: false,
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

			err := func() error { _, err := validation.ValidateAutoscalerConfig(isvc); return err }()

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

			err := validation.ValidateAutoscalerTargetUtilizationPercentage(isvc)

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
	validMetrics := []string{
		string(constants.AutoScalerMetricsCPU),
		string(constants.AutoScalerMetricsMemory),
	}

	for _, metric := range validMetrics {
		t.Run(metric, func(t *testing.T) {
			err := validation.ValidateHPAMetrics(metric)
			assert.NoError(t, err)
		})
	}

	t.Run("invalid metric", func(t *testing.T) {
		err := validation.ValidateHPAMetrics("invalid-metric")
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

			err := validation.ValidateEngineDecoderConfig(&isvc.Spec)

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

func TestInferenceService_OMENativeEngineDecoderCoupling(t *testing.T) {
	annot := func(mode string) v1beta1.ComponentExtensionSpec {
		return v1beta1.ComponentExtensionSpec{
			Annotations: map[string]string{constants.DeploymentMode: mode},
		}
	}
	withRunner := &v1beta1.RunnerSpec{Container: v1.Container{Image: "test-image:latest"}}

	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
		errMsg  string
	}{
		{
			name: "engine and decoder both OMENative - passes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: annot(string(constants.OMENative)),
						Runner:                 withRunner,
					},
					Decoder: &v1beta1.DecoderSpec{
						ComponentExtensionSpec: annot(string(constants.OMENative)),
					},
				},
			},
		},
		{
			name: "engine OMENative, decoder unset - rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: annot(string(constants.OMENative)),
						Runner:                 withRunner,
					},
					Decoder: &v1beta1.DecoderSpec{},
				},
			},
			wantErr: true,
			errMsg:  "InvalidDeploymentModeCombination",
		},
		{
			name: "engine OMENative, decoder RawDeployment - rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: annot(string(constants.OMENative)),
						Runner:                 withRunner,
					},
					Decoder: &v1beta1.DecoderSpec{
						ComponentExtensionSpec: annot(string(constants.RawDeployment)),
					},
				},
			},
			wantErr: true,
			errMsg:  "InvalidDeploymentModeCombination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			_, err := v.ValidateCreate(context.Background(), tt.isvc)
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

func TestInferenceService_LeaderWorkerPairing(t *testing.T) {
	sizePtr := func(v int) *int { return &v }
	withRunner := &v1beta1.RunnerSpec{Container: v1.Container{Image: "test-image:latest"}}

	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantErr bool
		errSub  string
	}{
		{
			name: "engine: leader + worker(size=3) - valid",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
					Engine: &v1beta1.EngineSpec{
						Runner: withRunner,
						Leader: &v1beta1.LeaderSpec{},
						Worker: &v1beta1.WorkerSpec{Size: sizePtr(3)},
					},
				},
			},
		},
		{
			name: "engine: leader only - rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Runner: withRunner,
						Leader: &v1beta1.LeaderSpec{},
					},
				},
			},
			wantErr: true,
			errSub:  "InvalidLeaderWorkerPairing",
		},
		{
			name: "engine: worker only - rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						Runner: withRunner,
						Worker: &v1beta1.WorkerSpec{Size: sizePtr(3)},
					},
				},
			},
			wantErr: true,
			errSub:  "InvalidLeaderWorkerPairing",
		},
		{
			name: "decoder: leader only - rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{Runner: withRunner},
					Decoder: &v1beta1.DecoderSpec{
						Leader: &v1beta1.LeaderSpec{},
					},
				},
			},
			wantErr: true,
			errSub:  "decoder.leader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{}
			_, err := v.ValidateCreate(context.Background(), tt.isvc)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSub != "" {
					assert.Contains(t, err.Error(), tt.errSub)
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
				Spec: v1beta1.InferenceServiceSpec{},
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
				Client:          fakeClient,
				RuntimeSelector: runtimeselector.New(fakeClient),
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
		Client:          fakeClient,
		RuntimeSelector: runtimeselector.New(fakeClient),
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
			errMsg:  "at least one of spec.model or spec.runtime must be set",
		},
		{
			// Lean path: model omitted, runtime named explicitly. The
			// validator must accept this — model parsing + runtime
			// auto-select is skipped by the controller, and the runtime
			// is fetched as-is.
			name: "no model + runtime named (lean path) - passes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-lean", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
					Engine:  &v1beta1.EngineSpec{},
				},
			},
		},
		{
			// Lean path failure: only spec.runtime.name=="" — caught by
			// the same Model-OR-Runtime rule.
			name: "no model + empty runtime name should fail",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-lean-bad", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: ""},
					Engine:  &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "at least one of spec.model or spec.runtime must be set",
		},
		// Explicit-pin admission validation.
		{
			name: "explicit revision pin with autoSync=false (cluster) passes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-ok", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-llama-pd",
						AutoSync: boolPtr(false),
						Revision: stringPtr("cr-srt-llama-pd-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
		},
		{
			name: "explicit revision pin with autoSync=true rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-conflict", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-llama-pd",
						AutoSync: boolPtr(true),
						Revision: stringPtr("cr-srt-llama-pd-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "AutoSync=true would silently ignore the pin",
		},
		{
			name: "explicit revision pin with autoSync omitted (defaults true) rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-defaultsync", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-llama-pd",
						Revision: stringPtr("cr-srt-llama-pd-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "AutoSync=true would silently ignore the pin",
		},
		{
			name: "explicit revision pin naming a DIFFERENT runtime rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-wrong-runtime", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-llama-pd",
						AutoSync: boolPtr(false),
						Revision: stringPtr("cr-srt-other-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "does not match the expected naming convention",
		},
		{
			name: "explicit revision pin with malformed hash rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-bad-hash", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-llama-pd",
						AutoSync: boolPtr(false),
						Revision: stringPtr("cr-srt-llama-pd-XYZ"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "does not match the expected naming convention",
		},
		{
			name: "explicit revision pin for namespaced runtime (correct scope) passes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-ns-ok", Namespace: "team-a"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-foo",
						Kind:     stringPtr("ServingRuntime"),
						AutoSync: boolPtr(false),
						Revision: stringPtr("r-team-a-srt-foo-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
		},
		{
			name: "explicit revision pin for namespaced runtime, wrong namespace prefix rejected",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "pin-ns-bad", Namespace: "team-a"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{
						Name:     "srt-foo",
						Kind:     stringPtr("ServingRuntime"),
						AutoSync: boolPtr(false),
						Revision: stringPtr("r-team-b-srt-foo-abc12345"),
					},
					Engine: &v1beta1.EngineSpec{},
				},
			},
			wantErr: true,
			errMsg:  "does not match the expected naming convention",
		},
		{
			name: "no explicit revision pin → validator no-op",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "no-pin", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Runtime: &v1beta1.ServingRuntimeRef{Name: "srt-llama-pd", AutoSync: boolPtr(false)},
					Engine:  &v1beta1.EngineSpec{},
				},
			},
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
				Client:          fakeClient,
				RuntimeSelector: runtimeselector.New(fakeClient),
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

func TestResolveModelAndRuntime_ExplicitRuntimeCompatIsAdvisory(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	// llama model.
	llamaModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-model"},
		Spec: v1beta1.BaseModelSpec{
			ModelArchitecture:  stringPtr("llama"),
			ModelParameterSize: stringPtr("7B"),
			ModelFormat: v1beta1.ModelFormat{
				Name:    "llama",
				Version: stringPtr("1.0.0"),
			},
		},
	}

	// A generic runtime that only *declares* support for a different
	// architecture/format ("gpt"). It does not enumerate "llama", so the
	// compatibility matcher reports a mismatch — but the operator named it
	// explicitly, so admission should warn-and-admit rather than reject.
	genericRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "generic-runtime"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat:       &v1beta1.ModelFormat{Name: "gpt", Version: stringPtr("1.0.0")},
					ModelArchitecture: stringPtr("gpt"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	// A runtime that genuinely declares support for llama.
	matchingRuntime := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "matching-runtime"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat:       &v1beta1.ModelFormat{Name: "llama", Version: stringPtr("1.0.0"), Weight: int64(1)},
					ModelArchitecture: stringPtr("llama"),
					AutoSelect:        boolPtr(true),
				},
			},
		},
	}

	t.Run("explicit runtime + format mismatch is admitted with advisory warning", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(llamaModel.DeepCopy(), genericRuntime.DeepCopy()).
			Build()
		validator := &InferenceServiceValidator{
			Client:          fakeClient,
			RuntimeSelector: runtimeselector.New(fakeClient),
		}
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
			Spec: v1beta1.InferenceServiceSpec{
				Model:   &v1beta1.ModelRef{Name: "llama-model"},
				Runtime: &v1beta1.ServingRuntimeRef{Name: "generic-runtime"},
			},
		}

		warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})

		assert.NoError(t, err, "explicit-runtime compat mismatch must be admitted, not rejected")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "does not declare support for model")
		assert.Contains(t, warnings[0], "proceeding because the runtime was named explicitly")
		// The dropped success-warning phrasing must not reappear.
		assert.NotContains(t, warnings[0], "is valid for model")
	})

	t.Run("explicit runtime that matches is admitted with no warning", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(llamaModel.DeepCopy(), matchingRuntime.DeepCopy()).
			Build()
		validator := &InferenceServiceValidator{
			Client:          fakeClient,
			RuntimeSelector: runtimeselector.New(fakeClient),
		}
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
			Spec: v1beta1.InferenceServiceSpec{
				Model:   &v1beta1.ModelRef{Name: "llama-model"},
				Runtime: &v1beta1.ServingRuntimeRef{Name: "matching-runtime"},
			},
		}

		warnings, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})

		assert.NoError(t, err)
		assert.Empty(t, warnings, "a matching explicit runtime emits no advisory warning")
	})

	t.Run("explicit runtime not found still hard-fails", func(t *testing.T) {
		// A non-existent runtime is not a declared-format mismatch; it must
		// stay a hard error even on the explicit path.
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(llamaModel.DeepCopy()).
			Build()
		validator := &InferenceServiceValidator{
			Client:          fakeClient,
			RuntimeSelector: runtimeselector.New(fakeClient),
		}
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
			Spec: v1beta1.InferenceServiceSpec{
				Model:   &v1beta1.ModelRef{Name: "llama-model"},
				Runtime: &v1beta1.ServingRuntimeRef{Name: "does-not-exist"},
			},
		}

		_, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("auto-select with no matching runtime still hard-fails", func(t *testing.T) {
		// Same model, only the non-matching generic runtime present, and NO
		// explicit runtime named: OME is choosing, so compat MUST gate.
		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(llamaModel.DeepCopy(), genericRuntime.DeepCopy()).
			Build()
		validator := &InferenceServiceValidator{
			Client:          fakeClient,
			RuntimeSelector: runtimeselector.New(fakeClient),
		}
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
			Spec: v1beta1.InferenceServiceSpec{
				Model: &v1beta1.ModelRef{Name: "llama-model"},
			},
		}

		_, err := validator.resolveModelAndRuntime(context.Background(), isvc, admission.Warnings{})

		assert.Error(t, err, "auto-select must still reject when nothing supports the model")
		assert.Contains(t, err.Error(), "no supporting runtime found for model llama-model")
	})
}

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
				Client:          fakeClient,
				RuntimeSelector: runtimeselector.New(fakeClient),
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
		Client:          fakeClient,
		RuntimeSelector: runtimeselector.New(fakeClient),
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
		Client:          fakeClient,
		RuntimeSelector: runtimeselector.New(fakeClient),
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
// MODEL EXISTS VALIDATION TESTS
// =============================================================================

func TestValidateModelExists(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	// Create test models
	clusterModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing-cluster-model",
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

	namespaceModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-namespace-model",
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

	tests := []struct {
		name    string
		objects []client.Object
		isvc    *v1beta1.InferenceService
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no model reference - should pass",
			objects: []client.Object{},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					// No Model reference
				},
			},
			wantErr: false,
		},
		{
			name:    "model reference with empty name - should pass",
			objects: []client.Object{},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "", // Empty name
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "referenced ClusterBaseModel exists - should pass",
			objects: []client.Object{clusterModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "existing-cluster-model",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "referenced BaseModel exists in namespace - should pass",
			objects: []client.Object{namespaceModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "existing-namespace-model",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "referenced model does not exist - should fail",
			objects: []client.Object{}, // No models
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
			errMsg:  "referenced model \"nonexistent-model\" not found in namespace \"default\"",
		},
		{
			name:    "referenced BaseModel exists but in different namespace - should fail",
			objects: []client.Object{namespaceModel}, // Model is in test-namespace
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "different-namespace", // Different namespace
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "existing-namespace-model",
					},
				},
			},
			wantErr: true,
			errMsg:  "referenced model \"existing-namespace-model\" not found in namespace \"different-namespace\"",
		},
		{
			name:    "ClusterBaseModel is accessible from any namespace - should pass",
			objects: []client.Object{clusterModel},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "any-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "existing-cluster-model",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := &InferenceServiceValidator{
				Client:          fakeClient,
				RuntimeSelector: runtimeselector.New(fakeClient),
			}

			err := validator.validateModelExists(context.Background(), tt.isvc)

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

// Test that validateModelExists is called during full validation
func TestValidateInferenceService_ModelExistsIntegration(t *testing.T) {
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
			name:    "full validation rejects missing model",
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
			errMsg:  "referenced model \"nonexistent-model\" not found in namespace \"default\"",
		},
		{
			name: "full validation passes with existing model",
			objects: []client.Object{
				&v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "existing-model",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelArchitecture:  stringPtr("llama"),
						ModelParameterSize: stringPtr("7B"),
						ModelFormat: v1beta1.ModelFormat{
							Name:    "llama",
							Version: stringPtr("1.0.0"),
						},
					},
				},
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "existing-model",
					},
					// No Engine, so runtime resolution won't be triggered
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			validator := &InferenceServiceValidator{
				Client:          fakeClient,
				RuntimeSelector: runtimeselector.New(fakeClient),
			}

			_, err := validator.validateInferenceService(context.Background(), tt.isvc)

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
// HELPER FUNCTIONS
// =============================================================================

func boolPtr(b bool) *bool {
	return &b
}

// TestValidateOverlays_StructuralChecks covers the schema-level overlay
// rejections (duplicates, self-reference, env-name collision) that fire
// before any apiserver lookups happen — no fake client needed.
func TestValidateOverlays_StructuralChecks(t *testing.T) {
	v := &InferenceServiceValidator{}
	clusterKind := "ClusterBaseModel"
	apiGroup := "ome.io"
	tests := []struct {
		name     string
		overlays []v1beta1.ModelOverlayRef
		wantErr  string // empty = expect success
	}{
		{
			name:     "empty overlays accepted",
			overlays: nil,
		},
		{
			name: "single overlay accepted",
			overlays: []v1beta1.ModelOverlayRef{
				{Name: "foo-pvc", Kind: &clusterKind, APIGroup: &apiGroup},
			},
		},
		{
			name: "empty overlay name rejected",
			overlays: []v1beta1.ModelOverlayRef{
				{Name: "", Kind: &clusterKind, APIGroup: &apiGroup},
			},
			wantErr: "overlay name cannot be empty",
		},
		{
			name: "overlay matching primary rejected",
			overlays: []v1beta1.ModelOverlayRef{
				{Name: "primary", Kind: &clusterKind, APIGroup: &apiGroup},
			},
			wantErr: "must not reference the primary model",
		},
		{
			name: "duplicate overlay name rejected",
			overlays: []v1beta1.ModelOverlayRef{
				{Name: "foo-pvc"}, {Name: "foo-pvc"},
			},
			wantErr: "declared more than once",
		},
		{
			name: "sanitized env-var collision rejected",
			overlays: []v1beta1.ModelOverlayRef{
				{Name: "foo-bar"}, {Name: "foo_bar"},
			},
			wantErr: "both sanitize to env var OVERLAY_FOO_BAR_MODEL_PATH",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{Name: "primary", Overlays: tc.overlays},
				},
			}
			err := v.validateOverlays(isvc)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TestDeploymentStrategyWarnings pins the footgun warning: deploymentStrategy is
// raw-only, so setting it on an OMENative/PD component is a silent no-op and must
// warn (pointing at lifecycle.updateStrategy); raw and no-strategy must not warn.
func TestDeploymentStrategyWarnings(t *testing.T) {
	strategy := func() *appsv1.DeploymentStrategy {
		return &appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	}
	engineWith := &v1beta1.EngineSpec{
		ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{DeploymentStrategy: strategy()},
	}
	modeAnn := func(m constants.DeploymentModeType) map[string]string {
		return map[string]string{constants.DeploymentMode: string(m)}
	}

	tests := []struct {
		name    string
		isvc    *v1beta1.InferenceService
		wantLen int
	}{
		{
			name: "OMENative engine with deploymentStrategy warns",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Annotations: modeAnn(constants.OMENative)},
				Spec:       v1beta1.InferenceServiceSpec{Engine: engineWith},
			},
			wantLen: 1,
		},
		{
			name: "RawDeployment engine with deploymentStrategy does not warn (honored)",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Annotations: modeAnn(constants.RawDeployment)},
				Spec:       v1beta1.InferenceServiceSpec{Engine: engineWith},
			},
			wantLen: 0,
		},
		{
			name: "OMENative without deploymentStrategy does not warn",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Annotations: modeAnn(constants.OMENative)},
				Spec:       v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}},
			},
			wantLen: 0,
		},
		{
			name: "no resolved mode does not warn",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: engineWith},
			},
			wantLen: 0,
		},
		{
			name: "PD decoder with deploymentStrategy warns",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Annotations: modeAnn(constants.PDDisaggregated)},
				Spec: v1beta1.InferenceServiceSpec{
					Decoder: &v1beta1.DecoderSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{DeploymentStrategy: strategy()},
					},
				},
			},
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Len(t, deploymentStrategyWarnings(tt.isvc), tt.wantLen)
		})
	}
}

// =============================================================================
// MIGRATION-REQUEST ANNOTATION VALIDATION (mailbox admission)
// =============================================================================

func TestValidateKEDAConfig(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		wantErr      bool
		errMsg       string
		wantWarnings int
	}{
		{
			name: "no KEDA config - should pass",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: false,
		},
		{
			name: "valid KEDA config with all fields",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						PromServerAddress: "http://prometheus.monitoring.svc:9090",
						ScalingThreshold:  "10",
						ScalingOperator:   "GreaterThanOrEqual",
						AuthModes:         "basic",
						AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
							Name: "my-auth",
							Kind: "TriggerAuthentication",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid KEDA config with HTTPS Prometheus address",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						PromServerAddress: "https://grafana-cloud.example.com",
						ScalingThreshold:  "5.5",
						ScalingOperator:   "LessThan",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid scaling operator in KedaConfig",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						ScalingOperator: "InvalidOperator",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid KEDA scaling operator",
		},
		{
			name: "invalid scaling operator in annotation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass:     string(constants.AutoscalerClassKEDA),
						constants.KedaScalingOperator: "BadOperator",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "invalid KEDA scaling operator",
		},
		{
			name: "invalid scaling threshold in KedaConfig",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						ScalingThreshold: "not-a-number",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid KEDA scaling threshold",
		},
		{
			name: "invalid scaling threshold in annotation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass:      string(constants.AutoscalerClassKEDA),
						constants.KedaScalingThreshold: "abc",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "invalid KEDA scaling threshold",
		},
		{
			name: "invalid Prometheus address - no scheme",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						PromServerAddress: "prometheus.monitoring.svc:9090",
					},
				},
			},
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name: "invalid Prometheus address - invalid scheme",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						PromServerAddress: "ftp://prometheus.monitoring.svc:9090",
					},
				},
			},
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name: "invalid Prometheus address in annotation",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass:             string(constants.AutoscalerClassKEDA),
						constants.KedaPrometheusServerAddress: "not-a-valid-url",
					},
				},
				Spec: v1beta1.InferenceServiceSpec{},
			},
			wantErr: true,
			errMsg:  "invalid KEDA Prometheus server address",
		},
		{
			name: "invalid auth mode",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						AuthModes: "invalid-auth-mode",
						AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
							Name: "my-auth",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid KEDA auth mode",
		},
		{
			name: "authModes without authenticationRef - should warn",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						AuthModes: "basic",
						// No AuthenticationRef
					},
				},
			},
			wantErr:      false,
			wantWarnings: 1,
		},
		{
			name: "multiple auth modes - valid",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						AuthModes: "tls,basic",
						AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
							Name: "my-auth",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple auth modes with one invalid",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
					Annotations: map[string]string{
						constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
					},
				},
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: &v1beta1.KedaConfig{
						AuthModes: "basic,invalid",
						AuthenticationRef: &v1beta1.ScalerAuthenticationRef{
							Name: "my-auth",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid KEDA auth mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := validateKEDAConfig(tt.isvc)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantWarnings > 0 {
					assert.Len(t, warnings, tt.wantWarnings)
				}
			}
		})
	}
}

func TestValidateKEDAScalingOperator(t *testing.T) {
	validOperators := []string{
		"GreaterThan",
		"GreaterThanOrEqual",
		"LessThan",
		"LessThanOrEqual",
	}

	for _, op := range validOperators {
		t.Run("valid_"+op, func(t *testing.T) {
			err := validateKEDAScalingOperator(op)
			assert.NoError(t, err)
		})
	}

	invalidOperators := []string{
		"greaterthan",
		"GREATERTHAN",
		"GreaterThanOrEquals",
		"Equal",
		"NotEqual",
		"",
		">=",
		"<=",
	}

	for _, op := range invalidOperators {
		t.Run("invalid_"+op, func(t *testing.T) {
			err := validateKEDAScalingOperator(op)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid KEDA scaling operator")
		})
	}
}

func TestValidateKEDAScalingThreshold(t *testing.T) {
	validThresholds := []string{
		"10",
		"0",
		"-5",
		"3.14",
		"0.5",
		"100.0",
		"1e10",
	}

	for _, threshold := range validThresholds {
		t.Run("valid_"+threshold, func(t *testing.T) {
			err := validateKEDAScalingThreshold(threshold)
			assert.NoError(t, err)
		})
	}

	invalidThresholds := []string{
		"not-a-number",
		"abc",
		"10abc",
		"",
		"10,5",
	}

	for _, threshold := range invalidThresholds {
		t.Run("invalid_"+threshold, func(t *testing.T) {
			err := validateKEDAScalingThreshold(threshold)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid KEDA scaling threshold")
		})
	}
}

func TestValidateKEDAPrometheusServerAddress(t *testing.T) {
	validAddresses := []string{
		"http://prometheus.monitoring.svc:9090",
		"https://grafana-cloud.example.com",
		"http://localhost:9090",
		"https://prometheus.example.com:443/api/v1",
		"http://10.0.0.1:9090",
	}

	for _, addr := range validAddresses {
		t.Run("valid_"+addr, func(t *testing.T) {
			err := validateKEDAPrometheusServerAddress(addr)
			assert.NoError(t, err)
		})
	}

	invalidAddresses := []struct {
		addr   string
		errMsg string
	}{
		{"prometheus.monitoring.svc:9090", "scheme must be http or https"},
		{"ftp://prometheus.monitoring.svc:9090", "scheme must be http or https"},
		{"://prometheus.monitoring.svc:9090", "invalid KEDA Prometheus server address"},
		{"http://", "host is required"},
	}

	for _, tc := range invalidAddresses {
		t.Run("invalid_"+tc.addr, func(t *testing.T) {
			err := validateKEDAPrometheusServerAddress(tc.addr)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestValidateKEDAAuthModes(t *testing.T) {
	validAuthModes := []string{
		"basic",
		"tls",
		"bearer",
		"custom",
		"basic,tls",
		"tls, bearer",
		"basic, tls, bearer, custom",
	}

	for _, mode := range validAuthModes {
		t.Run("valid_"+mode, func(t *testing.T) {
			err := validateKEDAAuthModes(mode)
			assert.NoError(t, err)
		})
	}

	invalidAuthModes := []string{
		"invalid",
		"Basic",
		"BASIC",
		"oauth",
		"basic,invalid",
		"api-key",
	}

	for _, mode := range invalidAuthModes {
		t.Run("invalid_"+mode, func(t *testing.T) {
			err := validateKEDAAuthModes(mode)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid KEDA auth mode")
		})
	}

	// Empty auth modes should pass (empty string, only whitespace)
	t.Run("empty auth modes", func(t *testing.T) {
		err := validateKEDAAuthModes("")
		assert.NoError(t, err)
	})
}

func TestInferenceService_KEDAAutoscalerIntegration(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		kedaConfig   *v1beta1.KedaConfig
		wantErr      bool
		errMsg       string
		wantWarnings int
	}{
		{
			name: "KEDA autoscaler with valid config",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
			},
			kedaConfig: &v1beta1.KedaConfig{
				PromServerAddress: "http://prometheus:9090",
				ScalingThreshold:  "10",
				ScalingOperator:   "GreaterThanOrEqual",
			},
			wantErr: false,
		},
		{
			name: "KEDA autoscaler with invalid operator",
			annotations: map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassKEDA),
			},
			kedaConfig: &v1beta1.KedaConfig{
				ScalingOperator: "Invalid",
			},
			wantErr: true,
			errMsg:  "invalid KEDA scaling operator",
		},
		{
			name: "KEDA autoscaler with annotation override",
			annotations: map[string]string{
				constants.AutoscalerClass:     string(constants.AutoscalerClassKEDA),
				constants.KedaScalingOperator: "GreaterThan",
			},
			kedaConfig: nil,
			wantErr:    false,
		},
		{
			name: "KEDA autoscaler with annotation invalid threshold",
			annotations: map[string]string{
				constants.AutoscalerClass:      string(constants.AutoscalerClassKEDA),
				constants.KedaScalingThreshold: "invalid",
			},
			kedaConfig: nil,
			wantErr:    true,
			errMsg:     "invalid KEDA scaling threshold",
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
				Spec: v1beta1.InferenceServiceSpec{
					KedaConfig: tt.kedaConfig,
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

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================
