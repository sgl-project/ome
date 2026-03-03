package modelconfig

import (
	"encoding/json"
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
		{
			name:         "Qwen2.5-VL 7B",
			configPath:   "testdata/qwen2.5_vl_7b.json",
			expectedType: "qwen2_5_vl",
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

func TestQwen2VLQuantizationConfig(t *testing.T) {
	jsonData := []byte(`{
		"architectures": ["Qwen2VLForConditionalGeneration"],
		"model_type": "qwen2_vl",
		"hidden_size": 3584,
		"intermediate_size": 18944,
		"num_hidden_layers": 28,
		"num_attention_heads": 28,
		"num_key_value_heads": 4,
		"max_position_embeddings": 32768,
		"vocab_size": 152064,
		"torch_dtype": "float8_e4m3fn",
		"vision_config": {
			"depth": 32,
			"embed_dim": 1280,
			"num_heads": 16,
			"hidden_size": 1280
		},
		"quantization_config": {
			"activation_scheme": "dynamic",
			"fmt": "e4m3",
			"quant_method": "fp8",
			"weight_block_size": [128, 128]
		}
	}`)

	config := &Qwen2VLConfig{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		t.Fatalf("Failed to unmarshal Qwen2VL FP8 config: %v", err)
	}

	if config.GetQuantizationType() != "fp8" {
		t.Errorf("Expected quantization type 'fp8', but got '%s'", config.GetQuantizationType())
	}

	if config.QuantizationConfig == nil {
		t.Fatal("Expected QuantizationConfig to be non-nil")
	}

	if config.QuantizationConfig.Format != "e4m3" {
		t.Errorf("Expected format 'e4m3', but got '%s'", config.QuantizationConfig.Format)
	}
}

func TestQwen2_5VLSpecificFields(t *testing.T) {
	config, err := LoadQwen2VLConfig("testdata/qwen2.5_vl_7b.json")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test qwen2.5-vl specific fields
	if config.InitializerRange == 0 {
		t.Error("InitializerRange should be set for qwen2.5-vl")
	}

	if config.RopeScaling.Type != "mrope" {
		t.Errorf("Expected RopeScaling.Type to be 'mrope', got '%s'", config.RopeScaling.Type)
	}

	if len(config.RopeScaling.MropeSection) != 3 {
		t.Errorf("Expected RopeScaling.MropeSection to have 3 elements, got %d", len(config.RopeScaling.MropeSection))
	}

	// Test vision config fields specific to qwen2.5-vl
	vc := config.VisionConfig
	if vc.IntermediateSize == 0 {
		t.Error("VisionConfig.IntermediateSize should be set for qwen2.5-vl")
	}

	if vc.OutHiddenSize == 0 {
		t.Error("VisionConfig.OutHiddenSize should be set for qwen2.5-vl")
	}

	if vc.WindowSize == 0 {
		t.Error("VisionConfig.WindowSize should be set for qwen2.5-vl")
	}

	if len(vc.FullattBlockIndexes) == 0 {
		t.Error("VisionConfig.FullattBlockIndexes should be set for qwen2.5-vl")
	}

	if vc.HiddenAct != "silu" {
		t.Errorf("Expected VisionConfig.HiddenAct to be 'silu', got '%s'", vc.HiddenAct)
	}
}
