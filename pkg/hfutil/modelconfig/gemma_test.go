package modelconfig

import (
	"testing"
)

func TestLoadGemmaConfig(t *testing.T) {
	tests := []struct {
		name         string
		configPath   string
		expectedType string
		minParams    int64
		maxParams    int64
	}{
		{
			name:         "Gemma 7B",
			configPath:   "testdata/gemma_7b.json",
			expectedType: "gemma",
			minParams:    6_500_000_000,
			maxParams:    8_000_000_000,
		},
		{
			name:         "Gemma2 2B",
			configPath:   "testdata/gemma_2_2b.json",
			expectedType: "gemma2",
			minParams:    2_000_000_000,
			maxParams:    3_000_000_000,
		},
		{
			name:         "Gemma2 9B",
			configPath:   "testdata/gemma_2_9b.json",
			expectedType: "gemma2",
			minParams:    8_000_000_000,
			maxParams:    10_000_000_000,
		},
		{
			name:         "Gemma3 Text",
			configPath:   "testdata/gemma3_text.json",
			expectedType: "gemma3_text",
			minParams:    800_000_000,
			maxParams:    1_500_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadGemmaConfig(tt.configPath)
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
				t.Error("HasVision() should return false for Gemma models")
			}
		})
	}
}

func TestGemmaConfigInterface(t *testing.T) {
	// Test that GemmaConfig implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*GemmaConfig)(nil)
}

func TestGemma3TextSpecificFields(t *testing.T) {
	config, err := LoadGemmaConfig("testdata/gemma3_text.json")
	if err != nil {
		t.Fatalf("Failed to load gemma3_text config: %v", err)
	}

	// Verify gemma3_text-specific fields
	if config.RopeLocalBaseFreq != 10000 {
		t.Errorf("Expected rope_local_base_freq to be 10000, got %f", config.RopeLocalBaseFreq)
	}

	if config.SlidingWindowPattern != 6 {
		t.Errorf("Expected sliding_window_pattern to be 6, got %d", config.SlidingWindowPattern)
	}

	if config.SlidingWindow != 512 {
		t.Errorf("Expected sliding_window to be 512, got %d", config.SlidingWindow)
	}

	if config.QueryPreAttnScalar != 256 {
		t.Errorf("Expected query_pre_attn_scalar to be 256, got %d", config.QueryPreAttnScalar)
	}
}
