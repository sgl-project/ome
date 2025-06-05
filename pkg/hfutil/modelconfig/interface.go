package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
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

// Map of model type to model loader functions with thread-safe access
var (
	modelLoadersMu sync.RWMutex
	modelLoaders   = make(map[string]func(string) (HuggingFaceModel, error))
)

// RegisterModelLoader safely registers a model loader function for a given model type
func RegisterModelLoader(modelType string, loader func(string) (HuggingFaceModel, error)) {
	if modelType == "" {
		panic("model type cannot be empty")
	}
	if loader == nil {
		panic("loader function cannot be nil")
	}

	modelLoadersMu.Lock()
	defer modelLoadersMu.Unlock()
	modelLoaders[modelType] = loader
}

// GetSupportedModelTypes returns a list of all supported model types
func GetSupportedModelTypes() []string {
	modelLoadersMu.RLock()
	defer modelLoadersMu.RUnlock()

	types := make([]string, 0, len(modelLoaders))
	for modelType := range modelLoaders {
		types = append(types, modelType)
	}
	return types
}

// LoadModelConfig loads a model configuration from a config.json file
// and returns the appropriate model implementation based on the "model_type" field.
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

	// Extract the model_type field
	var baseConfig struct {
		ModelType string `json:"model_type"`
	}

	if err := json.Unmarshal(data, &baseConfig); err != nil {
		return nil, fmt.Errorf("failed to parse model config JSON from '%s': %w", configPath, err)
	}

	if baseConfig.ModelType == "" {
		return nil, fmt.Errorf("model_type field is missing or empty in config file '%s'", configPath)
	}

	// Load using registered model loaders (thread-safe access)
	modelLoadersMu.RLock()
	loader, exists := modelLoaders[baseConfig.ModelType]
	modelLoadersMu.RUnlock()

	if !exists {
		supportedTypes := GetSupportedModelTypes()
		return nil, fmt.Errorf("unsupported model type '%s' in config file '%s'. Supported types are: %v",
			baseConfig.ModelType, configPath, supportedTypes)
	}

	model, err := loader(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s model from '%s': %w", baseConfig.ModelType, configPath, err)
	}

	return model, nil
}
