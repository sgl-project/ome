package utils

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func ConvertResourceGroupsToFlavorUsage(resourceGroups []kueuev1beta1.ResourceGroup) []kueuev1beta1.FlavorUsage {
	flattened := flattenResources(resourceGroups)
	flavorUsages := make([]kueuev1beta1.FlavorUsage, 0, len(flattened))

	for flavorName, resourceQuotas := range flattened {
		flavorUsage := kueuev1beta1.FlavorUsage{
			Name:      kueuev1beta1.ResourceFlavorReference(flavorName),
			Resources: []kueuev1beta1.ResourceUsage{},
		}
		for resourceName, quota := range resourceQuotas {
			flavorUsage.Resources = append(flavorUsage.Resources, kueuev1beta1.ResourceUsage{
				Name:  resourceName,
				Total: quota.DeepCopy(),
			})
		}
		flavorUsages = append(flavorUsages, flavorUsage)
	}

	return flavorUsages
}

func DeepCopyFlavorsUsage(original []kueuev1beta1.FlavorUsage) []kueuev1beta1.FlavorUsage {
	copied := make([]kueuev1beta1.FlavorUsage, len(original))
	for i, flavorUsage := range original {
		copied[i] = *flavorUsage.DeepCopy()
	}
	return copied
}

func CheckClusterQueueActive(clusterQueue *kueuev1beta1.ClusterQueue) bool {
	if clusterQueue.Status.Conditions == nil {
		return false
	}
	return meta.IsStatusConditionTrue(clusterQueue.Status.Conditions, kueuev1beta1.ClusterQueueActive)
}

// IsResourceSufficient checks whether resource is sufficient by comparing requested resources and available resources
// Ignore parameters related to borrowing and lending from cohort for now
func IsResourceSufficient(
	requestedResources []kueuev1beta1.ResourceGroup,
	availableResources []kueuev1beta1.FlavorUsage,
) (bool, error) {
	if requestedResources == nil || availableResources == nil {
		// TODO: remove placeholder and flip the condition
		// return false, nil
		return true, nil
	}
	// Step 1: Deep copy original availableResource
	availableCopy := make([]kueuev1beta1.FlavorUsage, len(availableResources))
	for i, availFlavor := range availableResources {
		availableCopy[i] = *availFlavor.DeepCopy()
	}

	// Step 2: Iterate over requested resource groups and their flavors
	for _, resourceGroup := range requestedResources {
		for _, reqFlavor := range resourceGroup.Flavors {
			// Check if the flavor exists in available resources
			found := false
			for i, availFlavor := range availableCopy {
				if reqFlavor.Name == availFlavor.Name {
					found = true
					// Step 3: Subtract requested resources from available resources
					isSufficient, err := compareAndAllocateResources(reqFlavor.Resources, availFlavor.Resources)
					if err != nil {
						return false, fmt.Errorf("failed for flavor %s: %v", reqFlavor.Name, err)
					}
					if !isSufficient {
						return false, nil
					}

					// Update the copy for this flavor
					availableCopy[i] = availFlavor
					break
				}
			}
			if !found {
				return false, nil // Requested flavor not found
			}
		}
	}

	// Step 4: If all checks pass, the resources are sufficient
	return true, nil
}

func compareAndAllocateResources(
	requestedQuotas []kueuev1beta1.ResourceQuota,
	availableCapacities []kueuev1beta1.ResourceUsage,
) (bool, error) {
	// Create a map for fast lookup and mutation of available capacities by resource name
	// Ignore parameters related to borrowing and lending from cohort for now
	capacityMap := make(map[corev1.ResourceName]*resource.Quantity)
	for i, capacity := range availableCapacities {
		capacityMap[capacity.Name] = &availableCapacities[i].Total
	}

	// Iterate over each requested resource quota
	for _, requested := range requestedQuotas {
		// Check if the requested resource name exists in available capacities
		availableCapacity, found := capacityMap[requested.Name]
		if !found {
			// Resource name is not available
			return false, fmt.Errorf("resource %s is unavailable", requested.Name)
		}

		// Compare the requested quota with the available capacity
		if availableCapacity.Cmp(requested.NominalQuota) < 0 {
			// Not enough resources available
			return false, nil
		}

		// Subtract the requested resource from the available capacity
		availableCapacity.Sub(requested.NominalQuota)
	}

	// All requested resources are satisfied
	return true, nil
}

func CalculateIncreasedResources(
	newRequested []kueuev1beta1.ResourceGroup,
	oldRequested []kueuev1beta1.ResourceGroup,
) ([]kueuev1beta1.ResourceGroup, bool) {
	// Flatten the new and old resource groups
	newFlattened := flattenResources(newRequested)
	oldFlattened := flattenResources(oldRequested)

	// Prepare the resulting ResourceGroup for increased resources
	increasedFlavors := []kueuev1beta1.FlavorQuotas{}

	hasIncrease := false

	// Iterate over the new resources to calculate increases
	for flavorName, newResources := range newFlattened {
		oldResources := oldFlattened[flavorName]
		increasedQuotas := []kueuev1beta1.ResourceQuota{}

		for resourceName, newQuota := range newResources {
			oldQuota := oldResources[resourceName]

			if newQuota.Cmp(oldQuota) > 0 {
				hasIncrease = true
				increasedQuotas = append(increasedQuotas, kueuev1beta1.ResourceQuota{
					Name:         resourceName,
					NominalQuota: *resource.NewQuantity(newQuota.Value()-oldQuota.Value(), resource.DecimalSI),
				})
			}
		}

		if len(increasedQuotas) > 0 {
			increasedFlavors = append(increasedFlavors, kueuev1beta1.FlavorQuotas{
				Name:      kueuev1beta1.ResourceFlavorReference(flavorName),
				Resources: increasedQuotas,
			})
		}
	}

	// If no increase, return nil and false
	if !hasIncrease {
		return nil, false
	}

	// Wrap the increased flavors in a single ResourceGroup
	return []kueuev1beta1.ResourceGroup{
		{
			Flavors: increasedFlavors,
		},
	}, true
}

// flattenResources flattens ResourceGroups to a list of flavors
func flattenResources(resourceGroups []kueuev1beta1.ResourceGroup) map[string]map[corev1.ResourceName]resource.Quantity {
	flattened := make(map[string]map[corev1.ResourceName]resource.Quantity)

	for _, group := range resourceGroups {
		for _, flavor := range group.Flavors {
			flavorName := string(flavor.Name)
			if _, exists := flattened[flavorName]; !exists {
				flattened[flavorName] = make(map[corev1.ResourceName]resource.Quantity)
			}

			for _, flavorResource := range flavor.Resources {
				if current, exists := flattened[flavorName][flavorResource.Name]; exists {
					// Add to the existing quantity
					current.Add(flavorResource.NominalQuota)
					flattened[flavorName][flavorResource.Name] = current
				} else {
					// Initialize the quantity
					flattened[flavorName][flavorResource.Name] = flavorResource.NominalQuota.DeepCopy()
				}
			}
		}
	}

	return flattened
}

func GetClusterAvailableResource() ([]kueuev1beta1.FlavorUsage, error) {
	// + input: clusterSnapshot *omev1beta1.clustersnapshot.ClusterSnapshot
	// Get resource data from clusterSnapshot provided by alfred simulator
	// TODO: get data from alfred simulator
	return nil, nil
}
