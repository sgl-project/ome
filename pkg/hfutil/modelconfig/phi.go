package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// PhiModelConfig represents the configuration for a Phi model
type PhiModelConfig struct {
	BaseModelConfig

	AttentionDropout          float64   `json:"attention_dropout"`
	AttentionProbsDropoutProb float64   `json:"attention_probs_dropout_prob"`
	BosTokenId                int       `json:"bos_token_id"`
	EmbdPdrop                 float64   `json:"embd_pdrop"`
	EosTokenId                int       `json:"eos_token_id"`
	HiddenAct                 string    `json:"hidden_act"`
	HiddenDropoutProb         float64   `json:"hidden_dropout_prob"`
	HiddenSize                int       `json:"hidden_size"`
	InitializerRange          float64   `json:"initializer_range"`
	IntermediateSize          int       `json:"intermediate_size"`
	LayerNormEps              float64   `json:"layer_norm_eps"`
	MaxPositionEmbeddings     int       `json:"max_position_embeddings"`
	NumAttentionHeads         int       `json:"num_attention_heads"`
	NumHiddenLayers           int       `json:"num_hidden_layers"`
	NumKeyValueHeads          int       `json:"num_key_value_heads"`
	PadTokenId                int       `json:"pad_token_id"`
	PartialRotaryFactor       float64   `json:"partial_rotary_factor"`
	QkLayernorm               bool      `json:"qk_layernorm"`
	ResidPdrop                float64   `json:"resid_pdrop"`
	RopeScaling               *struct{} `json:"rope_scaling"`
	RopeTheta                 float64   `json:"rope_theta"`
	TieWordEmbeddings         bool      `json:"tie_word_embeddings"`
	TypeVocabSize             int       `json:"type_vocab_size"`
	UseCache                  bool      `json:"use_cache"`
	VocabSize                 int       `json:"vocab_size"`
}

// LoadPhiModelConfig loads a Phi model configuration from a JSON file
func LoadPhiModelConfig(path string) (*PhiModelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Phi config file '%s': %w", path, err)
	}

	var cfg PhiModelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Phi config JSON from '%s': %w", path, err)
	}

	cfg.ConfigPath = path
	return &cfg, nil
}

// GetParameterCount returns the total number of parameters in the model
// It parses the safetensors file for an accurate count
func (c *PhiModelConfig) GetParameterCount() int64 {
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Phi models should have their parameter counts determined from safetensors
	// For this tiny test model, we'll return 0 if safetensors parsing fails
	return 0
}

// GetContextLength returns the maximum context length
func (c *PhiModelConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *PhiModelConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.GetTorchDtype())
}

// HasVision returns false since this is not a multimodal vision model
func (c *PhiModelConfig) HasVision() bool {
	return false
}

// IsEmbedding returns false since this is not an embedding model
func (c *PhiModelConfig) IsEmbedding() bool {
	return false
}

// Register the Phi model handler
func init() {
	RegisterModelLoader("phi", func(configPath string) (HuggingFaceModel, error) {
		return LoadPhiModelConfig(configPath)
	})
}
