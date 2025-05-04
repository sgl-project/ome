package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadDeepseekV3Config(t *testing.T) {
	configPath := filepath.Join("testdata", "deepseek_v3.json")

	// Load the config
	config, err := LoadDeepseekV3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load DeepSeek V3 config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "deepseek_v3" {
		t.Errorf("Incorrect model type, expected 'deepseek_v3', got %s", config.ModelType)
	}

	if config.HiddenSize != 7168 {
		t.Errorf("Incorrect hidden size, expected 7168, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 61 {
		t.Errorf("Incorrect number of layers, expected 61, got %d", config.NumHiddenLayers)
	}

	if config.NumRoutedExperts != 256 {
		t.Errorf("Incorrect number of experts, expected 256, got %d", config.NumRoutedExperts)
	}

	if config.VocabSize != 129280 {
		t.Errorf("Incorrect vocabulary size, expected 129280, got %d", config.VocabSize)
	}

	// Verify interface methods
	if config.GetModelType() != "deepseek_v3" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "DeepseekV3ForCausalLM" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 163840 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}

	// Since we don't have actual safetensors files for testing,
	// we expect the fallback formula to calculate a parameter count
	// or the hardcoded 685B for DeepSeek V3
	paramCount := config.GetParameterCount()
	if paramCount == 0 {
		t.Error("Parameter count should not be zero")
	}

	// For DeepSeek V3, we expect it to return the official 685B parameter count
	expectedParamCount := int64(685_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}
}

func TestLoadModelConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "deepseek_v3.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "deepseek_v3" {
		t.Errorf("Incorrect model type, expected 'deepseek_v3', got %s", config.GetModelType())
	}

	// Verify parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(685_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}
}

func TestDeepseekV3ModelMetadata(t *testing.T) {
	configPath := filepath.Join("testdata", "deepseek_v3.json")

	// Load the config
	config, err := LoadDeepseekV3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load DeepSeek V3 config: %v", err)
	}

	// Test parameter count
	paramCount := config.GetParameterCount()
	if paramCount != 685_000_000_000 {
		t.Errorf("Incorrect parameter count, expected 685B, got %d", paramCount)
	}

	// Test model size
	modelSizeBytes := config.GetModelSizeBytes()
	expectedSizeBytes := int64(685_000_000_000) * 2 // bfloat16 is 2 bytes per parameter
	if modelSizeBytes != expectedSizeBytes {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSizeBytes), FormatSize(modelSizeBytes))
	}

	// Test model metadata
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.46.3" {
		t.Errorf("Incorrect transformer version, expected '4.46.3', got '%s'", transformerVersion)
	}

	// Check quantization type (should be empty since this test data doesn't have quantization config)
	quantizationType := config.GetQuantizationType()
	if quantizationType != "" {
		t.Errorf("Expected empty quantization type for non-quantized test model, got '%s'", quantizationType)
	}

	// Check data type
	torchDtype := config.GetTorchDtype()
	if torchDtype != "bfloat16" {
		t.Errorf("Incorrect torch dtype, expected 'bfloat16', got '%s'", torchDtype)
	}

	// Check architecture
	architecture := config.GetArchitecture()
	if architecture != "DeepseekV3ForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'DeepseekV3ForCausalLM', got '%s'", architecture)
	}

	// Check model type
	modelType := config.GetModelType()
	if modelType != "deepseek_v3" {
		t.Errorf("Incorrect model type, expected 'deepseek_v3', got '%s'", modelType)
	}

	// Check context length
	contextLength := config.GetContextLength()
	if contextLength != 163840 {
		t.Errorf("Incorrect context length, expected 163840, got %d", contextLength)
	}

}
