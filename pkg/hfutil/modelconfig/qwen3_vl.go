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
	var languageParams int64
	// Check if this is a MoE model
	if tc.NumExperts > 0 {
		// Use MoE-specific parameter estimation
		languageParams = estimateQwen3VLMoEParams(
			tc.HiddenSize, tc.NumHiddenLayers, tc.MoeIntermediateSize,
			tc.NumExperts, tc.VocabSize,
		)
	} else {
		// Standard dense model estimation
		languageParams = estimateModelParams(
			tc.HiddenSize, tc.NumHiddenLayers, tc.IntermediateSize, tc.VocabSize,
		)
	}

	// Estimate vision module parameters
	visionParams := estimateQwen3VLVisionParams(c.VisionConfig)
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

// estimateQwen3VLMoEParams estimates parameters for Qwen3-VL MoE models
func estimateQwen3VLMoEParams(hiddenSize, numLayers, moeIntermediateSize, numExperts, vocabSize int) int64 {
	// Embeddings
	params := int64(hiddenSize * vocabSize)

	// For each layer
	params += int64(numLayers) * (
	// Self-attention (Q, K, V, O projections + bias)
	int64(4*hiddenSize*hiddenSize) +
		// MoE experts (each expert has gate, up, down projections)
		int64(numExperts*3*hiddenSize*moeIntermediateSize) +
		// Router (gate network for expert selection)
		int64(hiddenSize*numExperts) +
		// Layer norms (2 per layer: attention norm and MLP norm)
		int64(2*hiddenSize))

	// Output projection (if not tied to embeddings)
	params += int64(hiddenSize * vocabSize)
	return params
}

// estimateQwen3VLVisionParams estimates parameters for Qwen3-VL vision module
func estimateQwen3VLVisionParams(vc Qwen3VLVisionConfig) int64 {
	if vc.HiddenSize <= 0 || vc.Depth <= 0 {
		return 0
	}

	// Patch embedding: patch_size^2 * in_channels * hidden_size
	patchEmbedParams := int64(vc.PatchSize * vc.PatchSize * vc.InChannels * vc.HiddenSize)
	// Position embeddings: num_position_embeddings * hidden_size
	posEmbedParams := int64(vc.NumPositionEmbeddings * vc.HiddenSize)

	// Transformer layers: each layer has attention + MLP
	layerParams := int64(vc.Depth) * (
	// Self-attention (Q, K, V, O projections)
	int64(4*vc.HiddenSize*vc.HiddenSize) +
		// MLP (up and down projections)
		int64(2*vc.HiddenSize*vc.IntermediateSize) +
		// Layer norms (2 per layer)
		int64(2*vc.HiddenSize))

	// Output projection to text hidden size
	outputProjParams := int64(vc.HiddenSize * vc.OutHiddenSize)
	return patchEmbedParams + posEmbedParams + layerParams + outputProjParams
}

func init() {
	RegisterModelLoader("qwen3_vl_moe", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen3VLConfig(configPath)
	})
	RegisterModelLoader("qwen3_vl", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen3VLConfig(configPath)
	})
}
