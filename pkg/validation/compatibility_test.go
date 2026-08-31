package validation

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestCompareModelToFormat(t *testing.T) {
	tests := []struct {
		name   string
		model  *v1beta1.BaseModelSpec
		format v1beta1.SupportedModelFormat
		opts   []CompatibilityOption
		want   bool
	}{
		{
			name: "exact match",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			want: true,
		},
		{
			name: "format name mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "pytorch"},
			},
			want: false,
		},
		{
			name: "nil format.ModelFormat",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{},
			want:   false,
		},
		{
			name: "architecture mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("MistralForCausalLM"),
			},
			want: false,
		},
		{
			name: "model has architecture but runtime does not (asymmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "both nil architecture matches",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: true,
		},
		{
			name: "quantization mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationFP8),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:  &v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationINT4),
			},
			want: false,
		},
		{
			name: "framework mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "tensorrtllm"},
			},
			want: false,
		},
		{
			name: "model has framework but runtime does not (asymmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "format version mismatch (symmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "format version match",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			want: true,
		},
		{
			name: "sharded model without cache provider option skips cache check",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Distribution: ptr.To(v1beta1.DistributionSharded),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: true,
		},
		{
			name: "sharded model with cache provider option and no support",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Distribution: ptr.To(v1beta1.DistributionSharded),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			opts: []CompatibilityOption{WithModelCacheProvider("alluxio")},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CompareModelToFormat(tc.model, tc.format, tc.opts...)
			if got != tc.want {
				t.Fatalf("CompareModelToFormat() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetFormatMismatchReason(t *testing.T) {
	tests := []struct {
		name     string
		model    *v1beta1.BaseModelSpec
		format   v1beta1.SupportedModelFormat
		contains string
	}{
		{
			name: "architecture mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("MistralForCausalLM"),
			},
			contains: "architecture mismatch",
		},
		{
			name: "format name mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			contains: "format name mismatch",
		},
		{
			name: "quantization mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationFP8),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:  &v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationINT4),
			},
			contains: "quantization mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason := GetFormatMismatchReason(tc.model, tc.format)
			if reason == "" {
				t.Fatal("expected non-empty reason")
			}
			if !strings.Contains(reason, tc.contains) {
				t.Fatalf("reason %q does not contain %q", reason, tc.contains)
			}
		})
	}
}
