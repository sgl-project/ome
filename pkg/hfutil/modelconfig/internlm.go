package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// InternLMConfig defines the configuration for InternLM models
type InternLMConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id"`

	// Attention related
	HiddenAct   string      `json:"hidden_act"`
	RmsNormEps  float64     `json:"rms_norm_eps"`
	RopeTheta   float64     `json:"rope_theta"`
	RopeScaling interface{} `json:"rope_scaling"` // Can be null or RopeScalingConfig
	Bias        bool        `json:"bias"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
}

// LoadInternLMConfig loads an InternLM model configuration from a JSON file
func LoadInternLMConfig(configPath string) (*InternLMConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read InternLM config file '%s': %w", configPath, err)
	}

	var config InternLMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse InternLM config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *InternLMConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known InternLM model sizes
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // InternLM2 7B
	} else if c.HiddenSize == 6144 && c.NumHiddenLayers == 48 {
		return 20_000_000_000 // InternLM2 20B
	} else if c.HiddenSize == 2048 && c.NumHiddenLayers == 24 {
		return 1_800_000_000 // InternLM2 1.8B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *InternLMConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *InternLMConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *InternLMConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for InternLM base models
func (c *InternLMConfig) HasVision() bool {
	return false
}

// Register the InternLM model handler
func init() {
	RegisterModelLoader("internlm", func(configPath string) (HuggingFaceModel, error) {
		return LoadInternLMConfig(configPath)
	})
	RegisterModelLoader("internlm2", func(configPath string) (HuggingFaceModel, error) {
		return LoadInternLMConfig(configPath)
	})
}
