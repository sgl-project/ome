package modelconfig

import (
	"testing"
)

func TestLoadChatGLMConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
	}{
		{
			name:         "ChatGLM3 6B",
			configPath:   "testdata/chatglm3_6b.json",
			expectedType: "chatglm",
			minParams:    5_500_000_000,
			maxParams:    6_500_000_000,
		},
		{
			name:         "GLM-4 9B",
			configPath:   "testdata/glm4_9b.json",
			expectedType: "chatglm",
			minParams:    8_500_000_000,
			maxParams:    9_500_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadChatGLMConfig(tt.configPath)
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
				t.Error("HasVision() should return false for ChatGLM models")
			}
		})
	}
}

func TestChatGLMConfigInterface(t *testing.T) {
	// Test that ChatGLMConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*ChatGLMConfig)(nil)
}
