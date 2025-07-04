package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// CommandRConfig defines the configuration for Cohere Command-R models
type CommandRConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads,omitempty"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id,omitempty"`

	// Attention configuration
	LayerNorm          string  `json:"layer_norm,omitempty"`
	LayerNormEps       float64 `json:"layer_norm_eps"`
	UseQKNormalization bool    `json:"use_qk_normalization,omitempty"`

	// Activation
	HiddenAct string `json:"hidden_act"`

	// RoPE configuration
	RopeTheta float64 `json:"rope_theta,omitempty"`

	// Other settings
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`

	// Command-R specific
	LogitScale       float64 `json:"logit_scale,omitempty"`
	AttentionDropout float64 `json:"attention_dropout,omitempty"`
}

// LoadCommandRConfig loads a Command-R model configuration from a JSON file
func LoadCommandRConfig(configPath string) (*CommandRConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Command-R config file '%s': %w", configPath, err)
	}

	var config CommandRConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Command-R config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *CommandRConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known Command-R model sizes based on public information
	// Command-R: 35B parameters
	// Command-R+: 104B parameters
	if c.NumHiddenLayers == 40 && c.HiddenSize == 8192 {
		return 35_000_000_000 // Command-R 35B
	} else if c.NumHiddenLayers >= 64 && c.HiddenSize >= 12288 {
		return 104_000_000_000 // Command-R+ 104B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *CommandRConfig) GetContextLength() int {
	// Command-R models are known for their long context capabilities
	// Command-R: 128k context
	// Command-R+: 128k context
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *CommandRConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *CommandRConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Command-R base models
func (c *CommandRConfig) HasVision() bool {
	return false
}

// Register the Command-R model handler
func init() {
	RegisterModelLoader("cohere", func(configPath string) (HuggingFaceModel, error) {
		return LoadCommandRConfig(configPath)
	})
	RegisterModelLoader("command-r", func(configPath string) (HuggingFaceModel, error) {
		return LoadCommandRConfig(configPath)
	})
}
