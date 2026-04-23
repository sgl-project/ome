package modelconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestQwen2Config(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen2_7b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen2 config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen2" {
		t.Errorf("Expected model type 'qwen2' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen2Config
	qwen2Config, ok := config.(*Qwen2Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen2Config, but got %T", config)
	}

	// Check key fields
	if qwen2Config.HiddenSize != 3584 {
		t.Errorf("Expected hidden size to be 3584, but got %d", qwen2Config.HiddenSize)
	}

	if qwen2Config.NumHiddenLayers != 28 {
		t.Errorf("Expected hidden layers to be 28, but got %d", qwen2Config.NumHiddenLayers)
	}

	if qwen2Config.NumAttentionHeads != 28 {
		t.Errorf("Expected attention heads to be 28, but got %d", qwen2Config.NumAttentionHeads)
	}

	if qwen2Config.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", qwen2Config.NumKeyValueHeads)
	}

	// Check context length (should use max_position_embeddings)
	contextLength := config.GetContextLength()
	expectedLength := 131072
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(7_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen2)
	if qwen2Config.RopeTheta != 1000000.0 {
		t.Errorf("Expected RoPE theta to be 1000000.0, but got %f", qwen2Config.RopeTheta)
	}

	// Check sliding window
	if qwen2Config.SlidingWindow != 131072 {
		t.Errorf("Expected sliding window to be 131072, but got %d", qwen2Config.SlidingWindow)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen2, but got true")
	}

	// Check quantization type (should be empty for non-quantized model)
	if config.GetQuantizationType() != "" {
		t.Errorf("Expected empty quantization type, but got '%s'", config.GetQuantizationType())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}

func TestQwen2QuantizationConfig(t *testing.T) {
	jsonData := []byte(`{
		"architectures": ["Qwen2ForCausalLM"],
		"model_type": "qwen2",
		"hidden_size": 3584,
		"intermediate_size": 18944,
		"num_hidden_layers": 28,
		"num_attention_heads": 28,
		"num_key_value_heads": 4,
		"max_position_embeddings": 32768,
		"vocab_size": 152064,
		"torch_dtype": "float8_e4m3fn",
		"quantization_config": {
			"activation_scheme": "dynamic",
			"fmt": "e4m3",
			"quant_method": "fp8",
			"weight_block_size": [128, 128]
		}
	}`)

	config := &Qwen2Config{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		t.Fatalf("Failed to unmarshal Qwen2 FP8 config: %v", err)
	}

	if config.QuantizationConfig == nil {
		t.Fatal("Expected QuantizationConfig to be non-nil")
	}

	if config.GetQuantizationType() != "fp8" {
		t.Errorf("Expected quantization type 'fp8', but got '%s'", config.GetQuantizationType())
	}

	if config.QuantizationConfig.Format != "e4m3" {
		t.Errorf("Expected format 'e4m3', but got '%s'", config.QuantizationConfig.Format)
	}
}
