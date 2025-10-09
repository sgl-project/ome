package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadPhi3Config(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3.json")

	// Load the config
	config, err := LoadPhi3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3 config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "phi3" {
		t.Errorf("Incorrect model type, expected 'phi3', got %s", config.ModelType)
	}

	if config.HiddenSize != 3072 {
		t.Errorf("Incorrect hidden size, expected 3072, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 32 {
		t.Errorf("Incorrect number of layers, expected 32, got %d", config.NumHiddenLayers)
	}

	if config.NumAttentionHeads != 32 {
		t.Errorf("Incorrect number of attention heads, expected 32, got %d", config.NumAttentionHeads)
	}

	if config.VocabSize != 32064 {
		t.Errorf("Incorrect vocabulary size, expected 32064, got %d", config.VocabSize)
	}

	if config.IntermediateSize != 8192 {
		t.Errorf("Incorrect intermediate size, expected 8192, got %d", config.IntermediateSize)
	}

	// Verify interface methods
	if config.GetModelType() != "phi3" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "Phi3ForCausalLM" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}
}

func TestPhi3ViaGenericLoader(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "phi3" {
		t.Errorf("Incorrect model type, expected 'phi3', got %s", config.GetModelType())
	}

	// Check that it's parsed as a Phi3Config
	phi3Config, ok := config.(*Phi3Config)
	if !ok {
		t.Fatalf("Expected config to be of type *Phi3Config, but got %T", config)
	}

	// Verify parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(3_800_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Verify vision capability (should be false)
	if phi3Config.HasVision() {
		t.Error("Expected HasVision to return false for Phi3 (non-vision), but got true")
	}
}

func TestPhi3RopeScaling(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3.json")

	// Load the config
	config, err := LoadPhi3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3 config: %v", err)
	}

	// Verify rope scaling was parsed
	if config.RopeScaling == nil {
		t.Fatal("RopeScaling should not be nil")
	}

	// Verify rope scaling type
	if config.RopeScaling.Type != "longrope" {
		t.Errorf("Expected rope scaling type 'longrope', got '%s'", config.RopeScaling.Type)
	}

	// Verify long_factor array length (should be 48 elements)
	if len(config.RopeScaling.LongFactor) != 48 {
		t.Errorf("Expected 48 long_factor elements, got %d", len(config.RopeScaling.LongFactor))
	}

	// Verify short_factor array length (should be 48 elements)
	if len(config.RopeScaling.ShortFactor) != 48 {
		t.Errorf("Expected 48 short_factor elements, got %d", len(config.RopeScaling.ShortFactor))
	}

	// Verify first long_factor value
	expectedFirstLongFactor := 1.0700000524520874
	if config.RopeScaling.LongFactor[0] != expectedFirstLongFactor {
		t.Errorf("Expected first long_factor to be %f, got %f", expectedFirstLongFactor, config.RopeScaling.LongFactor[0])
	}

	// Verify first short_factor value
	expectedFirstShortFactor := 1.1
	if config.RopeScaling.ShortFactor[0] != expectedFirstShortFactor {
		t.Errorf("Expected first short_factor to be %f, got %f", expectedFirstShortFactor, config.RopeScaling.ShortFactor[0])
	}
}

func TestPhi3Metadata(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3.json")

	// Load the config
	config, err := LoadPhi3Config(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3 config: %v", err)
	}

	// Test parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(3_800_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Test model size (bfloat16 is 2 bytes per parameter)
	modelSizeBytes := config.GetModelSizeBytes()
	expectedSizeBytes := int64(3_800_000_000) * 2
	if modelSizeBytes != expectedSizeBytes {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSizeBytes), FormatSize(modelSizeBytes))
	}

	// Test transformer version
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.40.2" {
		t.Errorf("Incorrect transformer version, expected '4.40.2', got '%s'", transformerVersion)
	}

	// Check quantization type (should be empty for this model)
	quantizationType := config.GetQuantizationType()
	if quantizationType != "" {
		t.Errorf("Expected empty quantization type for non-quantized model, got '%s'", quantizationType)
	}

	// Check data type
	torchDtype := config.GetTorchDtype()
	if torchDtype != "bfloat16" {
		t.Errorf("Incorrect torch dtype, expected 'bfloat16', got '%s'", torchDtype)
	}

	// Check architecture
	architecture := config.GetArchitecture()
	if architecture != "Phi3ForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'Phi3ForCausalLM', got '%s'", architecture)
	}

	// Check model type
	modelType := config.GetModelType()
	if modelType != "phi3" {
		t.Errorf("Incorrect model type, expected 'phi3', got '%s'", modelType)
	}

	// Check context length
	contextLength := config.GetContextLength()
	if contextLength != 131072 {
		t.Errorf("Incorrect context length, expected 131072, got %d", contextLength)
	}

	// Check auto_map parsing
	if config.AutoMap == nil {
		t.Error("Expected auto_map to be parsed, but it is nil")
	} else {
		expectedAutoConfig := "configuration_phi3.Phi3Config"
		if config.AutoMap.AutoConfig != expectedAutoConfig {
			t.Errorf("Expected AutoConfig to be '%s', but got '%s'", expectedAutoConfig, config.AutoMap.AutoConfig)
		}

		expectedAutoModel := "modeling_phi3.Phi3ForCausalLM"
		if config.AutoMap.AutoModelForCausalLM != expectedAutoModel {
			t.Errorf("Expected AutoModelForCausalLM to be '%s', but got '%s'", expectedAutoModel, config.AutoMap.AutoModelForCausalLM)
		}
	}

	// Check original max position embeddings
	if config.OriginalMaxPositionEmbeddings != 4096 {
		t.Errorf("Expected original_max_position_embeddings to be 4096, got %d", config.OriginalMaxPositionEmbeddings)
	}

	// Check sliding window
	if config.SlidingWindow != 262144 {
		t.Errorf("Expected sliding_window to be 262144, got %d", config.SlidingWindow)
	}

	// Check rope theta
	if config.RopeTheta != 10000.0 {
		t.Errorf("Expected rope_theta to be 10000.0, got %f", config.RopeTheta)
	}

	// Check RMS norm epsilon
	expectedRmsNormEps := 1e-05
	if config.RmsNormEps != expectedRmsNormEps {
		t.Errorf("Expected rms_norm_eps to be %e, got %e", expectedRmsNormEps, config.RmsNormEps)
	}

	// Check attention bias
	if config.AttentionBias != false {
		t.Errorf("Expected attention_bias to be false, got %t", config.AttentionBias)
	}
}
