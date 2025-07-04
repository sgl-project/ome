package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestDBRXConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "dbrx_132b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load DBRX config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "dbrx" {
		t.Errorf("Incorrect model type, expected 'dbrx', got '%s'", config.GetModelType())
	}

	// Get the DBRXConfig by type assertion
	dbrxConfig, ok := config.(*DBRXConfig)
	if !ok {
		t.Fatalf("Failed to convert to DBRXConfig")
	}

	// Test specific fields
	if dbrxConfig.DModel != 6144 {
		t.Errorf("Incorrect d_model, expected 6144, got %d", dbrxConfig.DModel)
	}

	if dbrxConfig.NLayers != 48 {
		t.Errorf("Incorrect n_layers, expected 48, got %d", dbrxConfig.NLayers)
	}

	if dbrxConfig.MaxSeqLen != 32768 {
		t.Errorf("Incorrect max_seq_len, expected 32768, got %d", dbrxConfig.MaxSeqLen)
	}

	// Test architecture
	if config.GetArchitecture() != "DbrxForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'DbrxForCausalLM', got '%s'", config.GetArchitecture())
	}

	// Test parameter count (approximate for DBRX 132B)
	paramCount := config.GetParameterCount()
	expectedMin := int64(130_000_000_000)
	expectedMax := int64(135_000_000_000)
	if paramCount < expectedMin || paramCount > expectedMax {
		t.Errorf("Parameter count %d is outside expected range [%d, %d]",
			paramCount, expectedMin, expectedMax)
	}

	// Verify other interface methods
	if config.HasVision() {
		t.Errorf("DBRX should not have vision capabilities")
	}

	if config.GetTorchDtype() != "bfloat16" {
		t.Errorf("Expected torch dtype bfloat16, got %s", config.GetTorchDtype())
	}

	if config.GetQuantizationType() != "" {
		t.Errorf("Expected no quantization, got %s", config.GetQuantizationType())
	}

	// Verify MoE configuration
	if dbrxConfig.FFNConfig.MoENExperts != 16 {
		t.Errorf("Expected 16 experts, got %d", dbrxConfig.FFNConfig.MoENExperts)
	}
	if dbrxConfig.FFNConfig.MoETopK != 4 {
		t.Errorf("Expected top-k 4, got %d", dbrxConfig.FFNConfig.MoETopK)
	}
}

func TestDBRXAutoDetection(t *testing.T) {
	configPath := filepath.Join("testdata", "dbrx_132b.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to auto-detect and load config: %v", err)
	}

	if _, ok := config.(*DBRXConfig); !ok {
		t.Errorf("Expected DBRXConfig type, got %T", config)
	}
}
