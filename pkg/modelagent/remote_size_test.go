package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// makeHFInfo is a small helper for constructing hfModelInfo fixtures inline.
func makeHFInfo(t *testing.T, body string) *hfModelInfo {
	t.Helper()
	var info hfModelInfo
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &info
}

func TestEstimateHF_SafetensorsBF16(t *testing.T) {
	// meta-llama/Llama-3.1-8B-Instruct shape: single BF16 group, no quantization.
	body := `{
        "id": "meta-llama/Llama-3.1-8B-Instruct",
        "safetensors": {"parameters": {"BF16": 8030261248}, "total": 8030261248},
        "usedStorage": 32121044992
    }`
	info := makeHFInfo(t, body)
	bytes, detail := estimateHuggingFaceWeightBytes(info)
	want := int64(8030261248 * 2)
	if bytes != want {
		t.Errorf("BF16 size = %d, want %d", bytes, want)
	}
	if detail.Format != "safetensors" || detail.Method != "bf16" || detail.Strategy != "safetensors_dtype_count" {
		t.Errorf("classifier detail = %+v", detail)
	}
}

func TestEstimateHF_SafetensorsAWQUsedStorageCap(t *testing.T) {
	// AWQ-style: I32 over-count would inflate naive; min(naive, usedStorage)
	// caps to usedStorage which matches the real file size.
	// Synthetic numbers: 1 B I32 entries (naive = 4 GB) but real usedStorage = 1 GB.
	body := `{
        "id": "Qwen/Qwen2.5-7B-Instruct-AWQ",
        "tags": ["awq"],
        "safetensors": {"parameters": {"I32": 1000000000, "BF16": 100000000}, "total": 1100000000},
        "usedStorage": 1000000000
    }`
	info := makeHFInfo(t, body)
	bytes, detail := estimateHuggingFaceWeightBytes(info)
	if bytes != 1000000000 {
		t.Errorf("AWQ cap size = %d, want %d", bytes, 1000000000)
	}
	if detail.Method != "awq" {
		t.Errorf("expected method awq, got %q", detail.Method)
	}
	if !detail.UsedFallback {
		t.Errorf("expected UsedFallback=true when usedStorage caps naive")
	}
}

func TestEstimateHF_GGUFMultiVariantMax(t *testing.T) {
	// Three .gguf variants: Q4_K_M, Q5_K_M, Q8_0. We expect the largest (Q8_0).
	body := `{
        "id": "bartowski/Qwen2.5-7B-Instruct-GGUF",
        "gguf": {"total_file_size": 5000000000},
        "siblings": [
            {"rfilename": "model-Q4_K_M.gguf", "size": 4000000000},
            {"rfilename": "model-Q5_K_M.gguf", "size": 5000000000},
            {"rfilename": "model-Q8_0.gguf",   "size": 7000000000}
        ]
    }`
	info := makeHFInfo(t, body)
	bytes, detail := estimateHuggingFaceWeightBytes(info)
	if bytes != 7000000000 {
		t.Errorf("GGUF size = %d, want %d (largest variant)", bytes, 7000000000)
	}
	if detail.Format != "gguf" || detail.Strategy != "gguf_variant_max" {
		t.Errorf("classifier detail = %+v", detail)
	}
}

func TestEstimateHF_GGUFFallbackToTotal(t *testing.T) {
	// No siblings carry sizes → fall back to gguf.total_file_size.
	body := `{
        "id": "foo/bar",
        "gguf": {"total_file_size": 2500000000},
        "siblings": [
            {"rfilename": "config.json", "size": 1024}
        ]
    }`
	info := makeHFInfo(t, body)
	bytes, _ := estimateHuggingFaceWeightBytes(info)
	if bytes != 2500000000 {
		t.Errorf("GGUF fallback = %d, want %d", bytes, 2500000000)
	}
}

func TestEstimateHF_DiffusionComponentSum(t *testing.T) {
	// Stable Diffusion-style layout: prefer .fp16.safetensors and sum across
	// components.
	body := `{
        "id": "stabilityai/stable-diffusion-3-medium",
        "library_name": "diffusers",
        "siblings": [
            {"rfilename": "transformer/diffusion_pytorch_model.fp16.safetensors", "size": 10000000000},
            {"rfilename": "transformer/diffusion_pytorch_model.safetensors",      "size": 20000000000},
            {"rfilename": "vae/diffusion_pytorch_model.fp16.safetensors",          "size": 250000000},
            {"rfilename": "vae/diffusion_pytorch_model.safetensors",               "size": 500000000},
            {"rfilename": "text_encoder/model.fp16.safetensors",                   "size": 250000000},
            {"rfilename": "text_encoder/model.safetensors",                        "size": 500000000}
        ]
    }`
	info := makeHFInfo(t, body)
	bytes, detail := estimateHuggingFaceWeightBytes(info)
	want := int64(10000000000 + 250000000 + 250000000)
	if bytes != want {
		t.Errorf("diffusion sum = %d, want %d (fp16 preferred)", bytes, want)
	}
	if detail.Format != "diffusion" || detail.Strategy != "diffusion_component_sum" {
		t.Errorf("classifier detail = %+v", detail)
	}
}

func TestEstimateHF_FP8Detection(t *testing.T) {
	body := `{
        "id": "neuralmagic/Llama-3-8B-FP8",
        "safetensors": {"parameters": {"F8_E4M3": 8000000000, "BF16": 30000000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "fp8" {
		t.Errorf("FP8 method = %q, want %q", detail.Method, "fp8")
	}
}

func TestEstimateHF_UnknownFallsbackToSiblings(t *testing.T) {
	body := `{
        "id": "private/repo",
        "siblings": [
            {"rfilename": "pytorch_model.bin",  "size": 1000000000},
            {"rfilename": "model.safetensors",  "size": 2000000000},
            {"rfilename": "extra-file.txt",     "size": 1024}
        ]
    }`
	info := makeHFInfo(t, body)
	bytes, detail := estimateHuggingFaceWeightBytes(info)
	// safetensors should win the format vote since at least one is present.
	if detail.Format != "safetensors" {
		t.Errorf("expected safetensors format, got %q", detail.Format)
	}
	if bytes <= 0 {
		t.Errorf("expected positive size, got %d", bytes)
	}
}

func TestParseSafetensorsParams_FlatAndNested(t *testing.T) {
	flat := json.RawMessage(`{"BF16": 100, "F32": 4}`)
	out, groups, nested := parseSafetensorsParams(flat)
	if nested || groups != 1 || out["BF16"] != 100 || out["F32"] != 4 {
		t.Errorf("flat parse = %v (groups=%d nested=%v)", out, groups, nested)
	}

	nestedRaw := json.RawMessage(`{"variant1": {"BF16": 50}, "variant2": {"F16": 25}}`)
	out, groups, nested = parseSafetensorsParams(nestedRaw)
	if !nested || groups != 2 || out["BF16"] != 50 || out["F16"] != 25 {
		t.Errorf("nested parse = %v (groups=%d nested=%v)", out, groups, nested)
	}
}

func TestHFEstimator_BlobsTrueQueryParam(t *testing.T) {
	// Asserts the estimator hits /api/models/{id}?blobs=true. Without it,
	// siblings sizes are zero and the diffusion / sibling-sum paths fail open
	// silently.
	var seenRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
            "id": "foo/bar",
            "safetensors": {"parameters": {"BF16": 1000}}
        }`))
	}))
	defer srv.Close()

	info, err := fetchHFModelInfo(context.Background(), "foo/bar", srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if seenRawQuery != "blobs=true" {
		t.Errorf("expected ?blobs=true; got %q", seenRawQuery)
	}
	if info.ID != "foo/bar" {
		t.Errorf("decoded id = %q", info.ID)
	}
}

func TestLocalEstimator(t *testing.T) {
	dir := t.TempDir()
	files := map[string]int{
		"weights.bin":               4000,
		"sub/dir/model.safetensors": 6000,
		"sub/dir/config.json":       100,
	}
	for name, size := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		buf := make([]byte, size)
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	e := &localRemoteSizeEstimator{}
	got, detail, err := e.EstimateRemoteSize(context.Background(), "local://"+dir)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	var want int64
	for _, s := range files {
		want += int64(s)
	}
	if got != want {
		t.Errorf("local sum = %d, want %d", got, want)
	}
	if detail.Strategy != "local_walk_sum" {
		t.Errorf("strategy = %q", detail.Strategy)
	}
}

func TestLocalEstimator_Missing(t *testing.T) {
	e := &localRemoteSizeEstimator{}
	got, _, err := e.EstimateRemoteSize(context.Background(), "local:///path/that/does/not/exist")
	if err != nil {
		t.Fatalf("missing path should not error, got %v", err)
	}
	if got != 0 {
		t.Errorf("missing path size = %d, want 0", got)
	}
}

// Sanity-check that bytesPerDtype covers the dtypes we expect from the
// quantization survey (regression guard).
func TestBytesPerDtype_KeyCoverage(t *testing.T) {
	for _, d := range []string{"F32", "BF16", "F16", "F8_E4M3", "F8_E5M2", "I8", "U8", "I32", "U32", "I64", "BOOL"} {
		if _, ok := bytesPerDtype[d]; !ok {
			t.Errorf("bytesPerDtype missing %q", d)
		}
	}
	// Critical invariants from the survey:
	if bytesPerDtype["U8"] != 1 {
		t.Errorf("U8 must be 1 byte (BnB pre-packs); got %v", bytesPerDtype["U8"])
	}
	if bytesPerDtype["I32"] != 4 {
		t.Errorf("I32 must be 4 bytes (HQQ uses real I32; AWQ over-count caught by min(naive,usedStorage)); got %v", bytesPerDtype["I32"])
	}
}

// Sanity-check shard regex matches the survey-observed forms.
func TestShardRegex(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"model-00001-of-00007.safetensors", true},
		{"model-1-of-3.gguf", true},
		{"model-Q4_K_M-00001-of-00003.gguf", true},
		{"pytorch_model.bin", false},
		{"config.json", false},
	}
	for _, tc := range cases {
		got := shardRe.MatchString(tc.name)
		if got != tc.want {
			t.Errorf("shardRe.MatchString(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func ExampleEstimateDetail() {
	d := &EstimateDetail{Format: "safetensors", Method: "bf16", Strategy: "safetensors_dtype_count"}
	fmt.Printf("%s/%s/%s\n", d.Format, d.Method, d.Strategy)
	// Output: safetensors/bf16/safetensors_dtype_count
}

func TestClassifyMethod_GGUFBeatsFuzzyTagMatch(t *testing.T) {
	// A GGUF repo whose name contains "fp8" must not be mislabeled as "fp8";
	// extractGGUFQuant should be hit first.
	body := `{
        "id": "user/SomeModel-fp8-variant-GGUF",
        "tags": ["gguf", "fp8"],
        "gguf": {"total_file_size": 4000000000},
        "siblings": [
            {"rfilename": "model-Q4_K_M.gguf", "size": 4000000000}
        ]
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Format != "gguf" {
		t.Fatalf("format = %q, want gguf", detail.Format)
	}
	if detail.Method != "gguf-q4_k_m" {
		t.Errorf("method = %q, want gguf-q4_k_m (GGUF must win over the fp8 tag)", detail.Method)
	}
}

func TestClassifyMethod_CompressedTensorsNVFP4(t *testing.T) {
	body := `{
        "id": "RedHatAI/Qwen3.6-35B-A3B-NVFP4",
        "config": {"quantization_config": {"quant_method": "compressed-tensors", "format": "nvfp4-pack-quantized"}},
        "safetensors": {"parameters": {"U8": 18000000000, "F8_E8M0": 50000000, "BF16": 200000000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "nvfp4" {
		t.Errorf("method = %q, want nvfp4", detail.Method)
	}
}

func TestClassifyMethod_CompressedTensorsFloatQuantized(t *testing.T) {
	body := `{
        "id": "RedHatAI/Llama-3.2-1B-Instruct-FP8-dynamic",
        "config": {"quantization_config": {"quant_method": "compressed-tensors", "format": "float-quantized"}},
        "safetensors": {"parameters": {"F8_E4M3": 1000000000, "BF16": 200000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "fp8" {
		t.Errorf("method = %q, want fp8 (compressed-tensors float-quantized → fp8)", detail.Method)
	}
}

func TestClassifyMethod_NF4MapsToBitsandbytes(t *testing.T) {
	body := `{
        "id": "unsloth/Llama-3.1-8B-Instruct-bnb-4bit",
        "tags": ["nf4"],
        "safetensors": {"parameters": {"U8": 4000000000, "BF16": 50000000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "bitsandbytes" {
		t.Errorf("method = %q, want bitsandbytes (nf4 should collapse to bitsandbytes)", detail.Method)
	}
}

func TestClassifyMethod_NVFP4ViaTagRegex(t *testing.T) {
	body := `{
        "id": "nvidia/Gemma-4-31B-IT-NVFP4",
        "tags": ["nvfp4"],
        "safetensors": {"parameters": {"BF16": 1000000, "U8": 30000000000, "F8_E8M0": 60000000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "nvfp4" {
		t.Errorf("method = %q, want nvfp4 (via tag regex when no quant_method present)", detail.Method)
	}
}

func TestClassifyMethod_DtypeFallback_OrderingForQuantStorage(t *testing.T) {
	// Survey case: openai/gpt-oss-style — BF16 norms + U8 packed weights but
	// no quantization_config and no recognized tag. Should label as int-storage
	// (the U8 wins over BF16), not "bf16".
	body := `{
        "id": "some/private-quantized",
        "safetensors": {"parameters": {"BF16": 1000000, "U8": 5000000000, "F32": 2048}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "uint8_or_int8_storage" {
		t.Errorf("method = %q, want uint8_or_int8_storage (U8 must win over BF16)", detail.Method)
	}
}

func TestClassifyMethod_DtypeFallback_MXFP4Heuristic(t *testing.T) {
	// U8 weights + F8_E8M0 scales without compressed-tensors metadata
	// (e.g. some openai/gpt-oss variants) → "mxfp4", not "fp8".
	body := `{
        "id": "some/mxfp4-no-config",
        "safetensors": {"parameters": {"U8": 30000000000, "F8_E8M0": 60000000, "BF16": 100000}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "mxfp4" {
		t.Errorf("method = %q, want mxfp4 (U8 + F8_E8M0 must beat plain fp8)", detail.Method)
	}
}

func TestClassifyMethod_PureBF16StillWorks(t *testing.T) {
	// Regression guard: a real BF16 model (Llama-3.1 style) must still come
	// out as "bf16" after the reorder.
	body := `{
        "id": "meta-llama/Llama-3.1-8B-Instruct",
        "safetensors": {"parameters": {"BF16": 8000000000, "F32": 1024}}
    }`
	info := makeHFInfo(t, body)
	_, detail := estimateHuggingFaceWeightBytes(info)
	if detail.Method != "bf16" {
		t.Errorf("method = %q, want bf16 (pure half-precision model)", detail.Method)
	}
}
