package utils

import (
	"encoding/json"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
