package modelparser

import (
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestQuantAlgoToOMEEnum(t *testing.T) {
	tests := []struct {
		// Each name + input pair covers one realistic input the parser
		// encounters across the various tools that produce quantized
		// HF / safetensors / ModelOpt bundles. Adding a new quant
		// scheme means adding a row here, not editing the matcher.
		name string
		in   string
		want v1beta1.ModelQuantization
	}{
		// HF-native config.json values
		{name: "lowercase fp8", in: "fp8", want: v1beta1.ModelQuantizationFP8},
		{name: "fp8 per tensor variant", in: "fp8_per_tensor", want: v1beta1.ModelQuantizationFP8},
		{name: "uppercase FP8", in: "FP8", want: v1beta1.ModelQuantizationFP8},
		{name: "fbgemm_fp8 distinct from plain fp8", in: "fbgemm_fp8", want: v1beta1.ModelQuantizationFbgemmFP8},
		{name: "int4 plain", in: "int4", want: v1beta1.ModelQuantizationINT4},
		{name: "int4 AWQ variant", in: "INT4_AWQ", want: v1beta1.ModelQuantizationINT4},
		{name: "int4 GPTQ variant", in: "int4_gptq", want: v1beta1.ModelQuantizationINT4},
		{name: "w4a8 AWQ also INT4", in: "W4A8_AWQ", want: v1beta1.ModelQuantizationINT4},
		{name: "w4a16 AWQ also INT4", in: "w4a16_awq", want: v1beta1.ModelQuantizationINT4},

		// ModelOpt hf_quant_config.json values
		{name: "ModelOpt NVFP4", in: "NVFP4", want: v1beta1.ModelQuantizationNVFP4},
		{name: "lowercase nvfp4", in: "nvfp4", want: v1beta1.ModelQuantizationNVFP4},
		{name: "OCP MXFP4", in: "MXFP4", want: v1beta1.ModelQuantizationMXFP4},
		{name: "lowercase mxfp4", in: "mxfp4", want: v1beta1.ModelQuantizationMXFP4},

		// compressed-tensors container format (vLLM / llm-compressor) —
		// label only; the precision lives in config_groups, not the name.
		{name: "compressed-tensors hyphen", in: "compressed-tensors", want: v1beta1.ModelQuantizationCompressedTensors},
		{name: "compressed_tensors underscore", in: "compressed_tensors", want: v1beta1.ModelQuantizationCompressedTensors},
		{name: "compressed-tensors mixed case", in: "Compressed-Tensors", want: v1beta1.ModelQuantizationCompressedTensors},

		// Negatives
		{name: "empty string", in: "", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		{name: "unknown algorithm", in: "some-future-quant-format", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := QuantAlgoToOMEEnum(tc.in)
			if got != tc.want {
				t.Errorf("QuantAlgoToOMEEnum(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
