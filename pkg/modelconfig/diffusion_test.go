package modelconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDiffusionPipelineSpec(t *testing.T) {
	data := []byte(`{
  "_class_name": "StableDiffusionPipeline",
  "_diffusers_version": "0.24.0",
  "scheduler": ["diffusers", "EulerDiscreteScheduler"],
  "text_encoder": ["transformers", "CLIPTextModel"],
  "tokenizer": ["transformers", "CLIPTokenizer"],
  "unet": ["diffusers", "UNet2DConditionModel"],
  "vae": {"_class_name": "AutoencoderKL", "_library": "diffusers"},
  "safety_checker": ["diffusers", "StableDiffusionSafetyChecker"]
}`)

	parsed, err := ParseDiffusionPipelineSpec(data)
	assert.NoError(t, err)
	if assert.NotNil(t, parsed) {
		assert.Equal(t, "StableDiffusionPipeline", parsed.ClassName)
		assert.Equal(t, "0.24.0", parsed.DiffusersVersion)

		if assert.NotNil(t, parsed.Scheduler) {
			assert.Equal(t, "diffusers", parsed.Scheduler.Library)
			assert.Equal(t, "EulerDiscreteScheduler", parsed.Scheduler.Type)
		}
		if assert.NotNil(t, parsed.TextEncoder) {
			assert.Equal(t, "transformers", parsed.TextEncoder.Library)
			assert.Equal(t, "CLIPTextModel", parsed.TextEncoder.Type)
		}
		if assert.NotNil(t, parsed.Tokenizer) {
			assert.Equal(t, "transformers", parsed.Tokenizer.Library)
			assert.Equal(t, "CLIPTokenizer", parsed.Tokenizer.Type)
		}
		if assert.NotNil(t, parsed.Transformer) {
			assert.Equal(t, "diffusers", parsed.Transformer.Library)
			assert.Equal(t, "UNet2DConditionModel", parsed.Transformer.Type)
		}
		if assert.NotNil(t, parsed.VAE) {
			assert.Equal(t, "diffusers", parsed.VAE.Library)
			assert.Equal(t, "AutoencoderKL", parsed.VAE.Type)
		}

		if assert.NotNil(t, parsed.AdditionalComponents) {
			component, ok := parsed.AdditionalComponents["safety_checker"]
			assert.True(t, ok)
			assert.Equal(t, "diffusers", component.Library)
			assert.Equal(t, "StableDiffusionSafetyChecker", component.Type)
		}
	}
}

func TestParseDiffusionPipelineSpec_Empty(t *testing.T) {
	parsed, err := ParseDiffusionPipelineSpec([]byte(`{}`))
	assert.Error(t, err)
	assert.Nil(t, parsed)
}

func TestLoadDiffusionPipelineSpec(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "diffusion-spec")
	assert.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	payload := []byte(`{
  "_class_name": "StableDiffusionPipeline",
  "scheduler": ["diffusers", "EulerDiscreteScheduler"]
}`)
	path := filepath.Join(tempDir, "model_index.json")
	assert.NoError(t, os.WriteFile(path, payload, 0644))

	parsed, err := LoadDiffusionPipelineSpec(path)
	assert.NoError(t, err)
	if assert.NotNil(t, parsed) {
		assert.Equal(t, "StableDiffusionPipeline", parsed.ClassName)
		if assert.NotNil(t, parsed.Scheduler) {
			assert.Equal(t, "diffusers", parsed.Scheduler.Library)
			assert.Equal(t, "EulerDiscreteScheduler", parsed.Scheduler.Type)
		}
	}
}

func TestLoadDiffusionPipelineSpec_MissingFile(t *testing.T) {
	parsed, err := LoadDiffusionPipelineSpec(filepath.Join(t.TempDir(), "model_index.json"))
	assert.Error(t, err)
	assert.Nil(t, parsed)
}
