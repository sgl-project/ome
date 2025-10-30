package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwen3VLConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_vl_235b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3VL config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_vl_moe" {
		t.Errorf("Expected model type 'qwen3_vl_moe' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3VLConfig, ok := config.(*Qwen3VLConfig)
	textConfig := qwen3VLConfig.TextConfig
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3VLConfig, but got %T", config)
	}

	// Check key fields
	if textConfig.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", textConfig.HiddenSize)
	}

	if textConfig.NumHiddenLayers != 94 {
		t.Errorf("Expected hidden layers to be 94, but got %d", textConfig.NumHiddenLayers)
	}

	if textConfig.NumAttentionHeads != 64 {
		t.Errorf("Expected attention heads to be 64, but got %d", textConfig.NumAttentionHeads)
	}

	if textConfig.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", textConfig.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 235B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(235_000_000_000) // 235B parameters
	// Allow 5% tolerance for parameter estimation
	tolerance := expectedCount / 20 // 5% tolerance
	if paramCount < expectedCount-tolerance || paramCount > expectedCount+tolerance {
		t.Errorf("Expected parameter count to be around %d (±%d), but got %d", expectedCount, tolerance, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if textConfig.RopeTheta != 5000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", textConfig.RopeTheta)
	}

	// Test vision capability
	if !config.HasVision() {
		t.Errorf("Expected HasVision() to return true, got %v", config.HasVision())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}

func TestQwen3VLConfig30B(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_vl_30b_a3b_instruct.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3VL config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_vl_moe" {
		t.Errorf("Expected model type 'qwen3_vl_moe' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3VLConfig, ok := config.(*Qwen3VLConfig)
	textConfig := qwen3VLConfig.TextConfig
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3VLConfig, but got %T", config)
	}

	// Check key fields for 30B model
	if textConfig.HiddenSize != 2048 {
		t.Errorf("Expected hidden size to be 2048, but got %d", textConfig.HiddenSize)
	}

	if textConfig.NumHiddenLayers != 48 {
		t.Errorf("Expected hidden layers to be 48, but got %d", textConfig.NumHiddenLayers)
	}

	if textConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", textConfig.NumAttentionHeads)
	}

	if textConfig.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", textConfig.NumKeyValueHeads)
	}

	// Check MoE configuration
	if textConfig.NumExperts != 128 {
		t.Errorf("Expected num experts to be 128, but got %d", textConfig.NumExperts)
	}

	if textConfig.MoeIntermediateSize != 768 {
		t.Errorf("Expected MoE intermediate size to be 768, but got %d", textConfig.MoeIntermediateSize)
	}

	if textConfig.NumExpertsPerTok != 8 {
		t.Errorf("Expected num experts per token to be 8, but got %d", textConfig.NumExpertsPerTok)
	}

	// Check context length
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check RoPE theta value (specific to Qwen3)
	if textConfig.RopeTheta != 5000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", textConfig.RopeTheta)
	}

	// Test vision capability
	if !config.HasVision() {
		t.Errorf("Expected HasVision() to return true, got %v", config.HasVision())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	// Expected around 30B parameters with 10% tolerance
	expectedCount := int64(30_000_000_000) // 30B parameters
	tolerance := expectedCount / 20        // 5% tolerance
	if paramCount < expectedCount-tolerance || paramCount > expectedCount+tolerance {
		t.Errorf("Expected parameter count to be around %d (±%d), but got %d", expectedCount, tolerance, paramCount)
	}

	// Check vision config
	visionConfig := qwen3VLConfig.VisionConfig
	if visionConfig.HiddenSize != 1152 {
		t.Errorf("Expected vision hidden size to be 1152, but got %d", visionConfig.HiddenSize)
	}

	if visionConfig.Depth != 27 {
		t.Errorf("Expected vision depth to be 27, but got %d", visionConfig.Depth)
	}
}

func TestQwen3VLConfigNonMoE(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_vl_2b_instruct.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3VL config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_vl" {
		t.Errorf("Expected model type 'qwen3_vl' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3VLConfig, ok := config.(*Qwen3VLConfig)
	textConfig := qwen3VLConfig.TextConfig
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3VLConfig, but got %T", config)
	}

	// Check key fields for non-MoE model
	if textConfig.HiddenSize != 2048 {
		t.Errorf("Expected hidden size to be 2048, but got %d", textConfig.HiddenSize)
	}

	if textConfig.NumHiddenLayers != 28 {
		t.Errorf("Expected hidden layers to be 28, but got %d", textConfig.NumHiddenLayers)
	}

	if textConfig.NumAttentionHeads != 16 {
		t.Errorf("Expected attention heads to be 16, but got %d", textConfig.NumAttentionHeads)
	}

	if textConfig.NumKeyValueHeads != 8 {
		t.Errorf("Expected key-value heads to be 8, but got %d", textConfig.NumKeyValueHeads)
	}

	// Check that it's not a MoE model
	if textConfig.NumExperts != 0 {
		t.Errorf("Expected num experts to be 0 for non-MoE model, but got %d", textConfig.NumExperts)
	}

	// Check context length
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check RoPE theta value (specific to Qwen3)
	if textConfig.RopeTheta != 5000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", textConfig.RopeTheta)
	}

	// Test vision capability
	if !config.HasVision() {
		t.Errorf("Expected HasVision() to return true, got %v", config.HasVision())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	// For 2B model, expect around 2B parameters with 20% tolerance
	expectedCount := int64(2_000_000_000) // 2B parameters
	tolerance := expectedCount / 20       // 5% tolerance
	if paramCount < expectedCount-tolerance || paramCount > expectedCount+tolerance {
		t.Errorf("Expected parameter count to be around %d (±%d), but got %d", expectedCount, tolerance, paramCount)
	}
}
