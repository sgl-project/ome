package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadKimiK2Config(t *testing.T) {
	configPath := filepath.Join("testdata", "kimi_k2_instruct.json")

	// Load the config
	config, err := LoadKimiK2Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Kimi-K2 config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "kimi_k2" {
		t.Errorf("Incorrect model type, expected 'kimi_k2', got %s", config.ModelType)
	}

	if config.HiddenSize != 7168 {
		t.Errorf("Incorrect hidden size, expected 7168, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 61 {
		t.Errorf("Incorrect number of layers, expected 61, got %d", config.NumHiddenLayers)
	}

	if config.NumRoutedExperts != 384 {
		t.Errorf("Incorrect number of experts, expected 384, got %d", config.NumRoutedExperts)
	}

	if config.VocabSize != 163840 {
		t.Errorf("Incorrect vocabulary size, expected 163840, got %d", config.VocabSize)
	}

	// Verify interface methods
	if config.GetModelType() != "kimi_k2" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "DeepseekV3ForCausalLM" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}

	// Check if parameter count is reasonable (should be around 1.5T)
	paramCount := config.GetParameterCount()
	if paramCount == 0 {
		t.Error("Parameter count should not be zero")
	}
	expectedParamCount := int64(1_500_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}
}

func TestKimiK2LoadModelConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "kimi_k2_instruct.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "kimi_k2" {
		t.Errorf("Incorrect model type, expected 'kimi_k2', got %s", config.GetModelType())
	}

	// Verify it implements the interface correctly
	_, ok := config.(*KimiK2Config)
	if !ok {
		t.Error("LoadModelConfig should return a *KimiK2Config for kimi_k2 model type")
	}
}

func TestKimiK2ModelMetadata(t *testing.T) {
	configPath := filepath.Join("testdata", "kimi_k2_instruct.json")

	// Load the config
	config, err := LoadKimiK2Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Kimi-K2 config: %v", err)
	}

	// Test model metadata
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.48.3" {
		t.Errorf("Incorrect transformer version, expected '4.48.3', got '%s'", transformerVersion)
	}

	// Check quantization type (should be fp8 based on the config)
	quantizationType := config.GetQuantizationType()
	if quantizationType != "fp8" {
		t.Errorf("Expected 'fp8' quantization type, got '%s'", quantizationType)
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
	if modelType != "kimi_k2" {
		t.Errorf("Incorrect model type, expected 'kimi_k2', got '%s'", modelType)
	}

	// Check context length
	contextLength := config.GetContextLength()
	if contextLength != 131072 {
		t.Errorf("Incorrect context length, expected 131072, got %d", contextLength)
	}

	// Verify RoPE scaling configuration
	if config.RopeScaling.Type != "yarn" {
		t.Errorf("Incorrect RoPE scaling type, expected 'yarn', got '%s'", config.RopeScaling.Type)
	}

	if config.RopeScaling.Factor != 32.0 {
		t.Errorf("Incorrect RoPE scaling factor, expected 32.0, got %f", config.RopeScaling.Factor)
	}

	// Verify MoE configuration
	if config.NumExpertsPerTok != 8 {
		t.Errorf("Incorrect num_experts_per_tok, expected 8, got %d", config.NumExpertsPerTok)
	}

	if config.MoeLayerFreq != 1 {
		t.Errorf("Incorrect moe_layer_freq, expected 1, got %d", config.MoeLayerFreq)
	}

	// Verify vision capability (should be false)
	if config.HasVision() {
		t.Error("HasVision() should return false for Kimi-K2")
	}
}
