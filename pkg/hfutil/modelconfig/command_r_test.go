package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestCommandRConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "command_r_35b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Command-R config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "cohere" {
		t.Errorf("Incorrect model type, expected 'cohere', got '%s'", config.GetModelType())
	}

	// Get the CommandRConfig by type assertion
	commandRConfig, ok := config.(*CommandRConfig)
	if !ok {
		t.Fatalf("Failed to convert to CommandRConfig")
	}

	// Test specific fields
	if commandRConfig.HiddenSize != 8192 {
		t.Errorf("Incorrect hidden size, expected 8192, got %d", commandRConfig.HiddenSize)
	}

	if commandRConfig.NumHiddenLayers != 40 {
		t.Errorf("Incorrect hidden layers, expected 40, got %d", commandRConfig.NumHiddenLayers)
	}

	if commandRConfig.MaxPositionEmbeddings != 131072 {
		t.Errorf("Incorrect context length, expected 131072, got %d", commandRConfig.MaxPositionEmbeddings)
	}

	// Test architecture
	if config.GetArchitecture() != "CohereForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'CohereForCausalLM', got '%s'", config.GetArchitecture())
	}

	// Test parameter count (approximate for Command-R 35B)
	paramCount := config.GetParameterCount()
	expectedMin := int64(34_000_000_000)
	expectedMax := int64(36_000_000_000)
	if paramCount < expectedMin || paramCount > expectedMax {
		t.Errorf("Parameter count %d is outside expected range [%d, %d]",
			paramCount, expectedMin, expectedMax)
	}

	// Verify other interface methods
	if config.HasVision() {
		t.Errorf("Command-R should not have vision capabilities")
	}

	if config.GetTorchDtype() != "bfloat16" {
		t.Errorf("Expected torch dtype bfloat16, got %s", config.GetTorchDtype())
	}

	if config.GetQuantizationType() != "" {
		t.Errorf("Expected no quantization, got %s", config.GetQuantizationType())
	}
}

func TestCommandRAutoDetection(t *testing.T) {
	configPath := filepath.Join("testdata", "command_r_35b.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to auto-detect and load config: %v", err)
	}

	if _, ok := config.(*CommandRConfig); !ok {
		t.Errorf("Expected CommandRConfig type, got %T", config)
	}
}
