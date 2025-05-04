package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// LlamaConfig defines the configuration for Llama models (Llama 3 and Llama 3.1)
type LlamaConfig struct {
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
	BosTokenId interface{} `json:"bos_token_id"`
	EosTokenId interface{} `json:"eos_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	AttentionBias    bool    `json:"attention_bias"`
	MLPBias          bool    `json:"mlp_bias,omitempty"`

	// RoPE scaling for Llama-3 and Llama-3.1
	RopeScaling RopeScalingConfig `json:"rope_scaling"`

	// Quantization config for FP8 models like Llama-3.1-405B
	QuantizationConfig *struct {
		ActivationScaleUb   float64  `json:"activation_scale_ub"`
		ModulesToNotConvert []string `json:"modules_to_not_convert"`
		QuantMethod         string   `json:"quant_method"`
	} `json:"quantization_config,omitempty"`

	// Misc options
	PretrainingTP    int     `json:"pretraining_tp"`
	InitializerRange float64 `json:"initializer_range"`
	UseCache         bool    `json:"use_cache"`
}

// LoadLlamaConfig loads a Llama model configuration from a JSON file
func LoadLlamaConfig(configPath string) (*LlamaConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config LlamaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Llama config: %v", err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *LlamaConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return parameter count based on model size indicators
	// Check for well-known model architectures
	if c.HiddenSize == 8192 && c.NumHiddenLayers == 80 {
		// Return 70B for Llama-3-70B models
		return 70_000_000_000
	} else if c.HiddenSize == 8192 && c.NumHiddenLayers == 82 {
		// Return 70B for Llama-3.1-70B models
		return 70_000_000_000
	} else if c.HiddenSize == 16384 && c.NumHiddenLayers == 126 {
		// Return 405B for Llama-3.1-405B models
		return 405_000_000_000
	} else if c.HiddenSize == 3072 && c.NumHiddenLayers == 28 {
		// Return 3B for Llama-3.2-3B models
		return 3_000_000_000
	} else if c.HiddenSize == 2048 && c.NumHiddenLayers == 16 {
		// Return 1B for Llama-3.2-1B models
		return 1_000_000_000
	}

	// Fallback using a rough estimate based on model size
	return estimateParamsFromArchitecture(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize)
}

// Helper function to estimate parameters from model architecture
func estimateParamsFromArchitecture(hiddenSize, numLayers, intermediateSize int) int64 {
	// If intermediateSize is 0, use the common ratio for Llama models
	if intermediateSize == 0 {
		intermediateSize = hiddenSize * 8 / 3
	}

	// A rough estimate based on the architecture
	// This includes embeddings, attention, MLP, and layer norms
	// Formula derived from analyzing Llama model architectures
	paramCount := int64(
		// Embeddings (vocab size varies, but is fixed at a reasonable size for estimation)
		hiddenSize*32000 +
			// Self-attention layers
			numLayers*(3*hiddenSize*hiddenSize+hiddenSize) +
			// Feed-forward networks
			numLayers*(hiddenSize*intermediateSize*2+hiddenSize+intermediateSize) +
			// Layer norms (2 per layer)
			numLayers*2*hiddenSize)

	// Round to nearest billion for nicer reporting
	paramCountBillions := paramCount / 1_000_000_000
	return paramCountBillions * 1_000_000_000
}

// GetTransformerVersion returns the transformers library version
func (c *LlamaConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *LlamaConfig) GetQuantizationType() string {
	if c.QuantizationConfig != nil {
		return c.QuantizationConfig.QuantMethod
	}
	return "" // Base Llama models don't have quantization by default
}

// GetArchitecture returns the model architecture
func (c *LlamaConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "LlamaForCausalLM"
}

// GetModelType returns the model type
func (c *LlamaConfig) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *LlamaConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *LlamaConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *LlamaConfig) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns false since this is not a multimodal vision model
func (c *LlamaConfig) HasVision() bool {
	return false
}

// Register the Llama model handler
func init() {
	modelLoaders["llama"] = func(configPath string) (HuggingFaceModel, error) {
		return LoadLlamaConfig(configPath)
	}
}
