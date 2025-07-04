package modelconfig

import (
	"testing"
)

func TestLoadLLaVAConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
		hasVision    bool
	}{
		{
			name:         "LLaVA 1.5 7B HF",
			configPath:   "testdata/llava_1.5_7b_hf.json",
			expectedType: "llava",
			minParams:    7_000_000_000,
			maxParams:    8_000_000_000,
			hasVision:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadLLaVAConfig(tt.configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Test model type
			if config.ModelType != tt.expectedType {
				t.Errorf("Expected model type %s, got %s", tt.expectedType, config.ModelType)
			}

			// Test parameter count
			paramCount := config.GetParameterCount()
			if paramCount < tt.minParams || paramCount > tt.maxParams {
				t.Errorf("Parameter count %d is outside expected range [%d, %d]",
					paramCount, tt.minParams, tt.maxParams)
			}

			// Test vision capability
			if config.HasVision() != tt.hasVision {
				t.Errorf("Expected HasVision() to return %v, got %v", tt.hasVision, config.HasVision())
			}

			// Test interface methods
			if config.GetArchitecture() == "" {
				t.Error("GetArchitecture() returned empty string")
			}
			if config.GetContextLength() <= 0 {
				t.Error("GetContextLength() returned non-positive value")
			}
			if config.GetModelSizeBytes() <= 0 {
				t.Error("GetModelSizeBytes() returned non-positive value")
			}
		})
	}
}

func TestLLaVAConfigInterface(t *testing.T) {
	// Test that LLaVAConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*LLaVAConfig)(nil)
}
