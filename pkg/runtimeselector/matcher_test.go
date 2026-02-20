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
			name: "diffusion pipeline match",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "diffusers",
					Version: strPtr("1.0.0"),
				},
				DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
					ClassName: strPtr("QwenImagePipeline"),
					Scheduler: &v1beta1.DiffusionComponentSpec{
						Library: "diffusers",
						Type:    "FlowMatchEulerDiscreteScheduler",
					},
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "diffusers",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "diffusers",
							Version: strPtr("1.0.0"),
						},
						DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
							ClassName: strPtr("QwenImagePipeline"),
							Scheduler: &v1beta1.DiffusionComponentSpec{
								Library: "diffusers",
								Type:    "FlowMatchEulerDiscreteScheduler",
							},
						},
					},
				},
			},
			runtimeName: "diffusion-runtime",
			expectError: false,
		},
		{
			name: "diffusion pipeline runtime wildcard requirements",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "diffusers",
					Version: strPtr("1.0.0"),
				},
				DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
					ClassName: strPtr("QwenImagePipeline"),
					Scheduler: &v1beta1.DiffusionComponentSpec{
						Library: "diffusers",
						Type:    "FlowMatchEulerDiscreteScheduler",
					},
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "diffusers",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "diffusers",
							Version: strPtr("1.0.0"),
						},
						DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
							Scheduler: &v1beta1.DiffusionComponentSpec{
								Library: "diffusers",
							},
						},
					},
				},
			},
			runtimeName: "diffusion-runtime",
			expectError: false,
		},
		{
			name: "diffusion pipeline runtime with no requirements",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "diffusers",
					Version: strPtr("1.0.0"),
				},
				DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
					ClassName: strPtr("QwenImagePipeline"),
					Scheduler: &v1beta1.DiffusionComponentSpec{
						Library: "diffusers",
						Type:    "FlowMatchEulerDiscreteScheduler",
					},
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "diffusers",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "diffusers",
							Version: strPtr("1.0.0"),
						},
						DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{},
					},
				},
			},
			runtimeName: "diffusion-runtime",
			expectError: false,
		},
		{
			name: "diffusion pipeline mismatch",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "diffusers",
					Version: strPtr("1.0.0"),
				},
				DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
					ClassName: strPtr("QwenImagePipeline"),
					Scheduler: &v1beta1.DiffusionComponentSpec{
						Library: "diffusers",
						Type:    "FlowMatchEulerDiscreteScheduler",
					},
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "diffusers",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "diffusers",
							Version: strPtr("1.0.0"),
						},
						DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{
							ClassName: strPtr("StableDiffusionPipeline"),
							Scheduler: &v1beta1.DiffusionComponentSpec{
								Library: "diffusers",
								Type:    "DPMSolverMultistepScheduler",
							},
						},
					},
				},
			},
			runtimeName:   "diffusion-runtime",
			expectError:   true,
			errorContains: "pipeline class mismatch",
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
			isvc := &v1beta1.InferenceService{}
			report, err := matcher.GetCompatibilityDetails(tt.srSpec, tt.baseModel, isvc, tt.runtimeName)

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

		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, isvc, "test-runtime")
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

		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, isvc, "test-runtime")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible) // Runtime is compatible, just not auto-selectable
		assert.NotEmpty(t, report.Warnings)
		assert.Contains(t, report.Warnings[0], "runtime does not have auto-select enabled for any supported format")
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

		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(runtime, baseModel, isvc, "test-runtime")
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

func TestMatcherPicksRuntimeByModelSize(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil))
	isvc := &v1beta1.InferenceService{}

	model := &v1beta1.BaseModelSpec{
		ModelFormat:        v1beta1.ModelFormat{Name: "diffusers"},
		ModelParameterSize: ptr("7B"),
	}

	smallRuntime := &v1beta1.ServingRuntimeSpec{
		SupportedModelFormats: []v1beta1.SupportedModelFormat{
			{
				ModelFormat: &v1beta1.ModelFormat{Name: "diffusers"},
				AutoSelect:  ptr(true),
			},
		},
		ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
			Min: ptr("1B"),
			Max: ptr("5B"),
		},
	}

	midRuntime := &v1beta1.ServingRuntimeSpec{
		SupportedModelFormats: []v1beta1.SupportedModelFormat{
			{
				ModelFormat: &v1beta1.ModelFormat{Name: "diffusers"},
				AutoSelect:  ptr(true),
			},
		},
		ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
			Min: ptr("6B"),
			Max: ptr("13B"),
		},
	}

	smallReport, err := matcher.GetCompatibilityDetails(smallRuntime, model, isvc, "small-runtime")
	assert.NoError(t, err)
	assert.False(t, smallReport.IsCompatible)
	assert.False(t, smallReport.MatchDetails.SizeMatch)
	assert.NotEmpty(t, smallReport.IncompatibilityReasons)

	midReport, err := matcher.GetCompatibilityDetails(midRuntime, model, isvc, "mid-runtime")
	assert.NoError(t, err)
	assert.True(t, midReport.IsCompatible)
	assert.True(t, midReport.MatchDetails.SizeMatch)

	selected := ""
	if smallReport.IsCompatible {
		selected = "small-runtime"
	} else if midReport.IsCompatible {
		selected = "mid-runtime"
	}

	assert.Equal(t, "mid-runtime", selected)
}

func TestGetCompatibilityDetails_AcceleratorClasses(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil))

	mkRuntime := func(classes []string) *v1beta1.ServingRuntimeSpec {
		return &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{
						Name:   "pytorch",
						Weight: 10,
					},
					AutoSelect: ptr(true),
				},
			},
			AcceleratorRequirements: &v1beta1.AcceleratorRequirements{AcceleratorClasses: classes},
		}
	}

	baseModel := &v1beta1.BaseModelSpec{
		ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
	}

	t.Run("isvc selector matches class", func(t *testing.T) {
		rt := mkRuntime([]string{"nvidia-a100", "nvidia-tesla-t4"})
		cls := "nvidia-a100"
		isvc := &v1beta1.InferenceService{Spec: v1beta1.InferenceServiceSpec{AcceleratorSelector: &v1beta1.AcceleratorSelector{AcceleratorClass: &cls}}}

		report, err := matcher.GetCompatibilityDetails(rt, baseModel, isvc, "rt")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible)
	})

	t.Run("isvc selector mismatches class", func(t *testing.T) {
		rt := mkRuntime([]string{"nvidia-a100", "nvidia-tesla-t4"})
		cls := "H100"
		isvc := &v1beta1.InferenceService{Spec: v1beta1.InferenceServiceSpec{AcceleratorSelector: &v1beta1.AcceleratorSelector{AcceleratorClass: &cls}}}

		report, err := matcher.GetCompatibilityDetails(rt, baseModel, isvc, "rt")
		assert.NoError(t, err)
		assert.False(t, report.IsCompatible)
		assert.NotEmpty(t, report.IncompatibilityReasons)
		found := false
		for _, r := range report.IncompatibilityReasons {
			if strings.Contains(r, "required accelerator class") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("engine override matches class", func(t *testing.T) {
		rt := mkRuntime([]string{"H100"})
		cls := "H100"
		isvc := &v1beta1.InferenceService{Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{AcceleratorOverride: &v1beta1.AcceleratorSelector{AcceleratorClass: &cls}}}}

		report, err := matcher.GetCompatibilityDetails(rt, baseModel, isvc, "rt")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible)
	})

	t.Run("decoder override mismatches class", func(t *testing.T) {
		rt := mkRuntime([]string{"nvidia-a100"})
		cls := "H100"
		isvc := &v1beta1.InferenceService{Spec: v1beta1.InferenceServiceSpec{Decoder: &v1beta1.DecoderSpec{AcceleratorOverride: &v1beta1.AcceleratorSelector{AcceleratorClass: &cls}}}}

		report, err := matcher.GetCompatibilityDetails(rt, baseModel, isvc, "rt")
		assert.NoError(t, err)
		assert.False(t, report.IsCompatible)
	})

	t.Run("no accelerator classes in runtime => compatible", func(t *testing.T) {
		rt := mkRuntime([]string{}) // empty means no restriction
		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(rt, baseModel, isvc, "rt")
		assert.NoError(t, err)
		assert.True(t, report.IsCompatible)
	})
}

func TestIsCompatible_DisabledAndAcceleratorErrors(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil))

	baseModel := &v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: "pytorch"}}
	isvc := &v1beta1.InferenceService{}

	t.Run("disabled runtime returns error", func(t *testing.T) {
		rt := &v1beta1.ServingRuntimeSpec{Disabled: ptr(true)}
		ok, err := matcher.IsCompatible(rt, baseModel, isvc, "rt")
		assert.False(t, ok)
		assert.Error(t, err)
		assert.True(t, IsRuntimeDisabledError(err))
	})

	t.Run("accelerator mismatch returns compatibility error", func(t *testing.T) {
		rt := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats:   []v1beta1.SupportedModelFormat{{ModelFormat: &v1beta1.ModelFormat{Name: "pytorch"}}},
			AcceleratorRequirements: &v1beta1.AcceleratorRequirements{AcceleratorClasses: []string{"A100"}},
		}
		cls := "H100"
		isvc := &v1beta1.InferenceService{Spec: v1beta1.InferenceServiceSpec{AcceleratorSelector: &v1beta1.AcceleratorSelector{AcceleratorClass: &cls}}}

		ok, err := matcher.IsCompatible(rt, baseModel, isvc, "rt")
		assert.False(t, ok)
		assert.Error(t, err)
		assert.True(t, IsRuntimeCompatibilityError(err))
	})

	t.Run("size mismatch does not return error, only false", func(t *testing.T) {
		rt := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats:   []v1beta1.SupportedModelFormat{{ModelFormat: &v1beta1.ModelFormat{Name: "pytorch"}}},
			ModelSizeRange:          &v1beta1.ModelSizeRangeSpec{Min: ptr("1B"), Max: ptr("2B")},
			AcceleratorRequirements: &v1beta1.AcceleratorRequirements{},
		}
		model := &v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: "pytorch"}, ModelParameterSize: ptr("70B")}
		ok, err := matcher.IsCompatible(rt, model, isvc, "rt")
		assert.False(t, ok)
		assert.NoError(t, err)
	})
}

func TestCompareModelFormatVersions(t *testing.T) {
	ptrToString := func(s string) *string { return &s }
	ptrToRuntimeOp := func(s string) *v1beta1.RuntimeSelectorOperator {
		op := v1beta1.RuntimeSelectorOperator(s)
		return &op
	}

	matcher := NewDefaultRuntimeMatcher(NewConfig(nil)).(*DefaultRuntimeMatcher)

	tests := []struct {
		name            string
		supportedFormat *v1beta1.ModelFormat
		modelFormat     *v1beta1.ModelFormat
		expected        bool
	}{
		{
			name: "Equal versions with Equal operator",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.0.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.0.0"),
			},
			expected: true,
		},
		{
			name: "Equal versions with GreaterThan operator",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.0.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.0.0"),
			},
			expected: false,
		},
		{
			name: "GreaterThan - supported version is greater",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.8.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.7.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThan - supported version is not greater",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.7.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0"),
			},
			expected: false,
		},
		{
			name: "GreaterThanOrEqual - equal versions",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.8.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThanOrEqual - supported version is greater",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.9.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThanOrEqual - supported version is less",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.7.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0"),
			},
			expected: false,
		},
		{
			name: "Unofficial version forces equality check",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.8.0-dev"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0-dev"),
			},
			expected: true,
		},
		{
			name: "Unofficial version not equal",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.8.0-dev"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.8.0-alpha"),
			},
			expected: false,
		},
		{
			name: "Equal versions with precision 1 and Equal operator",
			supportedFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1"),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1"),
			},
			expected: true,
		},
		{
			name: "Precision mismatch",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("1.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.0.0"),
			},
			expected: false,
		},
		{
			name: "Major prefix mismatch",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("v1.0.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("1.0.0"),
			},
			expected: false,
		},
		{
			name: "Nil operator defaults to Equal",
			supportedFormat: &v1beta1.ModelFormat{
				Name:     "pytorch",
				Version:  ptrToString("v2.0.0"),
				Operator: nil,
			},
			modelFormat: &v1beta1.ModelFormat{
				Name:    "pytorch",
				Version: ptrToString("v2.0.0"),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.compareModelFormatVersions(tt.supportedFormat, tt.modelFormat)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestCompareModelFrameworkVersions(t *testing.T) {
	ptrToString := func(s string) *string { return &s }
	ptrToRuntimeOp := func(s string) *v1beta1.RuntimeSelectorOperator {
		op := v1beta1.RuntimeSelectorOperator(s)
		return &op
	}

	matcher := NewDefaultRuntimeMatcher(NewConfig(nil)).(*DefaultRuntimeMatcher)

	tests := []struct {
		name               string
		supportedFramework *v1beta1.ModelFrameworkSpec
		modelFramework     *v1beta1.ModelFrameworkSpec
		expected           bool
	}{
		{
			name: "Equal versions with Equal operator",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: true,
		},
		{
			name: "Equal versions with GreaterThan operator",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: false,
		},
		{
			name: "GreaterThan - supported version is greater",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.15.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThan - supported version is not greater",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.15.0"),
			},
			expected: false,
		},
		{
			name: "GreaterThanOrEqual - equal versions",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThanOrEqual - supported version is greater",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.15.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: true,
		},
		{
			name: "GreaterThanOrEqual - supported version is less",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.15.0"),
			},
			expected: false,
		},
		{
			name: "Unofficial version forces equality check",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0-dev"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0-dev"),
			},
			expected: true,
		},
		{
			name: "Unofficial version not equal",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10.0-dev"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0-alpha"),
			},
			expected: false,
		},
		{
			name: "Precision mismatch",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("1.10"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: false,
		},
		{
			name: "Major prefix mismatch",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("v1.10.0"),
				Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan)),
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "transformers",
				Version: ptrToString("1.10.0"),
			},
			expected: false,
		},
		{
			name: "Nil operator defaults to Equal",
			supportedFramework: &v1beta1.ModelFrameworkSpec{
				Name:     "transformers",
				Version:  ptrToString("v1.10.0"),
				Operator: nil,
			},
			modelFramework: &v1beta1.ModelFrameworkSpec{
				Name:    "onnxruntime",
				Version: ptrToString("v1.10.0"),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.compareModelFrameworkVersions(tt.supportedFramework, tt.modelFramework)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestGetFormatMismatchReason(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil)).(*DefaultRuntimeMatcher)

	t.Run("architecture mismatch", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:       v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("LlamaForCausalLM"),
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("MistralForCausalLM"),
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "architecture mismatch")
		assert.Contains(t, reason, "LlamaForCausalLM")
		assert.Contains(t, reason, "MistralForCausalLM")
	})

	t.Run("model has architecture but runtime does not", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:       v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("LlamaForCausalLM"),
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: nil,
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "model has architecture LlamaForCausalLM but runtime has no architecture requirement")
	})

	t.Run("quantization mismatch", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:  v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			Quantization: ptr(v1beta1.ModelQuantization("fp8")),
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:  &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			Quantization: ptr(v1beta1.ModelQuantization("int4")),
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "quantization mismatch")
	})

	t.Run("format name mismatch", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{Name: "pytorch", Version: ptr("1.0.0")},
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat: &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "format name mismatch")
	})

	t.Run("framework mismatch", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:    v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr("4.0.0")},
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "pytorch", Version: ptr("2.0.0")},
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "framework name mismatch")
	})

	t.Run("model has framework but runtime does not", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:    v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr("4.0.0")},
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelFramework: nil,
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "model has framework transformers but runtime has no framework requirement")
	})

	t.Run("multiple mismatches combined", func(t *testing.T) {
		model := &v1beta1.BaseModelSpec{
			ModelFormat:       v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("LlamaForCausalLM"),
			Quantization:      ptr(v1beta1.ModelQuantization("fp8")),
		}
		format := v1beta1.SupportedModelFormat{
			ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("MistralForCausalLM"),
			Quantization:      ptr(v1beta1.ModelQuantization("int4")),
		}
		reason := matcher.getFormatMismatchReason(model, format)
		assert.Contains(t, reason, "architecture mismatch")
		assert.Contains(t, reason, "quantization mismatch")
	})
}

func TestGetCompatibilityDetails_DetailedFormatMismatch(t *testing.T) {
	matcher := NewDefaultRuntimeMatcher(NewConfig(nil))

	t.Run("architecture mismatch provides detailed reason", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
					ModelArchitecture: ptr("MistralForCausalLM"),
				},
			},
		}
		model := &v1beta1.BaseModelSpec{
			ModelFormat:       v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
			ModelArchitecture: ptr("LlamaForCausalLM"),
		}

		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(runtime, model, isvc, "test-runtime")
		assert.NoError(t, err)
		assert.False(t, report.IsCompatible)
		assert.NotEmpty(t, report.IncompatibilityReasons)
		assert.Contains(t, report.IncompatibilityReasons[0], "architecture mismatch")
		assert.Contains(t, report.IncompatibilityReasons[0], "LlamaForCausalLM")
		assert.Contains(t, report.IncompatibilityReasons[0], "MistralForCausalLM")
	})

	t.Run("empty supported formats provides clear reason", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{},
		}
		model := &v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{Name: "safetensors", Version: ptr("1.0.0")},
		}

		isvc := &v1beta1.InferenceService{}
		report, err := matcher.GetCompatibilityDetails(runtime, model, isvc, "test-runtime")
		assert.NoError(t, err)
		assert.False(t, report.IsCompatible)
		assert.NotEmpty(t, report.IncompatibilityReasons)
		assert.Contains(t, report.IncompatibilityReasons[0], "no supported formats defined")
	})
}
