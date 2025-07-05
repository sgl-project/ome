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
		// Error cases
		{
			name:         "Unknown instance type",
			instanceType: "unknown-instance-type",
			expected:     "",
			expectError:  true,
		},
		{
			name:         "Empty instance type",
			instanceType: "",
			expected:     "",
			expectError:  true,
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
