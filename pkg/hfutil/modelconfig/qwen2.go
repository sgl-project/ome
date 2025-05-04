package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Qwen2Config defines the configuration for Qwen2 models
type Qwen2Config struct {
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

// LoadQwen2Config loads a Qwen2 model configuration from a JSON file
func LoadQwen2Config(configPath string) (*Qwen2Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Qwen2Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen2 config: %v", err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *Qwen2Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Hard-coded parameter counts based on model size
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // 7B parameters
	} else if c.HiddenSize == 8192 && c.NumHiddenLayers == 40 {
		return 34_000_000_000 // 34B parameters
	} else if c.HiddenSize == 7680 && c.NumHiddenLayers == 64 {
		return 72_000_000_000 // 72B parameters
	}

	// For unknown configurations, estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *Qwen2Config) GetContextLength() int {
	// Use the seq_length field if available as it's more specific for Qwen2
	if c.SeqLength > 0 {
		return c.SeqLength
	}
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Qwen2Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Qwen2Config) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Qwen2 base models
func (c *Qwen2Config) HasVision() bool {
	return false // Base Qwen2 models don't have vision capabilities
}

// Register the Qwen2 model handler
func init() {
	modelLoaders["qwen2"] = func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen2Config(configPath)
	}
}
