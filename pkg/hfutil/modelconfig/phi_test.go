package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadPhiModelConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "tiny-random-PhiModel", "config.json")

	// Load the config
	config, err := LoadPhiModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi model config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "phi" {
		t.Errorf("Incorrect model type, expected 'phi', got %s", config.ModelType)
	}

	if config.HiddenSize != 32 {
		t.Errorf("Incorrect hidden size, expected 32, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 2 {
		t.Errorf("Incorrect number of layers, expected 2, got %d", config.NumHiddenLayers)
	}

	if config.NumAttentionHeads != 4 {
		t.Errorf("Incorrect number of attention heads, expected 4, got %d", config.NumAttentionHeads)
	}

	if config.VocabSize != 1024 {
		t.Errorf("Incorrect vocabulary size, expected 1024, got %d", config.VocabSize)
	}

	if config.IntermediateSize != 37 {
		t.Errorf("Incorrect intermediate size, expected 37, got %d", config.IntermediateSize)
	}

	// Verify interface methods
	if config.GetModelType() != "phi" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "PhiModel" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 512 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}
}

func TestPhiModelParameterCount(t *testing.T) {
	configPath := filepath.Join("testdata", "tiny-random-PhiModel", "config.json")

	// Load the config
	config, err := LoadPhiModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi model config: %v", err)
	}

	// Get parameter count from safetensors file
	paramCount := config.GetParameterCount()

	// Expected parameter count from multiple safetensors files (2 files * 47,000 parameters each)
	expectedCount := int64(94000)
	tolerancePercent := 5 // Allow for 5% tolerance

	minAcceptable := expectedCount * (100 - int64(tolerancePercent)) / 100
	maxAcceptable := expectedCount * (100 + int64(tolerancePercent)) / 100

	if paramCount < minAcceptable || paramCount > maxAcceptable {
		t.Errorf("Parameter count should be approximately %d (±%d%%), got %d",
			expectedCount, tolerancePercent, paramCount)
	}

	t.Logf("Phi model parameter count: %d (expected ~%d)", paramCount, expectedCount)
}

func TestLoadPhiModelViaGenericLoader(t *testing.T) {
	configPath := filepath.Join("testdata", "tiny-random-PhiModel", "config.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "phi" {
		t.Errorf("Incorrect model type, expected 'phi', got %s", config.GetModelType())
	}

	// Verify parameter count works
	paramCount := config.GetParameterCount()
	if paramCount <= 0 {
		t.Errorf("Parameter count should be positive, got %d", paramCount)
	}

	t.Logf("Phi model parameter count via generic loader: %d", paramCount)
}

func TestPhiModelSize(t *testing.T) {
	configPath := filepath.Join("testdata", "tiny-random-PhiModel", "config.json")

	// Load the config
	config, err := LoadPhiModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi model config: %v", err)
	}

	// Get parameter count from safetensors file
	paramCount := config.GetParameterCount()

	// Check the parameter count
	expectedCount := int64(92566)
	tolerancePercent := 5 // Allow for 5% tolerance

	minAcceptable := expectedCount * (100 - int64(tolerancePercent)) / 100
	maxAcceptable := expectedCount * (100 + int64(tolerancePercent)) / 100

	if paramCount < minAcceptable || paramCount > maxAcceptable {
		t.Errorf("Parameter count should be approximately %d (±%d%%), got %d",
			expectedCount, tolerancePercent, paramCount)
	}

	// Get model size in bytes
	modelSizeBytes := config.GetModelSizeBytes()

	// Expected model size (based on float32 by default for this model)
	// 47,000 parameters * 4 bytes per parameter = ~188,000 bytes
	expectedSizeBytes := expectedCount * 4

	minSizeAcceptable := expectedSizeBytes * (100 - int64(tolerancePercent)) / 100
	maxSizeAcceptable := expectedSizeBytes * (100 + int64(tolerancePercent)) / 100

	if modelSizeBytes < minSizeAcceptable || modelSizeBytes > maxSizeAcceptable {
		t.Errorf("Model size should be approximately %s (±%d%%), got %s",
			FormatSize(expectedSizeBytes), tolerancePercent, FormatSize(modelSizeBytes))
	}
}

func TestPhiModelMetadata(t *testing.T) {
	configPath := filepath.Join("testdata", "tiny-random-PhiModel", "config.json")

	// Load the config
	config, err := LoadPhiModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi model config: %v", err)
	}

	// Test model metadata
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.40.0.dev0" {
		t.Errorf("Incorrect transformer version, expected '4.40.0.dev0', got '%s'", transformerVersion)
	}

	// Check quantization type (should be empty for this model)
	quantizationType := config.GetQuantizationType()
	// This tiny test model doesn't have quantization, so it should be empty
	if quantizationType != "" {
		t.Errorf("Expected empty quantization type for non-quantized model, got '%s'", quantizationType)
	}

	// Check data type
	torchDtype := config.GetTorchDtype()
	if torchDtype != "float32" {
		t.Errorf("Incorrect torch dtype, expected 'float32', got '%s'", torchDtype)
	}

	// Check architecture
	architecture := config.GetArchitecture()
	if architecture != "PhiModel" {
		t.Errorf("Incorrect architecture, expected 'PhiModel', got '%s'", architecture)
	}

	// Check model type
	modelType := config.GetModelType()
	if modelType != "phi" {
		t.Errorf("Incorrect model type, expected 'phi', got '%s'", modelType)
	}

	// Check context length
	contextLength := config.GetContextLength()
	if contextLength != 512 {
		t.Errorf("Incorrect context length, expected 512, got %d", contextLength)
	}
}
