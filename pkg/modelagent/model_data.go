// Package modelagent implements the model agent components for managing models in OME.
package modelagent

import (
	"encoding/json"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

// ModelStatus represents the status of a model on a node
type ModelStatus string

// Model status constants
const (
	// ModelStatusReady indicates the model is ready for use
	ModelStatusReady ModelStatus = "Ready"
	// ModelStatusUpdating indicates the model is currently being downloaded or updated
	ModelStatusUpdating ModelStatus = "Updating"
	// ModelStatusFailed indicates the model failed to download or initialize
	ModelStatusFailed ModelStatus = "Failed"
	// ModelStatusDeleted indicates the model was deleted
	ModelStatusDeleted ModelStatus = "Deleted"
)

// ConfigParsingAnnotation is the annotation key to skip config parsing
const ConfigParsingAnnotation = "ome.oracle.com/skip-config-parsing"

// ModelMetadata contains the extracted metadata about a model
type ModelMetadata struct {
	ModelType                 string
	ModelArchitecture         string
	ModelFramework            *v1beta1.ModelFrameworkSpec
	ModelFormat               v1beta1.ModelFormat
	ModelParameterSize        string
	MaxTokens                 int32
	ModelCapabilities         []string
	ModelConfiguration        []byte
	DecodedModelConfiguration map[string]interface{} `json:"DecodedModelConfiguration,omitempty"`
	Quantization              v1beta1.ModelQuantization
}

// ModelConfig represents the configuration of a model
// This is a structured version of the model metadata that is stored in the ConfigMap
type ModelConfig struct {
	// Core model identification
	ModelType         string `json:"modelType,omitempty"`         // e.g., "mistral", "llama", "phi"
	ModelArchitecture string `json:"modelArchitecture,omitempty"` // e.g., "MistralModel", "LlamaModel"

	// Framework and format information
	ModelFramework map[string]string `json:"modelFramework,omitempty"` // e.g., {"name":"transformers","version":"4.34.0"}
	ModelFormat    map[string]string `json:"modelFormat,omitempty"`    // e.g., {"name":"safetensors","version":"1.0.0"}

	// Model capabilities and size
	ModelParameterSize string   `json:"modelParameterSize,omitempty"` // Human-readable size, e.g., "7.11B"
	MaxTokens          int32    `json:"maxTokens,omitempty"`          // Maximum context length, e.g., 32768
	ModelCapabilities  []string `json:"modelCapabilities,omitempty"`  // e.g., ["TEXT_GENERATION", "TEXT_EMBEDDINGS"]

	// Advanced information
	DecodedModelConfiguration map[string]interface{} `json:"decodedModelConfiguration,omitempty"` // Detailed configuration
	Quantization              string                 `json:"quantization,omitempty"`              // Quantization type if applicable
}

// ModelEntry represents an entry in the node model ConfigMap
// This is the top-level structure stored for each model in the ConfigMap
type ModelEntry struct {
	Name   string       `json:"name"`             // Name of the model
	Status ModelStatus  `json:"status"`           // Current status of the model on this node
	Config *ModelConfig `json:"config,omitempty"` // Model configuration, may be nil if just tracking status
}

// ConvertMetadataToModelConfig converts internal ModelMetadata to a client-facing ModelConfig
// This transforms the internal representation to the structured format stored in ConfigMaps
func ConvertMetadataToModelConfig(metadata ModelMetadata) *ModelConfig {
	// Convert ModelFramework to map
	var modelFramework map[string]string
	if metadata.ModelFramework != nil {
		modelFramework = make(map[string]string)
		modelFramework["name"] = metadata.ModelFramework.Name
		if metadata.ModelFramework.Version != nil {
			modelFramework["version"] = *metadata.ModelFramework.Version
		}
	}

	// Convert ModelFormat to map
	var modelFormat map[string]string
	if metadata.ModelFormat.Name != "" {
		modelFormat = make(map[string]string)
		modelFormat["name"] = metadata.ModelFormat.Name
		if metadata.ModelFormat.Version != nil {
			modelFormat["version"] = *metadata.ModelFormat.Version
		}
	}

	// Convert Quantization to string
	var quantization string
	if metadata.Quantization != "" {
		quantization = string(metadata.Quantization)
	}

	// If DecodedModelConfiguration is nil but we have ModelConfiguration,
	// try to decode it
	decodedConfig := metadata.DecodedModelConfiguration
	if decodedConfig == nil && len(metadata.ModelConfiguration) > 0 {
		var configMap map[string]interface{}
		if err := json.Unmarshal(metadata.ModelConfiguration, &configMap); err == nil {
			decodedConfig = configMap
		}
	}

	return &ModelConfig{
		ModelType:                 metadata.ModelType,
		ModelArchitecture:         metadata.ModelArchitecture,
		ModelFramework:            modelFramework,
		ModelFormat:               modelFormat,
		ModelParameterSize:        metadata.ModelParameterSize,
		MaxTokens:                 metadata.MaxTokens,
		ModelCapabilities:         metadata.ModelCapabilities,
		DecodedModelConfiguration: decodedConfig,
		Quantization:              quantization,
	}
}

// GetModelKey returns a unique key for a model based on its namespace and name
// This generates consistent keys for both BaseModel and ClusterBaseModel types
func GetModelKey(namespace, name string) string {
	if namespace == "" {
		// For ClusterBaseModel, just use the name
		return name
	}
	// For BaseModel, use namespace_name format
	return namespace + "_" + name
}
