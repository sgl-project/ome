package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// StableLMConfig defines the configuration for StabilityAI StableLM models
type StableLMConfig struct {
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

	// Attention related
	HiddenAct           string  `json:"hidden_act"`
	LayerNormEps        float64 `json:"layer_norm_eps"`
	RopeTheta           float64 `json:"rope_theta"`
	PartialRotaryFactor float64 `json:"partial_rotary_factor"`
	UseQKVBias          bool    `json:"use_qkv_bias"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
}

// LoadStableLMConfig loads a StableLM model configuration from a JSON file
func LoadStableLMConfig(configPath string) (*StableLMConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read StableLM config file '%s': %w", configPath, err)
	}

	var config StableLMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse StableLM config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *StableLMConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known StableLM model sizes
	if c.HiddenSize == 2560 && c.NumHiddenLayers == 32 {
		return 3_000_000_000 // StableLM 3B
	} else if c.HiddenSize == 2048 && c.NumHiddenLayers == 24 {
		return 1_600_000_000 // StableLM 1.6B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *StableLMConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *StableLMConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *StableLMConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for StableLM base models
func (c *StableLMConfig) HasVision() bool {
	return false
}

// Register the StableLM model handler
func init() {
	RegisterModelLoader("stablelm", func(configPath string) (HuggingFaceModel, error) {
		return LoadStableLMConfig(configPath)
	})
}
