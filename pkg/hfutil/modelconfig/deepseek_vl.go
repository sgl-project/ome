package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// DeepSeekVLVisionConfig defines the vision configuration for DeepSeek VL models
type DeepSeekVLVisionConfig struct {
	Layers    int     `json:"layers"`
	MlpRatio  float64 `json:"mlp_ratio"`
	ModelName string  `json:"model_name"`
	ModelType string  `json:"model_type"`
	PatchSize int     `json:"patch_size"`
	Width     int     `json:"width"`
}

// DeepSeekVLProjectorConfig defines the projector configuration
type DeepSeekVLProjectorConfig struct {
	ModelType string `json:"model_type"`
	NEmbed    int    `json:"n_embed"`
}

// DeepSeekVLLanguageConfig defines the language model configuration
type DeepSeekVLLanguageConfig struct {
	HiddenSize            int    `json:"hidden_size"`
	IntermediateSize      int    `json:"intermediate_size"`
	NumHiddenLayers       int    `json:"num_hidden_layers"`
	NumAttentionHeads     int    `json:"num_attention_heads"`
	NumKeyValueHeads      int    `json:"num_key_value_heads"`
	MaxPositionEmbeddings int    `json:"max_position_embeddings"`
	VocabSize             int    `json:"vocab_size"`
	ModelType             string `json:"model_type"`
	TorchDtype            string `json:"torch_dtype"`

	// MoE fields for DeepSeek V2
	MoeIntermediateSize int `json:"moe_intermediate_size,omitempty"`
	NRoutedExperts      int `json:"n_routed_experts,omitempty"`
	NSharedExperts      int `json:"n_shared_experts,omitempty"`
	NumExpertsPerTok    int `json:"num_experts_per_tok,omitempty"`
}

// JanusConfigs for Janus models
type JanusAlignerConfig struct {
	Cls    string `json:"cls"`
	Params struct {
		Depth         int    `json:"depth"`
		InputDim      int    `json:"input_dim"`
		NEmbed        int    `json:"n_embed"`
		ProjectorType string `json:"projector_type"`
	} `json:"params"`
}

// DeepSeekVLConfig defines the configuration for DeepSeek VL multimodal models
type DeepSeekVLConfig struct {
	BaseModelConfig

	// Vision configuration
	VisionConfig *DeepSeekVLVisionConfig `json:"vision_config,omitempty"`

	// Language configuration
	LanguageConfig *DeepSeekVLLanguageConfig `json:"language_config,omitempty"`

	// Projector configuration
	ProjectorConfig *DeepSeekVLProjectorConfig `json:"projector_config,omitempty"`

	// Janus specific configs
	AlignerConfig    *JanusAlignerConfig `json:"aligner_config,omitempty"`
	GenAlignerConfig *JanusAlignerConfig `json:"gen_aligner_config,omitempty"`

	// Other fields
	TileTag              string  `json:"tile_tag,omitempty"`
	CandidateResolutions [][]int `json:"candidate_resolutions,omitempty"`
}

// LoadDeepSeekVLConfig loads a DeepSeek VL model configuration from a JSON file
func LoadDeepSeekVLConfig(configPath string) (*DeepSeekVLConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read DeepSeek VL config file '%s': %w", configPath, err)
	}

	var config DeepSeekVLConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek VL config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *DeepSeekVLConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Estimate based on components
	totalParams := int64(0)

	// Language model parameters
	if c.LanguageConfig != nil {
		if c.LanguageConfig.MoeIntermediateSize > 0 {
			// MoE model (DeepSeek V2 style)
			totalParams += estimateMoEParams(
				c.LanguageConfig.HiddenSize,
				c.LanguageConfig.NumHiddenLayers,
				c.LanguageConfig.IntermediateSize,
				c.LanguageConfig.MoeIntermediateSize,
				c.LanguageConfig.NRoutedExperts,
				c.LanguageConfig.NSharedExperts,
				c.LanguageConfig.VocabSize,
			)
		} else {
			// Standard transformer
			totalParams += estimateModelParams(
				c.LanguageConfig.HiddenSize,
				c.LanguageConfig.NumHiddenLayers,
				c.LanguageConfig.IntermediateSize,
				c.LanguageConfig.VocabSize,
			)
		}
	}

	// Vision encoder parameters
	if c.VisionConfig != nil {
		visionParams := int64(c.VisionConfig.Width * c.VisionConfig.Width * c.VisionConfig.Layers * 4)
		totalParams += visionParams
	}

	// Projector parameters
	if c.ProjectorConfig != nil {
		projectorParams := int64(c.ProjectorConfig.NEmbed * c.ProjectorConfig.NEmbed * 2)
		totalParams += projectorParams
	}

	// Known model sizes
	if c.LanguageConfig != nil {
		if c.LanguageConfig.HiddenSize == 1280 && c.LanguageConfig.NumHiddenLayers == 12 {
			return 1_000_000_000 // DeepSeek VL2 Tiny ~1B
		} else if c.LanguageConfig.HiddenSize == 2048 && c.LanguageConfig.NumHiddenLayers == 24 {
			return 1_300_000_000 // Janus 1.3B
		}
	}

	return totalParams
}

// GetContextLength returns the maximum context length supported by the model
func (c *DeepSeekVLConfig) GetContextLength() int {
	if c.LanguageConfig != nil {
		return c.LanguageConfig.MaxPositionEmbeddings
	}
	return 4096 // default
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *DeepSeekVLConfig) GetModelSizeBytes() int64 {
	dtype := c.TorchDtype
	if dtype == "" && c.LanguageConfig != nil {
		dtype = c.LanguageConfig.TorchDtype
	}
	return EstimateModelSizeBytes(c.GetParameterCount(), dtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *DeepSeekVLConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns true for DeepSeek VL models
func (c *DeepSeekVLConfig) HasVision() bool {
	return true
}

// GetArchitecture returns the model architecture
func (c *DeepSeekVLConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	// Default architectures for DeepSeek VL models
	if c.ModelType == "deepseek_vl_v2" {
		return "DeepseekVLV2ForCausalLM"
	} else if c.ModelType == "multi_modality" || c.ModelType == "janus" {
		return "JanusMultiModalityCausalLM"
	}
	return "DeepseekVLForCausalLM"
}

// Helper function to estimate MoE model parameters
func estimateMoEParams(hiddenSize, numLayers, intermediateSize, moeIntermediateSize, nRoutedExperts, nSharedExperts, vocabSize int) int64 {
	// Embeddings
	params := int64(hiddenSize * vocabSize)

	// For each layer
	params += int64(numLayers) * (
	// Self-attention
	int64(4*hiddenSize*hiddenSize) +
		// Shared experts
		int64(nSharedExperts*2*hiddenSize*intermediateSize) +
		// Routed experts
		int64(nRoutedExperts*2*hiddenSize*moeIntermediateSize) +
		// Router
		int64(hiddenSize*nRoutedExperts) +
		// Layer norms
		int64(2*hiddenSize))

	return params
}

// Register the DeepSeek VL model handlers
func init() {
	RegisterModelLoader("deepseek_vl_v2", func(configPath string) (HuggingFaceModel, error) {
		return LoadDeepSeekVLConfig(configPath)
	})
	RegisterModelLoader("janus", func(configPath string) (HuggingFaceModel, error) {
		return LoadDeepSeekVLConfig(configPath)
	})
	RegisterModelLoader("multi_modality", func(configPath string) (HuggingFaceModel, error) {
		return LoadDeepSeekVLConfig(configPath)
	})
}
