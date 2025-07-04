package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// BaichuanConfig defines the configuration for Baichuan models
type BaichuanConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	ModelMaxLength        int `json:"model_max_length"`
	VocabSize             int `json:"vocab_size"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id"`

	// Attention related
	HiddenAct  string  `json:"hidden_act"`
	RmsNormEps float64 `json:"rms_norm_eps"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
	ZLossWeight       float64 `json:"z_loss_weight"`
}

// LoadBaichuanConfig loads a Baichuan model configuration from a JSON file
func LoadBaichuanConfig(configPath string) (*BaichuanConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Baichuan config file '%s': %w", configPath, err)
	}

	var config BaichuanConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Baichuan config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *BaichuanConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known Baichuan model sizes
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // Baichuan2 7B
	} else if c.HiddenSize == 5120 && c.NumHiddenLayers == 40 {
		return 13_000_000_000 // Baichuan2 13B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *BaichuanConfig) GetContextLength() int {
	if c.ModelMaxLength > 0 {
		return c.ModelMaxLength
	}
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *BaichuanConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *BaichuanConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Baichuan base models
func (c *BaichuanConfig) HasVision() bool {
	return false
}

// Register the Baichuan model handler
func init() {
	RegisterModelLoader("baichuan", func(configPath string) (HuggingFaceModel, error) {
		return LoadBaichuanConfig(configPath)
	})
}
