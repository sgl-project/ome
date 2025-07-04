package modelconfig

import (
	"testing"
)

func TestLoadQwen2VLConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
		hasVision    bool
	}{
		{
			name:         "Qwen2-VL 2B",
			configPath:   "testdata/qwen2_vl_2b.json",
			expectedType: "qwen2_vl",
			minParams:    1_500_000_000,
			maxParams:    2_500_000_000,
			hasVision:    true,
		},
		{
			name:         "Qwen2-VL 7B",
			configPath:   "testdata/qwen2_vl_7b.json",
			expectedType: "qwen2_vl",
			minParams:    6_500_000_000,
			maxParams:    8_000_000_000,
			hasVision:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadQwen2VLConfig(tt.configPath)
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

func TestQwen2VLConfigInterface(t *testing.T) {
	// Test that Qwen2VLConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*Qwen2VLConfig)(nil)
}
