package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Qwen3VLConfig defines the configuration for Qwen3-VL multimodal models.
type Qwen3VLConfig struct {
	BaseModelConfig
	ImageTokenId       int                 `json:"image_token_id"`
	TextConfig         Qwen3VLTextConfig   `json:"text_config"`
	TieWordEmbeddings  bool                `json:"tie_word_embeddings"`
	VideoTokenId       int                 `json:"video_token_id"`
	VisionConfig       Qwen3VLVisionConfig `json:"vision_config"`
	VisionStartTokenId int                 `json:"vision_start_token_id"`
	VisionEndTokenId   int                 `json:"vision_end_token_id"`
}

// Qwen3VLTextConfig represents the text transformer configuration.
type Qwen3VLTextConfig struct {
	// Attention mechanism
	AttentionBias    bool    `json:"attention_bias"`
	AttentionDropout float64 `json:"attention_dropout"`

	// Special tokens and embeddings
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`
	VocabSize  int `json:"vocab_size"`

	// Model architecture
	HeadDim               int    `json:"head_dim"`
	HiddenSize            int    `json:"hidden_size"`
	IntermediateSize      int    `json:"intermediate_size"`
	MaxPositionEmbeddings int    `json:"max_position_embeddings"`
	ModelType             string `json:"model_type"`
	NumAttentionHeads     int    `json:"num_attention_heads"`
	NumKeyValueHeads      int    `json:"num_key_value_heads"`
	NumHiddenLayers       int    `json:"num_hidden_layers"`

	// Mixture-of-Experts (MoE)
	NumExperts          int  `json:"num_experts"`
	NumExpertsPerTok    int  `json:"num_experts_per_tok"`
	MoeIntermediateSize int  `json:"moe_intermediate_size"`
	NormTopkProb        bool `json:"norm_topk_prob"`

	// Activation and initialization
	HiddenAct        string  `json:"hidden_act"`
	InitializerRange float64 `json:"initializer_range"`

	// Rotary Position Embeddings (RoPE)
	RopeScaling Qwen3VLRopeScalingConfig `json:"rope_scaling"`
	RopeTheta   float64                  `json:"rope_theta"`

	// Miscellaneous
	DecoderSparseStep int      `json:"decoder_sparse_step"`
	Dtype             string   `json:"dtype"`
	UseCache          bool     `json:"use_cache"`
	MlpOnlyLayers     []string `json:"mlp_only_layers"`
}

// Qwen3VLVisionConfig defines the vision component configuration for Qwen3-VL models.
type Qwen3VLVisionConfig struct {
	DeepstackVisualIndexes []int   `json:"deepstack_visual_indexes"`
	Depth                  int     `json:"depth"`
	HiddenAct              string  `json:"hidden_act"`
	HiddenSize             int     `json:"hidden_size"`
	InChannels             int     `json:"in_channels"`
	InitializerRange       float64 `json:"initializer_range"`
	IntermediateSize       int     `json:"intermediate_size"`
	ModelType              string  `json:"model_type"`
	NumHeads               int     `json:"num_heads"`
	NumPositionEmbeddings  int     `json:"num_position_embeddings"`
	OutHiddenSize          int     `json:"out_hidden_size"`
	PatchSize              int     `json:"patch_size"`
	SpatialMergeSize       int     `json:"spatial_merge_size"`
	TemporalPatchSize      int     `json:"temporal_patch_size"`
}

// Qwen3VLRopeScalingConfig represents ROPE scaling configuration.
type Qwen3VLRopeScalingConfig struct {
	MropeInterleaved bool   `json:"mrope_interleaved"`
	MropeSection     []int  `json:"mrope_section"`
	RopeType         string `json:"rope_type"`
}

// LoadQwen3VLConfig loads a Qwen3VL model configuration from a JSON file.
func LoadQwen3VLConfig(configPath string) (*Qwen3VLConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen3-VL config file '%s': %w", configPath, err)
	}
	var config Qwen3VLConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen3-VL config JSON from '%s': %w", configPath, err)
	}
	config.ConfigPath = configPath
	return &config, nil
}

// GetParameterCount returns the total number of parameters in the model.
func (c *Qwen3VLConfig) GetParameterCount() int64 {
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)
	tc := c.TextConfig
	if tc.HiddenSize == 4096 && tc.NumHiddenLayers == 94 {
		return 235_000_000_000 // Qwen3-VL-235B
	}
	// Estimate text model parameters
	languageParams := estimateModelParams(
		tc.HiddenSize, tc.NumHiddenLayers, tc.IntermediateSize, tc.VocabSize,
	)
	// Rough estimate for vision module (ViT-style)
	vc := c.VisionConfig
	visionParams := int64(0)
	if vc.HiddenSize > 0 && vc.Depth > 0 {
		visionParams = int64(12) * int64(vc.HiddenSize) * int64(vc.HiddenSize) * int64(vc.Depth)
	}
	return languageParams + visionParams
}

// GetContextLength returns the maximum context length supported by the model.
func (c *Qwen3VLConfig) GetContextLength() int {
	return c.TextConfig.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes.
func (c *Qwen3VLConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any).
func (c *Qwen3VLConfig) GetQuantizationType() string {
	return ""
}

// HasVision returns true for Qwen3-VL models.
func (c *Qwen3VLConfig) HasVision() bool {
	return true
}

func init() {
	RegisterModelLoader("qwen3_vl_moe", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen3VLConfig(configPath)
	})
}
