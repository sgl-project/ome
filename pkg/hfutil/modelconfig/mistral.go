package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// MistralConfig defines the configuration for Mistral models
type MistralConfig struct {
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
	PadTokenId int `json:"pad_token_id,omitempty"`

	// Attention related
	HiddenAct        string      `json:"hidden_act"`
	RmsNormEps       float64     `json:"rms_norm_eps"`
	RopeTheta        float64     `json:"rope_theta"`
	SlidingWindow    interface{} `json:"sliding_window"`
	AttentionDropout float64     `json:"attention_dropout"`

	// Misc options
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
	InitializerRange  float64 `json:"initializer_range"`
}

// LoadMistralConfig loads a Mistral model configuration from a JSON file
func LoadMistralConfig(configPath string) (*MistralConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Mistral config file '%s': %w", configPath, err)
	}

	var config MistralConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Mistral config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Mistral configuration in '%s': %w", configPath, err)
	}

	return &config, nil
}

// Validate checks if the Mistral configuration is valid
func (c *MistralConfig) Validate() error {
	if c.HiddenSize <= 0 {
		return fmt.Errorf("hidden_size must be positive, got %d", c.HiddenSize)
	}
	if c.NumHiddenLayers <= 0 {
		return fmt.Errorf("num_hidden_layers must be positive, got %d", c.NumHiddenLayers)
	}
	if c.NumAttentionHeads <= 0 {
		return fmt.Errorf("num_attention_heads must be positive, got %d", c.NumAttentionHeads)
	}
	if c.NumKeyValueHeads <= 0 {
		return fmt.Errorf("num_key_value_heads must be positive, got %d", c.NumKeyValueHeads)
	}
	if c.VocabSize <= 0 {
		return fmt.Errorf("vocab_size must be positive, got %d", c.VocabSize)
	}
	if c.MaxPositionEmbeddings <= 0 {
		return fmt.Errorf("max_position_embeddings must be positive, got %d", c.MaxPositionEmbeddings)
	}
	return nil
}

// Implementation of HuggingFaceModel interface

func (c *MistralConfig) GetParameterCount() int64 {
	// Try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return official parameter count based on model size (if we know it)
	// For standard Mistral-7B
	if c.ModelType == "mistral" && c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // 7B parameters
	}

	// For other models, return 0 since we can't calculate without safetensors
	return 0
}

func (c *MistralConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

func (c *MistralConfig) GetQuantizationType() string {
	// Mistral doesn't have quantization in this config
	return ""
}

func (c *MistralConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "MistralModel"
}

func (c *MistralConfig) GetModelType() string {
	return c.ModelType
}

func (c *MistralConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

func (c *MistralConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

func (c *MistralConfig) GetTorchDtype() string {
	return c.TorchDtype
}

func (c *MistralConfig) HasVision() bool {
	return false
}

// Register the Mistral model handler
func init() {
	RegisterModelLoader("mistral", func(configPath string) (HuggingFaceModel, error) {
		return LoadMistralConfig(configPath)
	})
}
