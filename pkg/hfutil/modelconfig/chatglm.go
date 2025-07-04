package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// ChatGLMConfig defines the configuration for THUDM ChatGLM models
type ChatGLMConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize        int `json:"hidden_size"`
	FFNHiddenSize     int `json:"ffn_hidden_size"`
	NumLayers         int `json:"num_layers"`
	NumAttentionHeads int `json:"num_attention_heads"`
	SeqLength         int `json:"seq_length"`
	PaddedVocabSize   int `json:"padded_vocab_size"`

	// Attention configuration
	KVChannels                int     `json:"kv_channels"`
	MultiQueryAttention       bool    `json:"multi_query_attention"`
	MultiQueryGroupNum        int     `json:"multi_query_group_num"`
	ApplyQueryKeyLayerScaling bool    `json:"apply_query_key_layer_scaling"`
	AttentionDropout          float64 `json:"attention_dropout"`
	AttentionSoftmaxInFp32    bool    `json:"attention_softmax_in_fp32"`
	AddQKVBias                bool    `json:"add_qkv_bias"`

	// Layer norm and residual
	LayerNormEpsilon                     float64 `json:"layernorm_epsilon"`
	PostLayerNorm                        bool    `json:"post_layer_norm"`
	Rmsnorm                              bool    `json:"rmsnorm"`
	ApplyResidualConnectionPostLayernorm bool    `json:"apply_residual_connection_post_layernorm"`
	FP32ResidualConnection               bool    `json:"fp32_residual_connection"`

	// Other configurations
	AddBiasLinear     bool    `json:"add_bias_linear"`
	BiasDropoutFusion bool    `json:"bias_dropout_fusion"`
	HiddenDropout     float64 `json:"hidden_dropout"`
	OriginalRope      bool    `json:"original_rope"`
	UseCache          bool    `json:"use_cache"`
	TorchDtype        string  `json:"torch_dtype"`
}

// LoadChatGLMConfig loads a ChatGLM model configuration from a JSON file
func LoadChatGLMConfig(configPath string) (*ChatGLMConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ChatGLM config file '%s': %w", configPath, err)
	}

	var config ChatGLMConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse ChatGLM config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *ChatGLMConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known ChatGLM model sizes
	if c.HiddenSize == 4096 && c.NumLayers == 28 {
		return 6_000_000_000 // ChatGLM3 6B
	} else if c.HiddenSize == 4096 && c.NumLayers == 40 {
		return 9_000_000_000 // ChatGLM4 9B
	}

	// Fallback: estimate based on architecture
	// ChatGLM uses different architecture, estimate with FFN size
	vocabSize := c.PaddedVocabSize
	if vocabSize == 0 {
		vocabSize = 65024 // default vocab size
	}

	return int64(
		// Embeddings
		c.HiddenSize*vocabSize +
			// Attention layers (with multi-query attention)
			c.NumLayers*(c.HiddenSize*c.HiddenSize*4) +
			// FFN layers
			c.NumLayers*(c.HiddenSize*c.FFNHiddenSize*2+c.HiddenSize+c.FFNHiddenSize) +
			// Layer norms
			c.NumLayers*2*c.HiddenSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *ChatGLMConfig) GetContextLength() int {
	return c.SeqLength
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *ChatGLMConfig) GetModelSizeBytes() int64 {
	dtype := c.TorchDtype
	if dtype == "" {
		dtype = "float16" // ChatGLM models typically use float16
	}
	return EstimateModelSizeBytes(c.GetParameterCount(), dtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *ChatGLMConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for ChatGLM base models
func (c *ChatGLMConfig) HasVision() bool {
	return false
}

// GetTorchDtype returns the torch data type used by the model
func (c *ChatGLMConfig) GetTorchDtype() string {
	if c.TorchDtype != "" {
		return c.TorchDtype
	}
	return "float16" // Default for ChatGLM
}

// Register the ChatGLM model handler
func init() {
	RegisterModelLoader("chatglm", func(configPath string) (HuggingFaceModel, error) {
		return LoadChatGLMConfig(configPath)
	})
}
