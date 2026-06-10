package modelagent

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// Environment variable names. Values are typically provided by the Helm chart's
// model-agent ConfigMap, mirroring how INSTANCE_TYPE_MAP is wired.
const (
	GPUShapeMemoryGBEnvVar = "GPU_SHAPE_MEMORY_GB_MAP"
	VRAMSafetyFactorEnvVar = "MODEL_DOWNLOAD_VRAM_SAFETY_FACTOR"

	defaultVRAMSafetyFactor = 1.2
	minVRAMSafetyFactor     = 1.0
	maxVRAMSafetyFactor     = 4.0
)

// defaultGPUShapeMemoryGB is the fallback total node VRAM (GiB) map keyed by
// the short shape alias (matches values in pkg/utils/instance_type_util.go).
// Each value is the aggregate VRAM across all GPUs on a typical node of that
// shape (e.g. H100 = 8 × 80 GiB SXM). Override via the GPU_SHAPE_MEMORY_GB_MAP
// env var (Helm ConfigMap) when running non-standard GPU counts.
var defaultGPUShapeMemoryGB = map[string]int64{
	"A10":      96,   // BM.GPU.A10.4 — 4 × 24 GiB
	"A100-40G": 320,  // 8 × 40 GiB SXM4
	"A100-80G": 640,  // 8 × 80 GiB SXM4
	"H100":     640,  // 8 × 80 GiB SXM5
	"H200":     1128, // 8 × 141 GiB HBM3e
	"B200":     1536, // 8 × 192 GiB
	"L40":      384,  // 8 × 48 GiB
	"L40S":     384,  // 8 × 48 GiB
}

var (
	gpuShapeMemoryGBMap     map[string]int64
	gpuShapeMemoryGBMapErr  error
	gpuShapeMemoryGBMapOnce sync.Once

	vramSafetyFactorVal  float64
	vramSafetyFactorOnce sync.Once
)

// loadGPUShapeMemoryGBFromEnv parses GPU_SHAPE_MEMORY_GB_MAP. Values are GiB
// per single GPU. Falls back to defaults on unset/empty/parse-error.
func loadGPUShapeMemoryGBFromEnv() (map[string]int64, error) {
	envValue := os.Getenv(GPUShapeMemoryGBEnvVar)
	if envValue == "" {
		return defaultGPUShapeMemoryGB, nil
	}

	var configMap map[string]int64
	if err := json.Unmarshal([]byte(envValue), &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", GPUShapeMemoryGBEnvVar, err)
	}
	if len(configMap) == 0 {
		return defaultGPUShapeMemoryGB, nil
	}
	return configMap, nil
}

// gpuShapeMemoryGB returns the total node VRAM in GiB for the given shape
// alias (e.g. "H100", "A10"). Second return is false when the shape is
// unknown.
func gpuShapeMemoryGB(shape string) (int64, bool) {
	gpuShapeMemoryGBMapOnce.Do(func() {
		gpuShapeMemoryGBMap, gpuShapeMemoryGBMapErr = loadGPUShapeMemoryGBFromEnv()
	})
	if gpuShapeMemoryGBMapErr != nil || gpuShapeMemoryGBMap == nil {
		return 0, false
	}
	gb, ok := gpuShapeMemoryGBMap[shape]
	if !ok {
		return 0, false
	}
	return gb, true
}

// AvailableNodeVRAMBytes returns the aggregate VRAM in bytes for a node of
// the given shape. Tune the map values in the Helm ConfigMap when running non-
// standard GPU counts under the same alias.
//
// Returns 0 ("unknown") when the shape is not in the map; the caller (Gopher
// VRAM precheck) treats 0 as "skip the gate".
func AvailableNodeVRAMBytes(shape string) int64 {
	gb, ok := gpuShapeMemoryGB(shape)
	if !ok || gb <= 0 {
		return 0
	}
	return gb * (1 << 30)
}

// vramSafetyFactor parses MODEL_DOWNLOAD_VRAM_SAFETY_FACTOR once. The factor is
// multiplied into the estimated weight bytes to leave headroom for activations,
// KV cache, and CUDA workspace.
//
// Invalid values (non-numeric, < 1.0, > 4.0) silently fall back to the default
// (1.2). The clamp prevents foot-guns: a value < 1.0 would over-permit, and a
// huge value would block every model.
func vramSafetyFactor() float64 {
	vramSafetyFactorOnce.Do(func() {
		raw := os.Getenv(VRAMSafetyFactorEnvVar)
		if raw == "" {
			vramSafetyFactorVal = defaultVRAMSafetyFactor
			return
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < minVRAMSafetyFactor || v > maxVRAMSafetyFactor {
			vramSafetyFactorVal = defaultVRAMSafetyFactor
			return
		}
		vramSafetyFactorVal = v
	})
	return vramSafetyFactorVal
}
