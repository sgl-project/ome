package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Phi3RopeScaling defines the rope scaling configuration for Phi-3 models
type Phi3RopeScaling struct {
	Type        string    `json:"type"`
	LongFactor  []float64 `json:"long_factor"`
	ShortFactor []float64 `json:"short_factor"`
}

// Phi3Config defines the configuration for Phi-3 models
type Phi3Config struct {
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
	PadTokenId int `json:"pad_token_id"`

	// Attention and normalization
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	AttentionBias    bool    `json:"attention_bias"`
	SlidingWindow    int     `json:"sliding_window"`

	// Dropout
	EmbdPdrop  float64 `json:"embd_pdrop"`
	ResidPdrop float64 `json:"resid_pdrop"`

	// Rope scaling for extended context
	OriginalMaxPositionEmbeddings int              `json:"original_max_position_embeddings"`
	RopeScaling                   *Phi3RopeScaling `json:"rope_scaling,omitempty"`

	// Other configurations
	InitializerRange  float64 `json:"initializer_range"`
	TieWordEmbeddings bool    `json:"tie_word_embeddings"`
	UseCache          bool    `json:"use_cache"`
}

// LoadPhi3Config loads a Phi-3 model configuration from a JSON file
func LoadPhi3Config(configPath string) (*Phi3Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Phi3 config file '%s': %w", configPath, err)
	}

	var config Phi3Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Phi3 config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// GetParameterCount returns the total number of parameters in the model
func (c *Phi3Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Fallback: estimate based on known Phi-3 model sizes
	// Phi-3-mini: hidden_size=3072, num_hidden_layers=32 ≈ 3.8B parameters
	if c.HiddenSize == 3072 && c.NumHiddenLayers == 32 {
		return 3_800_000_000
	}

	// For unknown configs, return 0 to indicate estimation failed
	return 0
}

// GetContextLength returns the maximum context length supported by the model
func (c *Phi3Config) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Phi3Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Phi3Config) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false since this is not a multimodal vision model
func (c *Phi3Config) HasVision() bool {
	return false
}

// Register the Phi3 model handler
func init() {
	RegisterModelLoader("phi3", func(configPath string) (HuggingFaceModel, error) {
		return LoadPhi3Config(configPath)
	})
}
