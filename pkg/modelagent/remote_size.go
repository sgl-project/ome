package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/ome/pkg/utils/storage"
)

// EstimateDetail accompanies every remote-size estimate. It captures the
// reasoning behind a number so logs / metrics make it obvious why the gate
// decided what it did. Always non-nil on a successful call.
type EstimateDetail struct {
	Format string // "safetensors" | "gguf" | "diffusion" | "bin" | "unknown"
	Method string // "fp16" | "bf16" | "fp32" | "fp8" | "awq" | "gptq" | "bitsandbytes" |
	//        "hqq" | "compressed-tensors" | "gguf-q4" | "none" | "unknown"
	Strategy string // "diffusion_component_sum" | "safetensors_dtype_count" |
	//        "gguf_variant_max" | "siblings_byte_sum" | "oci_list_sum" |
	//        "s3_list_sum" | "local_walk_sum" | "noop"
	UsedFallback bool // true when caps/fallbacks tripped (e.g. min capped naive)
}

// RemoteSizeEstimator computes a conservative byte estimate of the model's
// downloaded weight footprint for a given storage URI. Returns (0, detail, nil)
// when the size is genuinely unknown so the caller can fail open.
type RemoteSizeEstimator interface {
	EstimateRemoteSize(ctx context.Context, uri string) (int64, *EstimateDetail, error)
}

// bytesPerDtype maps safetensors dtype tags to bytes per parameter. Validated
// against a survey of the HF top-300 models + 12 quantization families.
// Notes:
//   - U8 is 1 byte: BitsAndBytes and MXFP4 pre-pack two int4 into a uint8 and
//     report the count of those bytes, not of the underlying int4s.
//   - I32 is always 4 bytes: AWQ over-reports packed-int4 weights as I32, but
//     min(naive, usedStorage) absorbs that without needing AWQ-specific math;
//     HQQ stores genuine fp32 metadata as I32 (4 bytes is correct there too).
var bytesPerDtype = map[string]float64{
	"F64":     8,
	"I64":     8,
	"F32":     4,
	"I32":     4,
	"U32":     4,
	"BF16":    2,
	"F16":     2,
	"F8_E4M3": 1,
	"F8_E5M2": 1,
	"F8_E8M0": 1,
	"I8":      1,
	"U8":      1,
	"BOOL":    1,
}

var diffusionLibraries = map[string]bool{
	"diffusers":        true,
	"stable-diffusion": true,
	"flux":             true,
}

// shardRe matches sharded weight file names of any digit width:
//
//	model-00001-of-00007.safetensors, model-1-of-3.gguf, etc.
var shardRe = regexp.MustCompile(`-(\d+)-of-(\d+)\.(safetensors|bin|pt|pth|gguf|ckpt)$`)

// ggufQuantRe extracts a GGUF quantization tag from a filename, e.g.
//
//	Llama-3.1-8B.Q4_K_M.gguf  -> "Q4_K_M"
//	tinyllama-1.1b-Q8_0.gguf  -> "Q8_0"
var ggufQuantRe = regexp.MustCompile(`(?i)[-.](Q\d+_K(?:_[SML])?|Q\d+(?:_\d+)?|BF16|F16|FP16|IQ\d+\w*|F32)`)

// estimatorFor dispatches to the URI-only estimator for the given storage
// type. Returns nil for OCI / S3 / Vendor / PVC / Azure / GCS / GitHub: those
// either gate inline using their existing listing step (OCI, future S3) or
// have no useful size signal (Vendor / PVC). Caller treats nil as "no
// estimator-based precheck for this URI".
func (s *Gopher) estimatorFor(storageType storage.StorageType) RemoteSizeEstimator {
	switch storageType {
	case storage.StorageTypeHuggingFace:
		return &hfRemoteSizeEstimator{}
	case storage.StorageTypeLocal:
		return &localRemoteSizeEstimator{}
	default:
		return nil
	}
}

// ociListSumDetail is the EstimateDetail used by the inline OCI VRAM gate
// (see gopher.go) when the size is the sum of OCI ListObjects results.
func ociListSumDetail() *EstimateDetail {
	return &EstimateDetail{
		Format:   "unknown",
		Method:   "none",
		Strategy: "oci_list_sum",
	}
}

// ---------------------------------------------------------------------------
// Local
// ---------------------------------------------------------------------------

type localRemoteSizeEstimator struct{}

func (e *localRemoteSizeEstimator) EstimateRemoteSize(_ context.Context, uri string) (int64, *EstimateDetail, error) {
	detail := &EstimateDetail{Format: "unknown", Method: "none", Strategy: "local_walk_sum"}
	root := strings.TrimPrefix(uri, "local://")
	if root == "" {
		return 0, detail, fmt.Errorf("local precheck: empty path")
	}

	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return 0, detail, nil
		}
		return 0, detail, fmt.Errorf("local precheck: walk %s: %w", root, err)
	}
	return total, detail, nil
}

// ---------------------------------------------------------------------------
// HuggingFace
// ---------------------------------------------------------------------------

type hfRemoteSizeEstimator struct{}

// hfModelInfo is the subset of /api/models/{id}?blobs=true we read.
type hfModelInfo struct {
	ID          string         `json:"id"`
	LibraryName string         `json:"library_name"`
	Tags        []string       `json:"tags"`
	UsedStorage int64          `json:"usedStorage"`
	Siblings    []hfSibling    `json:"siblings"`
	Safetensors *hfSafetensors `json:"safetensors"`
	GGUF        *hfGGUFSummary `json:"gguf"`
	Config      *hfConfig      `json:"config"`
}

type hfSibling struct {
	RFilename string `json:"rfilename"`
	Size      int64  `json:"size"`
}

type hfSafetensors struct {
	// raw is the unparsed parameters payload so we can decide flat-vs-nested.
	Raw json.RawMessage `json:"parameters"`
	// Total is the helper "total" field HF often reports.
	Total int64 `json:"total"`
}

type hfGGUFSummary struct {
	TotalFileSize int64 `json:"total_file_size"`
}

type hfConfig struct {
	QuantizationConfig *hfQuantConfig `json:"quantization_config"`
}

type hfQuantConfig struct {
	QuantMethod string `json:"quant_method"`
	// Format is the compressed-tensors sub-format ("nvfp4-pack-quantized",
	// "float-quantized", "pack-quantized", "naive-quantized", ...). Empty for
	// other quant methods.
	Format     string `json:"format"`
	LoadIn4Bit bool   `json:"load_in_4bit"`
	LoadIn8Bit bool   `json:"load_in_8bit"`
}

func (e *hfRemoteSizeEstimator) EstimateRemoteSize(ctx context.Context, uri string) (int64, *EstimateDetail, error) {
	detail := &EstimateDetail{Format: "unknown", Method: "unknown", Strategy: "noop"}
	modelID := strings.TrimPrefix(uri, "hf://")
	modelID = strings.TrimPrefix(modelID, "huggingface://")
	if modelID == "" {
		return 0, detail, fmt.Errorf("hf precheck: empty model id in uri %q", uri)
	}

	info, err := fetchHFModelInfo(ctx, modelID, DefaultEndpoint)
	if err != nil {
		return 0, detail, err
	}
	bytes, d := estimateHuggingFaceWeightBytes(info)
	return bytes, d, nil
}

// fetchHFModelInfo issues one GET /api/models/{id}?blobs=true and decodes
// the subset we need. The blobs=true param is required so siblings carry
// per-file sizes; without it the diffusion and tree-walk paths degrade to
// zero (silent fail-open).
func fetchHFModelInfo(ctx context.Context, modelID, endpoint string) (*hfModelInfo, error) {
	base, err := hfModelMetaDataUrlWithEndpoint(modelID, endpoint)
	if err != nil {
		return nil, err
	}
	url := base + "?blobs=true"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("hf precheck: build request: %w", err)
	}
	resp, err := NewHTTPClientWithTimeout(DefaultRequestTimeout).Do(req)
	if err != nil {
		return nil, fmt.Errorf("hf precheck: get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hf precheck: %s: status %d", url, resp.StatusCode)
	}

	var info hfModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("hf precheck: decode %s: %w", url, err)
	}
	return &info, nil
}

// estimateHuggingFaceWeightBytes is the pure-data classifier+strategy. Split
// out so it is trivially testable with fixtures captured from the HF API.
func estimateHuggingFaceWeightBytes(info *hfModelInfo) (int64, *EstimateDetail) {
	format := classifyFormat(info)
	method := classifyMethod(info, format)
	detail := &EstimateDetail{Format: format, Method: method}

	switch format {
	case "diffusion":
		detail.Strategy = "diffusion_component_sum"
		return diffusionComponentSum(info), detail
	case "safetensors":
		detail.Strategy = "safetensors_dtype_count"
		size, fallback := safetensorsDtypeCount(info)
		detail.UsedFallback = fallback
		return size, detail
	case "gguf":
		detail.Strategy = "gguf_variant_max"
		return ggufVariantMax(info), detail
	case "bin", "unknown":
		detail.Strategy = "siblings_byte_sum"
		return siblingsByteSum(info), detail
	}
	return 0, detail
}

func classifyFormat(info *hfModelInfo) string {
	if diffusionLibraries[strings.ToLower(info.LibraryName)] {
		return "diffusion"
	}
	if info.Safetensors != nil && len(info.Safetensors.Raw) > 0 {
		return "safetensors"
	}
	hasSafetensors := false
	hasGGUF := false
	hasBin := false
	for _, s := range info.Siblings {
		name := strings.ToLower(s.RFilename)
		switch {
		case strings.HasSuffix(name, ".safetensors"):
			hasSafetensors = true
		case strings.HasSuffix(name, ".gguf"):
			hasGGUF = true
		case strings.HasSuffix(name, ".bin"),
			strings.HasSuffix(name, ".pt"),
			strings.HasSuffix(name, ".pth"):
			hasBin = true
		}
	}
	if hasGGUF {
		return "gguf"
	}
	// gguf summary present but no .gguf siblings (e.g. blobs=true elided them):
	// still classify as gguf so the variant-max strategy can fall back to
	// total_file_size.
	if info.GGUF != nil && info.GGUF.TotalFileSize > 0 {
		return "gguf"
	}
	if hasSafetensors {
		return "safetensors"
	}
	if hasBin {
		return "bin"
	}
	return "unknown"
}

func classifyMethod(info *hfModelInfo, format string) string {
	// GGUF first: a GGUF repo's filenames carry the canonical quant tag
	// (Q4_K_M, Q8_0, …). Doing this before the fuzzy ID/tag regex prevents
	// false positives when a GGUF repo's name happens to contain "int4",
	// "fp8", "gptq", etc.
	if format == "gguf" {
		return extractGGUFQuant(info.Siblings)
	}

	if info.Config != nil && info.Config.QuantizationConfig != nil {
		qc := info.Config.QuantizationConfig
		switch strings.ToLower(qc.QuantMethod) {
		case "awq":
			return "awq"
		case "gptq":
			return "gptq"
		case "bitsandbytes":
			return "bitsandbytes"
		case "hqq":
			return "hqq"
		case "compressed-tensors":
			// compressed-tensors is a wrapper; the actual scheme is in `format`.
			switch strings.ToLower(qc.Format) {
			case "nvfp4-pack-quantized":
				return "nvfp4"
			case "float-quantized":
				return "fp8"
			}
			return "compressed-tensors"
		case "aqlm":
			return "aqlm"
		}
		if qc.LoadIn4Bit || qc.LoadIn8Bit {
			return "bitsandbytes"
		}
	}

	hayLower := strings.ToLower(info.ID + " " + strings.Join(info.Tags, " "))
	for _, m := range []string{
		"awq", "gptq", "hqq", "aqlm", "exl2",
		"nvfp4", "mxfp4", "fp4",
		"bnb", "nf4",
		"int8", "int4", "fp8",
	} {
		if strings.Contains(hayLower, m) {
			// Collapse BnB variants into one label.
			if m == "bnb" || m == "nf4" {
				return "bitsandbytes"
			}
			return m
		}
	}

	if info.Safetensors != nil {
		params, _, _ := parseSafetensorsParams(info.Safetensors.Raw)
		// Order matters: prefer the quantization-indicator dtype over the
		// metadata / dense-precision dtype, otherwise a quantized model with
		// BF16 norms or F32 RoPE buffers would be mislabeled "bf16"/"fp32".
		// MXFP4 / NVFP4 (U8 weights + F8_E8M0 scales) get caught before plain FP8.
		switch {
		case hasAnyDtype(params, "F8_E8M0") && hasAnyDtype(params, "U8"):
			return "mxfp4"
		case hasAnyDtype(params, "F8_E4M3", "F8_E5M2", "F8_E8M0"):
			return "fp8"
		case hasAnyDtype(params, "U8", "I8"):
			return "uint8_or_int8_storage"
		case hasAnyDtype(params, "I32", "U32"):
			return "packed_or_metadata_32"
		case hasAnyDtype(params, "BF16", "F16"):
			return "bf16"
		case hasAnyDtype(params, "F32"):
			return "fp32"
		}
		return "none"
	}
	return "unknown"
}

func extractGGUFQuant(siblings []hfSibling) string {
	var largest hfSibling
	for _, s := range siblings {
		if strings.HasSuffix(strings.ToLower(s.RFilename), ".gguf") && s.Size > largest.Size {
			largest = s
		}
	}
	if largest.RFilename == "" {
		return "unknown"
	}
	m := ggufQuantRe.FindStringSubmatch(largest.RFilename)
	if len(m) < 2 {
		return "gguf-unknown"
	}
	return "gguf-" + strings.ToLower(m[1])
}

func hasAnyDtype(params map[string]int64, dts ...string) bool {
	for _, d := range dts {
		if params[d] > 0 {
			return true
		}
	}
	return false
}

// parseSafetensorsParams handles both flat and nested HF parameters payloads:
//
//	flat:   {"BF16": 7000000000, "F32": 1024}
//	nested: {"variant1": {"BF16": ...}, "variant2": {"F16": ...}}
//
// Returns the flattened {dtype: count} sum, the number of groups detected,
// and whether the payload was nested.
func parseSafetensorsParams(raw json.RawMessage) (map[string]int64, int, bool) {
	if len(raw) == 0 {
		return nil, 0, false
	}
	var flat map[string]int64
	if err := json.Unmarshal(raw, &flat); err == nil {
		return flat, 1, false
	}
	var nested map[string]map[string]int64
	if err := json.Unmarshal(raw, &nested); err == nil {
		out := make(map[string]int64)
		for _, group := range nested {
			for d, c := range group {
				out[d] += c
			}
		}
		return out, len(nested), true
	}
	return nil, 0, false
}

func safetensorsDtypeCount(info *hfModelInfo) (int64, bool) {
	// If the safetensors field or parameters payload is missing, fall back to
	// a sibling-byte sum. Some repos host *.safetensors but omit the structured
	// API field (private mirrors, very new uploads, etc.).
	if info.Safetensors == nil || len(info.Safetensors.Raw) == 0 {
		return siblingsByteSum(info), true
	}
	params, _, _ := parseSafetensorsParams(info.Safetensors.Raw)
	if len(params) == 0 {
		return siblingsByteSum(info), true
	}
	var naive float64
	for dtype, count := range params {
		bpp, ok := bytesPerDtype[strings.ToUpper(dtype)]
		if !ok {
			continue
		}
		naive += float64(count) * bpp
	}
	naiveBytes := int64(naive)
	if info.UsedStorage > 0 && naiveBytes > info.UsedStorage {
		return info.UsedStorage, true
	}
	return naiveBytes, false
}

var diffusionComponentDirs = []string{
	"transformer/",
	"unet/",
	"vae/",
	"text_encoder/",
	"text_encoder_2/",
	"text_encoder_3/",
	"image_encoder/",
}

func diffusionComponentSum(info *hfModelInfo) int64 {
	var total int64
	for _, dir := range diffusionComponentDirs {
		var inDir []hfSibling
		for _, s := range info.Siblings {
			if strings.HasPrefix(s.RFilename, dir) {
				inDir = append(inDir, s)
			}
		}
		total += pickBestWeightFile(inDir)
	}
	if total > 0 {
		return total
	}
	// No recognized component dirs — fall back to a tree-walk sum to remain useful
	// for older diffusion pipelines without the standard layout.
	return siblingsByteSum(info)
}

// pickBestWeightFile returns the byte size of the preferred weight artifact in
// a single component directory. Preference (lower index = better):
//  1. fp16/bf16 variant of safetensors
//  2. regular safetensors
//  3. anything else with a weight extension
//
// Sharded weights are summed across the shard family.
func pickBestWeightFile(files []hfSibling) int64 {
	if len(files) == 0 {
		return 0
	}
	type group struct {
		key      string
		priority int
		total    int64
	}
	groups := make(map[string]*group)
	for _, f := range files {
		nameLower := strings.ToLower(f.RFilename)
		var pri int
		switch {
		case strings.HasSuffix(nameLower, ".onnx"),
			strings.HasSuffix(nameLower, ".msgpack"),
			strings.HasSuffix(nameLower, ".h5"):
			continue
		case strings.HasSuffix(nameLower, ".safetensors") && (strings.Contains(nameLower, ".fp16.") || strings.Contains(nameLower, ".bf16.")):
			pri = 0
		case strings.HasSuffix(nameLower, ".safetensors"):
			pri = 1
		case strings.HasSuffix(nameLower, ".bin"),
			strings.HasSuffix(nameLower, ".pt"),
			strings.HasSuffix(nameLower, ".pth"),
			strings.HasSuffix(nameLower, ".ckpt"):
			pri = 2
		default:
			continue
		}
		key := shardFamilyKey(f.RFilename, pri)
		g, ok := groups[key]
		if !ok {
			g = &group{key: key, priority: pri}
			groups[key] = g
		}
		g.total += f.Size
	}
	if len(groups) == 0 {
		return 0
	}
	var best *group
	for _, g := range groups {
		switch {
		case best == nil:
			best = g
		case g.priority < best.priority:
			best = g
		case g.priority == best.priority && g.total > best.total:
			best = g
		}
	}
	if best == nil {
		return 0
	}
	return best.total
}

// shardFamilyKey returns a stable key that groups shards belonging to the same
// logical weight file (so they sum together) but separates distinct families
// like ".fp16.safetensors" vs ".safetensors".
func shardFamilyKey(name string, priority int) string {
	base := shardRe.ReplaceAllString(name, ".$3")
	return fmt.Sprintf("%d:%s", priority, base)
}

func ggufVariantMax(info *hfModelInfo) int64 {
	variants := make(map[string]int64)
	for _, s := range info.Siblings {
		if !strings.HasSuffix(strings.ToLower(s.RFilename), ".gguf") {
			continue
		}
		m := ggufQuantRe.FindStringSubmatch(s.RFilename)
		key := "default"
		if len(m) >= 2 {
			key = strings.ToUpper(m[1])
		}
		variants[key] += s.Size
	}
	if len(variants) == 0 {
		if info.GGUF != nil && info.GGUF.TotalFileSize > 0 {
			return info.GGUF.TotalFileSize
		}
		return 0
	}
	var max int64
	for _, v := range variants {
		if v > max {
			max = v
		}
	}
	return max
}

var weightExtensions = map[string]bool{
	".safetensors": true,
	".bin":         true,
	".pt":          true,
	".pth":         true,
	".gguf":        true,
	".ckpt":        true,
}

func siblingsByteSum(info *hfModelInfo) int64 {
	var total int64
	for _, s := range info.Siblings {
		ext := strings.ToLower(filepath.Ext(s.RFilename))
		if !weightExtensions[ext] {
			continue
		}
		if s.Size <= 0 {
			continue
		}
		total += s.Size
	}
	return total
}
