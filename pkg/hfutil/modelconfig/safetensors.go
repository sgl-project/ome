package modelconfig

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FindAndParseSafetensors looks for a safetensors file in the same directory as the config file
// and parses it to count the total number of parameters
func FindAndParseSafetensors(configPath string) (int64, error) {
	dir := filepath.Dir(configPath)
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to list directory: %v", err)
	}

	// Look for index.json file first, which might point to sharded safetensors
	indexPath := filepath.Join(dir, "model.safetensors.index.json")
	if _, err := os.Stat(indexPath); err == nil {
		return ParseSafetensorsIndex(indexPath)
	}

	// If no index file, process all safetensors files and sum their parameters
	var totalParams int64
	safetensorsFound := false

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".safetensors") {
			fullPath := filepath.Join(dir, f.Name())
			params, err := ParseSafetensors(fullPath)
			if err != nil {
				return 0, fmt.Errorf("error parsing %s: %v", f.Name(), err)
			}
			totalParams += params
			safetensorsFound = true
		}
	}

	if !safetensorsFound {
		return 0, fmt.Errorf("no .safetensors file found in %s", dir)
	}

	return totalParams, nil
}

// ParseSafetensors parses a single safetensors file and counts parameters
func ParseSafetensors(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	headerLenBuf := make([]byte, 8)
	if _, err := io.ReadFull(file, headerLenBuf); err != nil {
		return 0, fmt.Errorf("failed to read header length: %v", err)
	}
	headerLen := binary.LittleEndian.Uint64(headerLenBuf)

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(file, headerBytes); err != nil {
		return 0, fmt.Errorf("failed to read JSON header: %v", err)
	}

	var raw map[string]struct {
		Shape       []int64  `json:"shape"`
		Dtype       string   `json:"dtype"`
		DataOffsets [2]int64 `json:"data_offsets"`
	}

	if err := json.Unmarshal(headerBytes, &raw); err != nil {
		return 0, fmt.Errorf("failed to parse JSON header: %v", err)
	}

	var total int64 = 0
	for _, t := range raw {
		count := int64(1)
		for _, dim := range t.Shape {
			count *= dim
		}
		total += count
	}

	return total, nil
}

// ParseSafetensorsIndex parses a model.safetensors.index.json file for sharded models
func ParseSafetensorsIndex(indexPath string) (int64, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return 0, err
	}

	var index struct {
		Weight_map map[string]string `json:"weight_map"`
	}

	if err := json.Unmarshal(data, &index); err != nil {
		return 0, err
	}

	// Get unique shard files
	shardFiles := make(map[string]bool)
	for _, shard := range index.Weight_map {
		shardFiles[shard] = true
	}

	dir := filepath.Dir(indexPath)
	var total int64 = 0

	// Parse each shard file
	for shard := range shardFiles {
		shardPath := filepath.Join(dir, shard)
		count, err := ParseSafetensors(shardPath)
		if err != nil {
			return 0, fmt.Errorf("failed to parse shard %s: %v", shard, err)
		}
		total += count
	}

	return total, nil
}
