package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/client/clientset/versioned"
	"github.com/sgl-project/ome/pkg/hfutil/modelconfig"
)

const (
	// DefaultConfigFileName Default config file name used by Hugging Face models
	DefaultConfigFileName = "config.json"
)

// modelConfigLoader is a function type for loading model configurations
// This allows for easy mocking in tests
type modelConfigLoader func(configPath string) (modelconfig.HuggingFaceModel, error)

// ModelConfigParser is responsible for parsing model config files
// and updating the corresponding Model CRD
type ModelConfigParser struct {
	logger          *zap.SugaredLogger
	omeClient       versioned.Interface
	loadModelConfig modelConfigLoader // Function to load model configs
}

// NewModelConfigParser creates a new model config parser
func NewModelConfigParser(omeClient versioned.Interface, logger *zap.SugaredLogger) *ModelConfigParser {
	return &ModelConfigParser{
		logger:          logger,
		omeClient:       omeClient,
		loadModelConfig: modelconfig.LoadModelConfig, // Use the real implementation by default
	}
}

// ParseModelConfig reads the config.json file from the model directory and extracts metadata
// without updating any resources. This allows the caller to control when and how updates happen.
func (p *ModelConfigParser) ParseModelConfig(modelDir string, baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) (*ModelMetadata, error) {
	p.logger.Infof("Parsing model config at: %s", modelDir)

	// Skip if the directory doesn't exist
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		p.logger.Warnf("Model directory doesn't exist: %s", modelDir)
		return nil, nil
	}

	// Check if model should skip config parsing
	if shouldSkip := p.shouldSkipConfigParsing(baseModel, clusterBaseModel); shouldSkip {
		p.logger.Infof("Skipping config parsing due to annotation")
		return nil, nil
	}

	// Look for the config.json file - it could be at the root level or a subdirectory
	configPath, err := p.findConfigFile(modelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find config.json file: %w", err)
	}

	p.logger.Infof("Found config file at: %s", configPath)

	// Parse the config.json file using the hfutil.model_config module
	hfModel, err := p.loadModelConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file with hf_model_config: %w", err)
	}

	// Use the HuggingFaceModel interface to extract metadata
	metadata := p.extractModelMetadataFromHF(hfModel)
	p.logger.Infof("Extracted metadata: %+v", metadata)

	// Update BaseModel or ClusterBaseModel if provided
	if baseModel != nil {
		if err := p.updateBaseModel(baseModel, metadata); err != nil {
			p.logger.Errorf("Failed to update BaseModel: %v", err)
			// Continue anyway to return metadata
		}
	} else if clusterBaseModel != nil {
		if err := p.updateClusterBaseModel(clusterBaseModel, metadata); err != nil {
			p.logger.Errorf("Failed to update ClusterBaseModel: %v", err)
			// Continue anyway to return metadata
		}
	}

	return &metadata, nil
}

// findConfigFile searches for the config.json file in the model directory
// It checks the root directory and common subdirectories
func (p *ModelConfigParser) findConfigFile(modelDir string) (string, error) {
	// Check the root directory first
	rootConfigPath := filepath.Join(modelDir, DefaultConfigFileName)
	if _, err := os.Stat(rootConfigPath); err == nil {
		return rootConfigPath, nil
	}

	// Common places where config.json might be located
	possiblePaths := []string{
		filepath.Join(modelDir, "safetensors", DefaultConfigFileName),
		filepath.Join(modelDir, "pytorch_model", DefaultConfigFileName),
		filepath.Join(modelDir, "model", DefaultConfigFileName),
	}

	// Check if config.json exists in any of the possible paths
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If not found in common locations, do a recursive search (limited to avoid deep searching)
	var configPath string
	err := filepath.Walk(modelDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip error files
		}
		if !info.IsDir() && info.Name() == DefaultConfigFileName {
			configPath = path
			return filepath.SkipDir // Found it, stop searching
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if configPath == "" {
		return "", fmt.Errorf("config.json not found in %s", modelDir)
	}
	return configPath, nil
}

// updateModel is a generic function to update either a BaseModel or ClusterBaseModel
func (p *ModelConfigParser) updateModel(model interface{}, metadata ModelMetadata) error {
	// Get model info for logging
	var modelInfo string
	var namespace, name string

	// Update model spec fields from the extracted metadata
	switch m := model.(type) {
	case *v1beta1.BaseModel:
		p.updateModelSpec(&m.Spec, metadata)
		namespace, name = m.Namespace, m.Name
		modelInfo = fmt.Sprintf("BaseModel %s/%s", namespace, name)
	case *v1beta1.ClusterBaseModel:
		p.updateModelSpec(&m.Spec, metadata)
		name = m.Name
		modelInfo = fmt.Sprintf("ClusterBaseModel %s", name)
	default:
		return fmt.Errorf("unsupported model type: %T", model)
	}

	p.logger.Infof("Updating %s with extracted metadata", modelInfo)

	// Update the model in Kubernetes
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest interface{}
		var err error

		// Get the latest version of the model
		switch model.(type) {
		case *v1beta1.BaseModel:
			latest, err = p.omeClient.OmeV1beta1().BaseModels(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		case *v1beta1.ClusterBaseModel:
			latest, err = p.omeClient.OmeV1beta1().ClusterBaseModels().Get(context.TODO(), name, metav1.GetOptions{})
		}

		if err != nil {
			return fmt.Errorf("failed to get latest model: %w", err)
		}

		// Update the spec with our changes (reapply them to the latest version)
		switch m := latest.(type) {
		case *v1beta1.BaseModel:
			p.updateModelSpec(&m.Spec, metadata)
		case *v1beta1.ClusterBaseModel:
			p.updateModelSpec(&m.Spec, metadata)
		}

		// Update the model
		switch m := latest.(type) {
		case *v1beta1.BaseModel:
			_, err = p.omeClient.OmeV1beta1().BaseModels(namespace).Update(context.TODO(), m, metav1.UpdateOptions{})
		case *v1beta1.ClusterBaseModel:
			_, err = p.omeClient.OmeV1beta1().ClusterBaseModels().Update(context.TODO(), m, metav1.UpdateOptions{})
		}

		if err != nil {
			return fmt.Errorf("failed to update model: %w", err)
		}

		p.logger.Debugf("Successfully updated %s", modelInfo)
		return nil
	})
}

// updateBaseModel updates the BaseModel CRD with information from the extracted metadata
func (p *ModelConfigParser) updateBaseModel(model *v1beta1.BaseModel, metadata ModelMetadata) error {
	return p.updateModel(model, metadata)
}

// updateClusterBaseModel updates the ClusterBaseModel CRD with information from the extracted metadata
func (p *ModelConfigParser) updateClusterBaseModel(model *v1beta1.ClusterBaseModel, metadata ModelMetadata) error {
	return p.updateModel(model, metadata)
}

// extractModelMetadataFromHF extracts relevant metadata using the HuggingFaceModel interface
func (p *ModelConfigParser) extractModelMetadataFromHF(hfModel modelconfig.HuggingFaceModel) ModelMetadata {
	p.logger.Infof("Extracting metadata from HuggingFace model: type=%s, architecture=%s",
		hfModel.GetModelType(), hfModel.GetArchitecture())

	metadata := ModelMetadata{
		ModelType:          hfModel.GetModelType(),
		ModelArchitecture:  hfModel.GetArchitecture(),
		ModelParameterSize: modelconfig.FormatParamCount(hfModel.GetParameterCount()),
		MaxTokens:          int32(hfModel.GetContextLength()),
		ModelCapabilities:  p.determineModelCapabilitiesFromHF(hfModel),
	}

	// Set the model format (most models use SafeTensors)
	version := "1.0.0"
	metadata.ModelFormat = v1beta1.ModelFormat{
		Name:    "safetensors",
		Version: &version,
	}

	metadata.ModelFramework = &v1beta1.ModelFrameworkSpec{
		Name: "transformers",
	}

	// Set transformer version if available
	transformerVersion := hfModel.GetTransformerVersion()
	if transformerVersion != "" {
		metadata.ModelFramework.Version = &transformerVersion
		p.logger.Infof("Setting transformer version: %s", transformerVersion)
	}

	// Extract quantization information if available
	quantType := hfModel.GetQuantizationType()
	if quantType != "" {
		p.logger.Infof("Detected quantization type: %s", quantType)
		switch {
		case strings.Contains(strings.ToLower(quantType), "int4"):
			metadata.Quantization = v1beta1.ModelQuantizationINT4
			p.logger.Infof("Setting quantization to INT4")
		case strings.Contains(strings.ToLower(quantType), "fp8"):
			metadata.Quantization = v1beta1.ModelQuantizationFP8
			p.logger.Infof("Setting quantization to FP8")
		}
	}

	// Get the model size in bytes
	modelSizeBytes := hfModel.GetModelSizeBytes()
	if modelSizeBytes > 0 {
		p.logger.Infof("Model size in bytes: %d (%.2f GB)",
			modelSizeBytes, float64(modelSizeBytes)/1000000000.0)
	}

	// Get the raw JSON configuration for status
	configJSON, err := json.Marshal(struct {
		ModelType          string `json:"model_type"`
		Architecture       string `json:"architecture"`
		ContextLength      int    `json:"context_length"`
		ParameterCount     string `json:"parameter_count"`
		HasVision          bool   `json:"has_vision"`
		TransformerVersion string `json:"transformers_version"`
		TorchDtype         string `json:"torch_dtype"`
		ModelSizeBytes     int64  `json:"model_size_bytes"`
	}{
		ModelType:          hfModel.GetModelType(),
		Architecture:       hfModel.GetArchitecture(),
		ContextLength:      hfModel.GetContextLength(),
		ParameterCount:     modelconfig.FormatParamCount(hfModel.GetParameterCount()),
		HasVision:          hfModel.HasVision(),
		TransformerVersion: hfModel.GetTransformerVersion(),
		TorchDtype:         hfModel.GetTorchDtype(),
		ModelSizeBytes:     modelSizeBytes,
	})
	if err == nil {
		metadata.ModelConfiguration = configJSON
	} else {
		p.logger.Warnf("Failed to marshal model configuration: %v", err)
	}

	p.logger.Infof("Extracted metadata: type=%s, architecture=%s, size=%s, maxTokens=%d, capabilities=%v",
		metadata.ModelType, metadata.ModelArchitecture, metadata.ModelParameterSize,
		metadata.MaxTokens, metadata.ModelCapabilities)

	return metadata
}

func (p *ModelConfigParser) updateModelSpec(spec *v1beta1.BaseModelSpec, metadata ModelMetadata) {
	p.logger.Info("Updating model spec with extracted metadata")

	// Use a helper function for updating fields
	updateField := func(current interface{}, new interface{}, fieldName string) bool {
		isUpdated := false
		switch c := current.(type) {
		case **string:
			if *c == nil && new != nil {
				val := new.(string)
				*c = &val
				isUpdated = true
			}
		case *[]string:
			if len(*c) == 0 && len(new.([]string)) > 0 {
				*c = new.([]string)
				isUpdated = true
			}
		case *v1beta1.ModelFormat:
			if c.Name == "" {
				*c = new.(v1beta1.ModelFormat)
				isUpdated = true
			}
		case *[]v1beta1.ModelAPICapability:
			newAPICapabilities := new.([]v1beta1.ModelAPICapability)
			if len(*c) == 0 && len(newAPICapabilities) > 0 {
				*c = append([]v1beta1.ModelAPICapability(nil), newAPICapabilities...)
				isUpdated = true
			}
		case **v1beta1.ModelFrameworkSpec:
			if *c == nil && new != nil {
				*c = new.(*v1beta1.ModelFrameworkSpec)
				isUpdated = true
			}
		case **v1beta1.ModelQuantization:
			if *c == nil && new.(v1beta1.ModelQuantization) != "" {
				val := new.(v1beta1.ModelQuantization)
				*c = &val
				isUpdated = true
			}
		case *runtime.RawExtension:
			if new != nil && len(new.([]byte)) > 0 {
				c.Raw = new.([]byte)
				isUpdated = true
			}
		}

		if isUpdated {
			p.logger.Debugf("Setting %s: %v", fieldName, new)
		} else if current != nil {
			p.logger.Debugf("%s already set, not updating", fieldName)
		}

		return isUpdated
	}

	// Update each field using the helper function
	updateField(&spec.ModelType, metadata.ModelType, "ModelType")
	updateField(&spec.ModelArchitecture, metadata.ModelArchitecture, "ModelArchitecture")
	updateField(&spec.ModelFramework, metadata.ModelFramework, "ModelFramework")
	updateField(&spec.ModelFormat, metadata.ModelFormat, "ModelFormat")
	updateField(&spec.ModelParameterSize, metadata.ModelParameterSize, "ModelParameterSize")
	updateField(&spec.MaxTokens, metadata.MaxTokens, "MaxTokens")
	updateField(&spec.ModelCapabilities, metadata.ModelCapabilities, "ModelCapabilities")
	updateField(&spec.ApiCapabilities, metadata.ApiCapabilities, "ApiCapabilities")
	updateField(&spec.Quantization, metadata.Quantization, "Quantization")
	updateField(&spec.ModelConfiguration, metadata.ModelConfiguration, "ModelConfiguration")

	p.logger.Info("Model spec update complete")
}

// determineModelCapabilitiesFromHF determines the model capabilities based on the HuggingFaceModel
// shouldSkipConfigParsing checks if config parsing should be skipped for this model
func (p *ModelConfigParser) shouldSkipConfigParsing(baseModel *v1beta1.BaseModel, clusterBaseModel *v1beta1.ClusterBaseModel) bool {
	// Check base model annotations
	if baseModel != nil {
		if value, exists := baseModel.Annotations[ConfigParsingAnnotation]; exists {
			if strings.ToLower(value) == "true" {
				p.logger.Infof("Skipping config parsing for BaseModel %s/%s due to annotation", baseModel.Namespace, baseModel.Name)
				return true
			}
		}
	}

	// Check cluster base model annotations
	if clusterBaseModel != nil {
		if value, exists := clusterBaseModel.Annotations[ConfigParsingAnnotation]; exists {
			if strings.ToLower(value) == "true" {
				p.logger.Infof("Skipping config parsing for ClusterBaseModel %s due to annotation", clusterBaseModel.Name)
				return true
			}
		}
	}

	return false
}

func (p *ModelConfigParser) determineModelCapabilitiesFromHF(hfModel modelconfig.HuggingFaceModel) []string {
	var capabilities []string
	architecture := hfModel.GetArchitecture()
	modelType := hfModel.GetModelType()

	// For vision, only support image text capability right now
	if hfModel.HasVision() {
		return append(capabilities, string(v1beta1.ModelCapabilityImageTextToText))
	}

	// Check for text embedding capability
	if strings.Contains(strings.ToLower(architecture), "embedding") ||
		strings.Contains(strings.ToLower(architecture), "sentence") ||
		strings.Contains(strings.ToLower(modelType), "bert") ||
		// Special case for known embedding models
		(strings.Contains(strings.ToLower(modelType), "mistral") &&
			strings.Contains(strings.ToLower(architecture), "mistralmodel")) {
		return append(capabilities, string(v1beta1.ModelCapabilityEmbedding))
	}

	// Default to text-to-text capability
	return append(capabilities, string(v1beta1.ModelCapabilityTextToText))
}

// populateArtifactAttribute returns a pointer to an updated copy of currentModelMetadata
// where the Artifact field is set to the provided sha and parentPath, and
// ChildrenPaths is initialized as an empty slice. The original currentModelMetadata
// value passed in is not mutated (the update happens on a copy).
//
// Parameters:
//   - sha: artifact content hash
//   - parentPath: absolute or logical parent directory path where the artifact resides
//   - currentModelMetadata: existing model metadata to base the update on (passed by value)
//
// Returns:
//   - *ModelMetadata: pointer to the updated metadata with Artifact populated
func (p *ModelConfigParser) populateArtifactAttribute(sha string, parentPath string, currentModelMetadata ModelMetadata) *ModelMetadata {
	artifact := Artifact{
		Sha:           sha,
		ParentPath:    parentPath,
		ChildrenPaths: make([]string, 0),
	}
	currentModelMetadata.Artifact = artifact
	p.logger.Infof("current artifact is :%s", currentModelMetadata.Artifact)
	return &currentModelMetadata
}
