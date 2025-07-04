package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// XverseConfig defines the configuration for XVERSE models
type XverseConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize             int `json:"hidden_size"`
	IntermediateSize       int `json:"intermediate_size"`
	NumHiddenLayers        int `json:"num_hidden_layers"`
	NumAttentionHeads      int `json:"num_attention_heads"`
	MaxPositionEmbeddings  int `json:"max_position_embeddings"`
	MaxTokenizerTruncation int `json:"max_tokenizer_truncation"`
	VocabSize              int `json:"vocab_size"`

	// Special tokens
	PadTokenId int `json:"pad_token_id"`
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`

	// Attention related
	HiddenAct  string  `json:"hidden_act"`
	RmsNormEps float64 `json:"rms_norm_eps"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
}

// LoadXverseConfig loads an XVERSE model configuration from a JSON file
func LoadXverseConfig(configPath string) (*XverseConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read XVERSE config file '%s': %w", configPath, err)
	}

	var config XverseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse XVERSE config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *XverseConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known XVERSE model sizes
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // XVERSE 7B
	} else if c.HiddenSize == 5120 && c.NumHiddenLayers == 40 {
		return 13_000_000_000 // XVERSE 13B
	} else if c.HiddenSize == 6144 && c.NumHiddenLayers == 80 {
		return 65_000_000_000 // XVERSE 65B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *XverseConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *XverseConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *XverseConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for XVERSE base models
func (c *XverseConfig) HasVision() bool {
	return false
}

// Register the XVERSE model handler
func init() {
	RegisterModelLoader("xverse", func(configPath string) (HuggingFaceModel, error) {
		return LoadXverseConfig(configPath)
	})
}
