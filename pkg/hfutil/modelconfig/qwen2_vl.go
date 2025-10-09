package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Qwen2VisionConfig defines the vision component configuration for Qwen2-VL models
// Supports both qwen2_vl and qwen2_5_vl configurations with optional fields
type Qwen2VisionConfig struct {
	Depth               int    `json:"depth"`
	EmbedDim            int    `json:"embed_dim,omitempty"` // qwen2_vl only
	MlpRatio            int    `json:"mlp_ratio,omitempty"` // qwen2_vl only
	NumHeads            int    `json:"num_heads"`
	InChans             int    `json:"in_chans"`
	HiddenSize          int    `json:"hidden_size"`
	IntermediateSize    int    `json:"intermediate_size,omitempty"` // qwen2_5_vl only
	OutHiddenSize       int    `json:"out_hidden_size,omitempty"`   // qwen2_5_vl only
	PatchSize           int    `json:"patch_size"`
	SpatialMergeSize    int    `json:"spatial_merge_size"`
	SpatialPatchSize    int    `json:"spatial_patch_size"`
	WindowSize          int    `json:"window_size,omitempty"`           // qwen2_5_vl only
	FullattBlockIndexes []int  `json:"fullatt_block_indexes,omitempty"` // qwen2_5_vl only
	TokensPerSecond     int    `json:"tokens_per_second,omitempty"`     // qwen2_5_vl only
	TemporalPatchSize   int    `json:"temporal_patch_size,omitempty"`
	HiddenAct           string `json:"hidden_act,omitempty"` // qwen2_5_vl only
}

// Qwen2MRopeScaling defines the mrope scaling configuration for Qwen2-VL models
type Qwen2MRopeScaling struct {
	Type         string `json:"type"`
	MropeSection []int  `json:"mrope_section"`
}

// Qwen2VLConfig defines the configuration for Qwen2-VL multimodal models
// Supports both qwen2_vl and qwen2_5_vl model types
type Qwen2VLConfig struct {
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
	BosTokenId         int `json:"bos_token_id"`
	EosTokenId         int `json:"eos_token_id"`
	VisionStartTokenId int `json:"vision_start_token_id"`
	VisionEndTokenId   int `json:"vision_end_token_id"`
	VisionTokenId      int `json:"vision_token_id"`
	ImageTokenId       int `json:"image_token_id"`
	VideoTokenId       int `json:"video_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	SlidingWindow    int     `json:"sliding_window"`
	MaxWindowLayers  int     `json:"max_window_layers"`

	// Initialization and RoPE scaling
	InitializerRange float64           `json:"initializer_range,omitempty"`
	RopeScaling      Qwen2MRopeScaling `json:"rope_scaling,omitempty"`

	// Vision configuration
	VisionConfig Qwen2VisionConfig `json:"vision_config"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	UseCache          bool `json:"use_cache"`
	UseSlidingWindow  bool `json:"use_sliding_window"`
}

// LoadQwen2VLConfig loads a Qwen2-VL model configuration from a JSON file
func LoadQwen2VLConfig(configPath string) (*Qwen2VLConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen2-VL config file '%s': %w", configPath, err)
	}

	var config Qwen2VLConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen2-VL config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *Qwen2VLConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known Qwen2-VL and Qwen2.5-VL model sizes
	if c.HiddenSize == 1536 && c.NumHiddenLayers == 28 {
		return 2_000_000_000 // Qwen2-VL 2B
	} else if c.HiddenSize == 3584 && c.NumHiddenLayers == 28 {
		return 7_000_000_000 // Qwen2-VL 7B / Qwen2.5-VL 7B
	} else if c.HiddenSize == 8192 && c.NumHiddenLayers == 80 {
		return 72_000_000_000 // Qwen2-VL 72B / Qwen2.5-VL 72B
	}

	// Estimate including vision encoder parameters
	languageParams := estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)

	// Estimate vision encoder parameters
	visionParams := int64(0)
	if c.VisionConfig.Depth > 0 {
		// Vision transformer parameters estimation
		visionParams = int64(c.VisionConfig.EmbedDim * c.VisionConfig.EmbedDim * c.VisionConfig.Depth * 4)
	}

	return languageParams + visionParams
}

// GetContextLength returns the maximum context length supported by the model
func (c *Qwen2VLConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Qwen2VLConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Qwen2VLConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns true for Qwen2-VL models
func (c *Qwen2VLConfig) HasVision() bool {
	return true // Qwen2-VL models have vision capabilities
}

// Register the Qwen2-VL and Qwen2.5-VL model handlers
func init() {
	RegisterModelLoader("qwen2_vl", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen2VLConfig(configPath)
	})
	RegisterModelLoader("qwen2_5_vl", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwen2VLConfig(configPath)
	})
}
