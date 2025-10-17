package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadPhi3SmallConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3small.json")

	// Load the config
	config, err := LoadPhi3SmallConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3Small config: %v", err)
	}

	// Verify fields were parsed correctly
	if config.ModelType != "phi3small" {
		t.Errorf("Incorrect model type, expected 'phi3small', got %s", config.ModelType)
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

	if config.VocabSize != 100352 {
		t.Errorf("Incorrect vocabulary size, expected 100352, got %d", config.VocabSize)
	}

	if config.FfIntermediateSize != 14336 {
		t.Errorf("Incorrect ff_intermediate_size, expected 14336, got %d", config.FfIntermediateSize)
	}

	// Verify interface methods
	if config.GetModelType() != "phi3small" {
		t.Errorf("GetModelType() returned incorrect value: %s", config.GetModelType())
	}

	if config.GetArchitecture() != "Phi3SmallForCausalLM" {
		t.Errorf("GetArchitecture() returned incorrect value: %s", config.GetArchitecture())
	}

	if config.GetContextLength() != 8192 {
		t.Errorf("GetContextLength() returned incorrect value: %d", config.GetContextLength())
	}
}

func TestPhi3SmallViaGenericLoader(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3small.json")

	// Load the config using the generic loader
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load model config: %v", err)
	}

	// Verify the right type was loaded
	if config.GetModelType() != "phi3small" {
		t.Errorf("Incorrect model type, expected 'phi3small', got %s", config.GetModelType())
	}

	// Check that it's parsed as a Phi3SmallConfig
	phi3SmallConfig, ok := config.(*Phi3SmallConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *Phi3SmallConfig, but got %T", config)
	}

	// Verify parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(7_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Verify vision capability (should be false)
	if phi3SmallConfig.HasVision() {
		t.Error("Expected HasVision to return false for Phi3Small (non-vision), but got true")
	}
}

func TestPhi3SmallBlocksparseConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3small.json")

	// Load the config
	config, err := LoadPhi3SmallConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3Small config: %v", err)
	}

	// Verify blocksparse attention fields
	if config.BlocksparseBlockSize != 64 {
		t.Errorf("Expected blocksparse_block_size to be 64, got %d", config.BlocksparseBlockSize)
	}

	if config.BlocksparseHomoHeadPattern != false {
		t.Errorf("Expected blocksparse_homo_head_pattern to be false, got %t", config.BlocksparseHomoHeadPattern)
	}

	if config.BlocksparseNumLocalBlocks != 16 {
		t.Errorf("Expected blocksparse_num_local_blocks to be 16, got %d", config.BlocksparseNumLocalBlocks)
	}

	if config.BlocksparseTritonKernelBlockSize != 64 {
		t.Errorf("Expected blocksparse_triton_kernel_block_size to be 64, got %d", config.BlocksparseTritonKernelBlockSize)
	}

	if config.BlocksparseVertStride != 8 {
		t.Errorf("Expected blocksparse_vert_stride to be 8, got %d", config.BlocksparseVertStride)
	}

	if config.DenseAttentionEveryNLayers != 2 {
		t.Errorf("Expected dense_attention_every_n_layers to be 2, got %d", config.DenseAttentionEveryNLayers)
	}
}

func TestPhi3SmallMuPConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3small.json")

	// Load the config
	config, err := LoadPhi3SmallConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3Small config: %v", err)
	}

	// Verify muP (maximal update parameterization) fields
	if config.MupAttnMultiplier != 1.0 {
		t.Errorf("Expected mup_attn_multiplier to be 1.0, got %f", config.MupAttnMultiplier)
	}

	if config.MupEmbeddingMultiplier != 10.0 {
		t.Errorf("Expected mup_embedding_multiplier to be 10.0, got %f", config.MupEmbeddingMultiplier)
	}

	if config.MupUseScaling != true {
		t.Errorf("Expected mup_use_scaling to be true, got %t", config.MupUseScaling)
	}

	if config.MupWidthMultiplier != 8.0 {
		t.Errorf("Expected mup_width_multiplier to be 8.0, got %f", config.MupWidthMultiplier)
	}
}

func TestPhi3SmallMetadata(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3small.json")

	// Load the config
	config, err := LoadPhi3SmallConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3Small config: %v", err)
	}

	// Test parameter count
	paramCount := config.GetParameterCount()
	expectedParamCount := int64(7_000_000_000)
	if paramCount != expectedParamCount {
		t.Errorf("Incorrect parameter count, expected %d, got %d", expectedParamCount, paramCount)
	}

	// Test model size (bfloat16 is 2 bytes per parameter)
	modelSizeBytes := config.GetModelSizeBytes()
	expectedSizeBytes := int64(7_000_000_000) * 2
	if modelSizeBytes != expectedSizeBytes {
		t.Errorf("Incorrect model size, expected %s, got %s",
			FormatSize(expectedSizeBytes), FormatSize(modelSizeBytes))
	}

	// Test transformer version
	transformerVersion := config.GetTransformerVersion()
	if transformerVersion != "4.38.1" {
		t.Errorf("Incorrect transformer version, expected '4.38.1', got '%s'", transformerVersion)
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
	if architecture != "Phi3SmallForCausalLM" {
		t.Errorf("Incorrect architecture, expected 'Phi3SmallForCausalLM', got '%s'", architecture)
	}

	// Check model type
	modelType := config.GetModelType()
	if modelType != "phi3small" {
		t.Errorf("Incorrect model type, expected 'phi3small', got '%s'", modelType)
	}

	// Check context length
	contextLength := config.GetContextLength()
	if contextLength != 8192 {
		t.Errorf("Incorrect context length, expected 8192, got %d", contextLength)
	}

	// Check auto_map parsing
	if config.AutoMap == nil {
		t.Error("Expected auto_map to be parsed, but it is nil")
	} else {
		expectedAutoConfig := "configuration_phi3_small.Phi3SmallConfig"
		if config.AutoMap.AutoConfig != expectedAutoConfig {
			t.Errorf("Expected AutoConfig to be '%s', but got '%s'", expectedAutoConfig, config.AutoMap.AutoConfig)
		}

		expectedAutoModel := "modeling_phi3_small.Phi3SmallForCausalLM"
		if config.AutoMap.AutoModelForCausalLM != expectedAutoModel {
			t.Errorf("Expected AutoModelForCausalLM to be '%s', but got '%s'", expectedAutoModel, config.AutoMap.AutoModelForCausalLM)
		}
	}

	// Check GeGELU activation
	if config.HiddenAct != "gegelu" {
		t.Errorf("Expected hidden_act to be 'gegelu', got '%s'", config.HiddenAct)
	}

	if config.GegeluLimit != 20.0 {
		t.Errorf("Expected gegelu_limit to be 20.0, got %f", config.GegeluLimit)
	}

	if config.GegeluPadTo256 != true {
		t.Errorf("Expected gegelu_pad_to_256 to be true, got %t", config.GegeluPadTo256)
	}

	// Check RoPE parameters
	if config.RopeEmbeddingBase != 1000000 {
		t.Errorf("Expected rope_embedding_base to be 1000000, got %f", config.RopeEmbeddingBase)
	}

	if config.RopePositionScale != 1.0 {
		t.Errorf("Expected rope_position_scale to be 1.0, got %f", config.RopePositionScale)
	}

	// Check layer norm epsilon
	expectedLayerNormEps := 1e-05
	if config.LayerNormEpsilon != expectedLayerNormEps {
		t.Errorf("Expected layer_norm_epsilon to be %e, got %e", expectedLayerNormEps, config.LayerNormEpsilon)
	}

	// Check attention bias
	if config.AttentionBias != false {
		t.Errorf("Expected attention_bias to be false, got %t", config.AttentionBias)
	}

	// Check dropout values
	if config.AttentionDropoutProb != 0.0 {
		t.Errorf("Expected attention_dropout_prob to be 0.0, got %f", config.AttentionDropoutProb)
	}

	if config.EmbeddingDropoutProb != 0.1 {
		t.Errorf("Expected embedding_dropout_prob to be 0.1, got %f", config.EmbeddingDropoutProb)
	}

	if config.FfnDropoutProb != 0.1 {
		t.Errorf("Expected ffn_dropout_prob to be 0.1, got %f", config.FfnDropoutProb)
	}

	// Check nullable field
	if config.FfDimMultiplier != nil {
		t.Errorf("Expected ff_dim_multiplier to be nil, got %v", *config.FfDimMultiplier)
	}
}
