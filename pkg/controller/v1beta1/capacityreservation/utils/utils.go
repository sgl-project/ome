package utils

import (
	"sort"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

// ConvertResourceGroupsToFlavorUsage converts a list of resourceGroups into a lexicographically ordered list of flavorUsages.
// Ensures consistency in the return value to prevent unnecessary updates.
func ConvertResourceGroupsToFlavorUsage(resourceGroups []kueuev1beta1.ResourceGroup) []kueuev1beta1.FlavorUsage {
	flattened := ConvertResourceGroupsToMap(resourceGroups)
	flavorUsages := make([]kueuev1beta1.FlavorUsage, 0, len(flattened))

	// Get all flavor names and sort them
	flavorNames := make([]string, 0, len(flattened))
	for flavorName := range flattened {
		flavorNames = append(flavorNames, string(flavorName))
	}
	sort.Strings(flavorNames)

	// Create flavor usages in sorted order
	for _, flavorName := range flavorNames {
		flavorReferenceName := kueuev1beta1.ResourceFlavorReference(flavorName)
		resourceQuotas := flattened[flavorReferenceName]
		flavorUsage := kueuev1beta1.FlavorUsage{
			Name:      flavorReferenceName,
			Resources: []kueuev1beta1.ResourceUsage{},
		}

		// Get all resource names and sort them
		resourceNames := make([]v1.ResourceName, 0, len(resourceQuotas))
		for resourceName := range resourceQuotas {
			resourceNames = append(resourceNames, resourceName)
		}
		sort.Slice(resourceNames, func(i, j int) bool {
			return string(resourceNames[i]) < string(resourceNames[j])
		})

		// Add resources in sorted order
		for _, resourceName := range resourceNames {
			quota := resourceQuotas[resourceName]
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

func CheckClusterQueueInactive(clusterQueue *kueuev1beta1.ClusterQueue) bool {
	if clusterQueue.Status.Conditions == nil {
		return false
	}
	return meta.IsStatusConditionFalse(clusterQueue.Status.Conditions, kueuev1beta1.ClusterQueueActive)
}

func IsResourceSufficient(
	availableMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity,
	capacityMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity,
	changeMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity,
) bool {
	for flavor, changeResources := range changeMap {
		availableResources, availableExists := availableMap[flavor]
		if !availableExists {
			return false
		}
		capacityResources, capacityExists := capacityMap[flavor]
		for resourceName, changeQty := range changeResources {
			effectiveAvailable := availableResources[resourceName]
			if capacityExists {
				if capacityQty, exists := capacityResources[resourceName]; exists {
					effectiveAvailable.Sub(capacityQty)
				}
			}
			effectiveAvailable.Sub(changeQty)
			if effectiveAvailable.Sign() < 0 {
				return false
			}
		}
	}
	return true
}

// ConvertResourceGroupsToMap flattens ResourceGroups to a map of flavors : Resources
func ConvertResourceGroupsToMap(resourceGroups []kueuev1beta1.ResourceGroup) map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity {
	flattened := make(map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity)

	for _, group := range resourceGroups {
		for _, flavor := range group.Flavors {
			if _, exists := flattened[flavor.Name]; !exists {
				flattened[flavor.Name] = make(map[v1.ResourceName]resource.Quantity)
			}

			for _, res := range flavor.Resources {
				if existingQty, exists := flattened[flavor.Name][res.Name]; exists {
					// Add() handles quantity format such as BinarySI or DecimalSI
					existingQty.Add(res.NominalQuota)
					flattened[flavor.Name][res.Name] = existingQty
				} else {
					flattened[flavor.Name][res.Name] = res.NominalQuota
				}
			}
		}
	}

	return flattened
}

func GetClusterAvailableResource() (map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity, error) {
	// + input: clusterSnapshot *omev1beta1.clustersnapshot.ClusterSnapshot
	// Get resource data from clusterSnapshot provided by alfred simulator
	// TODO: get data from alfred simulator
	return hardcodeResourcesMap(), nil
	// return nil, nil
}

func hardcodeResourcesMap() map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity {
	// set wide limits to bypass resources sufficiency check
	flavors := []kueuev1beta1.ResourceFlavorReference{
		"bm-gpu-h100-8", "bm-gpu-a100-v2-8", "bm-gpu-b4-8", "bm-gpu4-8", "bm-gpu-a10-4",
	}
	resources := map[v1.ResourceName]resource.Quantity{
		"nvidia.com/gpu": resource.MustParse("512"),
		"cpu":            resource.MustParse("32768"),
		"memory":         resource.MustParse("256Ti"),
	}
	resourcesMap := make(map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity)
	for _, flavor := range flavors {
		resourcesMap[flavor] = resources
	}
	return resourcesMap
}

// GetTotalCapacitiesFromCapacityReservationList Get the sum of capacities of all capacity reservations in the list
// map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity is the map version of []kueuev1beta1.FlavorUsage
func GetTotalCapacitiesFromCapacityReservationList(
	capacityReservationList omev1beta1.ClusterCapacityReservationList,
) map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity {
	flavorMap := make(map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity)

	for _, capRes := range capacityReservationList.Items {
		// Allocatable = Capacity - sum of DAC usages. DAC usages have been counted already when get available resource in cluster
		for _, usage := range capRes.Status.Allocatable {
			if _, exists := flavorMap[usage.Name]; !exists {
				flavorMap[usage.Name] = make(map[v1.ResourceName]resource.Quantity)
			}
			for _, res := range usage.Resources {
				if existing, exists := flavorMap[usage.Name][res.Name]; exists {
					existing.Add(res.Total)
					flavorMap[usage.Name][res.Name] = existing
				} else {
					flavorMap[usage.Name][res.Name] = res.Total
				}
			}
		}
	}
	return flavorMap
}

func CompareResourcesChange(
	desired []kueuev1beta1.ResourceGroup,
	existing []kueuev1beta1.FlavorUsage,
) map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity {
	// Convert desired []ResourceGroup to a map representation
	changeMap := ConvertResourceGroupsToMap(desired)

	// Subtract existing []FlavorUsage from changeMap
	for _, usage := range existing {
		if _, exists := changeMap[usage.Name]; !exists {
			changeMap[usage.Name] = make(map[v1.ResourceName]resource.Quantity)
		}
		for _, res := range usage.Resources {
			if existingQty, exists := changeMap[usage.Name][res.Name]; exists {
				existingQty.Sub(res.Total)
				changeMap[usage.Name][res.Name] = existingQty
			} else {
				negQty := res.Total
				negQty.Neg()
				changeMap[usage.Name][res.Name] = negQty
			}
		}
	}

	return changeMap
}

func IsIncreased(
	changeMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity) bool {
	for _, resources := range changeMap {
		for _, changeQty := range resources {
			if changeQty.Sign() > 0 {
				return true
			}
		}
	}
	return false
}
