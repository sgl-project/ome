package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// MixtralConfig defines the configuration for Mixtral models
type MixtralConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// MoE specific parameters
	NumLocalExperts    int     `json:"num_local_experts"`
	NumExpertsPerTok   int     `json:"num_experts_per_tok"`
	OutputRouterLogits bool    `json:"output_router_logits"`
	RouterAuxLossCoef  float64 `json:"router_aux_loss_coef"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`

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

// LoadMixtralConfig loads a Mixtral model configuration from a JSON file
func LoadMixtralConfig(configPath string) (*MixtralConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Mixtral config file '%s': %w", configPath, err)
	}

	var config MixtralConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Mixtral config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Mixtral configuration in '%s': %w", configPath, err)
	}

	return &config, nil
}

// Validate checks if the Mixtral configuration is valid
func (c *MixtralConfig) Validate() error {
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
	if c.NumLocalExperts <= 0 {
		return fmt.Errorf("num_local_experts must be positive, got %d", c.NumLocalExperts)
	}
	if c.NumExpertsPerTok <= 0 {
		return fmt.Errorf("num_experts_per_tok must be positive, got %d", c.NumExpertsPerTok)
	}
	return nil
}

// Implementation of the HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *MixtralConfig) GetParameterCount() int64 {
	// Try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return the official parameter count for Mixtral-8x7B
	// Each expert is ~7B parameters, with 8 experts
	// However, the total is not 8*7B due to shared layers
	return 46_700_000_000 // 46.7B for Mixtral-8x7B
}

// GetTransformerVersion returns the transformers library version
func (c *MixtralConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *MixtralConfig) GetQuantizationType() string {
	return "" // No quantization config for Mixtral by default
}

// GetArchitecture returns the model architecture
func (c *MixtralConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "MixtralForCausalLM"
}

// GetModelType returns the model type
func (c *MixtralConfig) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *MixtralConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *MixtralConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *MixtralConfig) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns false since this is not a multimodal vision model
func (c *MixtralConfig) HasVision() bool {
	return false
}

// Register the Mixtral model handler
func init() {
	RegisterModelLoader("mixtral", func(configPath string) (HuggingFaceModel, error) {
		return LoadMixtralConfig(configPath)
	})
}
