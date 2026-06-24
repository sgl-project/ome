package modelconfig

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxSafetensorsHeaderSize = 10 * 1024 * 1024

	// SafetensorsExt is the file extension of a safetensors shard.
	SafetensorsExt = ".safetensors"
	// SafetensorsIndexFileName is the manifest sharded safetensors
	// models ship listing tensor_name → shard_filename.
	SafetensorsIndexFileName = "model.safetensors.index.json"
)

// SafetensorsFileInput provides a safetensors-related file from a non-filesystem source.
// Data for .safetensors files only needs to contain the 8-byte header length followed by
// the JSON header bytes; tensor payload bytes are not required for metadata parsing.
type SafetensorsFileInput struct {
	Path string
	Data []byte
}

// FindAndParseSafetensors looks for a safetensors file in the same directory as the config file
// and parses it to count the total number of parameters. Naive count — see
// FindAndParseSafetensorsWithQuantConfig for the packed-quant-aware variant.
func FindAndParseSafetensors(configPath string) (int64, error) {
	return FindAndParseSafetensorsWithQuantConfig(configPath, nil)
}

// FindAndParseSafetensorsWithQuantConfig is the quant-aware variant.
// Non-nil quantConfig enables per-tensor classification (packed-quant
// storage multiplier + scale-tensor skip + exclude_modules); nil
// preserves the legacy naive-count behavior. Prefers a sibling
// model.safetensors.index.json over scanning *.safetensors files
// because the index gives us deterministic shard ordering.
func FindAndParseSafetensorsWithQuantConfig(configPath string, quantConfig *HFQuantConfig) (int64, error) {
	if configPath == "" {
		return 0, fmt.Errorf("config path cannot be empty")
	}

	dir := filepath.Dir(configPath)

	indexPath := filepath.Join(dir, SafetensorsIndexFileName)
	if _, err := os.Stat(indexPath); err == nil {
		count, err := parseSafetensorsIndexFile(indexPath, quantConfig)
		if err != nil {
			return 0, fmt.Errorf("parse safetensors index %s: %w", indexPath, err)
		}
		return count, nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("list directory %s: %w", dir, err)
	}

	var totalParams int64
	safetensorsFound := false
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), SafetensorsExt) {
			continue
		}
		fullPath := filepath.Join(dir, f.Name())
		params, err := parseSafetensorsFile(fullPath, quantConfig)
		if err != nil {
			return 0, fmt.Errorf("parse safetensors file %s: %w", fullPath, err)
		}
		if totalParams > math.MaxInt64-params {
			return 0, fmt.Errorf("parameter count overflow when processing %s", fullPath)
		}
		totalParams += params
		safetensorsFound = true
	}
	if !safetensorsFound {
		return 0, fmt.Errorf("no .safetensors files found in directory %s", dir)
	}
	return totalParams, nil
}

// FindAndParseSafetensorsFiles parses safetensors metadata from an explicit set of files.
// It supports either model.safetensors.index.json plus referenced shard headers, or
// all .safetensors files in the set when no index is present. Naive count — see
// FindAndParseSafetensorsFilesWithQuantConfig for the packed-quant-aware variant.
func FindAndParseSafetensorsFiles(files []SafetensorsFileInput) (int64, error) {
	return FindAndParseSafetensorsFilesWithQuantConfig(files, nil)
}

// FindAndParseSafetensorsFilesWithQuantConfig is the quant-aware
// variant. See FindAndParseSafetensorsWithQuantConfig.
func FindAndParseSafetensorsFilesWithQuantConfig(files []SafetensorsFileInput, quantConfig *HFQuantConfig) (int64, error) {
	if len(files) == 0 {
		return 0, fmt.Errorf("safetensors file input set is empty")
	}

	byPath := make(map[string][]byte, len(files))
	var indexPaths []string
	var safetensorsPaths []string
	for _, file := range files {
		filePath := cleanSafetensorsInputPath(file.Path)
		if filePath == "" {
			continue
		}
		byPath[filePath] = file.Data
		switch path.Base(filePath) {
		case SafetensorsIndexFileName:
			indexPaths = append(indexPaths, filePath)
		default:
			if strings.HasSuffix(path.Base(filePath), SafetensorsExt) {
				safetensorsPaths = append(safetensorsPaths, filePath)
			}
		}
	}

	sort.Strings(indexPaths)
	if len(indexPaths) > 0 {
		indexPath := indexPaths[0]
		indexDir := path.Dir(indexPath)
		return parseSafetensorsIndexData(byPath[indexPath], indexPath, func(shard string) (int64, error) {
			shardPath := cleanSafetensorsInputPath(shard)
			if shardPath == "" {
				return 0, fmt.Errorf("empty shard filename found in index '%s'", indexPath)
			}
			if indexDir != "." {
				shardPath = path.Join(indexDir, shardPath)
			}
			data, ok := byPath[shardPath]
			if !ok {
				return 0, fmt.Errorf("shard '%s' referenced in index '%s' was not provided", shardPath, indexPath)
			}
			return ParseSafetensorsBytesWithQuantConfig(data, shardPath, quantConfig)
		})
	}

	sort.Strings(safetensorsPaths)
	if len(safetensorsPaths) == 0 {
		return 0, fmt.Errorf("no .safetensors files found in input set")
	}

	var totalParams int64
	for _, safetensorsPath := range safetensorsPaths {
		params, err := ParseSafetensorsBytesWithQuantConfig(byPath[safetensorsPath], safetensorsPath, quantConfig)
		if err != nil {
			return 0, fmt.Errorf("failed to parse safetensors file '%s': %w", safetensorsPath, err)
		}
		if totalParams > math.MaxInt64-params {
			return 0, fmt.Errorf("parameter count overflow when processing '%s': total would exceed maximum value", safetensorsPath)
		}
		totalParams += params
	}
	return totalParams, nil
}

// ParseSafetensors parses a single safetensors file (naive count).
func ParseSafetensors(path string) (int64, error) {
	return parseSafetensorsFile(path, nil)
}

func parseSafetensorsFile(path string, quantConfig *HFQuantConfig) (int64, error) {
	if path == "" {
		return 0, fmt.Errorf("safetensors file path cannot be empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open safetensors file '%s': %w", path, err)
	}
	defer file.Close()

	return parseSafetensorsReader(file, path, quantConfig)
}

// ParseSafetensorsBytes parses metadata from bytes containing the
// 8-byte header length followed by the JSON header (naive count).
func ParseSafetensorsBytes(data []byte, label string) (int64, error) {
	return ParseSafetensorsBytesWithQuantConfig(data, label, nil)
}

// ParseSafetensorsBytesWithQuantConfig is the quant-aware variant.
func ParseSafetensorsBytesWithQuantConfig(data []byte, label string, quantConfig *HFQuantConfig) (int64, error) {
	return parseSafetensorsReader(bytes.NewReader(data), label, quantConfig)
}

// ParseSafetensorsReader parses metadata from a reader (naive count).
func ParseSafetensorsReader(reader io.Reader, label string) (int64, error) {
	return parseSafetensorsReader(reader, label, nil)
}

func parseSafetensorsReader(reader io.Reader, label string, quantConfig *HFQuantConfig) (int64, error) {
	if label == "" {
		label = "safetensors input"
	}

	headerLenBuf := make([]byte, 8)
	if _, err := io.ReadFull(reader, headerLenBuf); err != nil {
		return 0, fmt.Errorf("read header length from %s: %w", label, err)
	}
	headerLen := binary.LittleEndian.Uint64(headerLenBuf)

	// Defensive cap — a corrupted file can claim an absurd header
	// length and exhaust memory if we blindly allocate.
	if headerLen > maxSafetensorsHeaderSize {
		return 0, fmt.Errorf("header length %d in %s exceeds maximum %d bytes",
			headerLen, label, maxSafetensorsHeaderSize)
	}

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return 0, fmt.Errorf("read JSON header from %s: %w", label, err)
	}
	return ParseSafetensorsHeaderQuantAware(headerBytes, label, quantConfig)
}

// ParseSafetensorsHeader counts parameters naively (sum of shape
// products). Backward-compat shim — new callers with a quant config
// should use ParseSafetensorsHeaderQuantAware for accurate counts on
// packed-quant formats.
func ParseSafetensorsHeader(headerBytes []byte, label string) (int64, error) {
	return ParseSafetensorsHeaderQuantAware(headerBytes, label, nil)
}

// ParseSafetensorsHeaderQuantAware counts logical parameters per
// tensor via CountTensorParams. Nil quantConfig matches the legacy
// naive sum.
func ParseSafetensorsHeaderQuantAware(headerBytes []byte, label string, quantConfig *HFQuantConfig) (int64, error) {
	if label == "" {
		label = "safetensors input"
	}

	var raw map[string]struct {
		Shape       []int64  `json:"shape"`
		Dtype       string   `json:"dtype"`
		DataOffsets [2]int64 `json:"data_offsets"`
	}

	if err := json.Unmarshal(headerBytes, &raw); err != nil {
		return 0, fmt.Errorf("failed to parse JSON header from '%s': %w", label, err)
	}

	var total int64 = 0
	for tensorName, tensor := range raw {
		if tensorName == "__metadata__" {
			continue
		}
		// Up-front overflow guard on the shape product so the error
		// message can blame the specific tensor — CountTensorParams's
		// own guards return 0 silently.
		if len(tensor.Shape) == 0 {
			continue
		}
		var dimProduct int64 = 1
		for i, dim := range tensor.Shape {
			if dim <= 0 {
				return 0, fmt.Errorf("tensor '%s' in '%s' has invalid dimension %d at index %d",
					tensorName, label, dim, i)
			}
			if dimProduct > 0 && dim > math.MaxInt64/dimProduct {
				return 0, fmt.Errorf("dimension overflow for tensor '%s' in '%s': multiplication would exceed maximum value",
					tensorName, label)
			}
			dimProduct *= dim
		}

		count := CountTensorParams(tensorName, tensor.Shape, tensor.Dtype, quantConfig)
		if count == 0 {
			continue
		}
		if total > math.MaxInt64-count {
			return 0, fmt.Errorf("parameter count overflow in '%s': total would exceed maximum value", label)
		}
		total += count
	}

	return total, nil
}

// ParseSafetensorsIndex parses model.safetensors.index.json for
// sharded models (naive count).
func ParseSafetensorsIndex(indexPath string) (int64, error) {
	return parseSafetensorsIndexFile(indexPath, nil)
}

func parseSafetensorsIndexFile(indexPath string, quantConfig *HFQuantConfig) (int64, error) {
	if indexPath == "" {
		return 0, fmt.Errorf("index path cannot be empty")
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read safetensors index file '%s': %w", indexPath, err)
	}

	dir := filepath.Dir(indexPath)
	return parseSafetensorsIndexData(data, indexPath, func(shard string) (int64, error) {
		shardPath := filepath.Join(dir, shard)
		return parseSafetensorsFile(shardPath, quantConfig)
	})
}

func parseSafetensorsIndexData(data []byte, indexPath string, parseShard func(string) (int64, error)) (int64, error) {
	var index struct {
		WeightMap map[string]string `json:"weight_map"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return 0, fmt.Errorf("parse safetensors index JSON from %s: %w", indexPath, err)
	}
	if len(index.WeightMap) == 0 {
		return 0, fmt.Errorf("no weight mappings found in safetensors index %s", indexPath)
	}

	// Dedupe shard filenames; the index maps every tensor name to a
	// shard, so the same shard appears many times.
	shardSet := make(map[string]struct{})
	for _, shard := range index.WeightMap {
		if shard == "" {
			return 0, fmt.Errorf("empty shard filename found in index %s", indexPath)
		}
		shardSet[shard] = struct{}{}
	}
	shards := make([]string, 0, len(shardSet))
	for shard := range shardSet {
		shards = append(shards, shard)
	}
	sort.Strings(shards)

	var total int64
	for _, shard := range shards {
		count, err := parseShard(shard)
		if err != nil {
			return 0, fmt.Errorf("parse shard %s referenced in index %s: %w", shard, indexPath, err)
		}
		if total > math.MaxInt64-count {
			return 0, fmt.Errorf("parameter count overflow processing shards from %s", indexPath)
		}
		total += count
	}
	return total, nil
}

func cleanSafetensorsInputPath(filePath string) string {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return ""
	}
	filePath = strings.TrimLeft(filepath.ToSlash(filePath), "/")
	return path.Clean(filePath)
}
