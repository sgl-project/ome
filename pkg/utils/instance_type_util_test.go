package utils

import (
	"os"
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

func TestLoadInstanceTypeMapFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedMap map[string]string
		expectError bool
	}{
		{
			name:        "empty env var returns default map",
			envValue:    "",
			expectedMap: defaultInstanceTypeMap,
			expectError: false,
		},
		{
			name:     "valid JSON returns parsed map",
			envValue: `{"custom-instance": "CUSTOM-GPU", "another-instance": "GPU-X"}`,
			expectedMap: map[string]string{
				"custom-instance":  "CUSTOM-GPU",
				"another-instance": "GPU-X",
			},
			expectError: false,
		},
		{
			name:        "invalid JSON returns error",
			envValue:    `{"invalid": json}`,
			expectedMap: nil,
			expectError: true,
		},
		{
			name:        "malformed JSON with trailing comma returns error",
			envValue:    `{"key": "value",}`,
			expectedMap: nil,
			expectError: true,
		},
		{
			name:        "empty JSON object returns default map",
			envValue:    `{}`,
			expectedMap: defaultInstanceTypeMap,
			expectError: false,
		},
		{
			name:     "single entry JSON",
			envValue: `{"BM.GPU.H100.8": "H100"}`,
			expectedMap: map[string]string{
				"BM.GPU.H100.8": "H100",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value and restore after test
			originalValue := os.Getenv(InstanceTypeMapEnvVar)
			defer os.Setenv(InstanceTypeMapEnvVar, originalValue)

			// Set test env value
			if tt.envValue == "" {
				os.Unsetenv(InstanceTypeMapEnvVar)
			} else {
				os.Setenv(InstanceTypeMapEnvVar, tt.envValue)
			}

			result, err := loadInstanceTypeMapFromEnv()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMap, result)
			}
		})
	}
}
