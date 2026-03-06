package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwen3Config(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_4b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3 config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3" {
		t.Errorf("Expected model type 'qwen3' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3Config, ok := config.(*Qwen3Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3Config, but got %T", config)
	}

	// Check key fields
	if qwen3Config.HiddenSize != 2560 {
		t.Errorf("Expected hidden size to be 2560, but got %d", qwen3Config.HiddenSize)
	}

	if qwen3Config.NumHiddenLayers != 36 {
		t.Errorf("Expected hidden layers to be 36, but got %d", qwen3Config.NumHiddenLayers)
	}

	if qwen3Config.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwen3Config.NumAttentionHeads)
	}

	if qwen3Config.NumKeyValueHeads != 8 {
		t.Errorf("Expected key-value heads to be 8, but got %d", qwen3Config.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(4_000_000_000) // 4B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if qwen3Config.RopeTheta != 5000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", qwen3Config.RopeTheta)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen3, but got true")
	}

	// Check quantization type (should be empty for non-quantized model)
	if config.GetQuantizationType() != "" {
		t.Errorf("Expected empty quantization type, but got '%s'", config.GetQuantizationType())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}

func TestQwen3FP8Config(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_8b_fp8.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3 FP8 config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3" {
		t.Errorf("Expected model type 'qwen3' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3Config, ok := config.(*Qwen3Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3Config, but got %T", config)
	}

	// Check quantization config details
	if qwen3Config.QuantizationConfig == nil {
		t.Fatal("Expected QuantizationConfig to be non-nil")
	}

	// Check quantization type
	if config.GetQuantizationType() != "fp8" {
		t.Errorf("Expected quantization type 'fp8', but got '%s'", config.GetQuantizationType())
	}

	if qwen3Config.QuantizationConfig.Format != "e4m3" {
		t.Errorf("Expected format 'e4m3', but got '%s'", qwen3Config.QuantizationConfig.Format)
	}

	if qwen3Config.QuantizationConfig.ActivationScheme != "dynamic" {
		t.Errorf("Expected activation scheme 'dynamic', but got '%s'", qwen3Config.QuantizationConfig.ActivationScheme)
	}

	if len(qwen3Config.QuantizationConfig.WeightBlockSize) != 2 {
		t.Errorf("Expected weight_block_size to have 2 elements, but got %d", len(qwen3Config.QuantizationConfig.WeightBlockSize))
	}

	// Verify key model fields are still parsed correctly
	if qwen3Config.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", qwen3Config.HiddenSize)
	}

	if qwen3Config.NumHiddenLayers != 36 {
		t.Errorf("Expected hidden layers to be 36, but got %d", qwen3Config.NumHiddenLayers)
	}
}

func TestQwen3EmbeddingConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_embedding_8b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3 embedding config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3" {
		t.Errorf("Expected model type 'qwen3' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3Config, ok := config.(*Qwen3Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3Config, but got %T", config)
	}

	// Check key fields
	if qwen3Config.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", qwen3Config.HiddenSize)
	}

	if qwen3Config.NumHiddenLayers != 36 {
		t.Errorf("Expected hidden layers to be 36, but got %d", qwen3Config.NumHiddenLayers)
	}

	if qwen3Config.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwen3Config.NumAttentionHeads)
	}

	if qwen3Config.NumKeyValueHeads != 8 {
		t.Errorf("Expected key-value heads to be 8, but got %d", qwen3Config.NumKeyValueHeads)
	}

	if qwen3Config.SimilarityFnName != "cosine" {
		t.Errorf("Expected similarity_fn_name to be cosine, but got %s", qwen3Config.SimilarityFnName)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 40960
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 8B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(8_000_000_000) // 8B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if qwen3Config.RopeTheta != 1000000.0 {
		t.Errorf("Expected RoPE theta to be 1000000.0, but got %f", qwen3Config.RopeTheta)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen3, but got true")
	}

	// Check embedding capability (should be true for this model)
	if !config.IsEmbedding() {
		t.Error("Expected IsEmbedding to return true for Qwen3, but got false")
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
