package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// MiniCPMConfig defines the configuration for MiniCPM models
type MiniCPMConfig struct {
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
	BosTokenId interface{} `json:"bos_token_id"` // Can be int or array
	EosTokenId interface{} `json:"eos_token_id"` // Can be int or array

	// MiniCPM3 specific fields
	QKNopeHeadDim int         `json:"qk_nope_head_dim,omitempty"`
	QKRopeHeadDim int         `json:"qk_rope_head_dim,omitempty"`
	QLoraRank     int         `json:"q_lora_rank,omitempty"`
	KVLoraRank    int         `json:"kv_lora_rank,omitempty"`
	RopeScaling   interface{} `json:"rope_scaling,omitempty"` // Can be float64 or object
	RopeTheta     float64     `json:"rope_theta,omitempty"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	InitializerRange float64 `json:"initializer_range"`

	// Misc options
	UseCache          bool    `json:"use_cache"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	ScaleEmb          float64 `json:"scale_emb,omitempty"`
	DimModelBase      int     `json:"dim_model_base,omitempty"`
}

// LoadMiniCPMConfig loads a MiniCPM model configuration from a JSON file
func LoadMiniCPMConfig(configPath string) (*MiniCPMConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MiniCPM config file '%s': %w", configPath, err)
	}

	var config MiniCPMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MiniCPM config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *MiniCPMConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known MiniCPM model sizes
	if c.HiddenSize == 2560 && c.NumHiddenLayers == 62 {
		return 4_000_000_000 // MiniCPM3 4B
	} else if c.HiddenSize == 2304 && c.NumHiddenLayers == 40 {
		return 2_400_000_000 // MiniCPM 2.4B
	} else if c.HiddenSize == 1536 && c.NumHiddenLayers == 52 {
		return 1_200_000_000 // MiniCPM 1.2B
	}

	// Fallback: estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *MiniCPMConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *MiniCPMConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *MiniCPMConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for base MiniCPM models
func (c *MiniCPMConfig) HasVision() bool {
	return false
}

// Register the MiniCPM model handler
func init() {
	RegisterModelLoader("minicpm", func(configPath string) (HuggingFaceModel, error) {
		return LoadMiniCPMConfig(configPath)
	})
	RegisterModelLoader("minicpm3", func(configPath string) (HuggingFaceModel, error) {
		return LoadMiniCPMConfig(configPath)
	})
}
