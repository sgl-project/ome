package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Phi3VConfig defines the configuration for Phi-3 Vision models
type Phi3VConfig struct {
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

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	SlidingWindow    int     `json:"sliding_window"`

	// Special for Phi3 Vision
	OriginalMaxPositionEmbeddings int                    `json:"original_max_position_embeddings"`
	RopeScaling                   map[string]interface{} `json:"rope_scaling"`
	AttnImplementation            string                 `json:"_attn_implementation"`

	// Vision specific fields
	ImgProcessor map[string]interface{} `json:"img_processor"`
	EmbdLayer    map[string]interface{} `json:"embd_layer"`
}

// LoadPhi3VConfig loads a Phi-3 Vision model configuration from a JSON file
func LoadPhi3VConfig(configPath string) (*Phi3VConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Phi3V config file '%s': %w", configPath, err)
	}

	var config Phi3VConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Phi3V config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *Phi3VConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return parameter count based on configuration
	// Hard-coded values based on official model specifications
	if c.HiddenSize == 3072 && c.NumHiddenLayers == 32 {
		return 14_000_000_000 // 14B parameters for Phi-3 Vision 14B
	}

	// For unknown configs, estimate based on model architecture (this is an approximation)
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *Phi3VConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Phi3VConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Phi3VConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns true since this is a multimodal vision model
func (c *Phi3VConfig) HasVision() bool {
	return c.ImgProcessor != nil
}

// Helper function to estimate model parameters
func estimateModelParams(hiddenSize, numLayers, intermediateSize, vocabSize int) int64 {
	// This is an approximation - actual parameter count may vary
	return int64(
		// Embeddings
		hiddenSize*vocabSize +
			// Self-attention layers
			numLayers*(3*hiddenSize*hiddenSize+hiddenSize) +
			// Feed-forward networks
			numLayers*(hiddenSize*intermediateSize*2+hiddenSize+intermediateSize) +
			// Layer norms
			numLayers*2*hiddenSize +
			// Final projection
			hiddenSize*vocabSize)
}

// Register the Phi3V model handler
func init() {
	RegisterModelLoader("phi3_v", func(configPath string) (HuggingFaceModel, error) {
		return LoadPhi3VConfig(configPath)
	})
}
