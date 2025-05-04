package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestMixtralConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "mixtral.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mixtral config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "mixtral" {
		t.Errorf("Incorrect model type, expected 'mixtral', got '%s'", config.GetModelType())
	}

	// Get the MixtralConfig by type assertion
	mixtralConfig, ok := config.(*MixtralConfig)
	if !ok {
		t.Fatalf("Failed to convert to MixtralConfig")
	}

	if mixtralConfig.HiddenSize != 4096 {
		t.Errorf("Incorrect hidden size, expected 4096, got %d", mixtralConfig.HiddenSize)
	}

	if mixtralConfig.NumHiddenLayers != 32 {
		t.Errorf("Incorrect hidden layers, expected 32, got %d", mixtralConfig.NumHiddenLayers)
	}

	if mixtralConfig.NumLocalExperts != 8 {
		t.Errorf("Incorrect number of experts, expected 8, got %d", mixtralConfig.NumLocalExperts)
	}

	if mixtralConfig.NumExpertsPerTok != 2 {
		t.Errorf("Incorrect experts per token, expected 2, got %d", mixtralConfig.NumExpertsPerTok)
	}

	if mixtralConfig.MaxPositionEmbeddings != 32768 {
		t.Errorf("Incorrect context length, expected 32768, got %d", mixtralConfig.MaxPositionEmbeddings)
	}

	// Test parameter count (will return official 46.7B for Mixtral-8x7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(46_700_000_000)
	if paramCount != expectedCount {
		t.Errorf("Incorrect parameter count, expected %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	// Test GetModelSizeBytes
	modelSize := config.GetModelSizeBytes()
	// Expected size for bfloat16 (2 bytes per parameter)
	expectedSize := int64(46_700_000_000 * 2)
	if modelSize != expectedSize {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSize), FormatSize(modelSize))
	}
}

func TestLoadModelWithMixtral(t *testing.T) {
	configPath := filepath.Join("testdata", "mixtral.json")

	// Test loading through the generic loader
	model, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mixtral model through generic loader: %v", err)
	}

	// Verify it's a Mixtral model
	if model.GetModelType() != "mixtral" {
		t.Errorf("Expected model type 'mixtral', got '%s'", model.GetModelType())
	}

	// Verify context length
	if model.GetContextLength() != 32768 {
		t.Errorf("Expected context length 32768, got %d", model.GetContextLength())
	}

	// Verify parameter count
	paramCount := model.GetParameterCount()
	expectedCount := int64(46_700_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	t.Logf("Mixtral model parameter count via generic loader: %s", FormatParamCount(paramCount))
}
