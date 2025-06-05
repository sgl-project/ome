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
// and parses it to count the total number of parameters
func FindAndParseSafetensors(configPath string) (int64, error) {
	if configPath == "" {
		return 0, fmt.Errorf("config path cannot be empty")
	}

	dir := filepath.Dir(configPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to list directory '%s': %w", dir, err)
	}

	// Look for index.json file first, which might point to sharded safetensors
	indexPath := filepath.Join(dir, "model.safetensors.index.json")
	if _, err := os.Stat(indexPath); err == nil {
		count, err := ParseSafetensorsIndex(indexPath)
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
			params, err := ParseSafetensors(fullPath)
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

// ParseSafetensors parses a single safetensors file and counts parameters
func ParseSafetensors(path string) (int64, error) {
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

		// Check for overflow when adding to total
		if total > math.MaxInt64-count {
			return 0, fmt.Errorf("parameter count overflow in '%s': total would exceed maximum value", path)
		}
		total += count
	}

	return total, nil
}

// ParseSafetensorsIndex parses a model.safetensors.index.json file for sharded models
func ParseSafetensorsIndex(indexPath string) (int64, error) {
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
		count, err := ParseSafetensors(shardPath)
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
