package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sgl-project/ome/pkg/constants"
)

// Qwen3Config defines the configuration for Qwen3 models
type Qwen3Config struct {
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
	BosTokenId int `json:"bos_token_id"`
	EosTokenId int `json:"eos_token_id"`

	// Attention related
	HiddenAct        string  `json:"hidden_act"`
	RmsNormEps       float64 `json:"rms_norm_eps"`
	RopeTheta        float64 `json:"rope_theta"`
	AttentionDropout float64 `json:"attention_dropout"`
	SlidingWindow    int     `json:"sliding_window"`

	// For extended context models
	SeqLength       int     `json:"seq_length"`
	MaxWindowLayers int     `json:"max_window_layers"`
	RotaryEmb_base  float64 `json:"rotary_emb_base"`

	// Misc options
	TieWordEmbeddings bool `json:"tie_word_embeddings"`
	UseCache          bool `json:"use_cache"`
	UseSlidingWindow  bool `json:"use_sliding_window"`

	// Embedding config
	SimilarityFnName string `json:"similarity_fn_name"`
}

// LoadQwen3Config loads a Qwen3 model configuration from a JSON file
func LoadQwen3Config(configPath string) (*Qwen3Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Qwen3 config file '%s': %w", configPath, err)
	}

	var config Qwen3Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen3 config JSON from '%s': %w", configPath, err)
	}

	config.ConfigPath = configPath
	return &config, nil
}

// Implementation of HuggingFaceModel interface

// GetParameterCount returns the total number of parameters in the model
func (c *Qwen3Config) GetParameterCount() int64 {
	// First try to get parameter count from safetensors files
	count, err := FindAndParseSafetensors(c.ConfigPath)
	if err == nil {
		return count
	}

	// Log the error
	fmt.Printf("Warning: failed to get parameter count from safetensors: %v\n", err)

	// Hard-coded parameter counts based on model size
	if c.HiddenSize == 2560 && c.NumHiddenLayers == 36 {
		return 4_000_000_000 // 4B parameters
	} else if c.HiddenSize == 4096 && c.NumHiddenLayers == 36 {
		return 8_000_000_000 // 8B parameters
	}

	// For unknown configurations, estimate based on architecture
	return estimateModelParams(c.HiddenSize, c.NumHiddenLayers, c.IntermediateSize, c.VocabSize)
}

// GetContextLength returns the maximum context length supported by the model
func (c *Qwen3Config) GetContextLength() int {
	// Use the seq_length field if available as it's more specific for Qwen3
	if c.SeqLength > 0 {
		return c.SeqLength
	}
	return c.MaxPositionEmbeddings
}

// GetModelSizeBytes returns the estimated size of the model in bytes
func (c *Qwen3Config) GetModelSizeBytes() int64 {
	return EstimateModelSizeBytes(c.GetParameterCount(), c.TorchDtype)
}

// HasVision returns false for Qwen3 base models
func (c *Qwen3Config) HasVision() bool {
	return false // Base Qwen3 models don't have vision capabilities
}

// IsEmbedding returns true if this Qwen3 model is an embedding model (i.e., if
// SimilarityFnName is set, as in Qwen3 Embedding models), and false for base models.
func (c *Qwen3Config) IsEmbedding() bool {
	return c.SimilarityFnName != ""
}

// GetSentenceTransformersConfigPath returns the full path to config_sentence_transformers.json
// residing in the same directory as the given configPath, and verifies its existence.
// Returns the full path if the file exists, or an error if not.
func GetSentenceTransformersConfigPath(configPath string) (string, error) {
	dir := filepath.Dir(configPath)
	stConfigPath := filepath.Join(dir, constants.SentenceTransformersConfigFileName)
	if _, err := os.Stat(stConfigPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s not found in %s", constants.SentenceTransformersConfigFileName, dir)
		}
		return "", fmt.Errorf("error checking %s: %w", stConfigPath, err)
	}
	return stConfigPath, nil
}

// Register the Qwen3 model handler
func init() {
	RegisterModelLoader("qwen3", func(configPath string) (HuggingFaceModel, error) {
		qwen3Config, err := LoadQwen3Config(configPath)
		if err != nil {
			return nil, err
		}
		stConfigPath, err := GetSentenceTransformersConfigPath(configPath)
		if err == nil {
			// If sentence transformers config exists, load it to get SimilarityFnName
			stConfig, err := LoadQwen3Config(stConfigPath)
			if err == nil {
				qwen3Config.SimilarityFnName = stConfig.SimilarityFnName
			} else {
				fmt.Printf("Warning: found config_sentence_transformers.json at %s but failed to parse: %v\n", stConfigPath, err)
			}
		}
		// If st config is not found, this is not an embedding model; that's OK.
		return qwen3Config, nil
	})
}
