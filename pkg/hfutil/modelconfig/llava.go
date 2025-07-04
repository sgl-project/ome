package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// LLaVATextConfig defines the language model configuration for LLaVA
type LLaVATextConfig struct {
	ModelType             string  `json:"model_type"`
	MaxPositionEmbeddings int     `json:"max_position_embeddings"`
	VocabSize             int     `json:"vocab_size"`
	HiddenSize            int     `json:"hidden_size,omitempty"`
	IntermediateSize      int     `json:"intermediate_size,omitempty"`
	NumHiddenLayers       int     `json:"num_hidden_layers,omitempty"`
	NumAttentionHeads     int     `json:"num_attention_heads,omitempty"`
	NumKeyValueHeads      int     `json:"num_key_value_heads,omitempty"`
	RmsNormEps            float64 `json:"rms_norm_eps,omitempty"`
	TorchDtype            string  `json:"torch_dtype,omitempty"`
}

// LLaVAVisionConfig defines the vision encoder configuration
type LLaVAVisionConfig struct {
	HiddenSize        int    `json:"hidden_size"`
	ImageSize         int    `json:"image_size"`
	IntermediateSize  int    `json:"intermediate_size"`
	ModelType         string `json:"model_type"`
	NumAttentionHeads int    `json:"num_attention_heads"`
	NumHiddenLayers   int    `json:"num_hidden_layers"`
	PatchSize         int    `json:"patch_size"`
	ProjectionDim     int    `json:"projection_dim"`
	VocabSize         int    `json:"vocab_size,omitempty"`
}

// LLaVAConfig defines the configuration for LLaVA multimodal models
type LLaVAConfig struct {
	BaseModelConfig

	// Special tokens
	IgnoreIndex     int `json:"ignore_index"`
	ImageTokenIndex int `json:"image_token_index"`
	PadTokenId      int `json:"pad_token_id"`

	// Model components
	TextConfig   LLaVATextConfig   `json:"text_config"`
	VisionConfig LLaVAVisionConfig `json:"vision_config"`

	// Projector settings
	ProjectorHiddenAct string `json:"projector_hidden_act"`

	// Vision feature settings
	VisionFeatureLayer          int    `json:"vision_feature_layer"`
	VisionFeatureSelectStrategy string `json:"vision_feature_select_strategy"`

	// Other settings
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	VocabSize         int  `json:"vocab_size"`
}

// LoadLLaVAConfig loads a LLaVA model configuration from a JSON file
func LoadLLaVAConfig(configPath string) (*LLaVAConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read LLaVA config file '%s': %w", configPath, err)
	}

	var config LLaVAConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse LLaVA config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *LLaVAConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	totalParams := int64(0)

	// Language model parameters
	// LLaVA typically uses Vicuna/Llama as the language model
	if c.TextConfig.ModelType == "llama" {
		// Try to load the actual text model config if possible
		textParams := int64(0)
		if c.TextConfig.HiddenSize > 0 {
			textParams = estimateModelParams(
				c.TextConfig.HiddenSize,
				c.TextConfig.NumHiddenLayers,
				c.TextConfig.IntermediateSize,
				c.TextConfig.VocabSize,
			)
		} else {
			// Estimate based on vocab size (typical LLaVA uses 7B or 13B Llama)
			if c.TextConfig.VocabSize >= 32000 {
				textParams = 7_000_000_000 // Default to 7B
			}
		}
		totalParams += textParams
	}

	// Vision encoder parameters (CLIP ViT)
	if c.VisionConfig.ModelType == "clip_vision_model" {
		// CLIP ViT-L/14 has ~304M parameters
		visionParams := int64(
			// Patch embedding
			c.VisionConfig.PatchSize*c.VisionConfig.PatchSize*3*c.VisionConfig.HiddenSize +
				// Position embeddings
				(c.VisionConfig.ImageSize/c.VisionConfig.PatchSize)*(c.VisionConfig.ImageSize/c.VisionConfig.PatchSize)*c.VisionConfig.HiddenSize +
				// Transformer layers
				c.VisionConfig.NumHiddenLayers*(
				// Self-attention
				4*c.VisionConfig.HiddenSize*c.VisionConfig.HiddenSize+
					// FFN
					2*c.VisionConfig.HiddenSize*c.VisionConfig.IntermediateSize+
					// Layer norms
					2*c.VisionConfig.HiddenSize))
		totalParams += visionParams
	}

	// Projector parameters (typically 2-layer MLP)
	if c.VisionConfig.HiddenSize > 0 && c.TextConfig.HiddenSize > 0 {
		projectorParams := int64(2 * c.VisionConfig.HiddenSize * c.TextConfig.HiddenSize)
		totalParams += projectorParams
	}

	// Known configurations
	if c.VisionConfig.NumHiddenLayers == 24 && c.TextConfig.VocabSize >= 32000 {
		return 7_500_000_000 // LLaVA 1.5 7B (7B Vicuna + ~300M CLIP + projector)
	}

	return totalParams
}

// GetContextLength returns the maximum context length supported by the model
func (c *LLaVAConfig) GetContextLength() int {
	return c.TextConfig.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *LLaVAConfig) GetModelSizeBytes() int64 {
	dtype := c.TorchDtype
	if dtype == "" && c.TextConfig.TorchDtype != "" {
		dtype = c.TextConfig.TorchDtype
	}
	return EstimateModelSizeBytes(c.GetParameterCount(), dtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *LLaVAConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns true for LLaVA models
func (c *LLaVAConfig) HasVision() bool {
	return true
}

// Register the LLaVA model handler
func init() {
	RegisterModelLoader("llava", func(configPath string) (HuggingFaceModel, error) {
		return LoadLLaVAConfig(configPath)
	})
}
