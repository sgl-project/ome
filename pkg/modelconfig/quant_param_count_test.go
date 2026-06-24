package modelconfig

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// safetensorsHeaderForTest builds an in-memory safetensors header
// (8-byte little-endian length prefix + JSON tensor map). Lets the
// per-tensor classification tests synthesize fixtures inline instead
// of shipping multi-GB binaries.
func safetensorsHeaderForTest(t *testing.T, tensors map[string]map[string]any) []byte {
	t.Helper()
	jsonBytes, err := json.Marshal(tensors)
	require.NoError(t, err)
	out := make([]byte, 8+len(jsonBytes))
	binary.LittleEndian.PutUint64(out[:8], uint64(len(jsonBytes)))
	copy(out[8:], jsonBytes)
	return out
}

// safetensorsIndexFile is the minimal shape of model.safetensors.index.json.
// We only read weight_map (tensor name → shard file) for the production
// fixture tests; the rest (metadata.total_size, etc.) is informational.
type safetensorsIndexFile struct {
	WeightMap map[string]string `json:"weight_map"`
}

// loadIndexTensorNames returns the tensor names from a real
// model.safetensors.index.json fixture under testdata/quant/<modelSlug>/.
// Used by the production-fixture tests below.
func loadIndexTensorNames(t *testing.T, modelSlug string) []string {
	t.Helper()
	indexPath := filepath.Join("testdata", "quant", modelSlug, "model.safetensors.index.json")
	data, err := os.ReadFile(indexPath)
	require.NoError(t, err, "fixture %s", indexPath)
	var idx safetensorsIndexFile
	require.NoError(t, json.Unmarshal(data, &idx))
	names := make([]string, 0, len(idx.WeightMap))
	for name := range idx.WeightMap {
		names = append(names, name)
	}
	return names
}

// ─── IsScaleTensor ───────────────────────────────────────────────────

func TestIsScaleTensor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		// ModelOpt / TensorRT-LLM dot-convention
		{name: "weight_scale per-tensor", in: "model.layers.5.mlp.gate_proj.weight_scale", want: true},
		{name: "weight_scale_2 group scale", in: "model.layers.5.mlp.gate_proj.weight_scale_2", want: true},
		{name: "input_scale activation", in: "model.layers.5.mlp.gate_proj.input_scale", want: true},
		{name: "kv cache scaling factor", in: "model.layers.5.self_attn.k_proj.kv_cache_scaling_factor", want: true},
		// AWQ / GPTQ
		{name: "AWQ scales", in: "model.layers.5.mlp.gate_proj.scales", want: true},
		{name: "AWQ qzeros", in: "model.layers.5.mlp.gate_proj.qzeros", want: true},
		{name: "GPTQ g_idx", in: "model.layers.5.mlp.gate_proj.g_idx", want: true},
		{name: "zero_point", in: "model.layers.5.mlp.gate_proj.weight_zero_point", want: true},
		// Negatives — actual weights/biases must NOT be classified as scales
		{name: "regular weight", in: "model.layers.5.mlp.gate_proj.weight", want: false},
		{name: "regular bias", in: "model.layers.5.mlp.gate_proj.bias", want: false},
		{name: "AWQ qweight (real packed weight)", in: "model.layers.5.mlp.gate_proj.qweight", want: false},
		{name: "embedding", in: "model.embed_tokens.weight", want: false},
		{name: "lm_head weight", in: "lm_head.weight", want: false},
		// Edge cases
		{name: "empty string", in: "", want: false},
		// "scaled_weight" basename ends with "weight" not "_scale" — must NOT match.
		// Belt-and-suspenders for the leading-underscore anchor in scaleSuffixes.
		{name: "false positive guard: scaled_weight", in: "model.layers.5.scaled_weight", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsScaleTensor(tc.in); got != tc.want {
				t.Errorf("IsScaleTensor(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsScaleTensor_NVFP4ProductionFixture(t *testing.T) {
	// Real NVIDIA Llama-3.1-8B-Instruct-FP4 (NVFP4 PTQ via ModelOpt).
	// Verifies the matcher correctly flips for every scale tensor in
	// the production model and ignores every actual weight.
	names := loadIndexTensorNames(t, "nvidia-llama-3-1-8b-nvfp4")

	var weightCount, scaleCount int
	for _, name := range names {
		isScale := IsScaleTensor(name)
		switch {
		case strings.HasSuffix(name, "weight_scale") ||
			strings.HasSuffix(name, "weight_scale_2") ||
			strings.HasSuffix(name, "input_scale") ||
			strings.HasSuffix(name, "k_scale") ||
			strings.HasSuffix(name, "v_scale"):
			scaleCount++
			assert.True(t, isScale, "expected scale tensor %s to match", name)
		case strings.HasSuffix(name, ".weight"):
			weightCount++
			assert.False(t, isScale, "real weight %s must NOT match scale", name)
		}
	}
	assert.Greater(t, weightCount, 100, "fixture must contain real weights")
	assert.Greater(t, scaleCount, 100, "fixture must contain scale tensors that the matcher catches")
}

func TestIsScaleTensor_KimiK2FP8ProductionFixture(t *testing.T) {
	// Real Kimi-K2-Instruct (FP8 with weight_scale_inv suffix — distinct
	// from ModelOpt's _scale_2 convention). Catches the scale_inv
	// regression that initially slipped through.
	names := loadIndexTensorNames(t, "kimi-k2-instruct-fp8")

	var scaleInvCount int
	for _, name := range names {
		if !strings.HasSuffix(name, "weight_scale_inv") {
			continue
		}
		scaleInvCount++
		assert.True(t, IsScaleTensor(name),
			"weight_scale_inv tensor %s must be classified as scale", name)
	}
	assert.Greater(t, scaleInvCount, 50, "kimi-k2 fixture must contain enough weight_scale_inv tensors to be a meaningful test")
}

func TestIsScaleTensor_GptOssMxfp4ProductionFixture(t *testing.T) {
	// Real openai/gpt-oss-20b (MXFP4). Distinct because it uses the
	// underscore-suffix naming convention: gate_up_proj_blocks (real
	// packed weight) paired with gate_up_proj_scales (scale). The
	// suffix matcher must catch _scales but not _blocks.
	names := loadIndexTensorNames(t, "openai-gpt-oss-20b-mxfp4")

	var scalesCount, blocksCount int
	for _, name := range names {
		switch {
		case strings.HasSuffix(name, "_scales"):
			scalesCount++
			assert.True(t, IsScaleTensor(name),
				"underscore-suffix scale tensor %s must match", name)
		case strings.HasSuffix(name, "_blocks"):
			blocksCount++
			assert.False(t, IsScaleTensor(name),
				"underscore-suffix WEIGHT tensor %s must NOT match scale (it's the actual packed data)", name)
		}
	}
	assert.Greater(t, scalesCount, 10, "gpt-oss fixture must contain underscore-suffix scales")
	assert.Greater(t, blocksCount, 10, "gpt-oss fixture must contain underscore-suffix _blocks weights")
}

// ─── IsExcludedTensor ────────────────────────────────────────────────

func TestIsExcludedTensor(t *testing.T) {
	excludeModules := []string{"layers.0.", "layers.61.", "self_attn", "lm_head"}

	tests := []struct {
		name   string
		tensor string
		want   bool
	}{
		// Layer prefix patterns
		{name: "first layer matches layers.0.", tensor: "model.layers.0.mlp.gate_proj.weight", want: true},
		{name: "last layer matches layers.61.", tensor: "model.layers.61.mlp.gate_proj.weight", want: true},
		// Module-name patterns (substring across the dot path)
		{name: "self_attn anywhere", tensor: "model.layers.5.self_attn.k_proj.weight", want: true},
		{name: "lm_head anywhere", tensor: "lm_head.weight", want: true},
		// Negatives — middle layers MLP weights are packed-quant, NOT excluded
		{name: "middle layer MLP weight", tensor: "model.layers.30.mlp.gate_proj.weight", want: false},
		{name: "middle layer router", tensor: "model.layers.30.block_sparse_moe.gate.weight", want: false},
		{name: "embedding stays excluded? no", tensor: "model.embed_tokens.weight", want: false},
		// Edge cases
		{name: "empty tensor name", tensor: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsExcludedTensor(tc.tensor, excludeModules); got != tc.want {
				t.Errorf("IsExcludedTensor(%q) = %v, want %v", tc.tensor, got, tc.want)
			}
		})
	}

	t.Run("empty exclude list", func(t *testing.T) {
		if got := IsExcludedTensor("model.layers.5.mlp.gate_proj.weight", nil); got {
			t.Errorf("expected false for nil exclude list")
		}
	})
}

func TestIsExcludedTensor_GlobPatternFromGptOssConfig(t *testing.T) {
	// Real gpt-oss-20b modules_to_not_convert patterns include globs:
	//   "model.layers.*.self_attn"
	//   "model.layers.*.mlp.router"
	//   "model.embed_tokens"  (no glob)
	//   "lm_head"             (no glob)
	excludeModules := []string{
		"model.layers.*.self_attn",
		"model.layers.*.mlp.router",
		"model.embed_tokens",
		"lm_head",
	}

	tests := []struct {
		name   string
		tensor string
		want   bool
	}{
		// Glob expansion: * matches one dot-segment (here, the layer index)
		{name: "self_attn glob matches layer 0", tensor: "model.layers.0.self_attn.k_proj.weight", want: true},
		{name: "self_attn glob matches layer 17", tensor: "model.layers.17.self_attn.q_proj.weight", want: true},
		{name: "router glob matches", tensor: "model.layers.5.mlp.router.weight", want: true},
		// Plain substring patterns
		{name: "embed_tokens substring", tensor: "model.embed_tokens.weight", want: true},
		{name: "lm_head substring", tensor: "lm_head.weight", want: true},
		// Negatives — MoE expert weights stay packed
		{name: "expert weight not excluded", tensor: "model.layers.5.mlp.experts.gate_up_proj_blocks", want: false},
		{name: "input_layernorm not excluded", tensor: "model.layers.5.input_layernorm.weight", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsExcludedTensor(tc.tensor, excludeModules)
			assert.Equal(t, tc.want, got, "tensor=%s", tc.tensor)
		})
	}
}

func TestIsExcludedTensor_FullModulePathFromFbgemmFP8Config(t *testing.T) {
	// Real Llama-3.1-405B-FP8 modules_to_not_convert is a list of
	// FULL module paths (no globs). Verify substring fallback works.
	excludeModules := []string{
		"model.layers.0.mlp.down_proj",
		"model.layers.0.mlp.gate_proj",
		"model.layers.0.mlp.up_proj",
		"model.layers.125.mlp.down_proj",
		"model.layers.0.self_attn.k_proj",
	}

	tests := []struct {
		name   string
		tensor string
		want   bool
	}{
		// Full-path matches
		{name: "layer 0 down_proj weight", tensor: "model.layers.0.mlp.down_proj.weight", want: true},
		{name: "layer 0 down_proj scale", tensor: "model.layers.0.mlp.down_proj.weight_scale", want: true},
		{name: "layer 125 down_proj", tensor: "model.layers.125.mlp.down_proj.weight", want: true},
		{name: "layer 0 self_attn k_proj", tensor: "model.layers.0.self_attn.k_proj.weight", want: true},
		// Negatives — middle layers stay packed
		{name: "layer 50 down_proj packed", tensor: "model.layers.50.mlp.down_proj.weight", want: false},
		{name: "layer 0 v_proj NOT in list", tensor: "model.layers.0.self_attn.v_proj.weight", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsExcludedTensor(tc.tensor, excludeModules)
			assert.Equal(t, tc.want, got, "tensor=%s", tc.tensor)
		})
	}
}

// ─── QuantPackingFactor ──────────────────────────────────────────────

func TestQuantPackingFactor(t *testing.T) {
	tests := []struct {
		name string
		algo string
		dt   string
		want int64
	}{
		// NVFP4 / MXFP4 / FP4 packed two per byte
		{name: "NVFP4 in U8 → 2", algo: "NVFP4", dt: "U8", want: 2},
		{name: "MXFP4 in U8 → 2", algo: "MXFP4", dt: "U8", want: 2},
		{name: "FP4 in I8 → 2", algo: "FP4", dt: "I8", want: 2},
		{name: "lowercase nvfp4 + uint8 alias → 2", algo: "nvfp4", dt: "uint8", want: 2},
		// INT4 packed differently depending on storage element size
		{name: "INT4_AWQ in I32 → 8", algo: "INT4_AWQ", dt: "I32", want: 8},
		{name: "INT4_GPTQ in I32 → 8", algo: "INT4_GPTQ", dt: "I32", want: 8},
		{name: "W4A8_AWQ in U8 → 2", algo: "W4A8_AWQ", dt: "U8", want: 2},
		{name: "INT4 in I16 → 4", algo: "INT4", dt: "I16", want: 4},
		// FP8 stored as F8 / U8 → 1 (no packing, 8 bits per element)
		{name: "FP8 in F8_E4M3 → 1", algo: "FP8", dt: "F8_E4M3", want: 1},
		{name: "FP8 in U8 → 1", algo: "FP8", dt: "U8", want: 1},
		// Native high-precision dtypes never get a multiplier
		{name: "NVFP4 weights stored as F16 (excluded module case) → 1", algo: "NVFP4", dt: "F16", want: 1},
		{name: "INT4_AWQ in BF16 → 1", algo: "INT4_AWQ", dt: "BF16", want: 1},
		// No quant config / unknown algo → 1
		{name: "empty algo → 1", algo: "", dt: "U8", want: 1},
		{name: "unknown algo → 1", algo: "some-future-format", dt: "U8", want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuantPackingFactor(tc.algo, tc.dt); got != tc.want {
				t.Errorf("QuantPackingFactor(%q, %q) = %d, want %d", tc.algo, tc.dt, got, tc.want)
			}
		})
	}
}

// ─── CountTensorParams ───────────────────────────────────────────────

func TestCountTensorParams_OverflowProtection(t *testing.T) {
	// Defensive: a corrupted shape with a bad dim should produce 0
	// (skipped) rather than a panic or wraparound.
	got := CountTensorParams("model.weight", []int64{0, 100}, "F16", nil)
	assert.Equal(t, int64(0), got, "zero dimension yields 0")

	got = CountTensorParams("model.weight", []int64{-1, 100}, "F16", nil)
	assert.Equal(t, int64(0), got, "negative dimension yields 0")

	got = CountTensorParams("model.weight", nil, "F16", nil)
	assert.Equal(t, int64(0), got, "nil shape yields 0")
}

// ─── ParseSafetensorsHeaderQuantAware ────────────────────────────────

func TestParseSafetensorsHeaderQuantAware_NoQuantConfigEqualsNaiveCount(t *testing.T) {
	// Backward compat: when quantConfig is nil, the function must
	// return the same naive sum as the legacy ParseSafetensorsHeader.
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.0.weight": {
			"shape": []int64{4096, 4096}, "dtype": "F16", "data_offsets": []int64{0, 4096 * 4096 * 2},
		},
		"model.layers.1.weight": {
			"shape": []int64{1024, 1024}, "dtype": "F16", "data_offsets": []int64{0, 1024 * 1024 * 2},
		},
	})

	naive, err := ParseSafetensorsHeader(header[8:], "test")
	require.NoError(t, err)
	quantAware, err := ParseSafetensorsHeaderQuantAware(header[8:], "test", nil)
	require.NoError(t, err)

	assert.Equal(t, naive, quantAware,
		"with nil quantConfig the two functions must agree (backward compat)")
	assert.Equal(t, int64(4096*4096+1024*1024), quantAware)
}

func TestParseSafetensorsHeaderQuantAware_NVFP4PackedTensorsGet2xMultiplier(t *testing.T) {
	// NVFP4 weight stored as U8[out, in/2] — naive numel is HALF the
	// logical params. With a NVFP4 quant config the function returns 2x.
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.5.mlp.gate_proj.weight": {
			// 1024 × 512 storage bytes; logical = 1024 × 1024 = 1048576
			"shape": []int64{1024, 512}, "dtype": "U8", "data_offsets": []int64{0, 1024 * 512},
		},
	})

	count, err := ParseSafetensorsHeaderQuantAware(header[8:], "test", &HFQuantConfig{
		QuantMethod:  "modelopt",
		Quantization: HFQuantConfigDetails{QuantAlgo: "NVFP4"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1024*1024), count,
		"NVFP4 U8 storage must be multiplied by 2 to recover logical param count")
}

func TestParseSafetensorsHeaderQuantAware_MixedPackedAndExcludedAndScales(t *testing.T) {
	// Realistic mini-fixture covering all three classification branches:
	//   excluded FP16:   layers.0.self_attn.k_proj.weight     (10 params, full precision)
	//   packed NVFP4:    layers.30.mlp.gate_proj.weight       (200 params logical, 100 stored)
	//   scale tensor:    layers.30.mlp.gate_proj.weight_scale (skipped entirely)
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.0.self_attn.k_proj.weight":     {"shape": []int64{2, 5}, "dtype": "F16", "data_offsets": []int64{0, 20}},
		"model.layers.30.mlp.gate_proj.weight":       {"shape": []int64{10, 10}, "dtype": "U8", "data_offsets": []int64{0, 100}},
		"model.layers.30.mlp.gate_proj.weight_scale": {"shape": []int64{10}, "dtype": "F32", "data_offsets": []int64{0, 40}},
	})

	count, err := ParseSafetensorsHeaderQuantAware(header[8:], "test", &HFQuantConfig{
		QuantMethod: "modelopt",
		Quantization: HFQuantConfigDetails{
			QuantAlgo:      "NVFP4",
			ExcludeModules: []string{"layers.0.", "self_attn", "lm_head"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(210), count,
		"mixed fixture: excluded FP16 + packed NVFP4 + skipped scale should sum to 210 logical params")

	naive, err := ParseSafetensorsHeader(header[8:], "test")
	require.NoError(t, err)
	assert.Equal(t, int64(120), naive,
		"sanity: legacy naive count is 120 (sum of all shape products including the scale tensor)")
}

func TestParseSafetensorsHeaderQuantAware_ScaleTensorsSkippedRegardlessOfQuantConfig(t *testing.T) {
	// Scale skip is gated on quantConfig presence: legacy callers
	// (no quantConfig) still get the naive sum INCLUDING any scale
	// tensors that happen to be in the header — backward compat.
	// With a quant config, scales ARE skipped.
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.5.mlp.gate_proj.weight":       {"shape": []int64{100}, "dtype": "F16", "data_offsets": []int64{0, 200}},
		"model.layers.5.mlp.gate_proj.weight_scale": {"shape": []int64{10}, "dtype": "F32", "data_offsets": []int64{0, 40}},
	})

	naive, err := ParseSafetensorsHeader(header[8:], "test")
	require.NoError(t, err)
	assert.Equal(t, int64(110), naive, "naive includes the scale tensor (110 = 100 + 10)")

	withQuant, err := ParseSafetensorsHeaderQuantAware(header[8:], "test", &HFQuantConfig{
		Quantization: HFQuantConfigDetails{QuantAlgo: "NVFP4"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(100), withQuant, "with quant config the scale tensor is skipped")
}

func TestParseSafetensorsHeaderQuantAware_NVFP4WithRealNamesAndExcludeModule(t *testing.T) {
	// End-to-end with patterns lifted directly from the NVIDIA
	// Llama-3.1-8B-FP4 hf_quant_config.json (exclude_modules:
	// ["lm_head"]). Verifies that lm_head stays at naive count
	// while regular weights get the 2x NVFP4 multiplier.
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"lm_head.weight": {
			"shape": []int64{1000}, "dtype": "F16", "data_offsets": []int64{0, 2000},
		},
		"model.layers.5.mlp.gate_proj.weight": {
			// Packed NVFP4 U8 → 100 storage * 2 = 200 logical
			"shape": []int64{10, 10}, "dtype": "U8", "data_offsets": []int64{0, 100},
		},
		"model.layers.5.mlp.gate_proj.weight_scale": {
			"shape": []int64{10}, "dtype": "F32", "data_offsets": []int64{0, 40},
		},
		"model.layers.5.self_attn.k_proj.k_scale": {
			// NEW pattern from NVFP4 fixture — must be classified as scale
			"shape": []int64{1}, "dtype": "F32", "data_offsets": []int64{0, 4},
		},
	})

	count, err := ParseSafetensorsHeaderQuantAware(header[8:], "test", &HFQuantConfig{
		QuantMethod: "modelopt",
		Quantization: HFQuantConfigDetails{
			QuantAlgo:      "NVFP4",
			ExcludeModules: []string{"lm_head"}, // verbatim from real fixture
		},
	})
	require.NoError(t, err)
	// 1000 (lm_head excluded) + 200 (NVFP4 packed) + 0 (scale) + 0 (k_scale) = 1200
	assert.Equal(t, int64(1200), count)
}

func TestParseSafetensorsHeaderQuantAware_StandaloneAndEmbeddedConflict(t *testing.T) {
	// Models in the wild sometimes ship BOTH a standalone
	// hf_quant_config.json AND an embedded quantization_config block.
	// The parser's resolution order is documented as standalone-wins
	// (see config_parser.go's "highest-priority signal" comment).
	// At THIS layer we just verify the two configs would yield
	// different counts so the parser's choice is observable; the
	// resolution itself is tested in pkg/modelparser.
	standalone := &HFQuantConfig{Quantization: HFQuantConfigDetails{QuantAlgo: "NVFP4"}}
	embedded := &HFQuantConfig{Quantization: HFQuantConfigDetails{QuantAlgo: "FP8"}}
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.5.mlp.gate_proj.weight": {
			"shape": []int64{10, 10}, "dtype": "U8", "data_offsets": []int64{0, 100},
		},
	})

	standaloneCount, err := ParseSafetensorsHeaderQuantAware(header[8:], "t", standalone)
	require.NoError(t, err)
	embeddedCount, err := ParseSafetensorsHeaderQuantAware(header[8:], "t", embedded)
	require.NoError(t, err)

	assert.Equal(t, int64(200), standaloneCount, "NVFP4 standalone packs 2x")
	assert.Equal(t, int64(100), embeddedCount, "FP8 embedded does not pack")
	assert.NotEqual(t, standaloneCount, embeddedCount,
		"different quant configs MUST yield different counts so the parser's resolution choice is observable")
}

func TestParseSafetensorsHeaderQuantAware_UnknownAlgoFallsBackToNaive(t *testing.T) {
	// Future quant scheme that QuantPackingFactor doesn't recognize
	// (e.g., MXFP6, FP3) → multiplier returns 1, so we count storage
	// elements as logical params. Under-count, but doesn't crash —
	// operators can override modelParameterSize manually.
	header := safetensorsHeaderForTest(t, map[string]map[string]any{
		"model.layers.5.mlp.gate_proj.weight": {
			"shape": []int64{10, 10}, "dtype": "U8", "data_offsets": []int64{0, 100},
		},
	})
	count, err := ParseSafetensorsHeaderQuantAware(header[8:], "t", &HFQuantConfig{
		Quantization: HFQuantConfigDetails{QuantAlgo: "MXFP6_FUTURE"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(100), count, "unknown quant algo → factor 1 → naive count, no crash")
}

// ─── QuantizationConfig.ToHFQuantConfig (real fixtures) ──────────────

func TestQuantizationConfig_ToHFQuantConfig_RealFixtures(t *testing.T) {
	// Verify ToHFQuantConfig synthesizes a usable HFQuantConfig from
	// each HF-native quant convention's quantization_config block.
	// Each fixture path is the SHIPPING config.json from a real HF
	// repo; if the schema upstream changes, this test lights up.
	tests := []struct {
		name               string
		path               string
		wantQuantAlgo      string
		wantExcludeNonZero bool
	}{
		{
			name:               "mxfp4 with glob exclude (gpt-oss)",
			path:               filepath.Join("testdata", "quant", "openai-gpt-oss-20b-mxfp4", "config.json"),
			wantQuantAlgo:      "mxfp4",
			wantExcludeNonZero: true,
		},
		{
			name:               "fbgemm_fp8 with full-path exclude (Llama-3.1-405B)",
			path:               filepath.Join("testdata", "llama3_1_405b.json"),
			wantQuantAlgo:      "fbgemm_fp8",
			wantExcludeNonZero: true,
		},
		{
			name:               "fp8 with no exclude (kimi-k2)",
			path:               filepath.Join("testdata", "quant", "kimi-k2-instruct-fp8", "config.json"),
			wantQuantAlgo:      "fp8",
			wantExcludeNonZero: false,
		},
		{
			name: "gptq + bits:4 normalized to INT4_GPTQ",
			path: filepath.Join("testdata", "quant", "gptq-llama-2-7b", "config.json"),
			// Real fixture has quant_method:"gptq" + bits:4. resolveAlgo
			// folds those together into INT4_GPTQ so QuantPackingFactor's
			// "int4" substring branch fires (factor 8 for I32 packing).
			wantQuantAlgo:      "INT4_GPTQ",
			wantExcludeNonZero: false,
		},
		{
			name: "compressed-tensors recognized as quant_method (per-tensor scheme not unpacked yet)",
			path: filepath.Join("testdata", "quant", "compressed-tensors-llama-3-1-8b", "config.json"),
			// We surface the producer name even though the actual FP8/INT8
			// scheme lives in nested config_groups — downstream pieces
			// (QuantPackingFactor, QuantAlgoToOMEEnum) return their
			// unknown-default and the safetensors counting falls back to
			// naive. Documented limitation; follow-up to deep-parse
			// config_groups.
			wantQuantAlgo:      "compressed-tensors",
			wantExcludeNonZero: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			require.NoError(t, err)

			model, err := ParseModelConfig(ModelConfigInput{Path: "config.json", Data: data})
			require.NoError(t, err)

			hfQuant := model.GetHFQuantConfig()
			require.NotNil(t, hfQuant, "quantization_config block should produce a synthesized HFQuantConfig")
			assert.Equal(t, tc.wantQuantAlgo, hfQuant.Quantization.QuantAlgo)
			if tc.wantExcludeNonZero {
				assert.NotEmpty(t, hfQuant.Quantization.ExcludeModules,
					"%s carries modules_to_not_convert which must propagate to ExcludeModules", tc.name)
			}
		})
	}
}

func TestQuantizationConfig_ToHFQuantConfig_PrefersQuantAlgoOverQuantMethod(t *testing.T) {
	// NVIDIA's embedded quantization_config (and its hf_quant_config.json)
	// puts quant_method:"modelopt" with the actual algorithm under
	// quant_algo:"NVFP4". If we only read quant_method we'd surface
	// "modelopt" — unrecognized by QuantPackingFactor (factor 1, no
	// 2x multiplier) and QuantAlgoToOMEEnum (empty enum, no
	// spec.Quantization). Both gaps would manifest as silent
	// undercounting on real production models.
	data, err := os.ReadFile(filepath.Join("testdata", "quant", "nvidia-llama-3-1-8b-nvfp4", "config.json"))
	require.NoError(t, err)
	model, err := ParseModelConfig(ModelConfigInput{Path: "config.json", Data: data})
	require.NoError(t, err)

	hfQuant := model.GetHFQuantConfig()
	require.NotNil(t, hfQuant)
	assert.Equal(t, "NVFP4", hfQuant.Quantization.QuantAlgo,
		"quant_algo:NVFP4 must win over quant_method:modelopt — otherwise downstream gets the producer name and silently undercounts")
}

func TestGenericModelConfig_GetHFQuantConfig_NilForUnquantized(t *testing.T) {
	// Unquantized models must return nil — empty struct or
	// zero-method config would falsely trigger the quant-aware
	// code path on every model in the catalog.
	data, err := os.ReadFile(filepath.Join("testdata", "llama3_1.json"))
	require.NoError(t, err)
	model, err := ParseModelConfig(ModelConfigInput{Path: "config.json", Data: data})
	require.NoError(t, err)
	assert.Nil(t, model.GetHFQuantConfig())
}
