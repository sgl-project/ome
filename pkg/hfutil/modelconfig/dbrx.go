package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// DBRXFFNConfig defines the feed-forward network configuration for DBRX
type DBRXFFNConfig struct {
	FFNActiveFn   string `json:"ffn_act_fn"`
	FFNHiddenSize int    `json:"ffn_hidden_size"`
}

// DBRXAttentionConfig defines the attention configuration for DBRX
type DBRXAttentionConfig struct {
	AttnPdrop float64 `json:"attn_pdrop"`
	ClipQKV   float64 `json:"clip_qkv,omitempty"`
	KVNHeads  int     `json:"kv_n_heads"`
	RopeTheta float64 `json:"rope_theta"`
}

// DBRXConfig defines the configuration for Databricks DBRX models
type DBRXConfig struct {
	BaseModelConfig

	// Model dimensions
	DModel    int `json:"d_model"`
	NHeads    int `json:"n_heads"`
	NLayers   int `json:"n_layers"`
	MaxSeqLen int `json:"max_seq_len"`
	VocabSize int `json:"vocab_size"`

	// MoE configuration
	FFNConfig struct {
		FFNActiveFn   string  `json:"ffn_act_fn"`
		FFNHiddenSize int     `json:"ffn_hidden_size"`
		MoEJitterEps  float64 `json:"moe_jitter_eps,omitempty"`
		MoeLossWeight float64 `json:"moe_loss_weight"`
		MoENExperts   int     `json:"moe_num_experts"`
		MoETopK       int     `json:"moe_top_k"`
	} `json:"ffn_config"`

	// Attention configuration
	AttnConfig DBRXAttentionConfig `json:"attn_config"`

	// Embeddings
	EmbPdrop float64 `json:"emb_pdrop"`

	// Other settings
	InitializerRange   float64 `json:"initializer_range"`
	OutputRouterLogits bool    `json:"output_router_logits"`
	ResidPdrop         float64 `json:"resid_pdrop"`
	TieWordEmbeddings  bool    `json:"tie_word_embeddings"`
	UseCache           bool    `json:"use_cache"`
}

// LoadDBRXConfig loads a DBRX model configuration from a JSON file
func LoadDBRXConfig(configPath string) (*DBRXConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read DBRX config file '%s': %w", configPath, err)
	}

	var config DBRXConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse DBRX config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *DBRXConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// DBRX is a MoE model with 132B total parameters
	// Active parameters per token: ~36B
	// Based on public information:
	// - 16 experts, top-4 routing
	// - 48 layers
	// - 6144 hidden size

	if c.NLayers == 48 && c.DModel == 6144 {
		return 132_000_000_000 // DBRX has 132B total parameters
	}

	// Fallback estimation for MoE
	totalParams := int64(0)

	// Embeddings
	totalParams += int64(c.VocabSize * c.DModel)

	// For each layer
	totalParams += int64(c.NLayers) * (
	// Attention (Q, K, V, O projections)
	int64(4*c.DModel*c.DModel) +
		// MoE FFN (each expert has its own FFN)
		int64(c.FFNConfig.MoENExperts*2*c.DModel*c.FFNConfig.FFNHiddenSize) +
		// Router
		int64(c.DModel*c.FFNConfig.MoENExperts) +
		// Layer norms
		int64(2*c.DModel))

	// Output projection
	if !c.TieWordEmbeddings {
		totalParams += int64(c.VocabSize * c.DModel)
	}

	return totalParams
}

// GetContextLength returns the maximum context length supported by the model
func (c *DBRXConfig) GetContextLength() int {
	return c.MaxSeqLen
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *DBRXConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *DBRXConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false for DBRX base models
func (c *DBRXConfig) HasVision() bool {
	return false
}

// GetArchitecture returns the model architecture
func (c *DBRXConfig) GetArchitecture() string {
	if len(c.Architectures) > 0 {
		return c.Architectures[0]
	}
	return "DbrxForCausalLM"
}

// Register the DBRX model handler
func init() {
	RegisterModelLoader("dbrx", func(configPath string) (HuggingFaceModel, error) {
		return LoadDBRXConfig(configPath)
	})
}
