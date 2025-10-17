package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Phi3SmallConfig defines the configuration for Phi-3 Small models
// Phi-3 Small uses blocksparse attention and muP (maximal update parameterization)
type Phi3SmallConfig struct {
	BaseModelConfig

	// Auto map for custom model loading
	AutoMap *AutoMap `json:"auto_map,omitempty"`

	// Model dimensions
	HiddenSize            int `json:"hidden_size"`
	FfIntermediateSize    int `json:"ff_intermediate_size"`
	NumHiddenLayers       int `json:"num_hidden_layers"`
	NumAttentionHeads     int `json:"num_attention_heads"`
	NumKeyValueHeads      int `json:"num_key_value_heads"`
	MaxPositionEmbeddings int `json:"max_position_embeddings"`
	VocabSize             int `json:"vocab_size"`

	// Special tokens
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`

	// Blocksparse attention configuration
	BlocksparseBlockSize             int  `json:"blocksparse_block_size"`
	BlocksparseHomoHeadPattern       bool `json:"blocksparse_homo_head_pattern"`
	BlocksparseNumLocalBlocks        int  `json:"blocksparse_num_local_blocks"`
	BlocksparseTritonKernelBlockSize int  `json:"blocksparse_triton_kernel_block_size"`
	BlocksparseVertStride            int  `json:"blocksparse_vert_stride"`
	DenseAttentionEveryNLayers       int  `json:"dense_attention_every_n_layers"`

	// Activation function (GeGELU)
	HiddenAct       string  `json:"hidden_act"`
	GegeluLimit     float64 `json:"gegelu_limit"`
	GegeluPadTo256  bool    `json:"gegelu_pad_to_256"`
	FfDimMultiplier *int    `json:"ff_dim_multiplier"` // nullable

	// Normalization
	LayerNormEpsilon float64 `json:"layer_norm_epsilon"`

	// Dropout
	AttentionDropoutProb float64 `json:"attention_dropout_prob"`
	EmbeddingDropoutProb float64 `json:"embedding_dropout_prob"`
	FfnDropoutProb       float64 `json:"ffn_dropout_prob"`

	// RoPE (Rotary Position Embedding)
	RopeEmbeddingBase float64 `json:"rope_embedding_base"`
	RopePositionScale float64 `json:"rope_position_scale"`

	// muP (Maximal Update Parameterization) scaling
	MupAttnMultiplier      float64 `json:"mup_attn_multiplier"`
	MupEmbeddingMultiplier float64 `json:"mup_embedding_multiplier"`
	MupUseScaling          bool    `json:"mup_use_scaling"`
	MupWidthMultiplier     float64 `json:"mup_width_multiplier"`

	// Other configurations
	AttentionBias             bool    `json:"attention_bias"`
	InitializerRange          float64 `json:"initializer_range"`
	PadSequenceToMultipleOf64 bool    `json:"pad_sequence_to_multiple_of_64"`
	ReorderAndUpcastAttn      bool    `json:"reorder_and_upcast_attn"`
	UseCache                  bool    `json:"use_cache"`
}

// LoadPhi3SmallConfig loads a Phi-3 Small model configuration from a JSON file
func LoadPhi3SmallConfig(configPath string) (*Phi3SmallConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Phi3Small config file '%s': %w", configPath, err)
	}

	var config Phi3SmallConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Phi3Small config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// GetParameterCount returns the total number of parameters in the model
func (c *Phi3SmallConfig) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Fallback: estimate based on known Phi-3 Small model sizes
	// Phi-3-small-8k-instruct: hidden_size=4096, num_hidden_layers=32 ≈ 7B parameters
	if c.HiddenSize == 4096 && c.NumHiddenLayers == 32 {
		return 7_000_000_000
	}

	// For unknown configs, return 0 to indicate estimation failed
	return 0
}

// GetContextLength returns the maximum context length supported by the model
func (c *Phi3SmallConfig) GetContextLength() int {
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Phi3SmallConfig) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// GetQuantizationType returns the quantization method used (if any)
func (c *Phi3SmallConfig) GetQuantizationType() string {
	return "" // No quantization by default
}

// HasVision returns false since this is not a multimodal vision model
func (c *Phi3SmallConfig) HasVision() bool {
	return false
}

// Register the Phi3Small model handler
func init() {
	RegisterModelLoader("phi3small", func(configPath string) (HuggingFaceModel, error) {
		return LoadPhi3SmallConfig(configPath)
	})
}
