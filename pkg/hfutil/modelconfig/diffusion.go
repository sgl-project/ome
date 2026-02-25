package modelconfig

import (
	"encoding/json"
	"fmt"
	"os"
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
		return nil, fmt.Errorf("failed to read model_index.json at %s: %w", modelIndexPath, err)
	}

	data = SanitizeJSONBytes(data)

	pipeline, err := ParseDiffusionPipelineSpec(data)
	if err != nil {
		return nil, err
	}

	return pipeline, nil
}

// ParseDiffusionPipelineSpec parses a diffusers model_index.json payload.
func ParseDiffusionPipelineSpec(data []byte) (*DiffusionPipelineSpec, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse model_index.json: %w", err)
	}

	pipeline := &DiffusionPipelineSpec{}
	pipeline.ClassName = parseJSONStringField(raw, "_class_name", "class_name", "className")
	pipeline.DiffusersVersion = parseJSONStringField(raw, "_diffusers_version", "diffusers_version")

	additional := map[string]DiffusionComponentSpec{}
	for key, value := range raw {
		if strings.HasPrefix(key, "_") {
			continue
		}

		component, ok := parseDiffusersComponent(value)
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "scheduler":
			pipeline.Scheduler = component
		case "text_encoder":
			pipeline.TextEncoder = component
		case "tokenizer":
			pipeline.Tokenizer = component
		case "transformer", "unet":
			pipeline.Transformer = component
		case "vae":
			pipeline.VAE = component
		default:
			additional[key] = *component
		}
	}

	if len(additional) > 0 {
		pipeline.AdditionalComponents = additional
	}

	if pipeline.ClassName == "" &&
		pipeline.Scheduler == nil &&
		pipeline.TextEncoder == nil &&
		pipeline.Tokenizer == nil &&
		pipeline.Transformer == nil &&
		pipeline.VAE == nil &&
		len(pipeline.AdditionalComponents) == 0 {
		return nil, fmt.Errorf("model_index.json did not contain diffusion pipeline metadata")
	}

	return pipeline, nil
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
