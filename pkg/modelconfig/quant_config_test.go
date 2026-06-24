package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHFQuantConfig_RealisticModelOptInput(t *testing.T) {
	// Verbatim from the Llama4x NVFP4 PTQ model that prompted this
	// fix. If the upstream ModelOpt JSON shape ever changes, this
	// fixture should change with it — the test covers the wire
	// format, not the abstract API.
	data := []byte(`{
		"quant_method": "modelopt",
		"quantization": {
			"quant_algo": "NVFP4",
			"group_size": 16,
			"kv_cache_quant_algo": "FP8",
			"exclude_modules": ["layers.0.", "layers.61.", "self_attn", "lm_head"]
		}
	}`)

	cfg, err := ParseHFQuantConfig(data)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "modelopt", cfg.QuantMethod)
	assert.Equal(t, "NVFP4", cfg.Quantization.QuantAlgo)
	assert.Equal(t, 16, cfg.Quantization.GroupSize)
	assert.Equal(t, "FP8", cfg.Quantization.KVCacheQuantAlgo)
	assert.Equal(t, []string{"layers.0.", "layers.61.", "self_attn", "lm_head"}, cfg.Quantization.ExcludeModules)
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
