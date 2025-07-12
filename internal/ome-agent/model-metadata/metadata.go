// Package modelmetadata provides functionality to extract metadata from model files
// stored in PVCs and update BaseModel/ClusterBaseModel CRs.
//
// This agent is designed to be run as a Kubernetes Job by the BaseModel controller.
// The controller will:
// 1. Create a Job with the model PVC mounted
// 2. Pass the model path and BaseModel details via command-line flags
// 3. The agent will extract metadata and update the CR
//
// Example invocation:
//
//	ome-agent model-metadata \
//	  --config /etc/ome-agent/model-metadata.yaml \
//	  --model-path /model \
//	  --basemodel-name llama-7b \
//	  --basemodel-namespace model-serving
package modelmetadata

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/hfutil/modelconfig"
	"github.com/sgl-project/ome/pkg/logging"
)

type MetadataExtractor struct {
	config *Config
	fs     afero.Fs
	client client.Client
	logger logging.Interface
}

func NewMetadataExtractor(config *Config, fs afero.Fs, client client.Client) (*MetadataExtractor, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	return &MetadataExtractor{
		config: config,
		fs:     fs,
		client: client,
		logger: config.Logger,
	}, nil
}

func (m *MetadataExtractor) Start() error {
	m.logger.Infof("Starting model metadata extraction for model at %s", m.config.ModelPath)

	// Try different config file names
	configFiles := []string{"config.json", "model_config.json", "configuration.json"}

	var configPath string
	for _, configFile := range configFiles {
		path := filepath.Join(m.config.ModelPath, configFile)
		exists, err := afero.Exists(m.fs, path)
		if err != nil {
			m.logger.Warnf("Error checking for %s: %v", path, err)
			continue
		}
		if exists {
			configPath = path
			break
		}
	}

	if configPath == "" {
		return errors.New("no model config file found (tried: config.json, model_config.json, configuration.json)")
	}

	m.logger.Infof("Found model config at %s", configPath)

	// Use hfutil/modelconfig to load the model
	model, err := modelconfig.LoadModelConfig(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load model config from %s", configPath)
	}

	// Update the CR
	if m.config.ClusterScoped {
		return m.updateClusterBaseModel(model)
	}
	return m.updateBaseModel(model)
}

func (m *MetadataExtractor) updateBaseModel(model modelconfig.HuggingFaceModel) error {
	ctx := context.Background()

	// Fetch the BaseModel
	baseModel := &v1beta1.BaseModel{}
	err := m.client.Get(ctx, types.NamespacedName{
		Name:      m.config.BaseModelName,
		Namespace: m.config.BaseModelNamespace,
	}, baseModel)
	if err != nil {
		return errors.Wrapf(err, "failed to get BaseModel %s/%s", m.config.BaseModelNamespace, m.config.BaseModelName)
	}

	// Update spec with extracted metadata
	updated := m.updateSpec(&baseModel.Spec, model)
	if !updated {
		m.logger.Info("No updates needed for BaseModel spec")
		return nil
	}

	// Update the BaseModel
	err = m.client.Update(ctx, baseModel)
	if err != nil {
		return errors.Wrapf(err, "failed to update BaseModel %s/%s", m.config.BaseModelNamespace, m.config.BaseModelName)
	}

	m.logger.Infof("Successfully updated BaseModel %s/%s", m.config.BaseModelNamespace, m.config.BaseModelName)
	return nil
}

func (m *MetadataExtractor) updateClusterBaseModel(model modelconfig.HuggingFaceModel) error {
	ctx := context.Background()

	// Fetch the ClusterBaseModel
	clusterBaseModel := &v1beta1.ClusterBaseModel{}
	err := m.client.Get(ctx, types.NamespacedName{Name: m.config.BaseModelName}, clusterBaseModel)
	if err != nil {
		return errors.Wrapf(err, "failed to get ClusterBaseModel %s", m.config.BaseModelName)
	}

	// Update spec with extracted metadata
	updated := m.updateSpec(&clusterBaseModel.Spec, model)
	if !updated {
		m.logger.Info("No updates needed for ClusterBaseModel spec")
		return nil
	}

	// Update the ClusterBaseModel
	err = m.client.Update(ctx, clusterBaseModel)
	if err != nil {
		return errors.Wrapf(err, "failed to update ClusterBaseModel %s", m.config.BaseModelName)
	}

	m.logger.Infof("Successfully updated ClusterBaseModel %s", m.config.BaseModelName)
	return nil
}

func (m *MetadataExtractor) updateSpec(spec *v1beta1.BaseModelSpec, model modelconfig.HuggingFaceModel) bool {
	if spec == nil || model == nil {
		return false
	}

	updated := false

	// Model type
	modelType := model.GetModelType()
	if spec.ModelType == nil && modelType != "" {
		spec.ModelType = &modelType
		updated = true
	}

	// Architecture
	arch := model.GetArchitecture()
	if spec.ModelArchitecture == nil && arch != "" {
		spec.ModelArchitecture = &arch
		updated = true
	}

	// Parameter count
	paramCount := model.GetParameterCount()
	if spec.ModelParameterSize == nil && paramCount > 0 {
		paramStr := modelconfig.FormatParamCount(paramCount)
		spec.ModelParameterSize = &paramStr
		updated = true
	}

	// Max tokens
	contextLength := int32(model.GetContextLength())
	if spec.MaxTokens == nil && contextLength > 0 {
		spec.MaxTokens = &contextLength
		updated = true
	}

	// Capabilities
	if len(spec.ModelCapabilities) == 0 {
		capabilities := m.inferCapabilities(model)
		if len(capabilities) > 0 {
			spec.ModelCapabilities = capabilities
			updated = true
		}
	}

	// Framework (default to pytorch for HF models)
	if spec.ModelFramework == nil {
		spec.ModelFramework = &v1beta1.ModelFrameworkSpec{
			Name: "transformers",
		}
		updated = true
	}

	// Torch dtype as model format
	torchDtype := model.GetTorchDtype()
	if spec.ModelFormat.Name == "" && torchDtype != "" {
		spec.ModelFormat = v1beta1.ModelFormat{
			Name: torchDtype,
		}
		updated = true
	}

	return updated
}

func (m *MetadataExtractor) inferCapabilities(model modelconfig.HuggingFaceModel) []string {
	var capabilities []string

	// Check for vision capability
	if model.HasVision() {
		capabilities = append(capabilities, "vision")
	}

	// Infer from architecture and model type
	arch := strings.ToLower(model.GetArchitecture())
	modelType := strings.ToLower(model.GetModelType())

	// Text generation models
	if strings.Contains(arch, "causallm") || strings.Contains(modelType, "gpt") ||
		strings.Contains(modelType, "llama") || strings.Contains(modelType, "llava") ||
		strings.Contains(modelType, "mistral") || strings.Contains(modelType, "falcon") ||
		strings.Contains(modelType, "opt") || strings.Contains(modelType, "bloom") ||
		strings.Contains(modelType, "qwen") {
		capabilities = append(capabilities, "text-generation")
	}

	// Embedding models
	if strings.Contains(arch, "embedding") || strings.Contains(modelType, "bert") ||
		strings.Contains(modelType, "sentence") || strings.Contains(modelType, "e5") ||
		strings.Contains(modelType, "bge") {
		capabilities = append(capabilities, "text-embeddings")
	}

	// Default to text-generation if no capabilities detected
	if len(capabilities) == 0 {
		capabilities = append(capabilities, "text-generation")
	}

	return capabilities
}
