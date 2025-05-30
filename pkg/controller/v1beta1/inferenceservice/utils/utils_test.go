package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			isvcEngine: nil,
			expectedEngine: &v1beta1.EngineSpec{
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
			expectError: false,
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
			name:            "both nil",
			runtimeDecoder:  nil,
			isvcDecoder:     nil,
			expectedDecoder: nil,
			expectError:     false,
		},
		{
			name:           "runtime nil, isvc not nil",
			runtimeDecoder: nil,
			isvcDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "decoder:latest",
						},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 5,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "decoder:latest",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "runtime not nil, isvc nil",
			runtimeDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime-decoder:v1",
						},
					},
				},
			},
			isvcDecoder: nil,
			expectedDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "ome-container",
							Image: "runtime-decoder:v1",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineEngineDeploymentMode(tt.engine)
			assert.Equal(t, tt.expectedMode, result)
		})
	}
}

func TestReconcileBaseModel(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name          string
		isvc          *v1beta1.InferenceService
		setupClient   func() client.Client
		expectedModel *v1beta1.BaseModelSpec
		expectedMeta  *metav1.ObjectMeta
		expectError   bool
		errorContains string
	}{
		{
			name: "successful reconciliation with namespace model",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
				},
			},
			setupClient: func() client.Client {
				mockClient := &MockClient{
					getFunc: func(key client.ObjectKey, obj client.Object) error {
						if key.Name == "test-model" && key.Namespace == "test-namespace" {
							if baseModel, ok := obj.(*v1beta1.BaseModel); ok {
								baseModel.ObjectMeta = metav1.ObjectMeta{
									Name:      "test-model",
									Namespace: "test-namespace",
								}
								baseModel.Spec = v1beta1.BaseModelSpec{
									ModelFormat: v1beta1.ModelFormat{
										Name: "pytorch",
									},
									ModelExtensionSpec: v1beta1.ModelExtensionSpec{
										Vendor: strPtr("meta"),
									},
									ModelParameterSize: strPtr("7B"),
									Storage: &v1beta1.StorageSpec{
										Path: strPtr("/models/test"),
									},
								}
								return nil
							}
						}
						return fmt.Errorf("not found")
					},
				}
				return mockClient
			},
			expectedModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelExtensionSpec: v1beta1.ModelExtensionSpec{
					Vendor: strPtr("meta"),
				},
				ModelParameterSize: strPtr("7B"),
				Storage: &v1beta1.StorageSpec{
					Path: strPtr("/models/test"),
				},
			},
			expectedMeta: &metav1.ObjectMeta{
				Name:      "test-model",
				Namespace: "test-namespace",
			},
			expectError: false,
		},
		{
			name: "model reference is nil",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: nil,
				},
			},
			setupClient:   func() client.Client { return &MockClient{} },
			expectedModel: nil,
			expectedMeta:  nil,
			expectError:   true,
			errorContains: "model reference is required",
		},
		{
			name: "model name is empty",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "",
					},
				},
			},
			setupClient:   func() client.Client { return &MockClient{} },
			expectedModel: nil,
			expectedMeta:  nil,
			expectError:   true,
			errorContains: "model reference is required",
		},
		{
			name: "disabled model",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "disabled-model",
					},
				},
			},
			setupClient: func() client.Client {
				mockClient := &MockClient{
					getFunc: func(key client.ObjectKey, obj client.Object) error {
						if key.Name == "disabled-model" {
							if baseModel, ok := obj.(*v1beta1.BaseModel); ok {
								baseModel.ObjectMeta = metav1.ObjectMeta{
									Name:      "disabled-model",
									Namespace: "test-namespace",
								}
								baseModel.Spec = v1beta1.BaseModelSpec{
									ModelExtensionSpec: v1beta1.ModelExtensionSpec{
										Disabled: boolPtr(true),
									},
								}
								return nil
							}
						}
						return fmt.Errorf("not found")
					},
				}
				return mockClient
			},
			expectedModel: nil,
			expectedMeta:  nil,
			expectError:   true,
			errorContains: "specified base model disabled-model is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := tt.setupClient()
			model, meta, err := ReconcileBaseModel(cl, tt.isvc)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedModel, model)
				assert.Equal(t, tt.expectedMeta, meta)
			}
		})
	}
}

func TestGetRuntimeForNewArchitecture(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		baseModel       *v1beta1.BaseModelSpec
		setupClient     func() client.Client
		expectedRuntime *v1beta1.ServingRuntimeSpec
		expectedName    string
		expectError     bool
		errorContains   string
	}{
		{
			name: "specified runtime found and supports model",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: &v1beta1.ServingRuntimeRef{
						Name: "test-runtime",
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("2"),
				},
				ModelParameterSize: strPtr("7B"),
			},
			setupClient: func() client.Client {
				return &MockClient{
					getFunc: func(key client.ObjectKey, obj client.Object) error {
						if key.Name == "test-runtime" && key.Namespace == "test-namespace" {
							if runtime, ok := obj.(*v1beta1.ServingRuntime); ok {
								runtime.Spec = v1beta1.ServingRuntimeSpec{
									SupportedModelFormats: []v1beta1.SupportedModelFormat{
										{
											Name: "pytorch",
											ModelFormat: &v1beta1.ModelFormat{
												Name:    "pytorch",
												Version: strPtr("2"),
											},
											Version:    strPtr("2"),
											AutoSelect: boolPtr(true),
										},
									},
									EngineConfig: &v1beta1.EngineSpec{
										PodSpec: v1beta1.PodSpec{
											Containers: []v1.Container{
												{Name: "ome-container", Image: "runtime:v1"},
											},
										},
									},
									ProtocolVersions: []constants.InferenceServiceProtocol{
										constants.OpenInferenceProtocolV2,
									},
								}
								return nil
							}
						}
						return fmt.Errorf("not found")
					},
				}
			},
			expectedRuntime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("2"),
						},
						Version:    strPtr("2"),
						AutoSelect: boolPtr(true),
					},
				},
				EngineConfig: &v1beta1.EngineSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{Name: "ome-container", Image: "runtime:v1"},
						},
					},
				},
				ProtocolVersions: []constants.InferenceServiceProtocol{
					constants.OpenInferenceProtocolV2,
				},
			},
			expectedName: "test-runtime",
			expectError:  false,
		},
		{
			name: "specified runtime does not support model",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: &v1beta1.ServingRuntimeRef{
						Name: "incompatible-runtime",
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "tensorflow",
				},
				ModelParameterSize: strPtr("7B"),
			},
			setupClient: func() client.Client {
				return &MockClient{
					getFunc: func(key client.ObjectKey, obj client.Object) error {
						if key.Name == "incompatible-runtime" {
							if runtime, ok := obj.(*v1beta1.ServingRuntime); ok {
								runtime.Spec = v1beta1.ServingRuntimeSpec{
									SupportedModelFormats: []v1beta1.SupportedModelFormat{
										{
											Name: "pytorch", // Different from model
											ModelFormat: &v1beta1.ModelFormat{
												Name: "pytorch",
											},
										},
									},
								}
								return nil
							}
						}
						return fmt.Errorf("not found")
					},
				}
			},
			expectedRuntime: nil,
			expectedName:    "",
			expectError:     true,
			errorContains:   "runtime incompatible-runtime does not support model test-model",
		},
		{
			name: "specified runtime is disabled",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: &v1beta1.ServingRuntimeRef{
						Name: "disabled-runtime",
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{},
			setupClient: func() client.Client {
				return &MockClient{
					getFunc: func(key client.ObjectKey, obj client.Object) error {
						if key.Name == "disabled-runtime" {
							if runtime, ok := obj.(*v1beta1.ServingRuntime); ok {
								runtime.Spec = v1beta1.ServingRuntimeSpec{
									Disabled: boolPtr(true),
								}
								return nil
							}
						}
						return fmt.Errorf("not found")
					},
				}
			},
			expectedRuntime: nil,
			expectedName:    "",
			expectError:     true,
			errorContains:   "specified runtime disabled-runtime is disabled",
		},
		{
			name: "auto-select runtime successfully",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: nil, // No runtime specified
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			setupClient: func() client.Client {
				return &MockClient{
					listFunc: func(list client.ObjectList, opts ...client.ListOption) error {
						// Mock runtime list for auto-selection
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "auto-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
										EngineConfig: &v1beta1.EngineSpec{
											PodSpec: v1beta1.PodSpec{
												Containers: []v1.Container{
													{Name: "ome-container", Image: "auto:v1"},
												},
											},
										},
										ProtocolVersions: []constants.InferenceServiceProtocol{
											constants.OpenInferenceProtocolV2,
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedRuntime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
						AutoSelect: boolPtr(true),
					},
				},
				EngineConfig: &v1beta1.EngineSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{Name: "ome-container", Image: "auto:v1"},
						},
					},
				},
				ProtocolVersions: []constants.InferenceServiceProtocol{
					constants.OpenInferenceProtocolV2,
				},
			},
			expectedName: "auto-runtime",
			expectError:  false,
		},
		{
			name: "auto-select runtime - no suitable runtime found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: nil,
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "custom-format",
				},
				ModelParameterSize: strPtr("7B"),
			},
			setupClient: func() client.Client {
				return &MockClient{
					listFunc: func(list client.ObjectList, opts ...client.ListOption) error {
						// Return empty runtime lists
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedRuntime: nil,
			expectedName:    "",
			expectError:     true,
			errorContains:   "no runtime found to support model test-model with format custom-format",
		},
		{
			name: "auto-select runtime - shows excluded runtime reasons",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "test-namespace",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "test-model",
					},
					Runtime: nil,
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "custom-format",
				},
				ModelParameterSize: strPtr("7B"),
			},
			setupClient: func() client.Client {
				return &MockClient{
					listFunc: func(list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "disabled-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										Disabled: boolPtr(true),
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "custom-format",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "custom-format",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "wrong-format-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedRuntime: nil,
			expectedName:    "",
			expectError:     true,
			errorContains:   "Excluded runtimes: disabled-runtime: runtime is disabled; wrong-format-runtime: runtime wrong-format-runtime does not support model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := tt.setupClient()
			runtime, name, err := GetRuntimeForNewArchitecture(cl, tt.isvc, tt.baseModel)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRuntime, runtime)
				assert.Equal(t, tt.expectedName, name)
			}
		})
	}
}

func TestMergeRuntimeSpecs(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		runtime         *v1beta1.ServingRuntimeSpec
		expectedEngine  *v1beta1.EngineSpec
		expectedDecoder *v1beta1.DecoderSpec
		expectError     bool
	}{
		{
			name: "merge with both engine and decoder",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MaxReplicas: 10,
						},
					},
					Decoder: &v1beta1.DecoderSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: intPtr(2),
						},
					},
				},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						MinReplicas: intPtr(1),
						MaxReplicas: 5,
					},
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{Name: "ome-container", Image: "engine:v1"},
						},
					},
				},
				DecoderConfig: &v1beta1.DecoderSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						MinReplicas: intPtr(1),
						MaxReplicas: 3,
					},
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{Name: "ome-container", Image: "decoder:v1"},
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
					MaxReplicas: 10,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{Name: "ome-container", Image: "engine:v1"},
					},
				},
			},
			expectedDecoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(2),
					MaxReplicas: 3,
				},
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{Name: "ome-container", Image: "decoder:v1"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "no runtime provided",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: intPtr(1),
						},
					},
				},
			},
			runtime: nil,
			expectedEngine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			expectedDecoder: nil,
			expectError:     false,
		},
		{
			name: "no isvc specs",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{},
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{
					PodSpec: v1beta1.PodSpec{
						Containers: []v1.Container{
							{Name: "ome-container", Image: "engine:v1"},
						},
					},
				},
			},
			expectedEngine: &v1beta1.EngineSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []v1.Container{
						{Name: "ome-container", Image: "engine:v1"},
					},
				},
			},
			expectedDecoder: nil,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, decoder, err := MergeRuntimeSpecs(tt.isvc, tt.runtime)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedEngine, engine)
				assert.Equal(t, tt.expectedDecoder, decoder)
			}
		})
	}
}

func TestConvertPodSpec(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Test cases
	tests := []struct {
		name        string
		input       *v1beta1.PodSpec
		expectError bool
		errorMsg    string
		validate    func(g *gomega.WithT, result *v1.PodSpec)
	}{
		{
			name:        "nil input",
			input:       nil,
			expectError: true,
			errorMsg:    "cannot convert nil PodSpec",
		},
		{
			name: "minimal valid PodSpec",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test-container",
						Image: "test:latest",
					},
				},
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.Containers[0].Name).To(gomega.Equal("test-container"))
				g.Expect(result.Containers[0].Image).To(gomega.Equal("test:latest"))
			},
		},
		{
			name: "full PodSpec with all fields",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "main-container",
						Image: "main:latest",
						Ports: []v1.ContainerPort{
							{Name: "http", ContainerPort: 8080},
						},
						Env: []v1.EnvVar{
							{Name: "ENV_VAR", Value: "value"},
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("500m"),
								v1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/config",
							},
						},
					},
				},
				InitContainers: []v1.Container{
					{
						Name:  "init-container",
						Image: "init:latest",
					},
				},
				Volumes: []v1.Volume{
					{
						Name: "config",
						VolumeSource: v1.VolumeSource{
							ConfigMap: &v1.ConfigMapVolumeSource{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "config-map",
								},
							},
						},
					},
				},
				ServiceAccountName: "custom-sa",
				NodeSelector: map[string]string{
					"node-type": "gpu",
				},
				Tolerations: []v1.Toleration{
					{
						Key:      "gpu",
						Operator: v1.TolerationOpEqual,
						Value:    "true",
						Effect:   v1.TaintEffectNoSchedule,
					},
				},
				Affinity: &v1.Affinity{
					NodeAffinity: &v1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
							NodeSelectorTerms: []v1.NodeSelectorTerm{
								{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      "gpu-type",
											Operator: v1.NodeSelectorOpIn,
											Values:   []string{"nvidia", "amd"},
										},
									},
								},
							},
						},
					},
				},
				RestartPolicy:     v1.RestartPolicyAlways,
				DNSPolicy:         v1.DNSClusterFirst,
				SchedulerName:     "custom-scheduler",
				PriorityClassName: "high-priority",
				ImagePullSecrets: []v1.LocalObjectReference{
					{Name: "docker-secret"},
				},
				HostNetwork: false,
				HostPID:     false,
				HostIPC:     false,
				SecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: &[]bool{true}[0],
					RunAsUser:    &[]int64{1000}[0],
					FSGroup:      &[]int64{2000}[0],
				},
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				// Validate containers
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.Containers[0].Name).To(gomega.Equal("main-container"))
				g.Expect(result.Containers[0].Ports).To(gomega.HaveLen(1))
				g.Expect(result.Containers[0].Env).To(gomega.HaveLen(1))
				g.Expect(result.Containers[0].Resources.Requests[v1.ResourceCPU]).To(gomega.Equal(resource.MustParse("100m")))
				g.Expect(result.Containers[0].VolumeMounts).To(gomega.HaveLen(1))

				// Validate init containers
				g.Expect(result.InitContainers).To(gomega.HaveLen(1))
				g.Expect(result.InitContainers[0].Name).To(gomega.Equal("init-container"))

				// Validate volumes
				g.Expect(result.Volumes).To(gomega.HaveLen(1))
				g.Expect(result.Volumes[0].Name).To(gomega.Equal("config"))

				// Validate other fields
				g.Expect(result.ServiceAccountName).To(gomega.Equal("custom-sa"))
				g.Expect(result.NodeSelector).To(gomega.HaveKeyWithValue("node-type", "gpu"))
				g.Expect(result.Tolerations).To(gomega.HaveLen(1))
				g.Expect(result.Affinity).ToNot(gomega.BeNil())
				g.Expect(result.RestartPolicy).To(gomega.Equal(v1.RestartPolicyAlways))
				g.Expect(result.DNSPolicy).To(gomega.Equal(v1.DNSClusterFirst))
				g.Expect(result.SchedulerName).To(gomega.Equal("custom-scheduler"))
				g.Expect(result.PriorityClassName).To(gomega.Equal("high-priority"))
				g.Expect(result.ImagePullSecrets).To(gomega.HaveLen(1))
				g.Expect(result.HostNetwork).To(gomega.BeFalse())
				g.Expect(result.SecurityContext).ToNot(gomega.BeNil())
				g.Expect(*result.SecurityContext.RunAsUser).To(gomega.Equal(int64(1000)))
			},
		},
		{
			name: "PodSpec with ephemeral containers",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "main",
						Image: "main:latest",
					},
				},
				EphemeralContainers: []v1.EphemeralContainer{
					{
						EphemeralContainerCommon: v1.EphemeralContainerCommon{
							Name:  "debug",
							Image: "debug:latest",
						},
					},
				},
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.EphemeralContainers).To(gomega.HaveLen(1))
				g.Expect(result.EphemeralContainers[0].Name).To(gomega.Equal("debug"))
			},
		},
		{
			name: "PodSpec with advanced scheduling features",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "app",
						Image: "app:latest",
					},
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "myapp",
							},
						},
					},
				},
				ReadinessGates: []v1.PodReadinessGate{
					{
						ConditionType: v1.PodConditionType("custom-condition"),
					},
				},
				RuntimeClassName:   &[]string{"nvidia"}[0],
				EnableServiceLinks: &[]bool{false}[0],
				PreemptionPolicy:   &[]v1.PreemptionPolicy{v1.PreemptLowerPriority}[0],
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.TopologySpreadConstraints).To(gomega.HaveLen(1))
				g.Expect(result.TopologySpreadConstraints[0].TopologyKey).To(gomega.Equal("kubernetes.io/hostname"))
				g.Expect(result.ReadinessGates).To(gomega.HaveLen(1))
				g.Expect(*result.RuntimeClassName).To(gomega.Equal("nvidia"))
				g.Expect(*result.EnableServiceLinks).To(gomega.BeFalse())
				g.Expect(*result.PreemptionPolicy).To(gomega.Equal(v1.PreemptLowerPriority))
			},
		},
		{
			name: "PodSpec with OS-specific fields",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "app",
						Image: "app:latest",
					},
				},
				OS: &v1.PodOS{
					Name: v1.Linux,
				},
				HostUsers:         &[]bool{false}[0],
				SetHostnameAsFQDN: &[]bool{true}[0],
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.OS).ToNot(gomega.BeNil())
				g.Expect(result.OS.Name).To(gomega.Equal(v1.Linux))
				g.Expect(*result.HostUsers).To(gomega.BeFalse())
				g.Expect(*result.SetHostnameAsFQDN).To(gomega.BeTrue())
			},
		},
		{
			name: "PodSpec with resource claims",
			input: &v1beta1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "app",
						Image: "app:latest",
					},
				},
				ResourceClaims: []v1.PodResourceClaim{
					{
						Name:              "gpu-claim",
						ResourceClaimName: &[]string{"shared-gpu"}[0],
					},
				},
			},
			expectError: false,
			validate: func(g *gomega.WithT, result *v1.PodSpec) {
				g.Expect(result.Containers).To(gomega.HaveLen(1))
				g.Expect(result.ResourceClaims).To(gomega.HaveLen(1))
				g.Expect(result.ResourceClaims[0].Name).To(gomega.Equal("gpu-claim"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertPodSpec(tt.input)

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.errorMsg))
				g.Expect(result).To(gomega.BeNil())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(result).ToNot(gomega.BeNil())
				if tt.validate != nil {
					tt.validate(g, result)
				}
			}
		})
	}
}
