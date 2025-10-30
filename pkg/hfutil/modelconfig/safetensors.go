package modelconfig

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// FindAndParseSafetensors looks for a safetensors file in the same directory as the config file
// and parses it to count the total number of parameters.
// If quantMethod is provided (e.g., from GetQuantizationType()), it will be used for quantization-aware adjustments.
// If quantMethod is empty or not provided, no quantization adjustments will be applied.
func FindAndParseSafetensors(configPath string, quantMethod ...string) (int64, error) {
	if configPath == "" {
		return 0, fmt.Errorf("config path cannot be empty")
	}

	dir := filepath.Dir(configPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to list directory '%s': %w", dir, err)
	}

	// Get quantization method from provided parameter
	var qt string
	if len(quantMethod) > 0 {
		qt = strings.ToLower(quantMethod[0])
	}

	// Look for index.json file first, which might point to sharded safetensors
	indexPath := filepath.Join(dir, "model.safetensors.index.json")
	if _, err := os.Stat(indexPath); err == nil {
		count, err := ParseSafetensorsIndexWithQuant(indexPath, qt)
		if err != nil {
			return 0, fmt.Errorf("failed to parse safetensors index '%s': %w", indexPath, err)
		}
		return count, nil
	}

	// If no index file, process all safetensors files and sum their parameters
	var totalParams int64
	safetensorsFound := false

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".safetensors") {
			fullPath := filepath.Join(dir, f.Name())
			params, err := ParseSafetensorsWithQuant(fullPath, qt)
			if err != nil {
				return 0, fmt.Errorf("failed to parse safetensors file '%s': %w", fullPath, err)
			}

			// Check for overflow when adding
			if totalParams > math.MaxInt64-params {
				return 0, fmt.Errorf("parameter count overflow when processing '%s': total would exceed maximum value", fullPath)
			}

			totalParams += params
			safetensorsFound = true
		}
	}

	if !safetensorsFound {
		return 0, fmt.Errorf("no .safetensors files found in directory '%s'", dir)
	}

	return totalParams, nil
}

// ParseSafetensorsWithQuant parses a safetensors file with quantization-aware adjustments
func ParseSafetensorsWithQuant(path string, quantMethod string) (int64, error) {
	if path == "" {
		return 0, fmt.Errorf("safetensors file path cannot be empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open safetensors file '%s': %w", path, err)
	}
	defer file.Close()

	headerLenBuf := make([]byte, 8)
	if _, err := io.ReadFull(file, headerLenBuf); err != nil {
		return 0, fmt.Errorf("failed to read header length from '%s': %w", path, err)
	}
	headerLen := binary.LittleEndian.Uint64(headerLenBuf)

	// Sanity check for header length to prevent excessive memory allocation
	const maxHeaderSize = 10 * 1024 * 1024 // 10MB max header size
	if headerLen > maxHeaderSize {
		return 0, fmt.Errorf("header length %d in '%s' exceeds maximum allowed size of %d bytes",
			headerLen, path, maxHeaderSize)
	}

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(file, headerBytes); err != nil {
		return 0, fmt.Errorf("failed to read JSON header from '%s': %w", path, err)
	}

	var raw map[string]struct {
		Shape       []int64  `json:"shape"`
		Dtype       string   `json:"dtype"`
		DataOffsets [2]int64 `json:"data_offsets"`
	}

	if err := json.Unmarshal(headerBytes, &raw); err != nil {
		return 0, fmt.Errorf("failed to parse JSON header from '%s': %w", path, err)
	}

	var total int64 = 0
	for tensorName, tensor := range raw {
		// Skip metadata tensors which may have empty shapes
		if tensorName == "__metadata__" {
			continue
		}

		if len(tensor.Shape) == 0 {
			// Skip tensors with empty shapes (like scalars or metadata)
			continue
		}

		count := int64(1)
		for i, dim := range tensor.Shape {
			if dim <= 0 {
				return 0, fmt.Errorf("tensor '%s' in '%s' has invalid dimension %d at index %d",
					tensorName, path, dim, i)
			}

			// Check for overflow before multiplication
			if count > 0 && dim > math.MaxInt64/count {
				return 0, fmt.Errorf("dimension overflow for tensor '%s' in '%s': multiplication would exceed maximum value",
					tensorName, path)
			}
			count *= dim
		}

		// Adjustments for quantized weights
		dtype := strings.ToLower(tensor.Dtype)
		quantMethodLower := strings.ToLower(quantMethod)

		// 4-bit quantization pack 2 values per byte (mxfp4, int4, gptq, awq variants)
		if strings.Contains(quantMethodLower, "mxfp4") || strings.Contains(quantMethodLower, "int4") ||
			strings.Contains(quantMethodLower, "gptq") || strings.Contains(quantMethodLower, "awq") {
			// Skip obvious auxiliary tensors in quantized checkpoints
			if isAuxTensor(tensorName) {
				continue
			}
			// In 4-bit quantization, weights are typically packed as uint8 with 2 params per byte
			if dtype == "u8" || dtype == "uint8" {
				if isWeightLike(tensorName) {
					// multiply logical parameter count by 2 (8 bits / 4 bits)
					if count > math.MaxInt64/2 {
						return 0, fmt.Errorf("parameter count overflow when adjusting for 4-bit quantization (%s) in '%s'", quantMethodLower, path)
					}
					count = count * 2
				}
			}
		}
		// 8-bit quantization (fp8, fbgemm_fp8) use 1 byte per parameter - no adjustment needed

		// Check for overflow when adding to total
		if total > math.MaxInt64-count {
			return 0, fmt.Errorf("parameter count overflow in '%s': total would exceed maximum value", path)
		}
		total += count
	}

	return total, nil
}

// ParseSafetensorsIndexWithQuant parses a model.safetensors.index.json file for sharded models with quantization-aware adjustments
func ParseSafetensorsIndexWithQuant(indexPath string, quantMethod string) (int64, error) {
	if indexPath == "" {
		return 0, fmt.Errorf("index path cannot be empty")
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read safetensors index file '%s': %w", indexPath, err)
	}

	var index struct {
		Weight_map map[string]string `json:"weight_map"`
	}

	if err := json.Unmarshal(data, &index); err != nil {
		return 0, fmt.Errorf("failed to parse safetensors index JSON from '%s': %w", indexPath, err)
	}

	if len(index.Weight_map) == 0 {
		return 0, fmt.Errorf("no weight mappings found in safetensors index '%s'", indexPath)
	}

	// Get unique shard files
	shardFiles := make(map[string]bool)
	for _, shard := range index.Weight_map {
		if shard == "" {
			return 0, fmt.Errorf("empty shard filename found in index '%s'", indexPath)
		}
		shardFiles[shard] = true
	}

	dir := filepath.Dir(indexPath)
	var total int64 = 0

	// Parse each shard file
	for shard := range shardFiles {
		shardPath := filepath.Join(dir, shard)
		count, err := ParseSafetensorsWithQuant(shardPath, quantMethod)
		if err != nil {
			return 0, fmt.Errorf("failed to parse shard '%s' referenced in index '%s': %w",
				shardPath, indexPath, err)
		}

		// Check for overflow when adding
		if total > math.MaxInt64-count {
			return 0, fmt.Errorf("parameter count overflow when processing shards from '%s': total would exceed maximum value", indexPath)
		}
		total += count
	}

	return total, nil
}

// isAuxTensor returns true if the tensor name looks like a quantization auxiliary tensor
func isAuxTensor(name string) bool {
	n := strings.ToLower(name)

	// Common aux patterns: per-(group/channel) scales/zeros, grouping, stats, masks, etc.
	auxNeedles := []string{
		"scale", "scales", "zero", "zeros", "qscale", "qzeros",
		"g_idx", "gidx", "group_idx", "group_index",
		"quant_state", "quantizer",
		"absmax", "amax", "inv_scale", "ninv",
		"clip", "saturation",
		"bits", "bit_width",
		"bias_mask", "weight_mask",
		"hist", "histogram",
	}

	for _, s := range auxNeedles {
		if strings.Contains(n, s) {
			return true
		}
	}

	return false
}

// isWeightLike returns true for main weight tensors
func isWeightLike(name string) bool {
	n := strings.ToLower(name)

	// Explicit inclusions (packed 4-bit weight tensors)
	if strings.Contains(n, "qweight") || strings.Contains(n, "packed_weight") {
		return true
	}

	// Typical HF naming for matmul weights
	// Note: Must use ".weight" (with dot) to avoid false matches like "weight_mask"
	if strings.HasSuffix(n, ".weight") {
		// Exclude common non-matmul params
		if strings.Contains(n, "norm") || strings.Contains(n, "layernorm") ||
			strings.Contains(n, "embed") || strings.Contains(n, "embedding") {
			return false
		}
		// Also ensure not aux
		return !isAuxTensor(n)
	}
	return false
}
