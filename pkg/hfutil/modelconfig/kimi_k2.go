package modelconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// KimiK2Config defines the configuration for Kimi-K2 models
// This model uses the DeepseekV3ForCausalLM architecture
type KimiK2Config struct {
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
	MoeLayerFreq        int `json:"moe_layer_freq"`
	FirstKDenseReplace  int `json:"first_k_dense_replace"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	AttentionBias    bool    `json:"attention_bias"`

	// Kimi-K2 specific parameters
	AuxLossAlpha          float64 `json:"aux_loss_alpha"`
	KvLoraRank            int     `json:"kv_lora_rank"`
	NGroup                int     `json:"n_group"`
	NormTopkProb          bool    `json:"norm_topk_prob"`
	NumNextnPredictLayers int     `json:"num_nextn_predict_layers"`
	PretrainingTP         int     `json:"pretraining_tp"`
	QLoraRank             int     `json:"q_lora_rank"`
	QkNopeHeadDim         int     `json:"qk_nope_head_dim"`
	QkRopeHeadDim         int     `json:"qk_rope_head_dim"`
	RoutedScalingFactor   float64 `json:"routed_scaling_factor"`
	ScoringFunc           string  `json:"scoring_func"`
	SeqAux                bool    `json:"seq_aux"`
	TopkGroup             int     `json:"topk_group"`
	TopkMethod            string  `json:"topk_method"`
	VHeadDim              int     `json:"v_head_dim"`

	// RoPE scaling (YARN type for Kimi-K2)
	RopeScaling RopeScalingConfig `json:"rope_scaling"`

	// Quantization settings
	QuantizationConfig *QuantizationConfig `json:"quantization_config,omitempty"`

	// Misc options
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
	InitializerRange  float64 `json:"initializer_range"`
}

// LoadKimiK2Config loads a Kimi-K2 configuration from a JSON file
func LoadKimiK2Config(path string) (*KimiK2Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Kimi-K2 config file '%s': %w", path, err)
	}

	var cfg KimiK2Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Kimi-K2 config JSON from '%s': %w", path, err)
	}

	cfg.ConfigPath = path

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Kimi-K2 configuration in '%s': %w", path, err)
	}

	return &cfg, nil
}

// Validate checks if the Kimi-K2 configuration is valid
func (c *KimiK2Config) Validate() error {
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
	if c.NumRoutedExperts <= 0 {
		return fmt.Errorf("n_routed_experts must be positive, got %d", c.NumRoutedExperts)
	}
	if c.NumExpertsPerTok <= 0 {
		return fmt.Errorf("num_experts_per_tok must be positive, got %d", c.NumExpertsPerTok)
	}
	return nil
}

// GetParameterCount returns the total number of parameters in the model
func (c *KimiK2Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error but continue with estimated parameter count
	log.Printf("Warning: failed to get parameter count from safetensors for %s: %v", c.ConfigPath, err)

	// Kimi-K2 is based on DeepSeek V3 architecture but with more experts (384 vs 256)
	// This model has approximately 1.5T parameters according to the model documentation
	return 1_500_000_000_000 // 1.5T parameters
}

// GetTransformerVersion returns the transformers library version
func (c *KimiK2Config) GetTransformerVersion() string {
	return c.BaseModelConfig.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *KimiK2Config) GetQuantizationType() string {
	if c.QuantizationConfig != nil && c.QuantizationConfig.QuantMethod != "" {
		return c.QuantizationConfig.QuantMethod
	}
	return ""
}

// GetArchitecture returns the model architecture
func (c *KimiK2Config) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return ""
}

// GetModelType returns the model type
func (c *KimiK2Config) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *KimiK2Config) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *KimiK2Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *KimiK2Config) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns false since this is not a multimodal vision model
func (c *KimiK2Config) HasVision() bool {
	return false
}

// Register the Kimi-K2 model handler
func init() {
	RegisterModelLoader("kimi_k2", func(configPath string) (HuggingFaceModel, error) {
		return LoadKimiK2Config(configPath)
	})
}
