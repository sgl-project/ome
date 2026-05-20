package pod

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func newResourceRequirements(cpuLimit, memoryLimit, cpuRequest, memoryRequest string) (v1.ResourceRequirements, error) {
	parsedCPULimit, err := parseResourceQuantity("cpuLimit", cpuLimit)
	if err != nil {
		return v1.ResourceRequirements{}, err
	}
	parsedMemoryLimit, err := parseResourceQuantity("memoryLimit", memoryLimit)
	if err != nil {
		return v1.ResourceRequirements{}, err
	}
	parsedCPURequest, err := parseResourceQuantity("cpuRequest", cpuRequest)
	if err != nil {
		return v1.ResourceRequirements{}, err
	}
	parsedMemoryRequest, err := parseResourceQuantity("memoryRequest", memoryRequest)
	if err != nil {
		return v1.ResourceRequirements{}, err
	}

	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceCPU:    parsedCPULimit,
			v1.ResourceMemory: parsedMemoryLimit,
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    parsedCPURequest,
			v1.ResourceMemory: parsedMemoryRequest,
		},
	}, nil
}

func parseResourceQuantity(fieldName, value string) (resource.Quantity, error) {
	if value == "" {
		return resource.Quantity{}, fmt.Errorf("%s is required", fieldName)
	}
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("invalid %s %q: %w", fieldName, value, err)
	}
	return quantity, nil
}
