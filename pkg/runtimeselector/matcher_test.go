package runtimeselector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

func TestRuntimeSupportsModel(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }
	ptrToModelQuant := func(s string) *v1beta1.ModelQuantization {
		mq := v1beta1.ModelQuantization(s)
		return &mq
	}

	tests := []struct {
		name          string
		baseModel     *v1beta1.BaseModelSpec
		srSpec        *v1beta1.ServingRuntimeSpec
		runtimeName   string
		expectError   bool
		errorContains string
	}{
		{
			name: "supported model format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
				ModelParameterSize: strPtr("7B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
						},
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "supported model format with all attributes matching",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
						},
						AutoSelect: boolPtr(true),
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "unsupported model format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "tensorflow",
					Version: strPtr("2.0.0"),
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
						},
					},
				},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model format 'mt:tensorflow:2.0.0' not in supported formats",
		},
		{
			name: "model size out of range",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
				ModelParameterSize: strPtr("70B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model size 70B is outside supported range [1B, 13B]",
		},
		{
			name: "model with architecture and quantization match",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      ptrToModelQuant("fp8"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model with nil parameter size",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: nil,
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "runtime with nil size range",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      ptrToModelQuant("fp8"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
					},
				},
				ModelSizeRange: nil,
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model size at minimum boundary",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: strPtr("1B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model size at maximum boundary",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: strPtr("13B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "empty supported formats",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model format 'mt:pytorch' not in supported formats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewDefaultRuntimeMatcher(NewConfig(nil))
			report, err := matcher.GetCompatibilityDetails(tt.srSpec, tt.baseModel, tt.runtimeName)

			assert.NoError(t, err)

			if tt.expectError {
				assert.False(t, report.IsCompatible)
				if tt.errorContains != "" {
					assert.NotEmpty(t, report.IncompatibilityReasons)
					found := false
					for _, reason := range report.IncompatibilityReasons {
						if assert.Contains(t, reason, tt.errorContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected error message not found in reasons: %v", report.IncompatibilityReasons)
				}
			} else {
				assert.True(t, report.IsCompatible)
				assert.Empty(t, report.IncompatibilityReasons)
			}
		})
	}
}

func TestCompareSupportedModelFormats(t *testing.T) {
	ptrToString := func(s string) *string { return &s }
	ptrToModelQuant := func(s string) *v1beta1.ModelQuantization {
		mq := v1beta1.ModelQuantization(s)
		return &mq
	}
	ptrToRuntimeOp := func(s string) *v1beta1.RuntimeSelectorOperator {
		op := v1beta1.RuntimeSelectorOperator(s)
		return &op
	}

	tests := []struct {
		name            string
		baseModel       *v1beta1.BaseModelSpec
		supportedFormat v1beta1.SupportedModelFormat
		expected        bool
	}{
		{
			name: "matching model format names",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			expected: true,
		},
		{
			name: "non-matching model format names",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "PyTorch",
					Version: ptrToString("1.0.0"),
				},
			},
			expected: false,
		},
		{
			name: "matching quantization",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			expected: true,
		},
		{
			name: "non-matching quantization",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptr(v1beta1.ModelQuantization("fp16")),
			},
			expected: false,
		},
		{
			name: "equal version comparison",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: true,
		},
		{
			name: "equal version comparison - not equal",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.9.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: false,
		},
		{
			name: "greater than version comparison - true",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.7.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: true,
		},
		{
			name: "greater than version comparison - false",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.7.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.7.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: false,
		},
		{
			name: "unofficial version comparison - forces equality",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.8.0-dev"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0-dev"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: true,
		},
		{
			name: "unofficial version comparison - not equal",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.8.0-dev"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0-alpha"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: false,
		},
		{
			name: "framework comparison - matching",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "ONNXRuntime", Version: ptr("1.10.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: true,
		},
		{
			name: "framework comparison - non-matching name",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "PyTorch", Version: ptr("1.10.0")},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewDefaultRuntimeMatcher(NewConfig(nil)).(*DefaultRuntimeMatcher)
			result := matcher.compareSupportedModelFormats(tt.baseModel, tt.supportedFormat)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestGetCompatibilityDetails(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil))

	t.Run("disabled runtime", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			Disabled: ptr(true),
		}
		baseModel := &v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
		}

		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, "test-runtime")
		assert.NoError(t, err)
		assert.False(t, report.IsCompatible)
		assert.Contains(t, report.IncompatibilityReasons, "runtime is disabled")
	})

	t.Run("no auto-select enabled", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:   "pytorch",
						Weight: 10,
					},
					AutoSelect: ptr(false),
				},
			},
		}
		baseModel := &v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
		}

		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, "test-runtime")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible) // Runtime is compatible, just not auto-selectable
		assert.NotEmpty(t, report.Warnings)
		assert.Contains(t, report.Warnings[0], "runtime does not have auto-select enabled")
	})

	t.Run("model size warning", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:   "pytorch",
						Weight: 10,
					},
					AutoSelect: ptr(true),
				},
			},
			ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
				Min: ptr("1B"),
				Max: ptr("10B"),
			},
		}
		baseModel := &v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
			// No ModelParameterSize specified
		}

		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, "test-runtime")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible)
		assert.NotEmpty(t, report.Warnings)
		found := false
		for _, warning := range report.Warnings {
			if strings.Contains(warning, "model does not specify size") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected warning about model size not found")
	})
}
