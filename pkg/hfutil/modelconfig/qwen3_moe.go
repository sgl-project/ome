package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Qwen3MoeConfig defines the configuration for Qwen3Moe models
type Qwen3MoeConfig struct {
	BaseModelConfig

	// Attention mechanism
	AttentionBias    bool    `json:"attention_bias"`
	AttentionDropout float64 `json:"attention_dropout"`

	// Special tokens and embeddings
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	VocabSize  int `json:"vocab_size"`

	// Model architecture
	HiddenAct             string `json:"hidden_act"`
	HiddenSize            int    `json:"hidden_size"`
	IntermediateSize      int    `json:"intermediate_size"`
	HeadDim               int    `json:"head_dim"`
	MaxPositionEmbeddings int    `json:"max_position_embeddings"`
	NumAttentionHeads     int    `json:"num_attention_heads"`
	NumKeyValueHeads      int    `json:"num_key_value_heads"`
	NumHiddenLayers       int    `json:"num_hidden_layers"`

	// Mixture-of-Experts (MoE)
	NumExperts          int  `json:"num_experts"`
	NumExpertsPerTok    int  `json:"num_experts_per_tok"`
	MoeIntermediateSize int  `json:"moe_intermediate_size"`
	NormTopkProb        bool `json:"norm_topk_prob"`

	// Extended context & decoding
	SeqLength        int  `json:"seq_length"`
	MaxWindowLayers  int  `json:"max_window_layers"`
	SlidingWindow    *int `json:"sliding_window"`
	UseSlidingWindow bool `json:"use_sliding_window"`

	// Rotary Position Embeddings (RoPE)
	RopeTheta      float64 `json:"rope_theta"`
	RopeScaling    any     `json:"rope_scaling"`
	RotaryEmb_base float64 `json:"rotary_emb_base"`

	// MLP and decoder
	MlpOnlyLayers     []string `json:"mlp_only_layers"`
	DecoderSparseStep int      `json:"decoder_sparse_step"`

	// Regularization and training
	InitializerRange   float64 `json:"initializer_range"`
	RmsNormEps         float64 `json:"rms_norm_eps"`
	OutputRouterLogits bool    `json:"output_router_logits"`
	RouterAuxLossCoef  float64 `json:"router_aux_loss_coef"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	UseCache          bool `json:"use_cache"`
}

// LoadQwen3MoeConfig loads a Qwen3Moe model configuration from a JSON file
func LoadQwen3MoeConfig(configPath string) (*Qwen3MoeConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen3Moe config file '%s': %w", configPath, err)
	}
	var config Qwen3MoeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen3Moe config JSON from '%s': %w", configPath, err)
	}
	config.ConfigPath = configPath
	return &config, nil
}

// GetParameterCount returns the total number of parameters in the model
func (c *Qwen3MoeConfig) GetParameterCount() int64 {
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)
	if c.HiddenSize == 2048 && c.NumHiddenLayers == 48 {
		return 30_000_000_000 // 30B parameters
	} else if c.HiddenSize == 4096 && c.NumHiddenLayers == 94 {
		return 235_000_000_000 // 235B parameters
	}
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *Qwen3MoeConfig) GetContextLength() int {
	if c.SeqLength > 0 {
		return c.SeqLength
	}
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Qwen3MoeConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Qwen3MoeConfig) GetQuantizationType() string {
	return ""
}

// HasVision returns false for Qwen3Moe base models
func (c *Qwen3MoeConfig) HasVision() bool {
	return false
}

func init() {
	RegisterModelLoader("qwen3_moe", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen3MoeConfig(configPath)
	})
}
