package modelconfig

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// DiffusionComponentSpec captures an individual component used by a diffusion pipeline.
type DiffusionComponentSpec struct {
	Library string
	Type    string
}

// DiffusionPipelineSpec captures pipeline-specific metadata for diffusion models.
type DiffusionPipelineSpec struct {
	ClassName            string
	DiffusersVersion     string
	Scheduler            *DiffusionComponentSpec
	TextEncoder          *DiffusionComponentSpec
	Tokenizer            *DiffusionComponentSpec
	Transformer          *DiffusionComponentSpec
	VAE                  *DiffusionComponentSpec
	AdditionalComponents map[string]DiffusionComponentSpec
}

// LoadDiffusionPipelineSpec loads and parses a diffusers model_index.json file.
func LoadDiffusionPipelineSpec(modelIndexPath string) (*DiffusionPipelineSpec, error) {
	if modelIndexPath == "" {
		return nil, fmt.Errorf("model index path cannot be empty")
	}
	data, err := os.ReadFile(modelIndexPath)
	if err != nil {
		return nil, fmt.Errorf("read model_index.json at %s: %w", modelIndexPath, err)
	}
	return ParseDiffusionPipelineSpec(SanitizeJSONBytes(data))
}

// ParseDiffusionPipelineSpec parses a diffusers model_index.json payload.
func ParseDiffusionPipelineSpec(data []byte) (*DiffusionPipelineSpec, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse model_index.json: %w", err)
	}

	pipeline := &DiffusionPipelineSpec{}
	pipeline.ClassName = parseJSONStringField(raw, "_class_name", "class_name", "className")
	pipeline.DiffusersVersion = parseJSONStringField(raw, "_diffusers_version", "diffusers_version")

	// Map well-known pipeline sub-component keys to the typed fields on
	// DiffusionPipelineSpec. "unet" and "transformer" collapse to the
	// same field — the diffusers library renamed the convention but
	// older SD1.x/SD2.x checkpoints still ship the legacy key.
	knownComponents := map[string]func(*DiffusionComponentSpec){
		"scheduler":    func(c *DiffusionComponentSpec) { pipeline.Scheduler = c },
		"text_encoder": func(c *DiffusionComponentSpec) { pipeline.TextEncoder = c },
		"tokenizer":    func(c *DiffusionComponentSpec) { pipeline.Tokenizer = c },
		"transformer":  func(c *DiffusionComponentSpec) { pipeline.Transformer = c },
		"unet":         func(c *DiffusionComponentSpec) { pipeline.Transformer = c },
		"vae":          func(c *DiffusionComponentSpec) { pipeline.VAE = c },
	}

	additional := map[string]DiffusionComponentSpec{}
	for key, value := range raw {
		if strings.HasPrefix(key, "_") {
			continue
		}
		component, ok := parseDiffusersComponent(value)
		if !ok {
			continue
		}
		if setter, known := knownComponents[strings.ToLower(key)]; known {
			setter(component)
		} else {
			additional[key] = *component
		}
	}
	if len(additional) > 0 {
		pipeline.AdditionalComponents = additional
	}

	if !pipelineHasMetadata(pipeline) {
		return nil, fmt.Errorf("model_index.json did not contain diffusion pipeline metadata")
	}
	return pipeline, nil
}

// pipelineHasMetadata reports whether at least one diffusion pipeline
// field was populated; an entirely empty pipeline is treated as a
// parse failure.
func pipelineHasMetadata(p *DiffusionPipelineSpec) bool {
	return p.ClassName != "" ||
		p.Scheduler != nil ||
		p.TextEncoder != nil ||
		p.Tokenizer != nil ||
		p.Transformer != nil ||
		p.VAE != nil ||
		len(p.AdditionalComponents) > 0
}

func parseDiffusersComponent(raw json.RawMessage) (*DiffusionComponentSpec, bool) {
	var parts []string
	if err := json.Unmarshal(raw, &parts); err == nil {
		switch len(parts) {
		case 0:
			return nil, false
		case 1:
			return &DiffusionComponentSpec{Type: parts[0]}, true
		default:
			return &DiffusionComponentSpec{Library: parts[0], Type: parts[1]}, true
		}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}

	className := parseJSONStringField(obj, "_class_name", "class_name", "className", "type")
	library := parseJSONStringField(obj, "_library", "library")
	if className == "" && library == "" {
		return nil, false
	}

	return &DiffusionComponentSpec{Library: library, Type: className}, true
}

func parseJSONStringField(values map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil && value != "" {
			return value
		}
	}
	return ""
}

// GenericDiffusionModelConfig is the HuggingFaceDiffusionModel
// implementation for diffusers model_index.json files. It walks the
// pipeline's component sub-directories (text_encoder, transformer or
// unet, vae, plus any AdditionalComponents) and sums their parameter
// counts via safetensors. Context length comes from the text_encoder's
// own config.json.
type GenericDiffusionModelConfig struct {
	BaseModelConfig

	DiffusersVersion  string
	DiffusionPipeline *DiffusionPipelineSpec
}

func (c *GenericDiffusionModelConfig) GetDiffusionModel() *DiffusionPipelineSpec {
	return c.DiffusionPipeline
}

// requiredDiffusionComponents lists the three component sub-directories
// every diffusion pipeline must have for a valid parameter count.
// transformer/ falls back to unet/ because older SD1.x/SD2.x checkpoints
// use the UNet naming.
var requiredDiffusionComponents = []string{"text_encoder", "transformer", "vae"}

// GetParameterCount sums safetensors counts across each required
// diffusion component plus any AdditionalComponents in the pipeline.
// Returns 0 if any required component fails to load — diffusion params
// are best-effort estimates, and a partial total is worse than no
// total.
func (c *GenericDiffusionModelConfig) GetParameterCount() int64 {
	if c.ConfigPath == "" {
		return 0
	}

	baseDir := filepath.Dir(c.ConfigPath)
	var total int64

	for _, name := range requiredDiffusionComponents {
		configPath := componentConfigPath(baseDir, name)
		count, ok := safetensorsCountForComponent(configPath)
		if !ok {
			return 0
		}
		if total > math.MaxInt64-count {
			return 0
		}
		total += count
	}

	// AdditionalComponents (safety_checker, image_encoder, ...) are
	// optional: a missing or unparsable sub-directory contributes 0
	// rather than zeroing out the entire total.
	for name := range c.DiffusionPipeline.AdditionalComponents {
		configPath := filepath.Join(baseDir, name, "config.json")
		if count, ok := safetensorsCountForComponent(configPath); ok {
			if total > math.MaxInt64-count {
				return 0
			}
			total += count
		}
	}

	return total
}

// componentConfigPath returns the path to <baseDir>/<name>/config.json,
// substituting unet/ for transformer/ on older SD checkpoints that
// pre-date the diffusers transformer-naming convention.
func componentConfigPath(baseDir, name string) string {
	primary := filepath.Join(baseDir, name, "config.json")
	if name == "transformer" {
		if _, err := os.Stat(primary); err != nil {
			unet := filepath.Join(baseDir, "unet", "config.json")
			if _, err := os.Stat(unet); err == nil {
				return unet
			}
		}
	}
	return primary
}

// safetensorsCountForComponent returns (count, true) on success and
// (0, false) when the sub-directory is missing or unparsable. The bool
// lets the caller distinguish "no params" from "couldn't load" without
// printing to stdout from inside library code.
func safetensorsCountForComponent(configPath string) (int64, bool) {
	if configPath == "" {
		return 0, false
	}
	if _, err := os.Stat(configPath); err != nil {
		return 0, false
	}
	count, err := FindAndParseSafetensors(configPath)
	if err != nil {
		return 0, false
	}
	return count, true
}

func (c *GenericDiffusionModelConfig) GetQuantizationType() string {
	// Quantization metadata for diffusers isn't standardized in HF.
	return ""
}

func (c *GenericDiffusionModelConfig) GetHFQuantConfig() *HFQuantConfig {
	// Diffusion pipelines don't carry HF-style quantization_config.
	return nil
}

func (c *GenericDiffusionModelConfig) GetContextLength() int {
	if c.ConfigPath == "" {
		return 0
	}

	textEncoderConfig := filepath.Join(filepath.Dir(c.ConfigPath), "text_encoder", "config.json")
	data, err := os.ReadFile(textEncoderConfig)
	if err != nil {
		return 0
	}

	data = SanitizeJSONBytes(data)

	var meta struct {
		MaxPositionEmbeds int `json:"max_position_embeddings"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return 0
	}
	return meta.MaxPositionEmbeds
}

func (c *GenericDiffusionModelConfig) GetModelSizeBytes() int64 {
	paramCount := c.GetParameterCount()
	if paramCount == 0 {
		return 0
	}
	return EstimateModelSizeBytes(paramCount, c.TorchDtype)
}

func (c *GenericDiffusionModelConfig) HasVision() bool {
	return true
}

// GetCapabilities returns the model's classified capabilities by
// running the modelconfig rule dispatcher against this config.
// Overrides BaseModelConfig's [CapabilityUnknown] default. The
// diffusionRule will fire (since this type satisfies
// HuggingFaceDiffusionModel) and use the pipeline class name to
// classify.
func (c *GenericDiffusionModelConfig) GetCapabilities() []Capability {
	return classifyCapabilities(c)
}

func (c *GenericDiffusionModelConfig) IsEmbedding() bool {
	return false
}
