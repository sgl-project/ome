package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// ExaoneConfig defines the configuration for LGAI ExaONE models
type ExaoneConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumLayers             int `json:"num_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`
	HeadDim               int `json:"head_dim"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id"`

	// Activation and dropout
	ActivationFunction string  `json:"activation_function"`
	AttentionDropout   float64 `json:"attention_dropout"`
	EmbedDropout       float64 `json:"embed_dropout"`

	// RoPE scaling
	RopeScaling *struct {
		Factor                        float64 `json:"factor"`
		HighFreqFactor                float64 `json:"high_freq_factor"`
		LowFreqFactor                 float64 `json:"low_freq_factor"`
		OriginalMaxPositionEmbeddings int     `json:"original_max_position_embeddings"`
		RopeType                      string  `json:"rope_type"`
	} `json:"rope_scaling"`

	// Layer norm
	LayerNormEpsilon float64 `json:"layer_norm_epsilon"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
}

// LoadExaoneConfig loads an ExaONE model configuration from a JSON file
func LoadExaoneConfig(configPath string) (*ExaoneConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ExaONE config file '%s': %w", configPath, err)
	}

	var config ExaoneConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse ExaONE config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *ExaoneConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known ExaONE model sizes
	if c.HiddenSize == 4096 && c.NumLayers == 32 {
		return 7_800_000_000 // ExaONE 3.5 7.8B
	} else if c.HiddenSize == 2560 && c.NumLayers == 42 {
		return 2_800_000_000 // ExaONE 3.0 2.8B
	}

	// Fallback: estimate based on architecture
	vocabSize := c.VocabSize
	if vocabSize == 0 {
		vocabSize = 102400 // default ExaONE vocab size
	}
	return estimateModelParams(c.HiddenSize, c.NumLayers, c.IntermediateSize, vocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *ExaoneConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *ExaoneConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *ExaoneConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for ExaONE base models
func (c *ExaoneConfig) HasVision() bool {
	return false
}

// Register the ExaONE model handler
func init() {
	RegisterModelLoader("exaone", func(configPath string) (HuggingFaceModel, error) {
		return LoadExaoneConfig(configPath)
	})
}
