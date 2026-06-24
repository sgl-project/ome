package modelparser

import (
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// QuantAlgoToOMEEnum maps a quantization algorithm name (from either
// config.json's quantization_config.quant_method or hf_quant_config.json's
// quantization.quant_algo) to the OME enum.
//
// Returns "" for unrecognized values so callers leave spec.quantization
// nil rather than write a misleading placeholder. Matching is fuzzy
// (case-insensitive, contains) because upstream tools mix conventions:
// "fp8" / "FP8" / "fp8_per_tensor" / "INT4_AWQ" / "w4a16_awq" all
// collapse to the same enum.
func QuantAlgoToOMEEnum(s string) v1beta1.ModelQuantization {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return ""
	}
	switch {
	case v == "fbgemm_fp8":
		return v1beta1.ModelQuantizationFbgemmFP8
	case v == "nvfp4":
		return v1beta1.ModelQuantizationNVFP4
	case v == "mxfp4":
		return v1beta1.ModelQuantizationMXFP4
	case strings.Contains(v, "int4") || strings.Contains(v, "w4a"):
		return v1beta1.ModelQuantizationINT4
	case strings.Contains(v, "fp8"):
		// fbgemm_fp8 is matched above; this is the catch-all for plain
		// fp8, fp8_per_tensor, fp8_e4m3, etc.
		return v1beta1.ModelQuantizationFP8
	}
	return ""
}
