package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// DeepseekV3Config defines the configuration for DeepSeek-V3 models
type DeepseekV3Config struct {
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
	NumRoutedExperts    int `json:"n_routed_experts"`
	NumSharedExperts    int `json:"n_shared_experts"`
	NumExpertsPerTok    int `json:"num_experts_per_tok"`
	MoeIntermediateSize int `json:"moe_intermediate_size"`
	EPSize              int `json:"ep_size"`
	FirstKDenseReplace  int `json:"first_k_dense_replace"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	PadTokenId int `json:"pad_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	SlidingWindow    int     `json:"sliding_window"`
	AttentionDropout float64 `json:"attention_dropout"`

	// RoPE scaling
	RopeScaling RopeScalingConfig `json:"rope_scaling"`

	// Quantization settings
	QuantizationConfig *QuantizationConfig `json:"quantization_config,omitempty"`

	// Misc options
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
	InitializerRange  float64 `json:"initializer_range"`
}

// LoadDeepseekV3Config loads a DeepSeek V3 configuration from a JSON file
func LoadDeepseekV3Config(path string) (*DeepseekV3Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg DeepseekV3Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %v", err)
	}

	cfg.ConfigPath = path
	return &cfg, nil
}

// GetParameterCount returns the total number of parameters in the model
// It first tries to parse the safetensors file for an accurate count
// If that fails, it falls back to the official parameter count
func (c *DeepseekV3Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error but continue with official parameter count
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// DeepSeek V3 official parameter count is 685B
	return 685_000_000_000 // 685B parameters
}

// GetTransformerVersion returns the transformers library version
func (c *DeepseekV3Config) GetTransformerVersion() string {
	return c.BaseModelConfig.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *DeepseekV3Config) GetQuantizationType() string {
	if c.QuantizationConfig != nil && c.QuantizationConfig.QuantMethod != "" {
		return c.QuantizationConfig.QuantMethod
	}
	return ""
}

// GetArchitecture returns the model architecture
func (c *DeepseekV3Config) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return ""
}

// GetModelType returns the model type
func (c *DeepseekV3Config) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *DeepseekV3Config) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *DeepseekV3Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *DeepseekV3Config) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns false since this is not a multimodal vision model
func (c *DeepseekV3Config) HasVision() bool {
	return false
}

// Register the DeepSeek V3 model handler
func init() {
	modelLoaders["deepseek_v3"] = func(configPath string) (HuggingFaceModel, error) {
		return LoadDeepseekV3Config(configPath)
	}
}
