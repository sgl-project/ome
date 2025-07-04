package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// GemmaConfig defines the configuration for Google Gemma models
type GemmaConfig struct {
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
	HiddenAct          string  `json:"hidden_act"`
	HiddenActivation   string  `json:"hidden_activation"`
	RmsNormEps         float64 `json:"rms_norm_eps"`
	RopeTheta          float64 `json:"rope_theta"`
	AttentionBias      bool    `json:"attention_bias"`
	AttentionDropout   float64 `json:"attention_dropout"`
	HeadDim            int     `json:"head_dim"`
	QueryPreAttnScalar int     `json:"query_pre_attn_scalar,omitempty"`

	// Gemma2 specific fields
	AttnLogitSoftcapping  float64 `json:"attn_logit_softcapping,omitempty"`
	FinalLogitSoftcapping float64 `json:"final_logit_softcapping,omitempty"`
	SlidingWindow         int     `json:"sliding_window,omitempty"`
	CacheImplementation   string  `json:"cache_implementation,omitempty"`

	// Misc options
	InitializerRange float64 `json:"initializer_range"`
	UseCache         bool    `json:"use_cache"`
}

// LoadGemmaConfig loads a Gemma model configuration from a JSON file
func LoadGemmaConfig(configPath string) (*GemmaConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gemma config file '%s': %w", configPath, err)
	}

	var config GemmaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Gemma config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Gemma configuration in '%s': %w", configPath, err)
	}

	return &config, nil
}

// Validate checks if the Gemma configuration is valid
func (c *GemmaConfig) Validate() error {
	if c.HiddenSize <= 0 {
		return fmt.Errorf("hidden_size must be positive, got %d", c.HiddenSize)
	}
	if c.NumHiddenLayers <= 0 {
		return fmt.Errorf("num_hidden_layers must be positive, got %d", c.NumHiddenLayers)
	}
	if c.NumAttentionHeads <= 0 {
		return fmt.Errorf("num_attention_heads must be positive, got %d", c.NumAttentionHeads)
	}
	if c.VocabSize <= 0 {
		return fmt.Errorf("vocab_size must be positive, got %d", c.VocabSize)
	}
	return nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *GemmaConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return parameter count based on known model configurations
	// Gemma models: 2B, 7B, 9B variants
	if c.HiddenSize == 2048 && c.NumHiddenLayers == 18 {
		return 2_000_000_000 // Gemma 2B
	} else if c.HiddenSize == 2304 && c.NumHiddenLayers == 26 {
		return 2_600_000_000 // Gemma2 2.6B
	} else if c.HiddenSize == 3072 && c.NumHiddenLayers == 28 {
		return 7_000_000_000 // Gemma 7B
	} else if c.HiddenSize == 3584 && c.NumHiddenLayers == 42 {
		return 9_000_000_000 // Gemma2 9B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *GemmaConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *GemmaConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *GemmaConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Gemma base models
func (c *GemmaConfig) HasVision() bool {
	return false
}

// Register the Gemma model handlers
func init() {
	// Register for both "gemma" and "gemma2" model types
	RegisterModelLoader("gemma", func(configPath string) (HuggingFaceModel, error) {
		return LoadGemmaConfig(configPath)
	})
	RegisterModelLoader("gemma2", func(configPath string) (HuggingFaceModel, error) {
		return LoadGemmaConfig(configPath)
	})
}
