package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwen3VLConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_vl_235b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3VL config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_vl_moe" {
		t.Errorf("Expected model type 'qwen3_vl_moe' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3VLConfig, ok := config.(*Qwen3VLConfig)
	textConfig := qwen3VLConfig.TextConfig
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3VLConfig, but got %T", config)
	}

	// Check key fields
	if textConfig.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", textConfig.HiddenSize)
	}

	if textConfig.NumHiddenLayers != 94 {
		t.Errorf("Expected hidden layers to be 94, but got %d", textConfig.NumHiddenLayers)
	}

	if textConfig.NumAttentionHeads != 64 {
		t.Errorf("Expected attention heads to be 64, but got %d", textConfig.NumAttentionHeads)
	}

	if textConfig.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", textConfig.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(235_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if textConfig.RopeTheta != 5000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", textConfig.RopeTheta)
	}

	// Test vision capability
	if !config.HasVision() {
		t.Errorf("Expected HasVision() to return true, got %v", config.HasVision())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
