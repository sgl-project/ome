package model

import (
	"encoding/json"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/modelagent"
	"go.uber.org/zap"
)

// parseModelEntry parses the model configuration from the data in ConfigMap
func parseModelEntry(data string) (*modelagent.ModelEntry, error) {
	var modelEntry modelagent.ModelEntry
	err := json.Unmarshal([]byte(data), &modelEntry)
	if err != nil {
		// For backwards compatibility, if we can't parse as JSON, assume it's just a status string
		return &modelagent.ModelEntry{
			Status: modelagent.ModelStatus(data),
		}, nil
	}
	// Log whether we have model configuration or just status
	hasConfig := "no"
	if modelEntry.Config != nil {
		hasConfig = "yes"
	}
	zap.S().Infof("Parsed ModelEntry for model %s with status %s, has config: %s",
		modelEntry.Name, modelEntry.Status, hasConfig)
	return &modelEntry, nil
}

// updateModelWithConfig updates the model resource with the configuration from ModelConfig
func updateModelWithConfig(model interface{}, config *modelagent.ModelConfig) {
	if config == nil {
		zap.S().Info("No model configuration provided")
		return
	}

	switch m := model.(type) {
	case *v1beta1.BaseModel:
		updateBaseModelWithConfig(m, config)
	case *v1beta1.ClusterBaseModel:
		updateClusterBaseModelWithConfig(m, config)
	}
}

// updateBaseModelWithConfig updates a BaseModel with configuration metadata
func updateBaseModelWithConfig(model *v1beta1.BaseModel, config *modelagent.ModelConfig) {
	zap.S().Infof("Updating BaseModel %s/%s with configuration data", model.Namespace, model.Name)
	// ModelType field is a pointer in the model spec
	if model.Spec.ModelType == nil && config.ModelType != "" {
		modelType := config.ModelType
		model.Spec.ModelType = &modelType
		zap.S().Infof("  - Updated ModelType: %s", modelType)
	} else if model.Spec.ModelType != nil {
		zap.S().Infof("  - ModelType already set to: %s", *model.Spec.ModelType)
	}

	// ModelArchitecture field is a pointer in the model spec
	if model.Spec.ModelArchitecture == nil && config.ModelArchitecture != "" {
		architecture := config.ModelArchitecture
		model.Spec.ModelArchitecture = &architecture
		zap.S().Infof("  - Updated ModelArchitecture: %s", architecture)
	} else if model.Spec.ModelArchitecture != nil {
		zap.S().Infof("  - ModelArchitecture already set to: %s", *model.Spec.ModelArchitecture)
	}

	// ModelParameterSize field is a pointer in the model spec
	if model.Spec.ModelParameterSize == nil && config.ModelParameterSize != "" {
		paramSize := config.ModelParameterSize
		model.Spec.ModelParameterSize = &paramSize
		zap.S().Infof("  - Updated ModelParameterSize: %s", paramSize)
	} else if model.Spec.ModelParameterSize != nil {
		zap.S().Infof("  - ModelParameterSize already set to: %s", *model.Spec.ModelParameterSize)
	}

	// Update capabilities if they exist in the config
	if len(config.ModelCapabilities) > 0 && len(model.Spec.ModelCapabilities) == 0 {
		model.Spec.ModelCapabilities = make([]string, len(config.ModelCapabilities))
		copy(model.Spec.ModelCapabilities, config.ModelCapabilities)
		zap.S().Infof("  - Updated ModelCapabilities: %v", config.ModelCapabilities)
	} else if len(model.Spec.ModelCapabilities) > 0 {
		zap.S().Infof("  - ModelCapabilities already set to: %v", model.Spec.ModelCapabilities)
	}

	// Update framework if it exists in the config
	if config.ModelFramework != nil && model.Spec.ModelFramework == nil {
		name := config.ModelFramework["name"]
		version := config.ModelFramework["version"]
		if name != "" {
			framework := &v1beta1.ModelFrameworkSpec{
				Name: name,
			}
			if version != "" {
				framework.Version = &version
			}
			model.Spec.ModelFramework = framework
			zap.S().Infof("  - Updated ModelFramework: %s", name)
		}
	} else if model.Spec.ModelFramework != nil {
		zap.S().Infof("  - ModelFramework already set to: %s", model.Spec.ModelFramework.Name)
	}

	// Update model format if it exists in the config
	if config.ModelFormat != nil {
		name := config.ModelFormat["name"]
		version := config.ModelFormat["version"]

		// Update name if provided and not already set
		if name != "" && model.Spec.ModelFormat.Name == "" {
			model.Spec.ModelFormat.Name = name
			zap.S().Infof("  - Updated ModelFormat.Name: %s", name)
		} else if model.Spec.ModelFormat.Name != "" {
			zap.S().Infof("  - ModelFormat.Name already set to: %s", model.Spec.ModelFormat.Name)
		}

		// Update version if provided and not already set
		if version != "" {
			if model.Spec.ModelFormat.Version == nil {
				versionValue := version
				model.Spec.ModelFormat.Version = &versionValue
				zap.S().Infof("  - Updated ModelFormat.Version: %s", version)
			} else {
				zap.S().Infof("  - ModelFormat.Version already set to: %s", *model.Spec.ModelFormat.Version)
			}
		}
	}
}

// updateClusterBaseModelWithConfig updates a ClusterBaseModel with configuration metadata
func updateClusterBaseModelWithConfig(model *v1beta1.ClusterBaseModel, config *modelagent.ModelConfig) {
	zap.S().Infof("Updating ClusterBaseModel %s with configuration data", model.Name)
	// ModelType field is a pointer in the model spec
	if model.Spec.ModelType == nil && config.ModelType != "" {
		modelType := config.ModelType
		model.Spec.ModelType = &modelType
		zap.S().Infof("  - Updated ModelType: %s", modelType)
	} else if model.Spec.ModelType != nil {
		zap.S().Infof("  - ModelType already set to: %s", *model.Spec.ModelType)
	}

	// ModelArchitecture field is a pointer in the model spec
	if model.Spec.ModelArchitecture == nil && config.ModelArchitecture != "" {
		architecture := config.ModelArchitecture
		model.Spec.ModelArchitecture = &architecture
		zap.S().Infof("  - Updated ModelArchitecture: %s", architecture)
	} else if model.Spec.ModelArchitecture != nil {
		zap.S().Infof("  - ModelArchitecture already set to: %s", *model.Spec.ModelArchitecture)
	}

	// ModelParameterSize field is a pointer in the model spec
	if model.Spec.ModelParameterSize == nil && config.ModelParameterSize != "" {
		paramSize := config.ModelParameterSize
		model.Spec.ModelParameterSize = &paramSize
		zap.S().Infof("  - Updated ModelParameterSize: %s", paramSize)
	} else if model.Spec.ModelParameterSize != nil {
		zap.S().Infof("  - ModelParameterSize already set to: %s", *model.Spec.ModelParameterSize)
	}

	// Update capabilities if they exist in the config
	if len(config.ModelCapabilities) > 0 && len(model.Spec.ModelCapabilities) == 0 {
		model.Spec.ModelCapabilities = make([]string, len(config.ModelCapabilities))
		copy(model.Spec.ModelCapabilities, config.ModelCapabilities)
		zap.S().Infof("  - Updated ModelCapabilities: %v", config.ModelCapabilities)
	} else if len(model.Spec.ModelCapabilities) > 0 {
		zap.S().Infof("  - ModelCapabilities already set to: %v", model.Spec.ModelCapabilities)
	}

	// Update framework if it exists in the config
	if config.ModelFramework != nil && model.Spec.ModelFramework == nil {
		name := config.ModelFramework["name"]
		version := config.ModelFramework["version"]
		if name != "" {
			framework := &v1beta1.ModelFrameworkSpec{
				Name: name,
			}
			if version != "" {
				framework.Version = &version
			}
			model.Spec.ModelFramework = framework
			zap.S().Infof("  - Updated ModelFramework: %s", name)
		}
	} else if model.Spec.ModelFramework != nil {
		zap.S().Infof("  - ModelFramework already set to: %s", model.Spec.ModelFramework.Name)
	}

	// Update model format if it exists in the config
	if config.ModelFormat != nil {
		name := config.ModelFormat["name"]
		version := config.ModelFormat["version"]

		// Update name if provided and not already set
		if name != "" && model.Spec.ModelFormat.Name == "" {
			model.Spec.ModelFormat.Name = name
			zap.S().Infof("  - Updated ModelFormat.Name: %s", name)
		} else if model.Spec.ModelFormat.Name != "" {
			zap.S().Infof("  - ModelFormat.Name already set to: %s", model.Spec.ModelFormat.Name)
		}

		// Update version if provided and not already set
		if version != "" {
			if model.Spec.ModelFormat.Version == nil {
				versionValue := version
				model.Spec.ModelFormat.Version = &versionValue
				zap.S().Infof("  - Updated ModelFormat.Version: %s", version)
			} else {
				zap.S().Infof("  - ModelFormat.Version already set to: %s", *model.Spec.ModelFormat.Version)
			}
		}
	}
}
