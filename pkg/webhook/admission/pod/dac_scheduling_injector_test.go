package pod

import (
	"context"
	"testing"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckAcSupport(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		expected    bool
	}{
		// Supported profiles
		{
			name:        "a10-x1 should be supported",
			profileName: "a10-x1",
			expected:    true,
		},
		{
			name:        "a10-x2 should be supported",
			profileName: "a10-x2",
			expected:    true,
		},
		{
			name:        "a10-x4 should be supported",
			profileName: "a10-x4",
			expected:    true,
		},
		{
			name:        "a10-x8 should be supported",
			profileName: "a10-x8",
			expected:    true,
		},
		{
			name:        "a100-40g-x1 should be supported",
			profileName: "a100-40g-x1",
			expected:    true,
		},
		{
			name:        "a100-40g-x2 should be supported",
			profileName: "a100-40g-x2",
			expected:    true,
		},
		{
			name:        "a100-80g-x4 should be supported",
			profileName: "a100-80g-x4",
			expected:    true,
		},
		{
			name:        "h100-x1 should be supported",
			profileName: "h100-x1",
			expected:    true,
		},
		{
			name:        "h100-x8 should be supported",
			profileName: "h100-x8",
			expected:    true,
		},
		{
			name:        "h200-x2 should be supported",
			profileName: "h200-x2",
			expected:    true,
		},
		// Unsupported profiles
		{
			name:        "empty profile should not be supported",
			profileName: "",
			expected:    false,
		},
		{
			name:        "a10-x3 should not be supported (invalid count)",
			profileName: "a10-x3",
			expected:    false,
		},
		{
			name:        "a10-x16 should not be supported (invalid count)",
			profileName: "a10-x16",
			expected:    false,
		},
		{
			name:        "v100-x1 should not be supported (unsupported GPU)",
			profileName: "v100-x1",
			expected:    false,
		},
		{
			name:        "A10-x1 should not be supported (case sensitive)",
			profileName: "A10-x1",
			expected:    false,
		},
		{
			name:        "H100-x1 should not be supported (case sensitive)",
			profileName: "H100-x1",
			expected:    false,
		},
		{
			name:        "a100-x1 should not be supported (missing memory spec)",
			profileName: "a100-x1",
			expected:    false,
		},
		{
			name:        "a100-20g-x1 should not be supported (invalid memory)",
			profileName: "a100-20g-x1",
			expected:    false,
		},
		{
			name:        "random-profile should not be supported",
			profileName: "random-profile",
			expected:    false,
		},
		{
			name:        "h100 without count should not be supported",
			profileName: "h100",
			expected:    false,
		},
		{
			name:        "prefix-a10-x1 should not be supported",
			profileName: "prefix-a10-x1",
			expected:    false,
		},
		{
			name:        "a10-x1-suffix should not be supported",
			profileName: "a10-x1-suffix",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAcSupport(tt.profileName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDedicatedAIClusterSchedulingInjector_InjectAffinity_SkipsWhenACSupported(t *testing.T) {
	// Create a DAC with an AC-supported profile
	dac := &omev1beta1.DedicatedAICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dac",
		},
		Spec: omev1beta1.DedicatedAIClusterSpec{
			Profile: "h100-x1",
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "gpu-type",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"h100"},
									},
								},
							},
						},
					},
				},
			},
			Tolerations: []v1.Toleration{
				{
					Key:      "gpu",
					Operator: v1.TolerationOpEqual,
					Value:    "h100",
					Effect:   v1.TaintEffectNoSchedule,
				},
			},
		},
	}

	// Create a DedicatedAIClusterProfile with AC-supported name
	profile := &omev1beta1.DedicatedAIClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "h100-x1",
		},
		Spec: omev1beta1.DedicatedAIClusterProfileSpec{
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "profile-gpu-type",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"h100"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the resources
	err := c.Create(context.TODO(), dac)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), dac)
	}()

	err = c.Create(context.TODO(), profile)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), profile)
	}()

	// Create a pod with DAC annotation
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				constants.DedicatedAICluster: "test-dac",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	// Create the injector and inject affinity
	injector := NewDedicatedAIClusterSchedulingInjector(c)
	err = injector.InjectAffinity(pod)
	require.NoError(t, err)

	// Verify that the pod's affinity was NOT modified (skipped due to AC support)
	assert.Nil(t, pod.Spec.Affinity, "Pod affinity should be nil when AC is supported")
	assert.Nil(t, pod.Spec.Tolerations, "Pod tolerations should be nil when AC is supported")
}

func TestDedicatedAIClusterSchedulingInjector_InjectAffinity_InjectsWhenACNotSupported(t *testing.T) {
	// Create a DAC with a non-AC-supported profile
	dac := &omev1beta1.DedicatedAICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dac-no-ac",
		},
		Spec: omev1beta1.DedicatedAIClusterSpec{
			Profile: "custom-profile",
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "gpu-type",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"custom"},
									},
								},
							},
						},
					},
				},
			},
			Tolerations: []v1.Toleration{
				{
					Key:      "gpu",
					Operator: v1.TolerationOpEqual,
					Value:    "custom",
					Effect:   v1.TaintEffectNoSchedule,
				},
			},
		},
	}

	// Create a DedicatedAIClusterProfile with non-AC-supported name
	profileNoAC := &omev1beta1.DedicatedAIClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-profile",
		},
		Spec: omev1beta1.DedicatedAIClusterProfileSpec{
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "profile-gpu-type",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"custom"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the resources
	err := c.Create(context.TODO(), dac)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), dac)
	}()

	err = c.Create(context.TODO(), profileNoAC)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), profileNoAC)
	}()

	// Create a pod with DAC annotation
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-no-ac",
			Namespace: "default",
			Annotations: map[string]string{
				constants.DedicatedAICluster: "test-dac-no-ac",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	// Create the injector and inject affinity
	injector := NewDedicatedAIClusterSchedulingInjector(c)
	err = injector.InjectAffinity(pod)
	require.NoError(t, err)

	// Verify that the pod's affinity WAS modified (injected because AC is not supported)
	assert.NotNil(t, pod.Spec.Affinity, "Pod affinity should be set when AC is not supported")
	assert.NotNil(t, pod.Spec.Tolerations, "Pod tolerations should be set when AC is not supported")
}

func TestDedicatedAIClusterSchedulingInjector_InjectAffinity_NoDACAnnotation(t *testing.T) {
	// Create a pod without DAC annotation
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-no-annotation",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	// Create the injector and inject affinity
	injector := NewDedicatedAIClusterSchedulingInjector(c)
	err := injector.InjectAffinity(pod)
	require.NoError(t, err)

	// Verify that the pod was not modified
	assert.Nil(t, pod.Spec.Affinity, "Pod affinity should remain nil when no DAC annotation")
	assert.Nil(t, pod.Spec.Tolerations, "Pod tolerations should remain nil when no DAC annotation")
}

func TestDedicatedAIClusterSchedulingInjector_InjectAffinity_InjectsGPUResources(t *testing.T) {
	gpuResourceName := v1.ResourceName("nvidia.com/gpu")
	gpuQuantity := resource.MustParse("2")
	cpuQuantity := resource.MustParse("8")

	// Create a DAC with Resources including GPU
	dac := &omev1beta1.DedicatedAICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dac-resources",
		},
		Spec: omev1beta1.DedicatedAIClusterSpec{
			Profile: "custom-profile-resources",
			Resources: &v1.ResourceRequirements{
				Limits: v1.ResourceList{
					gpuResourceName: gpuQuantity,
					v1.ResourceCPU:  cpuQuantity,
				},
				Requests: v1.ResourceList{
					gpuResourceName: gpuQuantity,
					v1.ResourceCPU:  cpuQuantity,
				},
			},
		},
	}

	// Create a DedicatedAIClusterProfile with non-AC-supported name
	profile := &omev1beta1.DedicatedAIClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-profile-resources",
		},
		Spec: omev1beta1.DedicatedAIClusterProfileSpec{},
	}

	// Create the resources
	err := c.Create(context.TODO(), dac)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), dac)
	}()

	err = c.Create(context.TODO(), profile)
	require.NoError(t, err)
	defer func() {
		_ = c.Delete(context.TODO(), profile)
	}()

	// Create a pod with DAC annotation and ome-container
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-resources",
			Namespace: "default",
			Annotations: map[string]string{
				constants.DedicatedAICluster: "test-dac-resources",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  constants.MainContainerName,
					Image: "test-image",
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							gpuResourceName: resource.MustParse("1"),
							v1.ResourceCPU:  resource.MustParse("4"),
						},
						Requests: v1.ResourceList{
							gpuResourceName: resource.MustParse("1"),
							v1.ResourceCPU:  resource.MustParse("4"),
						},
					},
				},
			},
		},
	}

	// Create the injector and inject affinity
	injector := NewDedicatedAIClusterSchedulingInjector(c)
	err = injector.InjectAffinity(pod)
	require.NoError(t, err)

	// Verify that only GPU resources were overridden
	container := pod.Spec.Containers[0]
	gpuLimits := container.Resources.Limits[gpuResourceName]
	gpuRequests := container.Resources.Requests[gpuResourceName]
	cpuLimits := container.Resources.Limits[v1.ResourceCPU]
	cpuRequests := container.Resources.Requests[v1.ResourceCPU]
	assert.Equal(t, gpuQuantity.String(), gpuLimits.String(), "GPU limits should be overridden")
	assert.Equal(t, gpuQuantity.String(), gpuRequests.String(), "GPU requests should be overridden")
	// CPU should remain unchanged
	assert.Equal(t, "4", cpuLimits.String(), "CPU limits should NOT be overridden")
	assert.Equal(t, "4", cpuRequests.String(), "CPU requests should NOT be overridden")
}
