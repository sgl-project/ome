package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGetGpuCountFromContainer(t *testing.T) {
	multiVendor := []string{"nvidia.com/gpu", "amd.com/gpu", "google.com/tpu"}

	tests := []struct {
		name                 string
		container            *v1.Container
		acceleratorResources []string
		expected             int
	}{
		{
			name:                 "nil container",
			container:            nil,
			acceleratorResources: multiVendor,
			expected:             0,
		},
		{
			name: "nvidia limits with configured list",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             2,
		},
		{
			name: "amd limits with configured list",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("amd.com/gpu"): resource.MustParse("4"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             4,
		},
		{
			name: "tpu limits with configured list",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("google.com/tpu"): resource.MustParse("8"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             8,
		},
		{
			name: "amd requests only, no limits, with configured list",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName("amd.com/gpu"): resource.MustParse("1"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             1,
		},
		{
			name: "limits preferred over requests for the same name",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("nvidia.com/gpu"): resource.MustParse("3"),
					},
					Requests: v1.ResourceList{
						v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             3,
		},
		{
			name: "empty limits and requests with configured list",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("4"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             0,
		},
		{
			name: "absent config preserves nvidia-only behavior",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
					},
				},
			},
			acceleratorResources: nil,
			expected:             1,
		},
		{
			name: "absent config does not recognize amd",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("amd.com/gpu"): resource.MustParse("1"),
					},
				},
			},
			acceleratorResources: nil,
			expected:             0,
		},
		{
			name: "empty configured list also falls back to nvidia-only",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("nvidia.com/gpu"): resource.MustParse("6"),
					},
				},
			},
			acceleratorResources: []string{},
			expected:             6,
		},
		{
			name: "first configured name wins when an earlier name is absent",
			container: &v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceName("google.com/tpu"): resource.MustParse("2"),
					},
				},
			},
			acceleratorResources: multiVendor,
			expected:             2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetGpuCountFromContainer(tt.container, tt.acceleratorResources)
			assert.Equal(t, tt.expected, result)
		})
	}
}
