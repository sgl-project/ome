package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"

	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func TestConvertResourceGroupsToFlavorUsage(t *testing.T) {
	resourceGroups := []kueuev1beta1.ResourceGroup{
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavor1",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         "cpu",
							NominalQuota: resource.MustParse("10"),
						},
						{
							Name:         "memory",
							NominalQuota: resource.MustParse("20Gi"),
						},
					},
				},
				{
					Name: "flavor2",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         "gpu",
							NominalQuota: resource.MustParse("2"),
						},
					},
				},
			},
		},
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavor1",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         "cpu",
							NominalQuota: resource.MustParse("5"),
						},
					},
				},
			},
		},
	}

	expected := []kueuev1beta1.FlavorUsage{
		{
			Name: "flavor1",
			Resources: []kueuev1beta1.ResourceUsage{
				{
					Name:  "cpu",
					Total: resource.MustParse("15"),
				},
				{
					Name:  "memory",
					Total: resource.MustParse("20Gi"),
				},
			},
		},
		{
			Name: "flavor2",
			Resources: []kueuev1beta1.ResourceUsage{
				{
					Name:  "gpu",
					Total: resource.MustParse("2"),
				},
			},
		},
	}

	result := ConvertResourceGroupsToFlavorUsage(resourceGroups)

	for i := range expected {
		if expected[i].Name != result[i].Name {
			t.Errorf("Expected flavor name %s, but got %s", expected[i].Name, result[i].Name)
		}
		for j := range expected[i].Resources {
			if !compareQuantities(t, map[corev1.ResourceName]resource.Quantity{expected[i].Resources[j].Name: expected[i].Resources[j].Total}, map[corev1.ResourceName]resource.Quantity{result[i].Resources[j].Name: result[i].Resources[j].Total}) {
				t.Errorf("Expected total for resource %s to be %v, but got %v",
					expected[i].Resources[j].Name, expected[i].Resources[j].Total, result[i].Resources[j].Total)
			}
		}
	}
}

func compareQuantities(t *testing.T, expected, actual map[corev1.ResourceName]resource.Quantity) bool {
	for resourceName, expectedQuantity := range expected {
		actualQuantity, exists := actual[resourceName]
		if !exists {
			t.Errorf("Resource %s not found in actual resources. Available resources: %v", resourceName, actual)
			return false
		}

		// Compare string representation
		stringMatch := expectedQuantity.String() == actualQuantity.String()
		// Compare zero status
		zeroMatch := expectedQuantity.IsZero() == actualQuantity.IsZero()
		// Compare format
		formatMatch := expectedQuantity.Format == actualQuantity.Format

		if !stringMatch || !zeroMatch || !formatMatch {
			t.Errorf("Expected total for resource %s to be %+v, but got %+v", resourceName, expectedQuantity, actualQuantity)
			return false
		}
	}

	return true
}

func TestIsResourceSufficient(t *testing.T) {
	tests := []struct {
		name                 string
		requestedResources   []kueuev1beta1.ResourceGroup
		availableResources   []kueuev1beta1.FlavorUsage
		expectedResult       bool
		expectedErrorMessage string
	}{
		{
			name: "Sufficient resources",
			requestedResources: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavorA",
							Resources: []kueuev1beta1.ResourceQuota{
								{
									Name:         corev1.ResourceCPU,
									NominalQuota: resource.MustParse("4"),
								},
							},
						},
					},
				},
			},
			availableResources: []kueuev1beta1.FlavorUsage{
				{
					Name: "flavorA",
					Resources: []kueuev1beta1.ResourceUsage{
						{
							Name:  corev1.ResourceCPU,
							Total: resource.MustParse("10"),
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "Insufficient resources",
			requestedResources: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavorA",
							Resources: []kueuev1beta1.ResourceQuota{
								{
									Name:         corev1.ResourceCPU,
									NominalQuota: resource.MustParse("12"),
								},
							},
						},
					},
				},
			},
			availableResources: []kueuev1beta1.FlavorUsage{
				{
					Name: "flavorA",
					Resources: []kueuev1beta1.ResourceUsage{
						{
							Name:  corev1.ResourceCPU,
							Total: resource.MustParse("10"),
						},
					},
				},
			},
			expectedResult: false,
		},
		{
			name: "Flavor not found",
			requestedResources: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavorB",
							Resources: []kueuev1beta1.ResourceQuota{
								{
									Name:         corev1.ResourceMemory,
									NominalQuota: resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
			availableResources: []kueuev1beta1.FlavorUsage{
				{
					Name: "flavorA",
					Resources: []kueuev1beta1.ResourceUsage{
						{
							Name:  corev1.ResourceCPU,
							Total: resource.MustParse("10"),
						},
					},
				},
			},
			expectedResult: false,
		},
		// TODO: uncomment when flip isResourceSufficient conditions
		//{
		//	name:               "Nil requestedResources",
		//	requestedResources: nil,
		//	availableResources: []kueuev1beta1.FlavorUsage{
		//		{
		//			Name: "flavorA",
		//			Resources: []kueuev1beta1.ResourceUsage{
		//				{
		//					Name:  corev1.ResourceCPU,
		//					Total: resource.MustParse("10"),
		//				},
		//			},
		//		},
		//	},
		//	expectedResult: false,
		//},
		//{
		//	name: "Nil availableResources",
		//	requestedResources: []kueuev1beta1.ResourceGroup{
		//		{
		//			Flavors: []kueuev1beta1.FlavorQuotas{
		//				{
		//					Name: "flavorA",
		//					Resources: []kueuev1beta1.ResourceQuota{
		//						{
		//							Name:         corev1.ResourceCPU,
		//							NominalQuota: resource.MustParse("4"),
		//						},
		//					},
		//				},
		//			},
		//		},
		//	},
		//	availableResources: nil,
		//	expectedResult:     false,
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsResourceSufficient(tt.requestedResources, tt.availableResources)
			if result != tt.expectedResult {
				t.Errorf("expected result %v, got %v", tt.expectedResult, result)
			}
			if err != nil && tt.expectedErrorMessage != "" && err.Error() != tt.expectedErrorMessage {
				t.Errorf("expected error %v, got %v", tt.expectedErrorMessage, err)
			}
		})
	}
}

func TestFlattenResources(t *testing.T) {
	resourceGroups := []kueuev1beta1.ResourceGroup{
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavorA",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
						{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
					},
				},
			},
		},
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavorA",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("2")},
					},
				},
				{
					Name: "flavorB",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("1")},
					},
				},
			},
		},
	}

	expected := map[string]map[corev1.ResourceName]resource.Quantity{
		"flavorA": {
			corev1.ResourceCPU:    resource.MustParse("6"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
		"flavorB": {
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	}

	result := flattenResources(resourceGroups)

	for flavor, resources := range expected {
		if _, exists := result[flavor]; !exists {
			t.Errorf("Flavor %s missing in result", flavor)
			continue
		}

		for resourceName, expectedQuantity := range resources {
			actualQuantity, exists := result[flavor][resourceName]
			if !exists {
				t.Errorf("Resource %s missing in flavor %s", resourceName, flavor)
				continue
			}
			if actualQuantity.Cmp(expectedQuantity) != 0 {
				t.Errorf("Mismatch for resource %s in flavor %s: expected %s, got %s",
					resourceName, flavor, expectedQuantity.String(), actualQuantity.String())
			}
		}
	}

	// Ensure no extra flavors are in the result
	for flavor := range result {
		if _, exists := expected[flavor]; !exists {
			t.Errorf("Unexpected flavor %s in result", flavor)
		}
	}
}

func TestCalculateIncreasedResources(t *testing.T) {
	tests := []struct {
		name            string
		newRequested    []kueuev1beta1.ResourceGroup
		oldRequested    []kueuev1beta1.ResourceGroup
		expectedOutput  []kueuev1beta1.ResourceGroup
		expectedHasIncr bool
	}{
		{
			name: "Increase one flavor",
			newRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("8")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("16Gi")},
							},
						},
					},
				},
			},
			oldRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
					},
				},
			},
			expectedOutput: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
					},
				},
			},
			expectedHasIncr: true,
		},
		{
			name: "No increase",
			newRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
					},
				},
			},
			oldRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
					},
				},
			},
			expectedOutput:  nil,
			expectedHasIncr: false,
		},
		{
			name: "Increase another flavor",
			newRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
						{
							Name: "flavor2",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("6")},
							},
						},
					},
				},
			},
			oldRequested: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, NominalQuota: resource.MustParse("8Gi")},
							},
						},
					},
				},
			},
			expectedOutput: []kueuev1beta1.ResourceGroup{
				{
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "flavor2",
							Resources: []kueuev1beta1.ResourceQuota{
								{Name: corev1.ResourceCPU, NominalQuota: resource.MustParse("6")},
							},
						},
					},
				},
			},
			expectedHasIncr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, hasIncrease := CalculateIncreasedResources(tt.newRequested, tt.oldRequested)
			assert.True(t, equality.Semantic.DeepEqual(tt.expectedHasIncr, hasIncrease))
		})
	}
}
