package utils

import (
	"testing"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/sgl-ome/pkg/constants"

	v1 "k8s.io/api/core/v1"
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
							Name:         v1.ResourceCPU,
							NominalQuota: resource.MustParse("10"),
						},
						{
							Name:         v1.ResourceMemory,
							NominalQuota: resource.MustParse("2Gi"),
						},
					},
				},
				{
					Name: "flavor2",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         constants.NvidiaGPUResourceType,
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
							Name:         v1.ResourceMemory,
							NominalQuota: resource.MustParse("512Mi"),
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
					Name:  v1.ResourceCPU,
					Total: resource.MustParse("10"),
				},
				{
					Name:  v1.ResourceMemory,
					Total: resource.MustParse("2560Mi"),
				},
			},
		},
		{
			Name: "flavor2",
			Resources: []kueuev1beta1.ResourceUsage{
				{
					Name:  constants.NvidiaGPUResourceType,
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
			if !compareQuantities(t, map[v1.ResourceName]resource.Quantity{expected[i].Resources[j].Name: expected[i].Resources[j].Total}, map[v1.ResourceName]resource.Quantity{result[i].Resources[j].Name: result[i].Resources[j].Total}) {
				t.Errorf("Expected total for resource %s to be %v, but got %v",
					expected[i].Resources[j].Name, expected[i].Resources[j].Total, result[i].Resources[j].Total)
			}
		}
	}
}

func compareQuantities(t *testing.T, expected, actual map[v1.ResourceName]resource.Quantity) bool {
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

func TestConvertResourceGroupsToMap(t *testing.T) {
	resourceGroups := []kueuev1beta1.ResourceGroup{
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavor1",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: v1.ResourceCPU, NominalQuota: resource.MustParse("4")},
						{Name: v1.ResourceMemory, NominalQuota: resource.MustParse("2Gi")},
					},
				},
			},
		},
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavor1",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: v1.ResourceMemory, NominalQuota: resource.MustParse("512Mi")},
					},
				},
				{
					Name: "flavor2",
					Resources: []kueuev1beta1.ResourceQuota{
						{Name: v1.ResourceCPU, NominalQuota: resource.MustParse("1")},
					},
				},
			},
		},
	}

	expected := map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
		"flavor1": {
			v1.ResourceCPU:    resource.MustParse("4"),
			v1.ResourceMemory: resource.MustParse("2560Mi"),
		},
		"flavor2": {
			v1.ResourceCPU: resource.MustParse("1"),
		},
	}

	result := ConvertResourceGroupsToMap(resourceGroups)

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

func TestIsResourceSufficient(t *testing.T) {
	tests := []struct {
		name         string
		availableMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity
		capacityMap  map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity
		changeMap    map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity
		expected     bool
	}{
		{
			name: "Sufficient resources",
			availableMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("10Gi"),
					v1.ResourceCPU:    resource.MustParse("4"),
				},
			},
			capacityMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("2Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			changeMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("1Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			expected: true,
		},
		{
			name: "Insufficient resources (memory)",
			availableMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("2Gi"),
					v1.ResourceCPU:    resource.MustParse("4"),
				},
			},
			capacityMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("2Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			changeMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("1Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			expected: false,
		},
		{
			name: "Flavor in changeMap not in availableMap",
			availableMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("10Gi"),
					v1.ResourceCPU:    resource.MustParse("4"),
				},
			},
			capacityMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceMemory: resource.MustParse("2Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			changeMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor2": {
					v1.ResourceMemory: resource.MustParse("2Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResourceSufficient(tt.availableMap, tt.capacityMap, tt.changeMap)
			if result != tt.expected {
				t.Errorf("IsResourceSufficient() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDeepCopyFlavorsUsage(t *testing.T) {
	original := []kueuev1beta1.FlavorUsage{
		{
			Name: "flavor1",
			Resources: []kueuev1beta1.ResourceUsage{
				{
					Name:  v1.ResourceCPU,
					Total: resource.MustParse("10"),
				},
			},
		},
	}

	copied := DeepCopyFlavorsUsage(original)

	// Ensure the copied slice is not the same as the original
	if &copied == &original {
		t.Error("DeepCopyFlavorsUsage() returned the same slice, expected a deep copy")
	}

	// Ensure the contents are the same
	if len(copied) != len(original) {
		t.Errorf("Expected %d flavors, got %d", len(original), len(copied))
	}

	for i := range original {
		if copied[i].Name != original[i].Name {
			t.Errorf("Expected flavor name %s, got %s", original[i].Name, copied[i].Name)
		}
		for j := range original[i].Resources {
			if copied[i].Resources[j].Name != original[i].Resources[j].Name {
				t.Errorf("Expected resource name %s, got %s", original[i].Resources[j].Name, copied[i].Resources[j].Name)
			}
			if copied[i].Resources[j].Total.Cmp(original[i].Resources[j].Total) != 0 {
				t.Errorf("Expected resource total %v, got %v", original[i].Resources[j].Total, copied[i].Resources[j].Total)
			}
		}
	}
}

func TestCheckClusterQueueActive(t *testing.T) {
	tests := []struct {
		name         string
		clusterQueue *kueuev1beta1.ClusterQueue
		expected     bool
	}{
		{
			name: "Active ClusterQueue",
			clusterQueue: &kueuev1beta1.ClusterQueue{
				Status: kueuev1beta1.ClusterQueueStatus{
					Conditions: []metav1.Condition{
						{
							Type:   kueuev1beta1.ClusterQueueActive,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Inactive ClusterQueue",
			clusterQueue: &kueuev1beta1.ClusterQueue{
				Status: kueuev1beta1.ClusterQueueStatus{
					Conditions: []metav1.Condition{
						{
							Type:   kueuev1beta1.ClusterQueueActive,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name:         "Nil Conditions",
			clusterQueue: &kueuev1beta1.ClusterQueue{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckClusterQueueActive(tt.clusterQueue)
			if result != tt.expected {
				t.Errorf("CheckClusterQueueActive() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckClusterQueueInactive(t *testing.T) {
	tests := []struct {
		name         string
		clusterQueue *kueuev1beta1.ClusterQueue
		expected     bool
	}{
		{
			name: "Inactive ClusterQueue",
			clusterQueue: &kueuev1beta1.ClusterQueue{
				Status: kueuev1beta1.ClusterQueueStatus{
					Conditions: []metav1.Condition{
						{
							Type:   kueuev1beta1.ClusterQueueActive,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Active ClusterQueue",
			clusterQueue: &kueuev1beta1.ClusterQueue{
				Status: kueuev1beta1.ClusterQueueStatus{
					Conditions: []metav1.Condition{
						{
							Type:   kueuev1beta1.ClusterQueueActive,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name:         "Nil Conditions",
			clusterQueue: &kueuev1beta1.ClusterQueue{},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckClusterQueueInactive(tt.clusterQueue)
			if result != tt.expected {
				t.Errorf("CheckClusterQueueInactive() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetTotalCapacitiesFromCapacityReservationList(t *testing.T) {
	capacityReservationList := omev1beta1.ClusterCapacityReservationList{
		Items: []omev1beta1.ClusterCapacityReservation{
			{
				Status: omev1beta1.CapacityReservationStatus{
					Allocatable: []kueuev1beta1.FlavorUsage{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceUsage{
								{
									Name:  v1.ResourceCPU,
									Total: resource.MustParse("10"),
								},
							},
						},
					},
				},
			},
			{
				Status: omev1beta1.CapacityReservationStatus{
					Allocatable: []kueuev1beta1.FlavorUsage{
						{
							Name: "flavor1",
							Resources: []kueuev1beta1.ResourceUsage{
								{
									Name:  v1.ResourceCPU,
									Total: resource.MustParse("5"),
								},
							},
						},
					},
				},
			},
		},
	}

	expected := map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
		"flavor1": {
			v1.ResourceCPU: resource.MustParse("15"),
		},
	}

	result := GetTotalCapacitiesFromCapacityReservationList(capacityReservationList)

	for flavor, resources := range expected {
		for resourceName, expectedQuantity := range resources {
			actualQuantity := result[flavor][resourceName]
			if actualQuantity.Cmp(expectedQuantity) != 0 {
				t.Errorf("Expected %v for resource %s in flavor %s, got %v", expectedQuantity, resourceName, flavor, actualQuantity)
			}
		}
	}
}

func TestCompareResourcesChange(t *testing.T) {
	desired := []kueuev1beta1.ResourceGroup{
		{
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "flavor1",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         v1.ResourceCPU,
							NominalQuota: resource.MustParse("10"),
						},
					},
				},
			},
		},
	}

	existing := []kueuev1beta1.FlavorUsage{
		{
			Name: "flavor1",
			Resources: []kueuev1beta1.ResourceUsage{
				{
					Name:  v1.ResourceCPU,
					Total: resource.MustParse("5"),
				},
			},
		},
	}

	expected := map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
		"flavor1": {
			v1.ResourceCPU: resource.MustParse("5"),
		},
	}

	result := CompareResourcesChange(desired, existing)

	for flavor, resources := range expected {
		for resourceName, expectedQuantity := range resources {
			actualQuantity := result[flavor][resourceName]
			if actualQuantity.Cmp(expectedQuantity) != 0 {
				t.Errorf("Expected %v for resource %s in flavor %s, got %v", expectedQuantity, resourceName, flavor, actualQuantity)
			}
		}
	}
}

func TestIsIncreased(t *testing.T) {
	tests := []struct {
		name      string
		changeMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity
		expected  bool
	}{
		{
			name: "Increased resources",
			changeMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceCPU: resource.MustParse("5"),
				},
			},
			expected: true,
		},
		{
			name: "No increased resources",
			changeMap: map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity{
				"flavor1": {
					v1.ResourceCPU: resource.MustParse("-5"),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIncreased(tt.changeMap)
			if result != tt.expected {
				t.Errorf("IsIncreased() = %v, want %v", result, tt.expected)
			}
		})
	}
}
