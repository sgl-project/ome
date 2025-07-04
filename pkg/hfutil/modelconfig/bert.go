package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// BertConfig defines the configuration for BERT-based models (including embeddings)
type BertConfig struct {
	BaseModelConfig

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	IntermediateSize      int `json:"intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`
	TypeVocabSize         int `json:"type_vocab_size"`

	// Dropout and activation
	HiddenAct                 string   `json:"hidden_act"`
	HiddenDropoutProb         float64  `json:"hidden_dropout_prob"`
	AttentionProbsDropoutProb float64  `json:"attention_probs_dropout_prob"`
	ClassifierDropout         *float64 `json:"classifier_dropout"`

	// Layer norm
	LayerNormEps float64 `json:"layer_norm_eps"`

	// Position embeddings
	PositionEmbeddingType string `json:"position_embedding_type"`

	// Special tokens
	PadTokenId int `json:"pad_token_id"`

	// Misc options
	InitializerRange      float64 `json:"initializer_range"`
	UseCache              bool    `json:"use_cache"`
	GradientCheckpointing bool    `json:"gradient_checkpointing"`
}

// LoadBertConfig loads a BERT model configuration from a JSON file
func LoadBertConfig(configPath string) (*BertConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read BERT config file '%s': %w", configPath, err)
	}

	var config BertConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse BERT config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *BertConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Known BERT model sizes
	if c.HiddenSize == 768 && c.NumHiddenLayers == 12 {
		return 110_000_000 // BERT-base
	} else if c.HiddenSize == 1024 && c.NumHiddenLayers == 24 {
		return 340_000_000 // BERT-large
	} else if c.HiddenSize == 1024 && c.NumHiddenLayers == 24 && c.VocabSize > 100000 {
		return 560_000_000 // BGE-large models with larger vocab
	}

	// Estimate for BERT architecture
	vocabSize := c.VocabSize
	if vocabSize == 0 {
		vocabSize = 30522 // default BERT vocab size
	}

	return int64(
		// Token embeddings + position embeddings + token type embeddings
		(vocabSize+c.MaxPositionEmbeddings+c.TypeVocabSize)*c.HiddenSize +
			// Encoder layers
			c.NumHiddenLayers*(
			// Self-attention
			4*c.HiddenSize*c.HiddenSize+
				// FFN
				2*c.HiddenSize*c.IntermediateSize+
				// Layer norms
				2*c.HiddenSize) +
			// Pooler
			c.HiddenSize*c.HiddenSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *BertConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *BertConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *BertConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for BERT models
func (c *BertConfig) HasVision() bool {
	return false
}

// Register the BERT model handler
func init() {
	RegisterModelLoader("bert", func(configPath string) (HuggingFaceModel, error) {
		return LoadBertConfig(configPath)
	})
}
