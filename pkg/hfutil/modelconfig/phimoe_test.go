package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadPhiMoEConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phimoe.json")

	// Load the config
	config, err := LoadPhiMoEConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load PhiMoE config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "phimoe" {
		t.Errorf("Incorrect model type, expected 'phimoe', got %s", config.ModelType)
	}

	if config.HiddenSize != 4096 {
		t.Errorf("Incorrect hidden size, expected 4096, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 32 {
		t.Errorf("Incorrect number of layers, expected 32, got %d", config.NumHiddenLayers)
	}

	if config.NumAttentionHeads != 32 {
		t.Errorf("Incorrect number of attention heads, expected 32, got %d", config.NumAttentionHeads)
	}

	if config.NumKeyValueHeads != 8 {
		t.Errorf("Incorrect number of key-value heads, expected 8, got %d", config.NumKeyValueHeads)
	}

	if config.VocabSize != 32064 {
		t.Errorf("Incorrect vocabulary size, expected 32064, got %d", config.VocabSize)
	}

	if config.IntermediateSize != 6400 {
		t.Errorf("Incorrect intermediate size, expected 6400, got %d", config.IntermediateSize)
	}

	// Verify interface methods
	if config.GetModelType() != "phimoe" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "PhiMoEForCausalLM" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}
}

func TestPhiMoEViaGenericLoader(t *testing.T) {
	configPath := filepath.Join("testdata", "phimoe.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "phimoe" {
		t.Errorf("Incorrect model type, expected 'phimoe', got %s", config.GetModelType())
	}

	// Check that it's parsed as a PhiMoEConfig
	phiMoEConfig, ok := config.(*PhiMoEConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *PhiMoEConfig, but got %T", config)
	}

	// Verify parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(16_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Verify vision capability (should be false)
	if phiMoEConfig.HasVision() {
		t.Error("Expected HasVision to return false for PhiMoE (non-vision), but got true")
	}
}

func TestPhiMoEMoEConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phimoe.json")

	// Load the config
	config, err := LoadPhiMoEConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load PhiMoE config: %v", err)
	}

	// Verify MoE-specific fields
	if config.NumLocalExperts != 16 {
		t.Errorf("Expected num_local_experts to be 16, got %d", config.NumLocalExperts)
	}

	if config.NumExpertsPerTok != 2 {
		t.Errorf("Expected num_experts_per_tok to be 2, got %d", config.NumExpertsPerTok)
	}

	if config.RouterAuxLossCoef != 0.0 {
		t.Errorf("Expected router_aux_loss_coef to be 0.0, got %f", config.RouterAuxLossCoef)
	}

	if config.RouterJitterNoise != 0.01 {
		t.Errorf("Expected router_jitter_noise to be 0.01, got %f", config.RouterJitterNoise)
	}

	if config.OutputRouterLogits != false {
		t.Errorf("Expected output_router_logits to be false, got %t", config.OutputRouterLogits)
	}
}

func TestPhiMoERopeScaling(t *testing.T) {
	configPath := filepath.Join("testdata", "phimoe.json")

	// Load the config
	config, err := LoadPhiMoEConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load PhiMoE config: %v", err)
	}

	// Verify rope scaling was parsed
	if config.RopeScaling == nil {
		t.Fatal("RopeScaling should not be nil")
	}

	// Verify rope scaling type
	if config.RopeScaling.Type != "longrope" {
		t.Errorf("Expected rope scaling type 'longrope', got '%s'", config.RopeScaling.Type)
	}

	// Verify long_factor array length (should be 64 elements)
	if len(config.RopeScaling.LongFactor) != 64 {
		t.Errorf("Expected 64 long_factor elements, got %d", len(config.RopeScaling.LongFactor))
	}

	// Verify short_factor array length (should be 64 elements)
	if len(config.RopeScaling.ShortFactor) != 64 {
		t.Errorf("Expected 64 short_factor elements, got %d", len(config.RopeScaling.ShortFactor))
	}

	// Verify first long_factor value
	expectedFirstLongFactor := 1.0199999809265137
	if config.RopeScaling.LongFactor[0] != expectedFirstLongFactor {
		t.Errorf("Expected first long_factor to be %f, got %f", expectedFirstLongFactor, config.RopeScaling.LongFactor[0])
	}

	// Verify first short_factor value
	expectedFirstShortFactor := 1.0
	if config.RopeScaling.ShortFactor[0] != expectedFirstShortFactor {
		t.Errorf("Expected first short_factor to be %f, got %f", expectedFirstShortFactor, config.RopeScaling.ShortFactor[0])
	}

	// Verify mscale values (unique to PhiMoE)
	expectedMscale := 1.243163121016122
	if config.RopeScaling.LongMscale != expectedMscale {
		t.Errorf("Expected long_mscale to be %f, got %f", expectedMscale, config.RopeScaling.LongMscale)
	}

	if config.RopeScaling.ShortMscale != expectedMscale {
		t.Errorf("Expected short_mscale to be %f, got %f", expectedMscale, config.RopeScaling.ShortMscale)
	}

	// Verify original_max_position_embeddings in rope_scaling
	if config.RopeScaling.OriginalMaxPositionEmbeddings != 4096 {
		t.Errorf("Expected rope_scaling.original_max_position_embeddings to be 4096, got %d",
			config.RopeScaling.OriginalMaxPositionEmbeddings)
	}
}

func TestPhiMoEMetadata(t *testing.T) {
	configPath := filepath.Join("testdata", "phimoe.json")

	// Load the config
	config, err := LoadPhiMoEConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load PhiMoE config: %v", err)
	}

	// Test parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(16_000_000_000) // 16B total parameters
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Test model size (bfloat16 is 2 bytes per parameter)
	modelSizeBytes := config.GetModelSizeBytes()
	expectedSizeBytes := int64(16_000_000_000) * 2
	if modelSizeBytes != expectedSizeBytes {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSizeBytes), FormatSize(modelSizeBytes))
	}

	// Test transformer version
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.43.3" {
		t.Errorf("Incorrect transformer version, expected '4.43.3', got '%s'", transformerVersion)
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
	if architecture != "PhiMoEForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'PhiMoEForCausalLM', got '%s'", architecture)
	}

	// Check model type
	modelType := config.GetModelType()
	if modelType != "phimoe" {
		t.Errorf("Incorrect model type, expected 'phimoe', got '%s'", modelType)
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
		expectedAutoConfig := "configuration_phimoe.PhiMoEConfig"
		if config.AutoMap.AutoConfig != expectedAutoConfig {
			t.Errorf("Expected AutoConfig to be '%s', but got '%s'", expectedAutoConfig, config.AutoMap.AutoConfig)
		}

		expectedAutoModel := "modeling_phimoe.PhiMoEForCausalLM"
		if config.AutoMap.AutoModelForCausalLM != expectedAutoModel {
			t.Errorf("Expected AutoModelForCausalLM to be '%s', but got '%s'", expectedAutoModel, config.AutoMap.AutoModelForCausalLM)
		}
	}

	// Check original max position embeddings
	if config.OriginalMaxPositionEmbeddings != 4096 {
		t.Errorf("Expected original_max_position_embeddings to be 4096, got %d", config.OriginalMaxPositionEmbeddings)
	}

	// Check sliding window
	if config.SlidingWindow != 131072 {
		t.Errorf("Expected sliding_window to be 131072, got %d", config.SlidingWindow)
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
	if config.AttentionBias != true {
		t.Errorf("Expected attention_bias to be true, got %t", config.AttentionBias)
	}

	// Check additional unique fields
	if config.InputJitterNoise != 0.01 {
		t.Errorf("Expected input_jitter_noise to be 0.01, got %f", config.InputJitterNoise)
	}

	if config.LmHeadBias != true {
		t.Errorf("Expected lm_head_bias to be true, got %t", config.LmHeadBias)
	}

	if config.HiddenDropout != 0.0 {
		t.Errorf("Expected hidden_dropout to be 0.0, got %f", config.HiddenDropout)
	}

	// Check hidden activation
	if config.HiddenAct != "silu" {
		t.Errorf("Expected hidden_act to be 'silu', got '%s'", config.HiddenAct)
	}
}
