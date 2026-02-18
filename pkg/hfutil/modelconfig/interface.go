package modelconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// HuggingFaceModel defines the common interface for all Hugging Face model configurations
type HuggingFaceModel interface {
	// GetParameterCount returns the total number of parameters in the model
	GetParameterCount() int64

	// GetTransformerVersion returns the transformers library version used for this model
	GetTransformerVersion() string

	// GetQuantizationType returns the quantization method used (if any)
	GetQuantizationType() string

	// GetArchitecture returns the model architecture (e.g., "LlamaForCausalLM")
	GetArchitecture() string

	// GetModelType returns the model type (e.g., "llama", "deepseek_v3")
	GetModelType() string

	// GetContextLength returns the maximum context length supported by the model
	GetContextLength() int

	// GetModelSizeBytes returns the estimated size of the model in bytes
	GetModelSizeBytes() int64

	// GetTorchDtype returns the torch data type used by the model
	GetTorchDtype() string

	// HasVision returns true if this is a multimodal vision model
	HasVision() bool
}

// HuggingFaceDiffusionModel represents diffusion models that expose pipeline metadata.
type HuggingFaceDiffusionModel interface {
	HuggingFaceModel
	GetDiffusionModel() *DiffusionPipelineSpec
}

// AutoMap defines the mapping of model classes for custom Hugging Face models
// This is used when models require custom code (e.g., models with "trust_remote_code=True")
type AutoMap struct {
	AutoConfig           string `json:"AutoConfig,omitempty"`
	AutoModel            string `json:"AutoModel,omitempty"`
	AutoModelForCausalLM string `json:"AutoModelForCausalLM,omitempty"`
}

// BaseModelConfig defines common fields shared across all Hugging Face model configurations
type BaseModelConfig struct {
	// Basic model information
	ModelType          string   `json:"model_type"`
	Architectures      []string `json:"architectures"`
	TorchDtype         string   `json:"torch_dtype"`
	TransformerVersion string   `json:"transformers_version"`

	// Internal fields (not in JSON)
	ConfigPath string `json:"-"`
}

// Common implementations for base methods
func (c *BaseModelConfig) GetModelType() string {
	return c.ModelType
}

func (c *BaseModelConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

func (c *BaseModelConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return ""
}

func (c *BaseModelConfig) GetTorchDtype() string {
	return c.TorchDtype
}

// Default implementation for HasVision - most models don't have vision capabilities
func (c *BaseModelConfig) HasVision() bool {
	return false
}

// QuantizationConfig defines the structure for quantization settings
type QuantizationConfig struct {
	ActivationScheme string `json:"activation_scheme"`
	Format           string `json:"fmt"`
	QuantMethod      string `json:"quant_method"`
	WeightBlockSize  []int  `json:"weight_block_size"`
}

// RopeScalingConfig defines the structure for RoPE (Rotary Position Embedding) scaling
type RopeScalingConfig struct {
	Type                          string  `json:"type,omitempty"`
	RopeType                      string  `json:"rope_type,omitempty"`
	Factor                        float64 `json:"factor"`
	LowFreqFactor                 float64 `json:"low_freq_factor,omitempty"`
	HighFreqFactor                float64 `json:"high_freq_factor,omitempty"`
	BetaFast                      float64 `json:"beta_fast,omitempty"`
	BetaSlow                      float64 `json:"beta_slow,omitempty"`
	MScale                        float64 `json:"mscale,omitempty"`
	MScaleAllDim                  float64 `json:"mscale_all_dim,omitempty"`
	OriginalMaxPositionEmbeddings int     `json:"original_max_position_embeddings"`
}

// DtypeSizeBytes maps torch data types to their size in bytes per parameter
var DtypeSizeBytes = map[string]float64{
	"float32":  4.0,
	"float":    4.0,
	"bfloat16": 2.0,
	"bf16":     2.0,
	"float16":  2.0,
	"fp16":     2.0,
	"half":     2.0,
	"int8":     1.0,
	"fp8":      1.0,
	"float8":   1.0,
	"e4m3":     1.0,
	"int4":     0.5,
	"4bit":     0.5,
}

// FormatSize formats the file size in a human-readable form
func FormatSize(size int64) string {
	// Define storage units in bytes
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)

	switch {
	case size < kb:
		return fmt.Sprintf("%d B", size)
	case size < mb:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(kb))
	case size < gb:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(mb))
	case size < tb:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(gb))
	default:
		return fmt.Sprintf("%.2f TB", float64(size)/float64(tb))
	}
}

// FormatParamCount converts a parameter count to a human-readable string
// Examples: 1000 -> "1K", 1500000 -> "1.5M", 685000000000 -> "685B"
func FormatParamCount(count int64) string {
	const (
		thousand = 1_000
		million  = 1_000_000
		billion  = 1_000_000_000
		trillion = 1_000_000_000_000
	)

	var value float64
	var suffix string

	switch {
	case count >= trillion:
		value = float64(count) / float64(trillion)
		suffix = "T"
	case count >= billion:
		value = float64(count) / float64(billion)
		suffix = "B"
	case count >= million:
		value = float64(count) / float64(million)
		suffix = "M"
	case count >= thousand:
		value = float64(count) / float64(thousand)
		suffix = "K"
	default:
		return fmt.Sprintf("%d", count)
	}

	// If the value is a whole number, don't show decimal places
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d%s", int64(value), suffix)
	}

	// If the value has less than 2 significant decimal places, only show what's needed
	if value*100 == float64(int64(value*100)) {
		return fmt.Sprintf("%.1f%s", value, suffix)
	}

	return fmt.Sprintf("%.2f%s", value, suffix)
}

// EstimateModelSizeBytes estimates model size in bytes based on parameter count and data type
func EstimateModelSizeBytes(paramCount int64, dtype string) int64 {
	sizePerParam, ok := DtypeSizeBytes[strings.ToLower(dtype)]
	if !ok {
		sizePerParam = 4.0 // default to float32
	}
	return int64(float64(paramCount) * sizePerParam)
}

// Map of model keys to model loader functions with thread-safe access.
// For transformer-style config.json, the key is model_type.
// For diffusion model_index.json, the key is the pipeline class name.
var (
	modelLoadersMu sync.RWMutex
	modelLoaders   = make(map[string]func(string) (HuggingFaceModel, error))
)

// GenericModelConfig is a fallback configuration for unsupported model types.
// It provides basic functionality by parsing common fields from the config.json
// and attempting to get parameter count from safetensors files.
type GenericModelConfig struct {
	BaseModelConfig

	// Common fields that most models have
	HiddenSize            int `json:"hidden_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	IntermediateSize      int `json:"intermediate_size"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// Quantization config (optional)
	QuantizationConfig *QuantizationConfig `json:"quantization_config,omitempty"`
}

// GetParameterCount attempts to get parameter count from safetensors, falls back to estimation
func (c *GenericModelConfig) GetParameterCount() int64 {
	// First try to get from safetensors
	if c.ConfigPath != "" {
		count, err := FindAndParseSafetensors(c.ConfigPath)
		if err == nil && count > 0 {
			return count
		}
	}

	// Fallback: estimate from architecture if we have the necessary fields
	if c.HiddenSize > 0 && c.NumHiddenLayers > 0 {
		return estimateGenericParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
	}

	return 0
}

// estimateGenericParams provides a rough parameter estimate for transformer models
func estimateGenericParams(hiddenSize, numLayers, intermediateSize, vocabSize int) int64 {
	if intermediateSize == 0 {
		intermediateSize = hiddenSize * 4 // Common default ratio
	}

	// Rough estimate based on typical transformer architecture:
	// - Embeddings: vocab_size * hidden_size
	// - Per layer: ~12 * hidden_size^2 (attention + MLP)
	// - Output: vocab_size * hidden_size (often tied with embeddings)
	embeddingParams := int64(vocabSize) * int64(hiddenSize)
	perLayerParams := int64(12) * int64(hiddenSize) * int64(hiddenSize)
	totalLayerParams := int64(numLayers) * perLayerParams

	return embeddingParams + totalLayerParams
}

func (c *GenericModelConfig) GetQuantizationType() string {
	if c.QuantizationConfig != nil && c.QuantizationConfig.QuantMethod != "" {
		return c.QuantizationConfig.QuantMethod
	}
	return ""
}

func (c *GenericModelConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

func (c *GenericModelConfig) GetModelSizeBytes() int64 {
	paramCount := c.GetParameterCount()
	if paramCount == 0 {
		return 0
	}
	return EstimateModelSizeBytes(paramCount, c.TorchDtype)
}

// GenericDiffusionModelConfig is a fallback configuration for diffusers model_index.json files.
// It includes common diffusion pipeline components for easy access.
type GenericDiffusionModelConfig struct {
	BaseModelConfig

	DiffusersVersion  string
	DiffusionPipeline *DiffusionPipelineSpec
}

func (c *GenericDiffusionModelConfig) GetDiffusionModel() *DiffusionPipelineSpec {
	return c.DiffusionPipeline
}

func (c *GenericDiffusionModelConfig) GetParameterCount() int64 {
	if c.ConfigPath == "" {
		return 0
	}

	baseDir := filepath.Dir(c.ConfigPath)
	textEncoderConfig := filepath.Join(baseDir, "text_encoder", "config.json")
	transformerConfig := filepath.Join(baseDir, "transformer", "config.json")
	if _, err := os.Stat(transformerConfig); err != nil {
		unetConfig := filepath.Join(baseDir, "unet", "config.json")
		if _, unetErr := os.Stat(unetConfig); unetErr == nil {
			transformerConfig = unetConfig
		}
	}
	vaeConfig := filepath.Join(baseDir, "vae", "config.json")

	total := int64(0)

	add := func(label, configPath string, required bool) bool {
		if configPath == "" {
			if required {
				fmt.Printf("Warning: %s config path is empty\n", label)
				return false
			}
			return true
		}
		if _, err := os.Stat(configPath); err != nil {
			if required {
				fmt.Printf("Warning: %s config not found at '%s': %v\n", label, configPath, err)
				return false
			}
			return true
		}

		count, err := FindAndParseSafetensors(configPath)
		if err != nil {
			if required {
				fmt.Printf("Warning: failed to parse %s safetensors: %v\n", label, err)
				return false
			}
			return true
		}
		if total > math.MaxInt64-count {
			fmt.Printf("Warning: parameter count overflow when adding %s\n", label)
			return false
		}
		total += count
		return true
	}

	if !add("text_encoder", textEncoderConfig, true) {
		return 0
	}
	if !add("transformer", transformerConfig, true) {
		return 0
	}
	if !add("vae", vaeConfig, true) {
		return 0
	}

	for name := range c.DiffusionPipeline.AdditionalComponents {
		configPath := filepath.Join(baseDir, name, "config.json")
		if !add(name, configPath, false) {
			return 0
		}
	}

	return total
}

func (c *GenericDiffusionModelConfig) GetQuantizationType() string {
	// Not supported. Doesn't seem to be standardized in HF.
	return ""
}

func (c *GenericDiffusionModelConfig) GetContextLength() int {
	if c.ConfigPath == "" {
		return 0
	}

	textEncoderConfig := filepath.Join(filepath.Dir(c.ConfigPath), "text_encoder", "config.json")
	data, err := os.ReadFile(textEncoderConfig)
	if err != nil {
		return 0
	}

	data = SanitizeJSONBytes(data)

	var meta struct {
		MaxPositionEmbeds int `json:"max_position_embeddings"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return 0
	}
	if meta.MaxPositionEmbeds > 0 {
		return meta.MaxPositionEmbeds
	}
	return 0
}

func (c *GenericDiffusionModelConfig) GetModelSizeBytes() int64 {
	paramCount := c.GetParameterCount()
	if paramCount == 0 {
		return 0
	}
	return EstimateModelSizeBytes(paramCount, c.TorchDtype)
}

func (c *GenericDiffusionModelConfig) HasVision() bool {
	return true
}

// loadGenericModelConfig loads a generic model configuration as a fallback.
// It also handles diffusers model_index.json files when model_type is absent.
func loadGenericModelConfig(configPath string) (HuggingFaceModel, error) {
	if filepath.Base(configPath) == "model_index.json" {
		pipeline, err := LoadDiffusionPipelineSpec(configPath)
		if err != nil {
			return nil, err
		}

		config := &GenericDiffusionModelConfig{
			DiffusersVersion:  pipeline.DiffusersVersion,
			DiffusionPipeline: pipeline,
		}
		config.ConfigPath = configPath
		config.ModelType = "diffusers"
		if pipeline.ClassName != "" {
			config.Architectures = []string{pipeline.ClassName}
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Sanitize JSON to handle non-standard values like Infinity, -Infinity, NaN
	// which are valid in Python but not in standard JSON
	data = SanitizeJSONBytes(data)

	var config GenericModelConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// RegisterModelLoader safely registers a model loader function for a given model key.
// For transformer-style config.json, the key is model_type.
// For diffusion model_index.json, the key is the pipeline class name.
func RegisterModelLoader(modelKey string, loader func(string) (HuggingFaceModel, error)) {
	if modelKey == "" {
		panic("model key cannot be empty")
	}
	if loader == nil {
		panic("loader function cannot be nil")
	}

	modelLoadersMu.Lock()
	defer modelLoadersMu.Unlock()
	modelLoaders[modelKey] = loader
}

// GetSupportedModelTypes returns a list of all registered model keys.
// These are model_type values for transformer configs and class names for diffusion pipelines.
func GetSupportedModelTypes() []string {
	modelLoadersMu.RLock()
	defer modelLoadersMu.RUnlock()

	types := make([]string, 0, len(modelLoaders))
	for modelKey := range modelLoaders {
		types = append(types, modelKey)
	}
	return types
}

// Regex patterns for sanitizing non-standard JSON values
var (
	// Match Infinity in JSON contexts (after colon, comma, or opening bracket)
	infinityRegex = regexp.MustCompile(`([:,\[]\s*)Infinity(\s*[,\]\}])`)
	// Match -Infinity
	negInfinityRegex = regexp.MustCompile(`([:,\[]\s*)-Infinity(\s*[,\]\}])`)
	// Match NaN in JSON contexts
	nanRegex = regexp.MustCompile(`([:,\[]\s*)NaN(\s*[,\]\}])`)
)

// SanitizeJSONBytes sanitizes JSON data by replacing JavaScript/Python-style
// special float values (Infinity, -Infinity, NaN) with JSON-compatible values.
// This is necessary because some model configs (e.g., NVIDIA Nemotron) contain
// these non-standard JSON values that Python's json module accepts but Go's doesn't.
func SanitizeJSONBytes(data []byte) []byte {
	s := string(data)

	// Replace Infinity with a very large number (close to float64 max)
	s = infinityRegex.ReplaceAllString(s, "${1}1e308${2}")

	// Replace -Infinity with a very small number
	s = negInfinityRegex.ReplaceAllString(s, "${1}-1e308${2}")

	// Replace NaN with null (NaN is not a valid JSON value)
	s = nanRegex.ReplaceAllString(s, "${1}null${2}")

	return []byte(s)
}

// LoadModelConfig loads a model configuration from a config.json or model_index.json file
// and returns the appropriate model implementation based on the model key:
// - transformer config.json: "model_type"
// - diffusion model_index.json: "_class_name" (pipeline class name)
// This is the main entry point for users who want to load any supported model
// without knowing its specific type in advance.
//
// Example usage:
//
//	config, err := huggingfaceconfig.LoadModelConfig("/path/to/config.json")
//	if err != nil {
//		// handle error
//	}
//	// Use the model interface methods
//	paramCount := config.GetParameterCount()
//	contextLength := config.GetContextLength()
//
// The function automatically detects the model type and returns the appropriate
// implementation that satisfies the HuggingFaceModel interface.
func LoadModelConfig(configPath string) (HuggingFaceModel, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	// Read the config file to determine model type
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read model config file '%s': %w", configPath, err)
	}

	// Sanitize JSON to handle non-standard values like Infinity, -Infinity, NaN
	// which are valid in Python but not in standard JSON
	data = SanitizeJSONBytes(data)

	// Extract the model_type field
	var baseConfig struct {
		ModelType string `json:"model_type"`
	}

	if err := json.Unmarshal(data, &baseConfig); err != nil {
		return nil, fmt.Errorf("failed to parse model config JSON from '%s': %w", configPath, err)
	}

	modelKey := baseConfig.ModelType
	diffusionClassName := ""
	if modelKey == "" && filepath.Base(configPath) == "model_index.json" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			diffusionClassName = parseJSONStringField(raw, "_class_name", "class_name", "className")
		}
		if diffusionClassName != "" {
			modelKey = diffusionClassName
		}
	}

	if modelKey == "" {
		return nil, fmt.Errorf("model_type or _class_name field is missing or empty in config file '%s'", configPath)
	}

	// Load using registered model loaders (thread-safe access)
	modelLoadersMu.RLock()
	loader, exists := modelLoaders[modelKey]
	modelLoadersMu.RUnlock()

	if !exists {
		// Fallback to generic config for unsupported model types
		// Log a warning but still return useful data
		log.Printf("Warning: model type '%s' is not fully supported, using generic config. "+
			"Parameter count will be estimated from safetensors or architecture.",
			baseConfig.ModelType)

		return loadGenericModelConfig(configPath)
	}

	model, err := loader(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s model from '%s': %w", modelKey, configPath, err)
	}

	return model, nil
}
