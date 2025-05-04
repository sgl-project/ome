package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLlamaConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "llama" {
		t.Errorf("Incorrect model type, expected 'llama', got '%s'", config.GetModelType())
	}

	// Get the LlamaConfig by type assertion
	llamaConfig, ok := config.(*LlamaConfig)
	if !ok {
		t.Fatalf("Failed to convert to LlamaConfig")
	}

	if llamaConfig.HiddenSize != 8192 {
		t.Errorf("Incorrect hidden size, expected 8192, got %d", llamaConfig.HiddenSize)
	}

	if llamaConfig.NumHiddenLayers != 80 {
		t.Errorf("Incorrect hidden layers, expected 80, got %d", llamaConfig.NumHiddenLayers)
	}

	if llamaConfig.MaxPositionEmbeddings != 8192 {
		t.Errorf("Incorrect context length, expected 8192, got %d", llamaConfig.MaxPositionEmbeddings)
	}

	// Test parameter count (should be 70B for both Llama-3-70B and Llama-3.1-70B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(70_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Incorrect parameter count, expected %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	// Test GetModelSizeBytes
	modelSize := config.GetModelSizeBytes()
	// Expected size for bfloat16 (2 bytes per parameter)
	expectedSize := int64(70_000_000_000 * 2)
	if modelSize != expectedSize {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSize), FormatSize(modelSize))
	}
}

func TestLlama31Config(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3_1.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama 3.1 config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "llama" {
		t.Errorf("Incorrect model type, expected 'llama', got '%s'", config.GetModelType())
	}

	// Get the LlamaConfig by type assertion
	llamaConfig, ok := config.(*LlamaConfig)
	if !ok {
		t.Fatalf("Failed to convert to LlamaConfig")
	}

	if llamaConfig.HiddenSize != 8192 {
		t.Errorf("Incorrect hidden size, expected 8192, got %d", llamaConfig.HiddenSize)
	}

	if llamaConfig.NumHiddenLayers != 80 {
		t.Errorf("Incorrect hidden layers, expected 80, got %d", llamaConfig.NumHiddenLayers)
	}

	// Llama 3.1 has extended context
	if llamaConfig.MaxPositionEmbeddings != 131072 {
		t.Errorf("Incorrect context length, expected 131072, got %d", llamaConfig.MaxPositionEmbeddings)
	}

	// Verify rope scaling configuration exists for Llama 3.1
	if llamaConfig.RopeScaling.RopeType == "" {
		t.Errorf("Expected RopeScaling.RopeType to be present for Llama 3.1")
	} else {
		if llamaConfig.RopeScaling.RopeType != "llama3" {
			t.Errorf("Incorrect rope scaling type, expected 'llama3', got '%s'", llamaConfig.RopeScaling.RopeType)
		}

		if llamaConfig.RopeScaling.Factor != 8.0 {
			t.Errorf("Incorrect rope scaling factor, expected 8.0, got %f", llamaConfig.RopeScaling.Factor)
		}
	}

	// Test parameter count (should be 70B for both Llama-3-70B and Llama-3.1-70B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(70_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Incorrect parameter count, expected %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	// Test GetModelSizeBytes
	modelSize := config.GetModelSizeBytes()
	// Expected size for bfloat16 (2 bytes per parameter)
	expectedSize := int64(70_000_000_000 * 2)
	if modelSize != expectedSize {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSize), FormatSize(modelSize))
	}
}

func TestLlama31_405BConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3_1_405b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama-3.1-405B config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "llama" {
		t.Errorf("Expected model type 'llama' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a LlamaConfig
	llamaConfig, ok := config.(*LlamaConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *LlamaConfig, but got %T", config)
	}

	// Check vision capability
	if config.HasVision() {
		t.Error("Expected HasVision to return false, but got true")
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	expectedCount := int64(405_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check key fields
	if llamaConfig.HiddenSize != 16384 {
		t.Errorf("Expected hidden size to be 16384, but got %d", llamaConfig.HiddenSize)
	}

	if llamaConfig.NumHiddenLayers != 126 {
		t.Errorf("Expected hidden layers to be 126, but got %d", llamaConfig.NumHiddenLayers)
	}

	// Check context length
	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length to be 131072, but got %d", config.GetContextLength())
	}

	// Check RoPE scaling
	if llamaConfig.RopeScaling.RopeType != "llama3" {
		t.Errorf("Expected RoPE scaling type to be 'llama3', but got '%s'", llamaConfig.RopeScaling.RopeType)
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}

	// Check quantization config
	if llamaConfig.QuantizationConfig == nil || llamaConfig.QuantizationConfig.QuantMethod != "fbgemm_fp8" {
		t.Errorf("Expected quantization method to be 'fbgemm_fp8', but got '%s'",
			config.GetQuantizationType())
	}
}

func TestLoadLlama32Models(t *testing.T) {
	tests := []struct {
		name               string
		configFile         string
		expectedHiddenSize int
		expectedLayers     int
		expectedParamCount int64
	}{
		{
			name:               "Llama-3.2-1B",
			configFile:         "llama3_2_1b.json",
			expectedHiddenSize: 2048,
			expectedLayers:     16,
			expectedParamCount: 1_000_000_000,
		},
		{
			name:               "Llama-3.2-3B",
			configFile:         "llama3_2_3b.json",
			expectedHiddenSize: 3072,
			expectedLayers:     28,
			expectedParamCount: 3_000_000_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join("testdata", tc.configFile)

			// Load the config
			config, err := LoadModelConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load %s config: %v", tc.name, err)
			}

			// Check that it's the correct model type
			if config.GetModelType() != "llama" {
				t.Errorf("Expected model type 'llama' but got '%s'", config.GetModelType())
			}

			// Check that it's parsed as a LlamaConfig
			llamaConfig, ok := config.(*LlamaConfig)
			if !ok {
				t.Fatalf("Expected config to be of type *LlamaConfig, but got %T", config)
			}

			// Check vision capability
			if config.HasVision() {
				t.Error("Expected HasVision to return false, but got true")
			}

			// Check parameter count
			paramCount := config.GetParameterCount()
			if paramCount != tc.expectedParamCount {
				t.Errorf("Expected parameter count to be %d, but got %d", tc.expectedParamCount, paramCount)
			}

			// Check key fields
			if llamaConfig.HiddenSize != tc.expectedHiddenSize {
				t.Errorf("Expected hidden size to be %d, but got %d", tc.expectedHiddenSize, llamaConfig.HiddenSize)
			}

			if llamaConfig.NumHiddenLayers != tc.expectedLayers {
				t.Errorf("Expected hidden layers to be %d, but got %d", tc.expectedLayers, llamaConfig.NumHiddenLayers)
			}

			// Verify context length
			expectedContext := 131072
			if config.GetContextLength() != expectedContext {
				t.Errorf("Expected context length to be %d, but got %d", expectedContext, config.GetContextLength())
			}

			// Check model size bytes (should be non-zero)
			if config.GetModelSizeBytes() <= 0 {
				t.Errorf("Expected model size bytes to be positive, but got %d", config.GetModelSizeBytes())
			}

			// Check RoPE scaling for Llama-3.2 (specific to this version)
			if llamaConfig.RopeScaling.RopeType != "llama3" {
				t.Errorf("Expected RoPE scaling type to be 'llama3', but got '%s'", llamaConfig.RopeScaling.RopeType)
			}
		})
	}
}

func TestLoadMLlamaConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3_2_11b_vision.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load MLlama config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "mllama" {
		t.Errorf("Expected model type 'mllama' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as an MLlamaConfig
	mllamaConfig, ok := config.(*MLlamaConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *MLlamaConfig, but got %T", config)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true, but got false")
	}

	// Check parameter count (should be approximately 11B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(11_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check some key fields
	if mllamaConfig.TextConfig.HiddenSize != 4096 {
		t.Errorf("Expected text hidden size to be 4096, but got %d", mllamaConfig.TextConfig.HiddenSize)
	}

	if mllamaConfig.TextConfig.NumHiddenLayers != 40 {
		t.Errorf("Expected text hidden layers to be 40, but got %d", mllamaConfig.TextConfig.NumHiddenLayers)
	}

	if mllamaConfig.VisionConfig.HiddenSize != 1280 {
		t.Errorf("Expected vision hidden size to be 1280, but got %d", mllamaConfig.VisionConfig.HiddenSize)
	}

	if mllamaConfig.VisionConfig.NumHiddenLayers != 32 {
		t.Errorf("Expected vision hidden layers to be 32, but got %d", mllamaConfig.VisionConfig.NumHiddenLayers)
	}

	// Check context length
	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length to be 131072, but got %d", config.GetContextLength())
	}

	// Check model size bytes (should be non-zero)
	if config.GetModelSizeBytes() <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", config.GetModelSizeBytes())
	}
}

func TestLlama32VisionModel(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3_2_11b_vision.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama-3.2-11B-Vision config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "mllama" {
		t.Errorf("Expected model type 'mllama' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as an MLlamaConfig
	_, ok := config.(*MLlamaConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *MLlamaConfig, but got %T", config)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true, but got false")
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	expectedCount := int64(11_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check model size bytes (should be non-zero)
	if config.GetModelSizeBytes() <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", config.GetModelSizeBytes())
	}
}

func TestLlama32_90BVisionConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "llama3_2_90b_vision.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama-3.2-90B-Vision config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "mllama" {
		t.Errorf("Expected model type 'mllama' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as an MLlamaConfig
	mllamaConfig, ok := config.(*MLlamaConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *MLlamaConfig, but got %T", config)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true, but got false")
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	expectedCount := int64(90_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check text model key fields
	if mllamaConfig.TextConfig.HiddenSize != 8192 {
		t.Errorf("Expected text hidden size to be 8192, but got %d", mllamaConfig.TextConfig.HiddenSize)
	}

	if mllamaConfig.TextConfig.NumHiddenLayers != 100 {
		t.Errorf("Expected text hidden layers to be 100, but got %d", mllamaConfig.TextConfig.NumHiddenLayers)
	}

	// Check vision model key fields
	if mllamaConfig.VisionConfig.HiddenSize != 1280 {
		t.Errorf("Expected vision hidden size to be 1280, but got %d", mllamaConfig.VisionConfig.HiddenSize)
	}

	if mllamaConfig.VisionConfig.NumHiddenLayers != 32 {
		t.Errorf("Expected vision hidden layers to be 32, but got %d", mllamaConfig.VisionConfig.NumHiddenLayers)
	}

	// Check context length
	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length to be 131072, but got %d", config.GetContextLength())
	}

	// Check RoPE scaling
	if mllamaConfig.TextConfig.RopeScaling.RopeType != "llama3" {
		t.Errorf("Expected RoPE scaling type to be 'llama3', but got '%s'", mllamaConfig.TextConfig.RopeScaling.RopeType)
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}

func TestLlama4Config(t *testing.T) {
	configPath := filepath.Join("testdata", "llama4.json")

	// Load the config directly through LoadModelConfig
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama4 config: %v", err)
	}

	// Check basic fields
	if config.GetModelType() != "llama4" {
		t.Errorf("Incorrect model type, expected 'llama4', got '%s'", config.GetModelType())
	}

	// Get the Llama4Config by type assertion
	llama4Config, ok := config.(*Llama4Config)
	if !ok {
		t.Fatalf("Failed to convert to Llama4Config")
	}

	if llama4Config.TextConfig.HiddenSize != 5120 {
		t.Errorf("Incorrect hidden size, expected 5120, got %d", llama4Config.TextConfig.HiddenSize)
	}

	if llama4Config.TextConfig.NumHiddenLayers != 48 {
		t.Errorf("Incorrect hidden layers, expected 48, got %d", llama4Config.TextConfig.NumHiddenLayers)
	}

	if llama4Config.TextConfig.MaxPositionEmbeddings != 1048576 {
		t.Errorf("Incorrect context length, expected 1048576, got %d", llama4Config.TextConfig.MaxPositionEmbeddings)
	}

	// Test MoE parameters
	if llama4Config.TextConfig.NumLocalExperts != 128 {
		t.Errorf("Incorrect number of experts, expected 128, got %d", llama4Config.TextConfig.NumLocalExperts)
	}

	// Test quantization config
	if llama4Config.QuantizationConfig.Format != "float-quantized" {
		t.Errorf("Incorrect quantization format, expected 'float-quantized', got '%s'",
			llama4Config.QuantizationConfig.Format)
	}

	// Test parameter count (should return 402B for the Maverick model)
	paramCount := config.GetParameterCount()
	expectedCount := int64(402_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Incorrect parameter count, expected %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	// Test GetModelSizeBytes
	modelSize := config.GetModelSizeBytes()
	// For FP8 models, we use 1 byte per parameter
	expectedSize := int64(402_000_000_000)
	if modelSize != expectedSize {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSize), FormatSize(modelSize))
	}

	// Test the quantization detection
	dtype := config.GetTorchDtype()
	if dtype != "float8" {
		t.Errorf("Incorrect torch dtype, expected 'float8', got '%s'", dtype)
	}

	// Test vision capability detection
	hasVision := config.HasVision()
	if !hasVision {
		t.Errorf("Expected Llama4 model to have vision capabilities, but HasVision returned false")
	}

	// Check if model has vision capabilities (through direct field access)
	if llama4Config.VisionConfig == nil {
		t.Errorf("Expected Llama4 model to have VisionConfig, but it was nil")
	}
}

func TestLoadModelWithLlama4(t *testing.T) {
	configPath := filepath.Join("testdata", "llama4.json")

	// Test loading through the generic loader
	model, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama4 model through generic loader: %v", err)
	}

	// Verify it's a Llama4 model
	if model.GetModelType() != "llama4" {
		t.Errorf("Expected model type 'llama4', got '%s'", model.GetModelType())
	}

	// Verify context length
	if model.GetContextLength() != 1048576 {
		t.Errorf("Expected context length 1048576, got %d", model.GetContextLength())
	}

	// Verify parameter count
	paramCount := model.GetParameterCount()
	expectedCount := int64(402_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count %s, got %s",
			FormatParamCount(expectedCount), FormatParamCount(paramCount))
	}

	t.Logf("Llama4 model parameter count via generic loader: %s", FormatParamCount(paramCount))
}

func TestLlama4ScoutConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "llama4_scout_17b_16e.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Llama-4-Scout configuration: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "llama4" {
		t.Errorf("Expected model type 'llama4' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Llama4Config
	llama4Config, ok := config.(*Llama4Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Llama4Config, but got %T", config)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true for Llama-4-Scout, but got false")
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	expectedCount := int64(17_000_000_000)
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check MoE configuration
	if llama4Config.TextConfig.NumLocalExperts != 16 {
		t.Errorf("Expected 16 experts, but got %d", llama4Config.TextConfig.NumLocalExperts)
	}

	// Check key model dimensions
	if llama4Config.TextConfig.HiddenSize != 5120 {
		t.Errorf("Expected hidden size to be 5120, but got %d", llama4Config.TextConfig.HiddenSize)
	}

	if llama4Config.TextConfig.NumHiddenLayers != 48 {
		t.Errorf("Expected 48 hidden layers, but got %d", llama4Config.TextConfig.NumHiddenLayers)
	}

	// Check context length
	if config.GetContextLength() != 10485760 {
		t.Errorf("Expected context length to be 10485760, but got %d", config.GetContextLength())
	}

	// Check RoPE scaling
	if llama4Config.TextConfig.RopeScaling.RopeType != "llama3" {
		t.Errorf("Expected RoPE scaling type to be 'llama3', but got '%s'",
			llama4Config.TextConfig.RopeScaling.RopeType)
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}
