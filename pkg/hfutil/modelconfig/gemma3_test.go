package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadGemma3Config(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	// Load the config
	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "gemma3" {
		t.Errorf("Expected model type 'gemma3' but got '%s'", config.GetModelType())
	}
}

func TestGemma3ConfigTextFields(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check text config key fields
	if config.TextConfig.HiddenSize != 5376 {
		t.Errorf("Expected text hidden size to be 5376, but got %d", config.TextConfig.HiddenSize)
	}

	if config.TextConfig.NumHiddenLayers != 62 {
		t.Errorf("Expected text hidden layers to be 62, but got %d", config.TextConfig.NumHiddenLayers)
	}

	if config.TextConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected num attention heads to be 32, but got %d", config.TextConfig.NumAttentionHeads)
	}

	if config.TextConfig.NumKeyValueHeads != 16 {
		t.Errorf("Expected num key value heads to be 16, but got %d", config.TextConfig.NumKeyValueHeads)
	}

	if config.TextConfig.HeadDim != 128 {
		t.Errorf("Expected head dim to be 128, but got %d", config.TextConfig.HeadDim)
	}

	if config.TextConfig.QueryPreAttnScalar != 168 {
		t.Errorf("Expected query pre attn scalar to be 168, but got %d", config.TextConfig.QueryPreAttnScalar)
	}

	if config.TextConfig.SlidingWindow != 1024 {
		t.Errorf("Expected sliding window to be 1024, but got %d", config.TextConfig.SlidingWindow)
	}
}

func TestGemma3ConfigVisionFields(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check vision config key fields
	if config.VisionConfig.HiddenSize != 1152 {
		t.Errorf("Expected vision hidden size to be 1152, but got %d", config.VisionConfig.HiddenSize)
	}

	if config.VisionConfig.NumHiddenLayers != 27 {
		t.Errorf("Expected vision hidden layers to be 27, but got %d", config.VisionConfig.NumHiddenLayers)
	}

	if config.VisionConfig.ImageSize != 896 {
		t.Errorf("Expected image size to be 896, but got %d", config.VisionConfig.ImageSize)
	}

	if config.VisionConfig.PatchSize != 14 {
		t.Errorf("Expected patch size to be 14, but got %d", config.VisionConfig.PatchSize)
	}

	if config.VisionConfig.VisionUseHead != false {
		t.Errorf("Expected vision_use_head to be false, but got %v", config.VisionConfig.VisionUseHead)
	}
}

func TestGemma3ConfigMultimodalFields(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check multimodal-specific fields
	if config.BoiTokenIndex != 255999 {
		t.Errorf("Expected boi_token_index to be 255999, but got %d", config.BoiTokenIndex)
	}

	if config.EoiTokenIndex != 256000 {
		t.Errorf("Expected eoi_token_index to be 256000, but got %d", config.EoiTokenIndex)
	}

	if config.ImageTokenIndex != 262144 {
		t.Errorf("Expected image_token_index to be 262144, but got %d", config.ImageTokenIndex)
	}

	if config.MmTokensPerImage != 256 {
		t.Errorf("Expected mm_tokens_per_image to be 256, but got %d", config.MmTokensPerImage)
	}

	if config.InitializerRange != 0.02 {
		t.Errorf("Expected initializer_range to be 0.02, but got %f", config.InitializerRange)
	}
}

func TestGemma3ConfigRopeScaling(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check RoPE scaling configuration
	if config.TextConfig.RopeScaling == nil {
		t.Fatal("Expected RopeScaling to be non-nil")
	}

	if config.TextConfig.RopeScaling.Factor != 8.0 {
		t.Errorf("Expected RoPE scaling factor to be 8.0, but got %f", config.TextConfig.RopeScaling.Factor)
	}

	if config.TextConfig.RopeScaling.RopeType != "linear" {
		t.Errorf("Expected RoPE type to be 'linear', but got '%s'", config.TextConfig.RopeScaling.RopeType)
	}
}

func TestGemma3ConfigHasVision(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true for Gemma3, but got false")
	}
}

func TestGemma3ConfigGetParameterCount(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check parameter count - should be in the range of 30B-35B
	paramCount := config.GetParameterCount()
	minExpected := int64(30_000_000_000)
	maxExpected := int64(35_000_000_000)

	if paramCount < minExpected || paramCount > maxExpected {
		t.Errorf("Expected parameter count to be between %d and %d, but got %d",
			minExpected, maxExpected, paramCount)
	}
}

func TestGemma3ConfigGetContextLength(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check context length - with RoPE scaling factor of 8.0 and sliding window 1024
	// Expected context length should be 1024 * 8.0 = 8192
	contextLength := config.GetContextLength()
	expectedLength := 8192

	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d",
			expectedLength, contextLength)
	}
}

func TestGemma3ConfigGetArchitecture(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check architecture
	architecture := config.GetArchitecture()
	expectedArchitecture := "Gemma3ForConditionalGeneration"

	if architecture != expectedArchitecture {
		t.Errorf("Expected architecture to be '%s', but got '%s'",
			expectedArchitecture, architecture)
	}
}

func TestGemma3ConfigGetTorchDtype(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check torch dtype
	torchDtype := config.GetTorchDtype()
	expectedDtype := "bfloat16"

	if torchDtype != expectedDtype {
		t.Errorf("Expected torch dtype to be '%s', but got '%s'",
			expectedDtype, torchDtype)
	}
}

func TestGemma3ConfigGetModelSizeBytes(t *testing.T) {
	configPath := filepath.Join("testdata", "gemma3.json")

	config, err := LoadGemma3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Gemma3 config: %v", err)
	}

	// Check model size in bytes
	modelSize := config.GetModelSizeBytes()

	// With ~32B parameters and bfloat16 (2 bytes per parameter)
	// Expected size should be around 64GB
	minExpectedSize := int64(60_000_000_000) // 60GB
	maxExpectedSize := int64(70_000_000_000) // 70GB

	if modelSize < minExpectedSize || modelSize > maxExpectedSize {
		t.Errorf("Expected model size to be between %d and %d bytes, but got %d",
			minExpectedSize, maxExpectedSize, modelSize)
	}
}

func TestGemma3ConfigInterface(t *testing.T) {
	// Test that Gemma3Config implements HuggingFaceModel interface
	var _ HuggingFaceModel = (*Gemma3Config)(nil)
}
