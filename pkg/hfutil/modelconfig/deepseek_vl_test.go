package modelconfig

import (
	"testing"
)

func TestLoadDeepSeekVLConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
		hasVision    bool
	}{
		{
			name:         "DeepSeek VL2 Tiny",
			configPath:   "testdata/deepseek_vl2_tiny.json",
			expectedType: "deepseek_vl_v2",
			minParams:    500_000_000,
			maxParams:    1_500_000_000,
			hasVision:    true,
		},
		{
			name:         "Janus 1.3B",
			configPath:   "testdata/janus_1.3b.json",
			expectedType: "multi_modality",
			minParams:    1_000_000_000,
			maxParams:    1_500_000_000,
			hasVision:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadDeepSeekVLConfig(tt.configPath)
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

func TestDeepSeekVLConfigInterface(t *testing.T) {
	// Test that DeepSeekVLConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*DeepSeekVLConfig)(nil)
}
