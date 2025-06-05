package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// MLlamaConfig defines the configuration for multimodal Llama models (Llama-3.2-Vision)
type MLlamaConfig struct {
	BaseModelConfig

	// Special tokens for multimodal support
	ImageTokenIndex int `json:"image_token_index"`

	// Text model config (language model)
	TextConfig MLlamaTextConfig `json:"text_config"`

	// Vision model config
	VisionConfig MLlamaVisionConfig `json:"vision_config"`
}

// MLlamaTextConfig defines the text/language model configuration
type MLlamaTextConfig struct {
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
	PadTokenId interface{} `json:"pad_token_id"`

	// Attention related
	HiddenAct  string  `json:"hidden_act"`
	RmsNormEps float64 `json:"rms_norm_eps"`
	RopeTheta  float64 `json:"rope_theta"`

	// RoPE scaling
	RopeScaling *RopeScalingConfig `json:"rope_scaling,omitempty"`

	// Cross-attention layers for vision integration
	CrossAttentionLayers []int `json:"cross_attention_layers"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	UseCache          bool `json:"use_cache"`
}

// MLlamaVisionConfig defines the vision model configuration
type MLlamaVisionConfig struct {
	HiddenSize       int     `json:"hidden_size"`
	IntermediateSize int     `json:"intermediate_size"`
	NumHiddenLayers  int     `json:"num_hidden_layers"`
	AttentionHeads   int     `json:"attention_heads"`
	NumGlobalLayers  int     `json:"num_global_layers"`
	HiddenAct        string  `json:"hidden_act"`
	NormEps          float64 `json:"norm_eps"`

	// Vision specific parameters
	ImageSize                 int   `json:"image_size"`
	PatchSize                 int   `json:"patch_size"`
	NumChannels               int   `json:"num_channels"`
	IntermediateLayersIndices []int `json:"intermediate_layers_indices"`

	// Vision output
	VisionOutputDim int `json:"vision_output_dim"`

	// Image processing parameters
	MaxNumTiles int `json:"max_num_tiles"`

	// Supported aspect ratios
	SupportedAspectRatios [][]int `json:"supported_aspect_ratios"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
}

// LoadMLlamaConfig loads a multimodal Llama model configuration from a JSON file
func LoadMLlamaConfig(configPath string) (*MLlamaConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MLlama config file '%s': %w", configPath, err)
	}

	var config MLlamaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MLlama config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *MLlamaConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Return parameter count based on model configuration
	// For Llama-3.2-11B-Vision, we return 11B as the parameter count
	if c.TextConfig.NumHiddenLayers == 40 && c.TextConfig.HiddenSize == 4096 {
		return 11_000_000_000
	}

	// For Llama-3.2-90B-Vision, we return 90B as the parameter count
	if c.TextConfig.NumHiddenLayers == 100 && c.TextConfig.HiddenSize == 8192 {
		return 90_000_000_000
	}

	// Fallback using a rough estimate based on model size
	textParams := estimateTextParams(c.TextConfig.HiddenSize, c.TextConfig.NumHiddenLayers, c.TextConfig.IntermediateSize)
	visionParams := estimateVisionParams(c.VisionConfig.HiddenSize, c.VisionConfig.NumHiddenLayers, c.VisionConfig.IntermediateSize)

	return textParams + visionParams
}

// Helper function to estimate text model parameters
func estimateTextParams(hiddenSize, layers, intermediateSize int) int64 {
	// This is a rough estimate; actual count could vary
	return int64(hiddenSize*hiddenSize*4*layers + hiddenSize*intermediateSize*2*layers)
}

// Helper function to estimate vision model parameters
func estimateVisionParams(hiddenSize, layers, intermediateSize int) int64 {
	// This is a rough estimate; actual count could vary
	return int64(hiddenSize*hiddenSize*4*layers + hiddenSize*intermediateSize*2*layers)
}

// GetTransformerVersion returns the transformers library version
func (c *MLlamaConfig) GetTransformerVersion() string {
	return c.TransformerVersion
}

// GetQuantizationType returns the quantization method used (if any)
func (c *MLlamaConfig) GetQuantizationType() string {
	return "" // MLlama models don't have quantization by default
}

// GetArchitecture returns the model architecture
func (c *MLlamaConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "MllamaForConditionalGeneration"
}

// GetModelType returns the model type
func (c *MLlamaConfig) GetModelType() string {
	return c.ModelType
}

// GetContextLength returns the maximum context length
func (c *MLlamaConfig) GetContextLength() int {
	return c.TextConfig.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *MLlamaConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// GetTorchDtype returns the torch data type used by the model
func (c *MLlamaConfig) GetTorchDtype() string {
	return c.TorchDtype
}

// HasVision returns true since this is a multimodal vision model
func (c *MLlamaConfig) HasVision() bool {
	return true
}

// Register the MLlama model handler
func init() {
	RegisterModelLoader("mllama", func(configPath string) (HuggingFaceModel, error) {
		return LoadMLlamaConfig(configPath)
	})
}
