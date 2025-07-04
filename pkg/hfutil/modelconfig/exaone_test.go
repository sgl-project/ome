package modelconfig

import (
	"testing"
)

func TestLoadExaoneConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
	}{
		{
			name:         "ExaONE 3.5 7.8B",
			configPath:   "testdata/exaone_7.8b.json",
			expectedType: "exaone",
			minParams:    7_000_000_000,
			maxParams:    8_500_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadExaoneConfig(tt.configPath)
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
			if config.HasVision() {
				t.Error("HasVision() should return false for ExaONE models")
			}
		})
	}
}

func TestExaoneConfigInterface(t *testing.T) {
	// Test that ExaoneConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*ExaoneConfig)(nil)
}
