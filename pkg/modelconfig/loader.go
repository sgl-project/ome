package modelconfig

import (
	"fmt"
	"os"
)

// ParseModelConfig parses a HuggingFace model configuration from
// in-memory bytes. Path is used as a label and to disambiguate
// model_index.json (diffusion pipelines) from config.json
// (transformer configs).
func ParseModelConfig(input ModelConfigInput) (HuggingFaceModel, error) {
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("model config data cannot be empty")
	}
	return parseGenericModelConfig(input)
}

// LoadModelConfig reads a HuggingFace model configuration from a
// config.json or model_index.json file and returns a HuggingFaceModel.
func LoadModelConfig(configPath string) (HuggingFaceModel, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read model config %s: %w", configPath, err)
	}
	return parseGenericModelConfig(ModelConfigInput{Path: configPath, Data: data})
}
