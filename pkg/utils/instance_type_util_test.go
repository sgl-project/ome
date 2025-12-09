package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInstanceTypeShortName(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		expected     string
		expectError  bool
	}{
		// Oracle Cloud tests
		{
			name:         "OCI A10 instance",
			instanceType: "BM.GPU.A10.4",
			expected:     "A10",
			expectError:  false,
		},
		{
			name:         "OCI A100-80G instance",
			instanceType: "BM.GPU.A100-v2.8",
			expected:     "A100-80G",
			expectError:  false,
		},
		{
			name:         "OCI H100 instance",
			instanceType: "BM.GPU.H100.8",
			expected:     "H100",
			expectError:  false,
		},
		{
			name:         "OCI H200 instance",
			instanceType: "BM.GPU.H200.8",
			expected:     "H200",
			expectError:  false,
		},
		// AWS tests
		{
			name:         "AWS H100 instance",
			instanceType: "p5.48xlarge",
			expected:     "H100",
			expectError:  false,
		},
		// Azure tests
		{
			name:         "Azure H100 instance",
			instanceType: "Standard_ND96isr_H100_v5",
			expected:     "H100",
			expectError:  false,
		},
		// Google Cloud tests
		{
			name:         "GCP H100 instance",
			instanceType: "a3-highgpu-8g",
			expected:     "H100",
			expectError:  false,
		},
		// CoreWeave tests
		{
			name:         "CoreWeave H100 instance",
			instanceType: "gd-8xh100ib-i128",
			expected:     "H100",
			expectError:  false,
		},
		{
			name:         "CoreWeave L40 instance",
			instanceType: "gd-8xl40-i128",
			expected:     "L40",
			expectError:  false,
		},
		// Nebius tests
		{
			name:         "Nebius H100 instance",
			instanceType: "gpu-h100-sxm",
			expected:     "H100",
			expectError:  false,
		},
		{
			name:         "Nebius H200 instance",
			instanceType: "gpu-h200-sxm",
			expected:     "H200",
			expectError:  false,
		},
		{
			name:         "Nebius B200 instance",
			instanceType: "gpu-b200-sxm",
			expected:     "B200",
			expectError:  false,
		},
		{
			name:         "Nebius L40S instance",
			instanceType: "gpu-l40s",
			expected:     "L40S",
			expectError:  false,
		},
		// Fallback cases
		{
			name:         "Unknown instance type",
			instanceType: "unknown-instance-type",
			expected:     "unknown-instance-type",
			expectError:  false,
		},
		{
			name:         "Empty instance type",
			instanceType: "",
			expected:     "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetInstanceTypeShortName(tt.instanceType)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsSupportedGPUType(t *testing.T) {
	tests := []struct {
		name     string
		gpuType  string
		expected bool
	}{
		// Supported GPU types
		{name: "H100 is supported", gpuType: "H100", expected: true},
		{name: "H200 is supported", gpuType: "H200", expected: true},
		{name: "A100-80G is supported", gpuType: "A100-80G", expected: true},
		{name: "A100-40G is supported", gpuType: "A100-40G", expected: true},
		{name: "A10 is supported", gpuType: "A10", expected: true},
		{name: "B200 is supported", gpuType: "B200", expected: true},
		{name: "L40 is supported", gpuType: "L40", expected: true},
		{name: "L40S is supported", gpuType: "L40S", expected: true},
		// Unsupported GPU types
		{name: "Empty string is not supported", gpuType: "", expected: false},
		{name: "Random string is not supported", gpuType: "RandomGPU", expected: false},
		{name: "Lowercase h100 is not supported", gpuType: "h100", expected: false},
		{name: "A100 without suffix is not supported", gpuType: "A100", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSupportedGPUType(tt.gpuType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSupportedGPUTypes(t *testing.T) {
	types := GetSupportedGPUTypes()

	// Should return at least some types (derived from instanceTypeMap)
	assert.NotEmpty(t, types, "Should return at least one supported GPU type")

	// Convert to map for easier checking
	typeMap := make(map[string]bool)
	for _, gpuType := range types {
		typeMap[gpuType] = true
	}

	// Verify some known GPU types that are in instanceTypeMap are present
	// These are values from the built-in instanceTypeMap
	knownTypes := []string{"H100", "H200", "A10"}
	for _, known := range knownTypes {
		assert.True(t, typeMap[known], "Expected GPU type %s to be in supported list (from instanceTypeMap)", known)
	}

	// Verify all returned types are also reported as supported by IsSupportedGPUType
	for _, gpuType := range types {
		assert.True(t, IsSupportedGPUType(gpuType), "GetSupportedGPUTypes returned %s but IsSupportedGPUType says it's not supported", gpuType)
	}
}

func TestGetInstanceTypeShortNameWithOverrides(t *testing.T) {
	tests := []struct {
		name            string
		instanceType    string
		gpuTypeOverride string
		customMappings  map[string]string
		expected        string
		expectError     bool
	}{
		// Priority 1: GPU type override takes precedence
		{
			name:            "GPU type override takes precedence over everything",
			instanceType:    "BM.GPU.H100.8",
			gpuTypeOverride: "A10",
			customMappings:  map[string]string{"BM.GPU.H100.8": "L40"},
			expected:        "A10",
			expectError:     false,
		},
		{
			name:            "GPU type override works for unknown instance type",
			instanceType:    "a3-megagpu-8g",
			gpuTypeOverride: "H100",
			customMappings:  nil,
			expected:        "H100",
			expectError:     false,
		},
		// Priority 2: Custom mappings
		{
			name:            "Custom mapping takes precedence over built-in",
			instanceType:    "BM.GPU.H100.8",
			gpuTypeOverride: "",
			customMappings:  map[string]string{"BM.GPU.H100.8": "A10"},
			expected:        "A10",
			expectError:     false,
		},
		{
			name:            "Custom mapping for unknown instance type",
			instanceType:    "a3-megagpu-8g",
			gpuTypeOverride: "",
			customMappings:  map[string]string{"a3-megagpu-8g": "H100"},
			expected:        "H100",
			expectError:     false,
		},
		{
			name:            "Custom mapping with multiple entries",
			instanceType:    "custom-instance-xyz",
			gpuTypeOverride: "",
			customMappings: map[string]string{
				"a3-megagpu-8g":       "H100",
				"custom-instance-xyz": "A100-80G",
			},
			expected:    "A100-80G",
			expectError: false,
		},
		// Priority 3: Built-in map fallback (using OCI instance type that exists in built-in map)
		{
			name:            "Falls back to built-in map when no overrides",
			instanceType:    "BM.GPU.H100.8",
			gpuTypeOverride: "",
			customMappings:  nil,
			expected:        "H100",
			expectError:     false,
		},
		{
			name:            "Falls back to built-in map when custom mapping doesn't have entry",
			instanceType:    "BM.GPU.H100.8",
			gpuTypeOverride: "",
			customMappings:  map[string]string{"other-instance": "L40"},
			expected:        "H100",
			expectError:     false,
		},
		// Fallback to original instance type
		{
			name:            "Returns original instance type when not found anywhere",
			instanceType:    "completely-unknown-type",
			gpuTypeOverride: "",
			customMappings:  nil,
			expected:        "completely-unknown-type",
			expectError:     false,
		},
		{
			name:            "Empty custom mappings map",
			instanceType:    "unknown-type",
			gpuTypeOverride: "",
			customMappings:  map[string]string{},
			expected:        "unknown-type",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetInstanceTypeShortNameWithOverrides(tt.instanceType, tt.gpuTypeOverride, tt.customMappings)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
