package modelconfig

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHFQuantConfig_RealisticModelOptInput(t *testing.T) {
	// Verbatim hf_quant_config.json from nvidia/Llama-3.3-70B-Instruct-FP4
	// If the upstream ModelOpt JSON shape
	// ever changes, this fixture should change with it — the test covers
	// the wire format, not the abstract API.
	data := []byte(`{
		"producer": {
			"name": "modelopt",
			"version": "0.23.0"
		},
		"quantization": {
			"quant_algo": "NVFP4",
			"kv_cache_quant_algo": "FP8",
			"group_size": 16,
			"exclude_modules": [
				"lm_head"
			]
		}
	}`)

	cfg, err := ParseHFQuantConfig(data)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	// Real ModelOpt exports identify the tool under "producer", not a
	// top-level "quant_method" — so QuantMethod parses as empty here.
	assert.Empty(t, cfg.QuantMethod)
	assert.Equal(t, "NVFP4", cfg.Quantization.QuantAlgo)
	assert.Equal(t, 16, cfg.Quantization.GroupSize)
	assert.Equal(t, "FP8", cfg.Quantization.KVCacheQuantAlgo)
	assert.Equal(t, []string{"lm_head"}, cfg.Quantization.ExcludeModules)
}

func TestParseHFQuantConfig_QuantMethodField(t *testing.T) {
	// Some pipelines surface the config.json-style "quant_method" key at
	// the top level; keep the field wired even though ModelOpt's own
	// hf_quant_config.json reports the tool under "producer" instead.
	cfg, err := ParseHFQuantConfig([]byte(`{"quant_method": "modelopt", "quantization": {"quant_algo": "NVFP4"}}`))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "modelopt", cfg.QuantMethod)
	assert.Equal(t, "NVFP4", cfg.Quantization.QuantAlgo)
}

func TestParseHFQuantConfig_RealisticMoEExcludeList(t *testing.T) {
	// Verbatim hf_quant_config.json from nvidia/Qwen3-235B-A22B-NVFP4:
	// a MoE checkpoint where every expert-router gate (94 layers) plus
	// lm_head is kept at higher precision. Exercises a large multi-entry
	// exclude_modules list on the real wire format.
	data := []byte(`{
		"producer": {
			"name": "modelopt",
			"version": "0.33.0"
		},
		"quantization": {
			"quant_algo": "NVFP4",
			"kv_cache_quant_algo": "FP8",
			"group_size": 16,
			"exclude_modules": [
				"model.layers.0.mlp.gate", "model.layers.1.mlp.gate", "model.layers.10.mlp.gate",
				"model.layers.11.mlp.gate", "model.layers.12.mlp.gate", "model.layers.13.mlp.gate",
				"model.layers.14.mlp.gate", "model.layers.15.mlp.gate", "model.layers.16.mlp.gate",
				"model.layers.17.mlp.gate", "model.layers.18.mlp.gate", "model.layers.19.mlp.gate",
				"model.layers.2.mlp.gate", "model.layers.20.mlp.gate", "model.layers.21.mlp.gate",
				"model.layers.22.mlp.gate", "model.layers.23.mlp.gate", "model.layers.24.mlp.gate",
				"model.layers.25.mlp.gate", "model.layers.26.mlp.gate", "model.layers.27.mlp.gate",
				"model.layers.28.mlp.gate", "model.layers.29.mlp.gate", "model.layers.3.mlp.gate",
				"model.layers.30.mlp.gate", "model.layers.31.mlp.gate", "model.layers.32.mlp.gate",
				"model.layers.33.mlp.gate", "model.layers.34.mlp.gate", "model.layers.35.mlp.gate",
				"model.layers.36.mlp.gate", "model.layers.37.mlp.gate", "model.layers.38.mlp.gate",
				"model.layers.39.mlp.gate", "model.layers.4.mlp.gate", "model.layers.40.mlp.gate",
				"model.layers.41.mlp.gate", "model.layers.42.mlp.gate", "model.layers.43.mlp.gate",
				"model.layers.44.mlp.gate", "model.layers.45.mlp.gate", "model.layers.46.mlp.gate",
				"model.layers.47.mlp.gate", "model.layers.48.mlp.gate", "model.layers.49.mlp.gate",
				"model.layers.5.mlp.gate", "model.layers.50.mlp.gate", "model.layers.51.mlp.gate",
				"model.layers.52.mlp.gate", "model.layers.53.mlp.gate", "model.layers.54.mlp.gate",
				"model.layers.55.mlp.gate", "model.layers.56.mlp.gate", "model.layers.57.mlp.gate",
				"model.layers.58.mlp.gate", "model.layers.59.mlp.gate", "model.layers.6.mlp.gate",
				"model.layers.60.mlp.gate", "model.layers.61.mlp.gate", "model.layers.62.mlp.gate",
				"model.layers.63.mlp.gate", "model.layers.64.mlp.gate", "model.layers.65.mlp.gate",
				"model.layers.66.mlp.gate", "model.layers.67.mlp.gate", "model.layers.68.mlp.gate",
				"model.layers.69.mlp.gate", "model.layers.7.mlp.gate", "model.layers.70.mlp.gate",
				"model.layers.71.mlp.gate", "model.layers.72.mlp.gate", "model.layers.73.mlp.gate",
				"model.layers.74.mlp.gate", "model.layers.75.mlp.gate", "model.layers.76.mlp.gate",
				"model.layers.77.mlp.gate", "model.layers.78.mlp.gate", "model.layers.79.mlp.gate",
				"model.layers.8.mlp.gate", "model.layers.80.mlp.gate", "model.layers.81.mlp.gate",
				"model.layers.82.mlp.gate", "model.layers.83.mlp.gate", "model.layers.84.mlp.gate",
				"model.layers.85.mlp.gate", "model.layers.86.mlp.gate", "model.layers.87.mlp.gate",
				"model.layers.88.mlp.gate", "model.layers.89.mlp.gate", "model.layers.9.mlp.gate",
				"model.layers.90.mlp.gate", "model.layers.91.mlp.gate", "model.layers.92.mlp.gate",
				"model.layers.93.mlp.gate",
				"lm_head"
			]
		}
	}`)

	cfg, err := ParseHFQuantConfig(data)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "NVFP4", cfg.Quantization.QuantAlgo)
	assert.Equal(t, 16, cfg.Quantization.GroupSize)
	assert.Equal(t, "FP8", cfg.Quantization.KVCacheQuantAlgo)

	// The file lists the 94 gate modules in lexicographic order
	// (0, 1, 10, 11, …, 19, 2, 20, …) with lm_head appended last —
	// rebuild that expectation instead of duplicating 95 literals.
	gates := make([]string, 0, 94)
	for i := 0; i < 94; i++ {
		gates = append(gates, fmt.Sprintf("model.layers.%d.mlp.gate", i))
	}
	sort.Strings(gates)
	expected := append(gates, "lm_head")

	assert.Len(t, cfg.Quantization.ExcludeModules, 95)
	assert.Equal(t, expected, cfg.Quantization.ExcludeModules)
}

func TestParseHFQuantConfig_EmptyBytes(t *testing.T) {
	// "Optional file" pattern: callers pass nil/[] to skip the
	// fallback path. Returning (nil, nil) lets them check len() != 0
	// before attempting the call.
	cfg, err := ParseHFQuantConfig(nil)
	assert.NoError(t, err)
	assert.Nil(t, cfg)

	cfg, err = ParseHFQuantConfig([]byte{})
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestParseHFQuantConfig_MalformedJSON(t *testing.T) {
	cfg, err := ParseHFQuantConfig([]byte(`{"quant_method": "modelopt"`))
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestParseHFQuantConfig_MinimalJSON(t *testing.T) {
	// Only quant_algo populated — covers the case where a tool emits
	// the file but doesn't bother with group_size / exclude_modules.
	// Mapping should still work since that's the only field OME reads
	// today.
	cfg, err := ParseHFQuantConfig([]byte(`{"quantization": {"quant_algo": "FP8"}}`))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "FP8", cfg.Quantization.QuantAlgo)
	assert.Equal(t, 0, cfg.Quantization.GroupSize)
	assert.Empty(t, cfg.Quantization.ExcludeModules)
}
