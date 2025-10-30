package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// GptOssConfig defines the configuration for GPT-OSS models
type GptOssConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`
	HeadDim               int `json:"head_dim"`

	// MoE specific parameters
	NumLocalExperts    int     `json:"num_local_experts"`
	NumExpertsPerTok   int     `json:"num_experts_per_tok"`
	ExpertsPerToken    int     `json:"experts_per_token,omitempty"`
	OutputRouterLogits bool    `json:"output_router_logits"`
	RouterAuxLossCoef  float64 `json:"router_aux_loss_coef"`

	// Special tokens
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	AttentionBias    bool    `json:"attention_bias"`
	SlidingWindow    int     `json:"sliding_window"`
	SwigluLimit      float64 `json:"swiglu_limit"`

	// Context and position handling
	InitialContextLength int `json:"initial_context_length"`

	// Layer configuration
	LayerTypes []string `json:"layer_types"`

	// RoPE scaling configuration
	RopeScaling struct {
		BetaFast                      float64 `json:"beta_fast"`
		BetaSlow                      float64 `json:"beta_slow"`
		Factor                        float64 `json:"factor"`
		OriginalMaxPositionEmbeddings int     `json:"original_max_position_embeddings"`
		RopeType                      string  `json:"rope_type"`
		Truncate                      bool    `json:"truncate"`
	} `json:"rope_scaling"`

	// Quantization config
	QuantizationConfig *struct {
		ModulesToNotConvert []string `json:"modules_to_not_convert"`
		QuantMethod         string   `json:"quant_method"`
	} `json:"quantization_config,omitempty"`

	// Misc options
	InitializerRange  float64 `json:"initializer_range"`
	UseCache          bool    `json:"use_cache"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
}

// LoadGptOssConfig loads a GPT-OSS model configuration from a JSON file
func LoadGptOssConfig(configPath string) (*GptOssConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GPT-OSS config file '%s': %w", configPath, err)
	}

	var config GptOssConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse GPT-OSS config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid GPT-OSS configuration in '%s': %w", configPath, err)
	}

	return &config, nil
}

// Validate checks if the GPT-OSS configuration is valid
func (c *GptOssConfig) Validate() error {
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
	if c.NumExpertsPerTok <= 0 && c.ExpertsPerToken <= 0 {
		return fmt.Errorf("either num_experts_per_tok or experts_per_token must be positive")
	}
	// Validate that KV heads is not more than attention heads
	if c.NumKeyValueHeads > c.NumAttentionHeads {
		return fmt.Errorf("num_key_value_heads (%d) cannot be greater than num_attention_heads (%d)",
			c.NumKeyValueHeads, c.NumAttentionHeads)
	}
	return nil
}

// Implementation of the HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *GptOssConfig) GetParameterCount() int64 {
	// Try to get parameter count from safetensors files with quantization-aware parsing
	count, err := FindAndParseSafetensors(c.ConfigPath, c.GetQuantizationType())
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Estimate based on known GPT-OSS configurations
	// GPT-OSS 20B: 24 layers, 32 experts, 4 experts per token
	if c.HiddenSize == 2880 && c.NumHiddenLayers == 24 && c.NumLocalExperts == 32 {
		return 20_000_000_000 // 20B parameters
	}
	// GPT-OSS 120B: 36 layers, 128 experts, 4 experts per token
	if c.HiddenSize == 2880 && c.NumHiddenLayers == 36 && c.NumLocalExperts == 128 {
		return 120_000_000_000 // 120B parameters
	}

	// Fallback estimation for MoE architectures
	return c.estimateGptOssParams()
}

// estimateGptOssParams estimates parameters for GPT-OSS MoE architecture
func (c *GptOssConfig) estimateGptOssParams() int64 {
	// Basic transformer parameters (shared layers)
	embedParams := int64(c.VocabSize * c.HiddenSize)

	// Self-attention parameters per layer
	attentionParams := int64(c.NumHiddenLayers) * int64(4*c.HiddenSize*c.HiddenSize) // Q, K, V, O projections

	// MoE feed-forward parameters
	// Each expert has gate, up, and down projections
	expertParams := int64(c.NumLocalExperts) * int64(3*c.HiddenSize*c.IntermediateSize)

	// Router parameters (one per layer)
	routerParams := int64(c.NumHiddenLayers) * int64(c.HiddenSize*c.NumLocalExperts)

	// Layer norms (2 per layer)
	normParams := int64(c.NumHiddenLayers) * int64(2*c.HiddenSize)

	// Output head (if not tied)
	outputParams := int64(0)
	if !c.TieWordEmbeddings {
		outputParams = int64(c.HiddenSize * c.VocabSize)
	}

	totalParams := embedParams + attentionParams + expertParams + routerParams + normParams + outputParams

	// Round to nearest billion for cleaner reporting
	return (totalParams / 1_000_000_000) * 1_000_000_000
}

// GetTransformerVersion returns the transformers library version
func (c *GptOssConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *GptOssConfig) GetQuantizationType() string {
	if c.QuantizationConfig != nil {
		return c.QuantizationConfig.QuantMethod
	}
	return ""
}

// GetArchitecture returns the model architecture
func (c *GptOssConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "GptOssForCausalLM"
}

// GetModelType returns the model type
func (c *GptOssConfig) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *GptOssConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *GptOssConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *GptOssConfig) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns false since this is not a multimodal vision model
func (c *GptOssConfig) HasVision() bool {
	return false
}

// Register the GPT-OSS model handler
func init() {
	RegisterModelLoader("gpt_oss", func(configPath string) (HuggingFaceModel, error) {
		return LoadGptOssConfig(configPath)
	})
}
