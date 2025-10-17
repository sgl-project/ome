package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwen3MoeConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_30b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3Moe config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_moe" {
		t.Errorf("Expected model type 'qwen3_moe' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3MoeConfig, ok := config.(*Qwen3MoeConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3MoeConfig, but got %T", config)
	}

	// Check key fields
	if qwen3MoeConfig.HiddenSize != 2048 {
		t.Errorf("Expected hidden size to be 2048, but got %d", qwen3MoeConfig.HiddenSize)
	}

	if qwen3MoeConfig.NumHiddenLayers != 48 {
		t.Errorf("Expected hidden layers to be 48, but got %d", qwen3MoeConfig.NumHiddenLayers)
	}

	if qwen3MoeConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwen3MoeConfig.NumAttentionHeads)
	}

	if qwen3MoeConfig.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", qwen3MoeConfig.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(30_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if qwen3MoeConfig.RopeTheta != 10000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", qwen3MoeConfig.RopeTheta)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen3, but got true")
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
