package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// PhiMoERopeScaling defines the rope scaling configuration for PhiMoE models
// PhiMoE uses enhanced LongRoPE with mscale factors
type PhiMoERopeScaling struct {
	Type                          string    `json:"type"`
	LongFactor                    []float64 `json:"long_factor"`
	ShortFactor                   []float64 `json:"short_factor"`
	LongMscale                    float64   `json:"long_mscale"`
	ShortMscale                   float64   `json:"short_mscale"`
	OriginalMaxPositionEmbeddings int       `json:"original_max_position_embeddings"`
}

// PhiMoEConfig defines the configuration for Phi-3.5-MoE models
// PhiMoE is a Mixture of Experts variant of Phi-3
type PhiMoEConfig struct {
	BaseModelConfig

	// Auto map for custom model loading
	AutoMap *AutoMap `json:"auto_map,omitempty"`

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

	// MoE (Mixture of Experts) specific
	NumLocalExperts    int     `json:"num_local_experts"`
	NumExpertsPerTok   int     `json:"num_experts_per_tok"`
	RouterAuxLossCoef  float64 `json:"router_aux_loss_coef"`
	RouterJitterNoise  float64 `json:"router_jitter_noise"`
	OutputRouterLogits bool    `json:"output_router_logits"`

	// Attention and normalization
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	AttentionBias    bool    `json:"attention_bias"`
	SlidingWindow    int     `json:"sliding_window"`

	// Dropout and noise
	HiddenDropout    float64 `json:"hidden_dropout"`
	InputJitterNoise float64 `json:"input_jitter_noise"`

	// Rope scaling for extended context (enhanced with mscale)
	OriginalMaxPositionEmbeddings int                `json:"original_max_position_embeddings"`
	RopeScaling                   *PhiMoERopeScaling `json:"rope_scaling,omitempty"`

	// Other configurations
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	LmHeadBias        bool    `json:"lm_head_bias"`
	UseCache          bool    `json:"use_cache"`
}

// LoadPhiMoEConfig loads a PhiMoE model configuration from a JSON file
func LoadPhiMoEConfig(configPath string) (*PhiMoEConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PhiMoE config file '%s': %w", configPath, err)
	}

	var config PhiMoEConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse PhiMoE config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// GetParameterCount returns the total number of parameters in the model
func (c *PhiMoEConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Fallback: estimate based on known PhiMoE model sizes
	// Phi-3.5-MoE-instruct: hidden_size=4096, num_hidden_layers=32, 16 experts ≈ 16B parameters (active: ~6.6B)
	// Note: MoE models have total params vs active params
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 && c.NumLocalExperts == 16 {
		return 16_000_000_000 // Total parameters (not active)
	}

	// For unknown configs, return 0 to indicate estimation failed
	return 0
}

// GetContextLength returns the maximum context length supported by the model
func (c *PhiMoEConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *PhiMoEConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *PhiMoEConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false since this is not a multimodal vision model
func (c *PhiMoEConfig) HasVision() bool {
	return false
}

// Register the PhiMoE model handler
func init() {
	RegisterModelLoader("phimoe", func(configPath string) (HuggingFaceModel, error) {
		return LoadPhiMoEConfig(configPath)
	})
}
