package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestMistralConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "mistral.json")

	// Load the config directly through LoadModelConfig
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mistral config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "mistral" {
		t.Errorf("Incorrect model type, expected 'mistral', got '%s'", config.GetModelType())
	}

	// Get the MistralConfig by type assertion
	mistralConfig, ok := config.(*MistralConfig)
	if !ok {
		t.Fatalf("Failed to convert to MistralConfig")
	}

	if mistralConfig.HiddenSize != 4096 {
		t.Errorf("Incorrect hidden size, expected 4096, got %d", mistralConfig.HiddenSize)
	}

	if mistralConfig.NumHiddenLayers != 32 {
		t.Errorf("Incorrect hidden layers, expected 32, got %d", mistralConfig.NumHiddenLayers)
	}

	if mistralConfig.MaxPositionEmbeddings != 32768 {
		t.Errorf("Incorrect context length, expected 32768, got %d", mistralConfig.MaxPositionEmbeddings)
	}

	// Test parameter count (will return official 7B for this model)
	paramCount := config.GetParameterCount()
	expectedCount := int64(7_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Incorrect parameter count, expected %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	// Test GetModelSizeBytes
	modelSize := config.GetModelSizeBytes()
	expectedSize := int64(7_000_000_000 * 2) // float16 = 2 bytes per parameter
	if modelSize != expectedSize {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSize), FormatSize(modelSize))
	}
}

func TestLoadModelWithMistral(t *testing.T) {
	configPath := filepath.Join("testdata", "mistral.json")

	// Test loading through the generic loader
	model, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mistral model through generic loader: %v", err)
	}

	// Verify it's a Mistral model
	if model.GetModelType() != "mistral" {
		t.Errorf("Expected model type 'mistral', got '%s'", model.GetModelType())
	}

	// Verify context length
	if model.GetContextLength() != 32768 {
		t.Errorf("Expected context length 32768, got %d", model.GetContextLength())
	}

	// Verify parameter count
	paramCount := model.GetParameterCount()
	expectedCount := int64(7_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	t.Logf("Mistral model parameter count via generic loader: %s", FormatParamCount(paramCount))
}

// TestMistralConfigNoArchitectures verifies that when the Architectures field
// is missing from config.json, GetArchitecture() returns an empty string rather
// than a misleading fallback like "MistralModel" which would cause the model to
// be misclassified as an embedding model by the config parser.
// Regression test for https://github.com/ome-projects/ome/issues/601
func TestMistralConfigNoArchitectures(t *testing.T) {
	configPath := filepath.Join("testdata", "mistral_no_architectures.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mistral config without architectures: %v", err)
	}

	// GetArchitecture must return empty string when Architectures is absent
	arch := config.GetArchitecture()
	if arch != "" {
		t.Errorf("Expected empty architecture when Architectures field is missing, got '%s'", arch)
	}

	// IsEmbedding must return false for a generic Mistral model
	if config.IsEmbedding() {
		t.Error("Expected IsEmbedding() to return false for a generic Mistral model without explicit embedding architecture")
	}

	// Verify it's still recognized as a Mistral model
	if config.GetModelType() != "mistral" {
		t.Errorf("Expected model type 'mistral', got '%s'", config.GetModelType())
	}
}

// TestMistralConfigEmbedding verifies that a Mistral model with
// "architectures": ["MistralModel"] is correctly identified as an embedding
// model (e.g. intfloat/e5-mistral-7b-instruct).
// This is the load-bearing positive case for config_parser.go's EMBEDDING classification.
func TestMistralConfigEmbedding(t *testing.T) {
	configPath := filepath.Join("testdata", "mistral_embedding.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mistral embedding config: %v", err)
	}

	// GetArchitecture must return "MistralModel"
	arch := config.GetArchitecture()
	if arch != "MistralModel" {
		t.Errorf("Expected architecture 'MistralModel', got '%s'", arch)
	}

	// GetModelType must return "mistral"
	if config.GetModelType() != "mistral" {
		t.Errorf("Expected model type 'mistral', got '%s'", config.GetModelType())
	}

	// IsEmbedding must return true for MistralModel architecture
	if !config.IsEmbedding() {
		t.Error("Expected IsEmbedding() to return true for a model with 'MistralModel' architecture")
	}
}

func TestMistralInstructConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "mistral_7b_instruct.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Mistral-7B-Instruct config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "mistral" {
		t.Errorf("Expected model type 'mistral' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a MistralConfig
	mistralConfig, ok := config.(*MistralConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *MistralConfig, but got %T", config)
	}

	// Check key fields
	if mistralConfig.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", mistralConfig.HiddenSize)
	}

	if mistralConfig.NumHiddenLayers != 32 {
		t.Errorf("Expected hidden layers to be 32, but got %d", mistralConfig.NumHiddenLayers)
	}

	if mistralConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", mistralConfig.NumAttentionHeads)
	}

	if mistralConfig.NumKeyValueHeads != 8 {
		t.Errorf("Expected key-value heads to be 8, but got %d", mistralConfig.NumKeyValueHeads)
	}

	// Check context length
	contextLength := config.GetContextLength()
	expectedLength := 32768
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(7_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (unique to Mistral)
	if mistralConfig.RopeTheta != 1000000.0 {
		t.Errorf("Expected RoPE theta to be 1000000.0, but got %f", mistralConfig.RopeTheta)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Mistral-7B-Instruct, but got true")
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
