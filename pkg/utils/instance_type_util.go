package utils

import (
	"sort"

	"github.com/sgl-project/ome/pkg/imds"
	"github.com/sgl-project/ome/pkg/logging"
)

var instanceTypeMap = map[string]string{
	// Oracle Cloud (OCI) shapes
	"BM.GPU.A10.4":     "A10",
	"BM.GPU.A100-v2.8": "A100-80G",
	"BM.GPU4.8":        "A100-40G",
	"BM.GPU.B4.8":      "A100-40G",
	"BM.GPU.H100.8":    "H100",
	"BM.GPU.H100-NC.8": "H100",
	"BM.GPU.H200.8":    "H200",
	"BM.GPU.H200-NC.8": "H200",

	// AWS instance types
	"p5.48xlarge": "H100",

	// Azure instance types
	"Standard_ND96isr_H100_v5": "H100",

	// Google Cloud instance types
	"a3-highgpu-8g": "H100",

	// CoreWeave instance types
	"gd-8xh100ib-i128": "H100",
	"gd-8xh200ib-i128": "H200",
	"gd-8xl40-i128":    "L40",

	// Nebius instance types
	"gpu-h100-sxm": "H100",
	"gpu-h200-sxm": "H200",
	"gpu-b200-sxm": "B200",
	"gpu-l40s":     "L40S",
}

// Pre-calculated supported GPU types for O(1) lookup and deterministic ordering.
// Initialized once at package load time via init().
var (
	supportedGPUTypes   []string
	supportedGPUTypeSet map[string]bool
)

func init() {
	// Build set of unique GPU types from instanceTypeMap values
	supportedGPUTypeSet = make(map[string]bool)
	for _, gpuType := range instanceTypeMap {
		supportedGPUTypeSet[gpuType] = true
	}

	// Build sorted slice for deterministic output
	supportedGPUTypes = make([]string, 0, len(supportedGPUTypeSet))
	for gpuType := range supportedGPUTypeSet {
		supportedGPUTypes = append(supportedGPUTypes, gpuType)
	}
	sort.Strings(supportedGPUTypes)
}

// IsSupportedGPUType checks if the given GPU type is in the list of supported GPU types.
// Supported GPU types are derived from the values in instanceTypeMap.
// Uses O(1) map lookup for efficiency.
func IsSupportedGPUType(gpuType string) bool {
	return supportedGPUTypeSet[gpuType]
}

// GetSupportedGPUTypes returns a sorted slice of all unique supported GPU type names.
// These are derived from the values in instanceTypeMap and pre-calculated at init time
// for consistent ordering across calls.
func GetSupportedGPUTypes() []string {
	return supportedGPUTypes
}

// GetNodeInstanceType retrieves the instance type of the node.
// NOTE: This implementation is currently specific to Oracle Cloud Infrastructure (OCI)
// and uses the OCI IMDS client. A future refactor is needed to support
// other cloud providers' metadata services.
func GetNodeInstanceType(logger logging.Interface) (string, error) {
	client, err := imds.NewClient(imds.DefaultConfig(), logger)
	if err != nil {
		return "", err
	}
	return client.GetInstanceShape()
}

func GetInstanceTypeShortName(currentInstanceType string) (string, error) {
	if shortName, ok := instanceTypeMap[currentInstanceType]; ok {
		return shortName, nil
	}
	// Return the original instance type as a fallback for unknown shapes
	return currentInstanceType, nil
}

// GetInstanceTypeShortNameWithOverrides returns the GPU short name for an instance type,
// checking overrides in priority order: gpuTypeOverride > customMappings > built-in instanceTypeMap.
// This allows users to configure custom instance-to-GPU mappings without code changes.
func GetInstanceTypeShortNameWithOverrides(currentInstanceType, gpuTypeOverride string, customMappings map[string]string) (string, error) {
	// Priority 1: Direct GPU type override (highest priority)
	if gpuTypeOverride != "" {
		return gpuTypeOverride, nil
	}

	// Priority 2: Custom mappings from ConfigMap
	if customMappings != nil {
		if shortName, ok := customMappings[currentInstanceType]; ok {
			return shortName, nil
		}
	}

	// Priority 3: Built-in instance type map (fallback)
	return GetInstanceTypeShortName(currentInstanceType)
}
