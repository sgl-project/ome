package modelconfig

import (
	"encoding/json"
	"fmt"
)

// HFQuantConfig is the schema for hf_quant_config.json, a sibling
// file to config.json shipped by NVIDIA ModelOpt / TensorRT-LLM. The
// HF-native pipelines surface the same data inside config.json under
// "quantization_config", but ModelOpt-style models often don't modify
// config.json — OME readers honor either source.
type HFQuantConfig struct {
	QuantMethod  string               `json:"quant_method"`
	Quantization HFQuantConfigDetails `json:"quantization"`
}

// HFQuantConfigDetails mirrors ModelOpt's "quantization" sub-object.
// OME currently reads QuantAlgo + ExcludeModules; the rest are
// informational.
type HFQuantConfigDetails struct {
	// QuantAlgo: "FP8", "NVFP4", "MXFP4", "INT4_AWQ", "INT8_SQ", "W4A8_AWQ", etc.
	// Drives spec.quantization via QuantAlgoToOMEEnum.
	QuantAlgo string `json:"quant_algo"`
	// GroupSize: per-group scale size for grouped formats (NVFP4 = 16).
	GroupSize int `json:"group_size,omitempty"`
	// KVCacheQuantAlgo: KV-cache scheme (often "FP8" when weights are NVFP4).
	KVCacheQuantAlgo string `json:"kv_cache_quant_algo,omitempty"`
	// ExcludeModules: tensor-name patterns kept at higher precision —
	// required for accurate counting on packed-quant models.
	ExcludeModules []string `json:"exclude_modules,omitempty"`
}

const HFQuantConfigFileName = "hf_quant_config.json"

// ParseHFQuantConfig returns nil + nil on empty input so callers can
// use it for the "file is optional" pattern.
func ParseHFQuantConfig(data []byte) (*HFQuantConfig, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var cfg HFQuantConfig
	if err := json.Unmarshal(SanitizeJSONBytes(data), &cfg); err != nil {
		return nil, fmt.Errorf("parse hf_quant_config.json: %w", err)
	}
	return &cfg, nil
}
