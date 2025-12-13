package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// MockClient is a mock implementation of client.Client for testing
type MockClient struct {
	client.Client
	getFunc  func(key client.ObjectKey, obj client.Object) error
	listFunc func(list client.ObjectList, opts ...client.ListOption) error
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.getFunc != nil {
		return m.getFunc(key, obj)
	}
	return fmt.Errorf("not found")
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listFunc != nil {
		return m.listFunc(list, opts...)
	}
	return fmt.Errorf("not found")
}

func TestIsMergedFineTunedWeight(t *testing.T) {
	tests := []struct {
		name            string
		fineTunedWeight *v1beta1.FineTunedWeight
		expectedResult  bool
		expectError     bool
	}{
		{
			name: "merged weights true",
			fineTunedWeight: &v1beta1.FineTunedWeight{
				Spec: v1beta1.FineTunedWeightSpec{
					Configuration: runtime.RawExtension{
						Raw: marshalJSONHelper(map[string]interface{}{
							constants.FineTunedWeightMergedWeightsConfigKey: true,
						}),
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "merged weights false",
			fineTunedWeight: &v1beta1.FineTunedWeight{
				Spec: v1beta1.FineTunedWeightSpec{
					Configuration: runtime.RawExtension{
						Raw: marshalJSONHelper(map[string]interface{}{
							constants.FineTunedWeightMergedWeightsConfigKey: false,
						}),
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "merged weights key not present",
			fineTunedWeight: &v1beta1.FineTunedWeight{
				Spec: v1beta1.FineTunedWeightSpec{
					Configuration: runtime.RawExtension{
						Raw: marshalJSONHelper(map[string]interface{}{
							"other_config": "value",
						}),
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "invalid json configuration",
			fineTunedWeight: &v1beta1.FineTunedWeight{
				Spec: v1beta1.FineTunedWeightSpec{
					Configuration: runtime.RawExtension{
						Raw: []byte(`{invalid json`),
					},
				},
			},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsMergedFineTunedWeight(tt.fineTunedWeight)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestIsEmptyModelDirVolumeRequired(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "both annotations empty",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name: "model init injection true",
			annotations: map[string]string{
				constants.ModelInitInjectionKey: "true",
			},
			expected: true,
		},
		{
			name: "fine tuned adapter injection annotation present",
			annotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: "amaaaaaask7dceya3ro4ls2wit3tu5dkk2u2ijvbbu4gmhbrsjeytwc2yagq",
			},
			expected: true,
		},
		{
			name: "both annotations present - model init true",
			annotations: map[string]string{
				constants.ModelInitInjectionKey:        "true",
				constants.FineTunedAdapterInjectionKey: "amaaaaaask7dceya3ro4ls2wit3tu5dkk2u2ijvbbu4gmhbrsjeytwc2yagq",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmptyModelDirVolumeRequired(tt.annotations)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestIsOriginalModelVolumeMountNecessary(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    true,
		},
		{
			name: "model init injection true",
			annotations: map[string]string{
				constants.ModelInitInjectionKey: "true",
			},
			expected: false,
		},
		{
			name: "ft serving with merged weights true",
			annotations: map[string]string{
				constants.FTServingWithMergedWeightsAnnotationKey: "true",
			},
			expected: false,
		},
		{
			name: "both annotations true",
			annotations: map[string]string{
				constants.ModelInitInjectionKey:                   "true",
				constants.FTServingWithMergedWeightsAnnotationKey: "true",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOriginalModelVolumeMountNecessary(tt.annotations)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestIsCohereCommand1TFewFTServing(t *testing.T) {
	tests := []struct {
		name       string
		objectMeta *metav1.ObjectMeta
		expected   bool
	}{
		{
			name: "cohere command 1 TFew FT serving",
			objectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.BaseModelVendorAnnotationKey: string(constants.Cohere),
					constants.FineTunedWeightFTStrategyKey: string(constants.TFewTrainingStrategy),
				},
			},
			expected: true,
		},
		{
			name: "all conditions not met - Llama LoRA FT Serving",
			objectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.BaseModelVendorAnnotationKey:            string(constants.Meta),
					constants.FineTunedWeightFTStrategyKey:            string(constants.LoraTrainingStrategy),
					constants.FTServingWithMergedWeightsAnnotationKey: "true",
				},
			},
			expected: false,
		},
		{
			name: "not matched strategy plus merged weights - Cohere Command R LoRA FT Serving",
			objectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.BaseModelVendorAnnotationKey:            string(constants.Cohere),
					constants.FineTunedWeightFTStrategyKey:            string(constants.LoraTrainingStrategy),
					constants.FTServingWithMergedWeightsAnnotationKey: "true",
				},
			},
			expected: false,
		},
		{
			name: "Cohere Command R TFew FT Serving",
			objectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.BaseModelVendorAnnotationKey:            string(constants.Cohere),
					constants.FineTunedWeightFTStrategyKey:            string(constants.TFewTrainingStrategy),
					constants.FTServingWithMergedWeightsAnnotationKey: "true",
				},
			},
			expected: false,
		},
		{
			name: "missing FT strategy annotation",
			objectMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.BaseModelVendorAnnotationKey:            string(constants.Cohere),
					constants.FTServingWithMergedWeightsAnnotationKey: "false",
				},
			},
			expected: false,
		},
		{
			name:       "empty annotations",
			objectMeta: &metav1.ObjectMeta{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCohereCommand1TFewFTServing(tt.objectMeta)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestUpdateVolumeMount(t *testing.T) {
	tests := []struct {
		name           string
		container      *v1.Container
		volumeMount    *v1.VolumeMount
		expectedMounts []v1.VolumeMount
	}{
		{
			name: "update existing volume mount",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "test-volume",
						MountPath: "/old/path",
						SubPath:   "old-sub-path",
						ReadOnly:  false,
					},
				},
			},
			volumeMount: &v1.VolumeMount{
				Name:      "test-volume",
				MountPath: "/new/path",
				SubPath:   "new-sub-path",
				ReadOnly:  true,
			},
			expectedMounts: []v1.VolumeMount{
				{
					Name:      "test-volume",
					MountPath: "/new/path",
					SubPath:   "new-sub-path",
					ReadOnly:  true,
				},
			},
		},
		{
			name: "add new volume mount",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "existing-volume",
						MountPath: "/existing/path",
					},
				},
			},
			volumeMount: &v1.VolumeMount{
				Name:      "new-volume",
				MountPath: "/new/path",
				SubPath:   "new-sub-path",
				ReadOnly:  true,
			},
			expectedMounts: []v1.VolumeMount{
				{
					Name:      "existing-volume",
					MountPath: "/existing/path",
				},
				{
					Name:      "new-volume",
					MountPath: "/new/path",
					SubPath:   "new-sub-path",
					ReadOnly:  true,
				},
			},
		},
		{
			name: "nil volume mount",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "existing-volume",
						MountPath: "/existing/path",
					},
				},
			},
			volumeMount: nil,
			expectedMounts: []v1.VolumeMount{
				{
					Name:      "existing-volume",
					MountPath: "/existing/path",
				},
			},
		},
		{
			name: "update one of multiple volume mounts",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "volume-1",
						MountPath: "/path/1",
					},
					{
						Name:      "volume-2",
						MountPath: "/old/path",
						SubPath:   "old-sub-path",
					},
					{
						Name:      "volume-3",
						MountPath: "/path/3",
					},
				},
			},
			volumeMount: &v1.VolumeMount{
				Name:      "volume-2",
				MountPath: "/new/path",
				SubPath:   "new-sub-path",
				ReadOnly:  true,
			},
			expectedMounts: []v1.VolumeMount{
				{
					Name:      "volume-1",
					MountPath: "/path/1",
				},
				{
					Name:      "volume-2",
					MountPath: "/new/path",
					SubPath:   "new-sub-path",
					ReadOnly:  true,
				},
				{
					Name:      "volume-3",
					MountPath: "/path/3",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateVolumeMount(tt.container, tt.volumeMount)
			assert.Equal(t, tt.expectedMounts, tt.container.VolumeMounts, "Test case: %s", tt.name)
		})
	}
}

func TestUpdateEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		container   *v1.Container
		envVar      *v1.EnvVar
		expectedEnv []v1.EnvVar
	}{
		{
			name: "update existing env var",
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name:  "TEST_VAR",
						Value: "old-value",
					},
				},
			},
			envVar: &v1.EnvVar{
				Name:  "TEST_VAR",
				Value: "new-value",
			},
			expectedEnv: []v1.EnvVar{
				{
					Name:  "TEST_VAR",
					Value: "new-value",
				},
			},
		},
		{
			name: "add new env var",
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name:  "EXISTING_VAR",
						Value: "existing-value",
					},
				},
			},
			envVar: &v1.EnvVar{
				Name:  "NEW_VAR",
				Value: "new-value",
			},
			expectedEnv: []v1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "NEW_VAR",
					Value: "new-value",
				},
			},
		},
		{
			name: "update one of multiple env vars",
			container: &v1.Container{
				Env: []v1.EnvVar{
					{
						Name:  "VAR1",
						Value: "value1",
					},
					{
						Name:  "VAR2",
						Value: "old-value2",
					},
					{
						Name:  "VAR3",
						Value: "value3",
					},
				},
			},
			envVar: &v1.EnvVar{
				Name:  "VAR2",
				Value: "new-value2",
			},
			expectedEnv: []v1.EnvVar{
				{
					Name:  "VAR1",
					Value: "value1",
				},
				{
					Name:  "VAR2",
					Value: "new-value2",
				},
				{
					Name:  "VAR3",
					Value: "value3",
				},
			},
		},
		{
			name:      "empty container env",
			container: &v1.Container{},
			envVar: &v1.EnvVar{
				Name:  "NEW_VAR",
				Value: "new-value",
			},
			expectedEnv: []v1.EnvVar{
				{
					Name:  "NEW_VAR",
					Value: "new-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateEnvVars(tt.container, tt.envVar)
			assert.Equal(t, tt.expectedEnv, tt.container.Env, "Test case: %s", tt.name)
		})
	}
}

// Helper function to marshal JSON and panic on error (for test data setup only)
func marshalJSONHelper(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func TestMergeRouterSpec(t *testing.T) {
	// Create a sample RouterSpec for use in tests
	isvcRouter := &v1beta1.RouterSpec{
		PodSpec: v1beta1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "isvc-container",
					Image: "isvc-image",
				},
			},
		},
	}

	runtimeRouter := &v1beta1.RouterSpec{
		PodSpec: v1beta1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "runtime-container",
					Image: "runtime-image",
				},
			},
		},
	}

	tests := []struct {
		name          string
		isvcRouter    *v1beta1.RouterSpec
		runtimeRouter *v1beta1.RouterSpec
		expected      *v1beta1.RouterSpec
		expectError   bool
	}{
		{
			name:          "isvc router is nil",
			isvcRouter:    nil,
			runtimeRouter: runtimeRouter,
			expected:      nil,
			expectError:   false,
		},
		{
			name:          "runtime router is nil",
			isvcRouter:    isvcRouter,
			runtimeRouter: nil,
			expected:      isvcRouter,
			expectError:   false,
		},
		{
			name:          "both routers are nil",
			isvcRouter:    nil,
			runtimeRouter: nil,
			expected:      nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := MergeRouterSpec(tt.isvcRouter, tt.runtimeRouter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, merged)
			}
		})
	}
}

func TestMergeEngineSpec(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name           string
		runtimeEngine  *v1beta1.EngineSpec
		isvcEngine     *v1beta1.EngineSpec
		expectedEngine *v1beta1.EngineSpec
		expectError    bool
	}{
		{
			name:           "both nil",
			runtimeEngine:  nil,
			isvcEngine:     nil,
			expectedEngine: nil,
			expectError:    false,
		},
		{
			name:          "runtime nil, isvc not nil",
			runtimeEngine: nil,
			isvcEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "engine:latest",
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "engine:latest",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "runtime not nil, isvc nil",
			runtimeEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime-engine:v1",
						},
					},
				},
			},
			isvcEngine:     nil,
			expectedEngine: nil,
			expectError:    false,
		},
		{
			name: "merge min/max replicas - isvc overrides",
			runtimeEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 10,
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 10,
				},
			},
			expectError: false,
		},
		{
			name: "merge containers - isvc overrides",
			runtimeEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime:v1",
							Env: []v1.EnvVar{
								{Name: "ENV1", Value: "runtime-value"},
								{Name: "ENV2", Value: "runtime-value2"},
							},
						},
						{
							Name:  "sidecar",
							Image: "sidecar:v1",
						},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "isvc:v2",
							Env: []v1.EnvVar{
								{Name: "ENV1", Value: "isvc-value"},
								{Name: "ENV3", Value: "isvc-value3"},
							},
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "isvc:v2",
							Env: []v1.EnvVar{
								{Name: "ENV1", Value: "isvc-value"},
								{Name: "ENV3", Value: "isvc-value3"},
								{Name: "ENV2", Value: "runtime-value2"},
							},
						},
						{
							Name:  "sidecar",
							Image: "sidecar:v1",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge runner spec",
			runtimeEngine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "runtime-runner",
						Image: "runtime-runner:v1",
						Args:  []string{"--arg1", "runtime"},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "isvc-runner",
						Image: "isvc-runner:v2",
						Args:  []string{"--arg1", "isvc", "--arg2", "new"},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "isvc-runner",
						Image: "isvc-runner:v2",
						Args:  []string{"--arg1", "isvc", "--arg2", "new"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge leader and worker specs",
			runtimeEngine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader",
								Image: "runtime-leader:v1",
							},
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(2),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "runtime-worker:v1",
							},
						},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader",
								Image: "isvc-leader:v2",
							},
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(4),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "isvc-worker:v2",
							},
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader",
								Image: "isvc-leader:v2",
							},
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(4),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "isvc-worker:v2",
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge pod spec fields - volumes, nodeSelector, tolerations",
			runtimeEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "model-volume",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/models",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"gpu":  "true",
						"zone": "us-west-1a",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "gpu",
							Operator: v1.TolerationOpEqual,
							Value:    "true",
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "cache-volume",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
					NodeSelector: map[string]string{
						"zone": "us-west-1b",
						"type": "inference",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "inference",
							Operator: v1.TolerationOpEqual,
							Value:    "true",
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "cache-volume",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "model-volume",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/models",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"gpu":  "true",
						"zone": "us-west-1b",
						"type": "inference",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      "inference",
							Operator: v1.TolerationOpEqual,
							Value:    "true",
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "complex merge scenario - partial overrides",
			runtimeEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 5,
					ScaleTarget: intPtr(50),
					ScaleMetric: (*v1beta1.ScaleMetric)(strPtr("concurrency")),
				},
				PodSpec: v1beta1.PodSpec{
					ServiceAccountName: "runtime-sa",
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime:v1",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("1"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "node-type",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"gpu"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MaxReplicas: 10,
					ScaleTarget: intPtr(80),
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "ome-container",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("2"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
									"nvidia.com/gpu":  resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 10,
					ScaleTarget: intPtr(80),
					ScaleMetric: (*v1beta1.ScaleMetric)(strPtr("concurrency")),
				},
				PodSpec: v1beta1.PodSpec{
					ServiceAccountName: "runtime-sa",
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime:v1",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("2"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
									"nvidia.com/gpu":  resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "node-type",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"gpu"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge with nil fields in runtime",
			runtimeEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MaxReplicas: 5,
				},
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "runner",
						Image: "runner:latest",
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 5,
				},
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "runner",
						Image: "runner:latest",
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge with empty containers in isvc overriding runtime",
			runtimeEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "runtime-container",
							Image: "runtime:v1",
						},
					},
				},
			},
			isvcEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "runtime-container",
							Image: "runtime:v1",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MergeEngineSpec(tt.runtimeEngine, tt.isvcEngine)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedEngine, result)
			}
		})
	}
}

func TestMergeDecoderSpec(t *testing.T) {
	intPtr := func(i int) *int { return &i }
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name            string
		runtimeDecoder  *v1beta1.DecoderSpec
		isvcDecoder     *v1beta1.DecoderSpec
		expectedDecoder *v1beta1.DecoderSpec
		expectError     bool
	}{
		{
			name:            "nil inputs",
			runtimeDecoder:  nil,
			isvcDecoder:     nil,
			expectedDecoder: nil,
			expectError:     false,
		},
		{
			name: "isvc spec is nil, runtime spec is not nil",
			runtimeDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime:v1",
						},
					},
				},
			},
			isvcDecoder:     nil,
			expectedDecoder: nil,
			expectError:     false,
		},
		{
			name:           "runtime spec is nil, isvc spec is not nil",
			runtimeDecoder: nil,
			isvcDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "isvc:v1",
						},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "isvc:v1",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge with leader/worker specs for multi-node decoder",
			runtimeDecoder: &v1beta1.DecoderSpec{
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader",
								Image: "runtime-leader:v1",
							},
						},
						NodeSelector: map[string]string{
							"node-role": "leader",
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(2),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "runtime-worker:v1",
							},
						},
					},
				},
			},
			isvcDecoder: &v1beta1.DecoderSpec{
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(4),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "isvc-worker:v2",
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										"nvidia.com/gpu": resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				Leader: &v1beta1.LeaderSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "leader",
								Image: "runtime-leader:v1",
							},
						},
						NodeSelector: map[string]string{
							"node-role": "leader",
						},
					},
				},
				Worker: &v1beta1.WorkerSpec{
					Size: intPtr(4),
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "worker",
								Image: "isvc-worker:v2",
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										"nvidia.com/gpu": resource.MustParse("1"),
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "complex PD-disaggregated decoder merge",
			runtimeDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 8,
					ScaleTarget: intPtr(80),
					ScaleMetric: (*v1beta1.ScaleMetric)(strPtr("memory")),
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "pd-decoder:v1",
							Env: []v1.EnvVar{
								{Name: "KV_CACHE_SIZE", Value: "16GB"},
								{Name: "DECODE_BATCH_SIZE", Value: "32"},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "kv-cache",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{
									Medium: v1.StorageMediumMemory,
								},
							},
						},
					},
				},
			},
			isvcDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MaxReplicas: 16,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "ome-container",
							Env: []v1.EnvVar{
								{Name: "DECODE_BATCH_SIZE", Value: "64"},
								{Name: "MAX_TOKENS", Value: "2048"},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("2"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
									"nvidia.com/gpu":  resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 16,
					ScaleTarget: intPtr(80),
					ScaleMetric: (*v1beta1.ScaleMetric)(strPtr("memory")),
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "pd-decoder:v1",
							Env: []v1.EnvVar{
								{Name: "KV_CACHE_SIZE", Value: "16GB"},
								{Name: "DECODE_BATCH_SIZE", Value: "64"},
								{Name: "MAX_TOKENS", Value: "2048"},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("2"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
									"nvidia.com/gpu":  resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "kv-cache",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{
									Medium: v1.StorageMediumMemory,
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "merge runner spec with nested container fields",
			runtimeDecoder: &v1beta1.DecoderSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:    "decoder-runner",
						Image:   "runtime-decoder:v1",
						Command: []string{"/bin/decode"},
						Args:    []string{"--mode", "streaming"},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "models",
								MountPath: "/models",
							},
						},
					},
				},
			},
			isvcDecoder: &v1beta1.DecoderSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Args: []string{"--mode", "batch", "--batch-size", "64"},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "cache",
								MountPath: "/cache",
							},
							{
								Name:      "models",
								MountPath: "/models",
							},
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								"nvidia.com/gpu": resource.MustParse("1"),
							},
						},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Image:   "runtime-decoder:v1",
						Command: []string{"/bin/decode"},
						Args:    []string{"--mode", "batch", "--batch-size", "64"},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "cache",
								MountPath: "/cache",
							},
							{
								Name:      "models",
								MountPath: "/models",
							},
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								"nvidia.com/gpu": resource.MustParse("1"),
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MergeDecoderSpec(tt.runtimeDecoder, tt.isvcDecoder)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDecoder, result)
			}
		})
	}
}

func TestDetermineEngineDeploymentMode(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name         string
		engine       *v1beta1.EngineSpec
		expectedMode constants.DeploymentModeType
	}{
		{
			name:         "nil engine spec",
			engine:       nil,
			expectedMode: constants.RawDeployment,
		},
		{
			name: "multi-node with leader and worker",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "multi-node with only leader",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "multi-node with only worker",
			engine: &v1beta1.EngineSpec{
				Worker: &v1beta1.WorkerSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "serverless with min replicas 0",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			expectedMode: constants.Serverless,
		},
		{
			name: "raw deployment with min replicas > 0",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			expectedMode: constants.RawDeployment,
		},
		{
			name: "raw deployment with only runner",
			engine: &v1beta1.EngineSpec{
				Runner: &v1beta1.RunnerSpec{
					Container: v1.Container{
						Name:  "runner",
						Image: "runner:latest",
					},
				},
			},
			expectedMode: constants.RawDeployment,
		},
		{
			name:         "raw deployment with empty spec",
			engine:       &v1beta1.EngineSpec{},
			expectedMode: constants.RawDeployment,
		},
		{
			name: "multi-node takes precedence over serverless",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
				Leader: &v1beta1.LeaderSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "annotation with MultiNodeRayVLLM takes highest precedence",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{
						constants.DeploymentMode: string(constants.MultiNodeRayVLLM),
					},
					MinReplicas: intPtr(1),
				},
				Leader: &v1beta1.LeaderSpec{},
			},
			expectedMode: constants.MultiNodeRayVLLM,
		},
		{
			name: "annotation with MultiNode takes highest precedence",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{
						constants.DeploymentMode: string(constants.MultiNode),
					},
					MinReplicas: intPtr(0),
				},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "invalid annotation is ignored, falls back to leader check",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{
						constants.DeploymentMode: "InvalidMode",
					},
					MinReplicas: intPtr(1),
				},
				Leader: &v1beta1.LeaderSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "empty annotations map, falls back to leader check",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{},
					MinReplicas: intPtr(1),
				},
				Leader: &v1beta1.LeaderSpec{},
			},
			expectedMode: constants.MultiNode,
		},
		{
			name: "annotation overrides leader and min replicas 0",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					Annotations: map[string]string{
						constants.DeploymentMode: string(constants.RawDeployment),
					},
					MinReplicas: intPtr(0),
				},
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			expectedMode: constants.RawDeployment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineEngineDeploymentMode(tt.engine)
			assert.Equal(t, tt.expectedMode, result)
		})
	}
}

func TestDetermineEntrypointComponent(t *testing.T) {
	tests := []struct {
		name               string
		isvc               *v1beta1.InferenceService
		expectedEntrypoint v1beta1.ComponentType
	}{
		{
			name: "engine only",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			expectedEntrypoint: v1beta1.EngineComponent,
		},
		{
			name: "engine + router - router takes precedence",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
					Router: &v1beta1.RouterSpec{},
				},
			},
			expectedEntrypoint: v1beta1.RouterComponent,
		},
		{
			name: "all components - router takes precedence",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine:  &v1beta1.EngineSpec{},
					Decoder: &v1beta1.DecoderSpec{},
					Router:  &v1beta1.RouterSpec{},
				},
			},
			expectedEntrypoint: v1beta1.RouterComponent,
		},
		{
			name: "engine + decoder - engine is entrypoint (no router)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine:  &v1beta1.EngineSpec{},
					Decoder: &v1beta1.DecoderSpec{},
				},
			},
			expectedEntrypoint: v1beta1.EngineComponent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entrypoint := DetermineEntrypointComponent(tt.isvc)
			assert.Equal(t, tt.expectedEntrypoint, entrypoint)
		})
	}
}

func TestGetTargetServicePort(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		services     []v1.Service
		expectedPort int32
		expectError  bool
	}{
		{
			name: "raw deployment mode - engine only with custom port",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			services: []v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-engine",
						Namespace: "default",
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{Port: 8081},
						},
					},
				},
			},
			expectedPort: 8081,
			expectError:  false,
		},
		{
			name: "raw deployment mode - with router and custom port",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
					Router: &v1beta1.RouterSpec{},
				},
			},
			services: []v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-router",
						Namespace: "default",
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{Port: 8082},
						},
					},
				},
			},
			expectedPort: 8082,
			expectError:  false,
		},
		{
			name: "raw deployment mode - service not found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			services:     []v1.Service{},
			expectedPort: 0,
			expectError:  true,
		},
		{
			name: "raw deployment mode - service with no ports uses default",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			services: []v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-engine",
						Namespace: "default",
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{},
					},
				},
			},
			expectedPort: constants.CommonISVCPort,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build fake client with services
			objs := make([]runtime.Object, 0, len(tt.services))
			for i := range tt.services {
				objs = append(objs, &tt.services[i])
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			port, err := GetTargetServicePort(context.Background(), fakeClient, tt.isvc)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPort, port)
			}
		})
	}
}

func TestGetTargetServicePort_ServiceNameResolution(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	// Test that the correct service names are used based on mode and router presence
	tests := []struct {
		name                string
		isvc                *v1beta1.InferenceService
		expectedServiceName string
	}{
		{
			name: "raw mode - engine only uses EngineServiceName",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-model",
					Namespace: "test-ns",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
				},
			},
			expectedServiceName: constants.EngineServiceName("my-model"), // my-model-engine
		},
		{
			name: "raw mode - with router uses RouterServiceName",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-model",
					Namespace: "test-ns",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{},
					Router: &v1beta1.RouterSpec{},
				},
			},
			expectedServiceName: constants.RouterServiceName("my-model"), // my-model-router
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service with the expected name
			svc := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.expectedServiceName,
					Namespace: tt.isvc.Namespace,
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Port: 9999}, // Use distinct port to verify correct service was found
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(svc).
				Build()

			port, err := GetTargetServicePort(context.Background(), fakeClient, tt.isvc)

			assert.NoError(t, err)
			assert.Equal(t, int32(9999), port, "Should find the service with expected name: %s", tt.expectedServiceName)
		})
	}
}

func TestAddPreferredNodeAffinityForModel(t *testing.T) {
	tests := []struct {
		name          string
		podSpec       *v1.PodSpec
		baseModelMeta *metav1.ObjectMeta
		wantAffinity  bool
		wantLabelKey  string
	}{
		{
			name:    "ClusterBaseModel - adds node affinity",
			podSpec: &v1.PodSpec{},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "test-cluster-model",
				Namespace: "", // Empty namespace indicates ClusterBaseModel
			},
			wantAffinity: true,
			wantLabelKey: "models.ome.io/clusterbasemodel.test-cluster-model",
		},
		{
			name:    "BaseModel (namespace-scoped) - adds node affinity",
			podSpec: &v1.PodSpec{},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "test-model",
				Namespace: "default",
			},
			wantAffinity: true,
			wantLabelKey: "models.ome.io/default.basemodel.test-model",
		},
		{
			name:          "nil podSpec - no panic",
			podSpec:       nil,
			baseModelMeta: &metav1.ObjectMeta{Name: "test-model", Namespace: "default"},
			wantAffinity:  false,
		},
		{
			name:          "nil baseModelMeta - no panic",
			podSpec:       &v1.PodSpec{},
			baseModelMeta: nil,
			wantAffinity:  false,
		},
		{
			name: "existing affinity - appends without duplicating",
			podSpec: &v1.PodSpec{
				Affinity: &v1.Affinity{
					NodeAffinity: &v1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
							{
								Weight: 50,
								Preference: v1.NodeSelectorTerm{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      "existing-label",
											Operator: v1.NodeSelectorOpIn,
											Values:   []string{"value"},
										},
									},
								},
							},
						},
					},
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "test-model",
				Namespace: "default",
			},
			wantAffinity: true,
			wantLabelKey: "models.ome.io/default.basemodel.test-model",
		},
		{
			name: "duplicate affinity check - does not add duplicate",
			podSpec: &v1.PodSpec{
				Affinity: &v1.Affinity{
					NodeAffinity: &v1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
							{
								Weight: 100,
								Preference: v1.NodeSelectorTerm{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      "models.ome.io/default.basemodel.test-model",
											Operator: v1.NodeSelectorOpIn,
											Values:   []string{"Ready"},
										},
									},
								},
							},
						},
					},
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "test-model",
				Namespace: "default",
			},
			wantAffinity: true,
			wantLabelKey: "models.ome.io/default.basemodel.test-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialTermCount := 0
			if tt.podSpec != nil && tt.podSpec.Affinity != nil &&
				tt.podSpec.Affinity.NodeAffinity != nil {
				initialTermCount = len(tt.podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
			}

			AddPreferredNodeAffinityForModel(tt.podSpec, tt.baseModelMeta)

			if !tt.wantAffinity {
				// For nil cases, just verify no panic occurred
				return
			}

			assert.NotNil(t, tt.podSpec.Affinity, "Affinity should not be nil")
			assert.NotNil(t, tt.podSpec.Affinity.NodeAffinity, "NodeAffinity should not be nil")

			preferredTerms := tt.podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			assert.NotEmpty(t, preferredTerms, "PreferredDuringSchedulingIgnoredDuringExecution should not be empty")

			// Check that the expected label key exists in one of the terms
			found := false
			for _, term := range preferredTerms {
				for _, expr := range term.Preference.MatchExpressions {
					if expr.Key == tt.wantLabelKey {
						found = true
						assert.Equal(t, v1.NodeSelectorOpIn, expr.Operator)
						assert.Contains(t, expr.Values, "Ready")
						assert.Equal(t, int32(100), term.Weight)
						break
					}
				}
			}
			assert.True(t, found, "Expected label key %s not found in affinity terms", tt.wantLabelKey)

			// For the duplicate test case, verify no additional term was added
			if tt.name == "duplicate affinity check - does not add duplicate" {
				assert.Equal(t, initialTermCount, len(preferredTerms),
					"Should not add duplicate affinity term")
			}

			// For the existing affinity case, verify the existing term is preserved
			if tt.name == "existing affinity - appends without duplicating" {
				assert.Equal(t, initialTermCount+1, len(preferredTerms),
					"Should append new affinity term")
				// Check the existing term is still there
				existingFound := false
				for _, term := range preferredTerms {
					for _, expr := range term.Preference.MatchExpressions {
						if expr.Key == "existing-label" {
							existingFound = true
							break
						}
					}
				}
				assert.True(t, existingFound, "Existing affinity term should be preserved")
			}
		})
	}
}
