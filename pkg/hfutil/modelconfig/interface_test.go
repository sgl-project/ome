package modelconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatParamCount(t *testing.T) {
	testCases := []struct {
		count    int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{10500, "10.5K"},
		{150000, "150K"},
		{151500, "151.5K"},
		{1000000, "1M"},
		{1500000, "1.5M"},
		{10000000, "10M"},
		{10500000, "10.5M"},
		{150000000, "150M"},
		{151500000, "151.5M"},
		{1000000000, "1B"},
		{1500000000, "1.5B"},
		{10000000000, "10B"},
		{10500000000, "10.5B"},
		{150000000000, "150B"},
		{151500000000, "151.5B"},
		{685000000000, "685B"},
		{1000000000000, "1T"},
		{1500000000000, "1.5T"},
		{1512300000000, "1.51T"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatParamCount(tc.count)
			if result != tc.expected {
				t.Errorf("FormatParamCount(%d) = %s, expected %s", tc.count, result, tc.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{999, "999 B"},
		{1000, "1000 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 10, "10.00 KB"},
		{1024*1024 - 1, "1024.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1.5, "1.50 MB"},
		{1024 * 1024 * 10, "10.00 MB"},
		{1024*1024*1024 - 1, "1024.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1.5, "1.50 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{1024 * 1024 * 1024 * 1024 * 1.5, "1.50 TB"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatSize(tc.bytes)
			if result != tc.expected {
				t.Errorf("FormatSize(%d) = %s, expected %s", tc.bytes, result, tc.expected)
			}
		})
	}
}

func TestGenericModelLoading(t *testing.T) {
	// Test cases for different model types
	testCases := []struct {
		name           string
		configFile     string
		expectedType   string
		expectedParams int64
		hasVision      bool
	}{
		{
			name:           "Llama",
			configFile:     "llama3.json",
			expectedType:   "llama",
			expectedParams: 70_000_000_000, // 70B (actual parameter count from llama3.json)
			hasVision:      false,
		},
		{
			name:           "Mistral",
			configFile:     "mistral.json",
			expectedType:   "mistral",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
		{
			name:           "Mixtral",
			configFile:     "mixtral.json",
			expectedType:   "mixtral",
			expectedParams: 46_700_000_000, // Updated to match actual model params
			hasVision:      false,
		},
		{
			name:           "DeepSeek V3",
			configFile:     "deepseek_v3.json",
			expectedType:   "deepseek_v3",
			expectedParams: 685_000_000_000, // Updated to match actual model params
			hasVision:      false,
		},
		{
			name:           "Phi-3 Vision",
			configFile:     "phi3_v.json",
			expectedType:   "phi3_v",
			expectedParams: 14_000_000_000, // 14B
			hasVision:      true,
		},
		{
			name:           "Qwen2",
			configFile:     "qwen2_7b.json",
			expectedType:   "qwen2",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
		{
			name:           "Mistral 7B Instruct",
			configFile:     "mistral_7b_instruct.json",
			expectedType:   "mistral",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join("testdata", tc.configFile)

			// Skip test if file doesn't exist
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Skipf("Skipping test for %s: file %s not found", tc.name, configPath)
				return
			}

			// Load the model configuration
			config, err := LoadModelConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load model config for %s: %v", tc.name, err)
			}

			// Check model type
			modelType := config.GetModelType()
			if modelType != tc.expectedType {
				t.Errorf("Expected model type %s but got %s", tc.expectedType, modelType)
			}

			// Check parameter count
			paramCount := config.GetParameterCount()
			if paramCount != tc.expectedParams {
				t.Errorf("Expected parameter count %d but got %d", tc.expectedParams, paramCount)
			}

			// Check vision capabilities
			hasVision := config.HasVision()
			if hasVision != tc.hasVision {
				t.Errorf("Expected HasVision() to be %v but got %v", tc.hasVision, hasVision)
			}

			// Verify that non-zero values are returned
			contextLength := config.GetContextLength()
			if contextLength <= 0 {
				t.Errorf("Expected positive context length but got %d", contextLength)
			}

			modelSize := config.GetModelSizeBytes()
			if modelSize <= 0 {
				t.Errorf("Expected positive model size but got %d", modelSize)
			}

			// Verify transformers version is not empty
			transformersVersion := config.GetTransformerVersion()
			if transformersVersion == "" {
				t.Errorf("Expected non-empty transformers version")
			}
		})
	}
}

func TestUnsupportedModelType(t *testing.T) {
	configPath := filepath.Join("testdata", "clip_vision_model.json")

	// Verify the test file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: file %s not found", configPath)
		return
	}

	// Try to load the unsupported model type
	_, err := LoadModelConfig(configPath)

	// Verify that an error was returned
	if err == nil {
		t.Fatalf("Expected an error when loading unsupported model type, but got nil")
	}

	// Check that the error message mentions the model type
	if !strings.Contains(err.Error(), "clip_vision_model") {
		t.Errorf("Error message does not mention the model type: %v", err)
	}

	// Check that the error message mentions supported types
	if !strings.Contains(err.Error(), "Supported types") {
		t.Errorf("Error message does not mention supported types: %v", err)
	}

	t.Logf("Got expected error for unsupported model: %v", err)
}
