package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// QwenConfig defines the configuration for Qwen v1 models (original Qwen)
type QwenConfig struct {
	BaseModelConfig

	// Auto map for custom model loading
	AutoMap *AutoMap `json:"auto_map,omitempty"`

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	KvChannels            int `json:"kv_channels"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// Qwen v1 specific fields
	SeqLength         int     `json:"seq_length"`
	LayerNormEpsilon  float64 `json:"layer_norm_epsilon"`
	InitializerRange  float64 `json:"initializer_range"`
	RotaryEmbBase     float64 `json:"rotary_emb_base"`
	RotaryPct         float64 `json:"rotary_pct"`
	ScaleAttnWeights  bool    `json:"scale_attn_weights"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`

	// Qwen v1 special features
	UseDynamicNTK bool        `json:"use_dynamic_ntk"`
	UseFlashAttn  interface{} `json:"use_flash_attn"` // Can be bool or "auto"
	UseLogNAttn   bool        `json:"use_logn_attn"`

	// Dropout probabilities
	AttnDropoutProb float64 `json:"attn_dropout_prob"`
	EmbDropoutProb  float64 `json:"emb_dropout_prob"`

	// Additional flags
	NoBias         bool   `json:"no_bias"`
	BF16           bool   `json:"bf16"`
	FP16           bool   `json:"fp16"`
	FP32           bool   `json:"fp32"`
	OnnxSafe       *bool  `json:"onnx_safe"`
	TokenizerClass string `json:"tokenizer_class"`
}

// LoadQwenConfig loads a Qwen v1 model configuration from a JSON file
func LoadQwenConfig(configPath string) (*QwenConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen config file '%s': %w", configPath, err)
	}

	var config QwenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *QwenConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Hard-coded parameter counts based on known Qwen v1 model sizes
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000 // 7B parameters
	} else if c.HiddenSize == 5120 && c.NumHiddenLayers == 40 {
		return 14_000_000_000 // 14B parameters
	} else if c.HiddenSize == 8192 && c.NumHiddenLayers == 80 {
		return 72_000_000_000 // 72B parameters
	}

	// For unknown configurations, estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *QwenConfig) GetContextLength() int {
	// Qwen v1 uses seq_length as the primary context length field
	if c.SeqLength > 0 {
		return c.SeqLength
	}
	// Fallback to max_position_embeddings if seq_length is not set
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *QwenConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *QwenConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for Qwen v1 base models
func (c *QwenConfig) HasVision() bool {
	return false // Base Qwen v1 models don't have vision capabilities
}

// Register the Qwen v1 model handler
func init() {
	RegisterModelLoader("qwen", func(configPath string) (HuggingFaceModel, error) {
		return LoadQwenConfig(configPath)
	})
}
