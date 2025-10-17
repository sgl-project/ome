package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Qwen3Config defines the configuration for Qwen3 models
type Qwen3Config struct {
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
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	SlidingWindow    int     `json:"sliding_window"`

	// For extended context models
	SeqLength       int     `json:"seq_length"`
	MaxWindowLayers int     `json:"max_window_layers"`
	RotaryEmb_base  float64 `json:"rotary_emb_base"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	UseCache          bool `json:"use_cache"`
	UseSlidingWindow  bool `json:"use_sliding_window"`
}

// LoadQwen3Config loads a Qwen3 model configuration from a JSON file
func LoadQwen3Config(configPath string) (*Qwen3Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen3 config file '%s': %w", configPath, err)
	}

	var config Qwen3Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen3 config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *Qwen3Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Hard-coded parameter counts based on model size
	if c.HiddenSize == 2560 && c.NumHiddenLayers == 36 {
		return 4_000_000_000 // 4B parameters
	}

	// For unknown configurations, estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *Qwen3Config) GetContextLength() int {
	// Use the seq_length field if available as it's more specific for Qwen3
	if c.SeqLength > 0 {
		return c.SeqLength
	}
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Qwen3Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Qwen3Config) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Qwen3 base models
func (c *Qwen3Config) HasVision() bool {
	return false // Base Qwen3 models don't have vision capabilities
}

// Register the Qwen3 model handler
func init() {
	RegisterModelLoader("qwen3", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen3Config(configPath)
	})
}
