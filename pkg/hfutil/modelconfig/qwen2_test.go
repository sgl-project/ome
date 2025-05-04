package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwen2Config(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen2_7b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen2 config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen2" {
		t.Errorf("Expected model type 'qwen2' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen2Config
	qwen2Config, ok := config.(*Qwen2Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen2Config, but got %T", config)
	}

	// Check key fields
	if qwen2Config.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", qwen2Config.HiddenSize)
	}

	if qwen2Config.NumHiddenLayers != 32 {
		t.Errorf("Expected hidden layers to be 32, but got %d", qwen2Config.NumHiddenLayers)
	}

	if qwen2Config.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwen2Config.NumAttentionHeads)
	}

	if qwen2Config.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", qwen2Config.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 65536
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(7_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen2)
	if qwen2Config.RopeTheta != 1000000.0 {
		t.Errorf("Expected RoPE theta to be 1000000.0, but got %f", qwen2Config.RopeTheta)
	}

	// Check sliding window
	if qwen2Config.SlidingWindow != 65536 {
		t.Errorf("Expected sliding window to be 65536, but got %d", qwen2Config.SlidingWindow)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen2, but got true")
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
