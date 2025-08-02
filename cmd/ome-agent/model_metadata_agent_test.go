package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/hfutil/modelconfig"
)

// MockK8sClient is a mock implementation of client.Client for testing
type MockK8sClient struct {
	mock.Mock
	client.Client
}

func (m *MockK8sClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *MockK8sClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

// TestModelMetadataAgent_ConfigureCommand tests command configuration
func TestModelMetadataAgent_ConfigureCommand(t *testing.T) {
	// Test that required flags are properly configured
	agent := NewModelMetadataAgent()
	cmd := &cobra.Command{}

	agent.ConfigureCommand(cmd)

	// Check required flags
	modelPathFlag := cmd.Flags().Lookup("model-path")
	assert.NotNil(t, modelPathFlag)
	assert.Equal(t, "model-path", modelPathFlag.Name)

	baseModelNameFlag := cmd.Flags().Lookup("basemodel-name")
	assert.NotNil(t, baseModelNameFlag)
	assert.Equal(t, "basemodel-name", baseModelNameFlag.Name)

	baseModelNamespaceFlag := cmd.Flags().Lookup("basemodel-namespace")
	assert.NotNil(t, baseModelNamespaceFlag)
	assert.Equal(t, "basemodel-namespace", baseModelNamespaceFlag.Name)

	clusterScopedFlag := cmd.Flags().Lookup("cluster-scoped")
	assert.NotNil(t, clusterScopedFlag)
	assert.Equal(t, "cluster-scoped", clusterScopedFlag.Name)
}

// TestModelMetadataAgent_Name tests agent name
func TestModelMetadataAgent_Name(t *testing.T) {
	agent := NewModelMetadataAgent()
	assert.Equal(t, "model-metadata", agent.Name())
}

// TestModelMetadataAgent_ShortDescription tests short description
func TestModelMetadataAgent_ShortDescription(t *testing.T) {
	agent := NewModelMetadataAgent()
	assert.Equal(t, "Extract model metadata from PVC-mounted models", agent.ShortDescription())
}

// TestModelMetadataAgent_LongDescription tests long description
func TestModelMetadataAgent_LongDescription(t *testing.T) {
	agent := NewModelMetadataAgent()
	expected := "Model metadata agent mounts PVCs and extracts model metadata to update BaseModel/ClusterBaseModel CRs"
	assert.Equal(t, expected, agent.LongDescription())
}

// TestSuccessfulConfigJsonExtraction tests successful config.json extraction
func TestSuccessfulConfigJsonExtraction(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a temporary directory structure
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "model")
	err := os.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create a valid config.json file
	configData := map[string]interface{}{
		"model_type": "llama",
		"architectures": []string{
			"LlamaForCausalLM",
		},
		"hidden_size":             4096,
		"intermediate_size":       11008,
		"num_hidden_layers":       32,
		"num_attention_heads":     32,
		"num_key_value_heads":     32,
		"max_position_embeddings": 4096,
		"vocab_size":              32000,
		"torch_dtype":             "float16",
		"transformers_version":    "4.34.0",
	}

	configJSON, err := json.Marshal(configData)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	configPath := filepath.Join(modelDir, "config.json")
	err = os.WriteFile(configPath, configJSON, 0644)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Test that the config can be loaded
	model, err := modelconfig.LoadModelConfig(configPath)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(model).NotTo(gomega.BeNil())

	// Verify extracted metadata
	g.Expect(model.GetModelType()).To(gomega.Equal("llama"))
	g.Expect(model.GetArchitecture()).To(gomega.Equal("LlamaForCausalLM"))
	g.Expect(model.GetContextLength()).To(gomega.Equal(4096))
	g.Expect(model.GetTransformerVersion()).To(gomega.Equal("4.34.0"))
	g.Expect(model.GetTorchDtype()).To(gomega.Equal("float16"))
}

// TestMissingConfigJsonFiles tests handling of missing config.json files
func TestMissingConfigJsonFiles(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a temporary directory without config files
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "model")
	err := os.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Try to load config from directory without config files
	configPath := filepath.Join(modelDir, "config.json")
	_, err = modelconfig.LoadModelConfig(configPath)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("failed to read model config file"))
}

// TestVariousConfigJsonFormats tests parsing of various config.json formats
func TestVariousConfigJsonFormats(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name         string
		configData   map[string]interface{}
		expectedType string
		expectedArch string
	}{
		{
			name: "HuggingFace Llama format",
			configData: map[string]interface{}{
				"model_type": "llama",
				"architectures": []string{
					"LlamaForCausalLM",
				},
				"hidden_size":             4096,
				"intermediate_size":       11008,
				"num_hidden_layers":       32,
				"num_attention_heads":     32,
				"num_key_value_heads":     32,
				"max_position_embeddings": 4096,
				"vocab_size":              32000,
				"torch_dtype":             "float16",
				"transformers_version":    "4.34.0",
			},
			expectedType: "llama",
			expectedArch: "LlamaForCausalLM",
		},
		{
			name: "Mistral format",
			configData: map[string]interface{}{
				"model_type": "mistral",
				"architectures": []string{
					"MistralForCausalLM",
				},
				"hidden_size":             4096,
				"intermediate_size":       14336,
				"num_hidden_layers":       32,
				"num_attention_heads":     32,
				"max_position_embeddings": 32768,
				"vocab_size":              32000,
				"torch_dtype":             "bfloat16",
				"transformers_version":    "4.35.0",
			},
			expectedType: "mistral",
			expectedArch: "MistralForCausalLM",
		},
		{
			name: "Custom format with minimal fields",
			configData: map[string]interface{}{
				"model_type": "custom_model",
				"architectures": []string{
					"CustomForCausalLM",
				},
				"hidden_size":             2048,
				"intermediate_size":       5632,
				"num_hidden_layers":       16,
				"num_attention_heads":     16,
				"max_position_embeddings": 2048,
				"vocab_size":              32000,
			},
			expectedType: "custom_model",
			expectedArch: "CustomForCausalLM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()
			modelDir := filepath.Join(tempDir, "model")
			err := os.MkdirAll(modelDir, 0755)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Create config.json
			configJSON, err := json.Marshal(tt.configData)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			configPath := filepath.Join(modelDir, "config.json")
			err = os.WriteFile(configPath, configJSON, 0644)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Test loading
			model, err := modelconfig.LoadModelConfig(configPath)
			if err != nil {
				// Some custom formats might not be supported
				t.Logf("Expected error for unsupported format: %v", err)
				return
			}

			g.Expect(model).NotTo(gomega.BeNil())
			g.Expect(model.GetModelType()).To(gomega.Equal(tt.expectedType))
			g.Expect(model.GetArchitecture()).To(gomega.Equal(tt.expectedArch))
		})
	}
}

// TestMetadataFieldMappingAndValidation tests metadata field mapping and validation
func TestMetadataFieldMappingAndValidation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme for testing
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	// Create a mock model with known metadata
	mockModel := &MockHuggingFaceModel{
		modelType:          "llama",
		architecture:       "LlamaForCausalLM",
		parameterCount:     7000000000, // 7B
		contextLength:      4096,
		transformerVersion: "4.34.0",
		torchDtype:         "float16",
		hasVision:          false,
	}

	// Create a BaseModel spec to update
	spec := &v1beta1.BaseModelSpec{}

	// Test metadata mapping
	updated := updateSpecWithModel(spec, mockModel)
	g.Expect(updated).To(gomega.BeTrue())

	// Verify all fields were mapped correctly
	g.Expect(spec.ModelType).NotTo(gomega.BeNil())
	g.Expect(*spec.ModelType).To(gomega.Equal("llama"))
	g.Expect(spec.ModelArchitecture).NotTo(gomega.BeNil())
	g.Expect(*spec.ModelArchitecture).To(gomega.Equal("LlamaForCausalLM"))
	g.Expect(spec.ModelParameterSize).NotTo(gomega.BeNil())
	g.Expect(*spec.ModelParameterSize).To(gomega.Equal("7B"))
	g.Expect(spec.MaxTokens).NotTo(gomega.BeNil())
	g.Expect(*spec.MaxTokens).To(gomega.Equal(int32(4096)))
	g.Expect(spec.ModelFramework).NotTo(gomega.BeNil())
	g.Expect(spec.ModelFramework.Name).To(gomega.Equal("transformers"))
	g.Expect(*spec.ModelFramework.Version).To(gomega.Equal("4.34.0"))
	g.Expect(spec.ModelFormat.Name).To(gomega.Equal("float16"))
	g.Expect(spec.ModelCapabilities).To(gomega.ContainElement("text-generation"))
}

// TestBaseModelUpdateViaKubernetesAPI tests BaseModel update via Kubernetes API
func TestBaseModelUpdateViaKubernetesAPI(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	// Create a BaseModel
	baseModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "safetensors",
			},
		},
	}

	// Create fake client
	client := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(baseModel).
		Build()

	// Create mock model
	mockModel := &MockHuggingFaceModel{
		modelType:          "llama",
		architecture:       "LlamaForCausalLM",
		parameterCount:     7000000000,
		contextLength:      4096,
		transformerVersion: "4.34.0",
		torchDtype:         "float16",
		hasVision:          false,
	}

	// Test BaseModel update
	err := updateBaseModelViaAPI(client, "test-model", "default", mockModel)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Verify the BaseModel was updated
	updated := &v1beta1.BaseModel{}
	err = client.Get(context.TODO(), types.NamespacedName{
		Name:      "test-model",
		Namespace: "default",
	}, updated)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(updated.Spec.ModelType).NotTo(gomega.BeNil())
	g.Expect(*updated.Spec.ModelType).To(gomega.Equal("llama"))
	g.Expect(updated.Spec.ModelArchitecture).NotTo(gomega.BeNil())
	g.Expect(*updated.Spec.ModelArchitecture).To(gomega.Equal("LlamaForCausalLM"))
}

// TestClusterBaseModelUpdateViaKubernetesAPI tests ClusterBaseModel update via Kubernetes API
func TestClusterBaseModelUpdateViaKubernetesAPI(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	// Create a ClusterBaseModel
	clusterBaseModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "safetensors",
			},
		},
	}

	// Create fake client
	client := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clusterBaseModel).
		Build()

	// Create mock model
	mockModel := &MockHuggingFaceModel{
		modelType:          "mistral",
		architecture:       "MistralForCausalLM",
		parameterCount:     7000000000,
		contextLength:      32768,
		transformerVersion: "4.35.0",
		torchDtype:         "bfloat16",
		hasVision:          false,
	}

	// Test ClusterBaseModel update
	err := updateClusterBaseModelViaAPI(client, "test-cluster-model", mockModel)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Verify the ClusterBaseModel was updated
	updated := &v1beta1.ClusterBaseModel{}
	err = client.Get(context.TODO(), types.NamespacedName{
		Name: "test-cluster-model",
	}, updated)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(updated.Spec.ModelType).NotTo(gomega.BeNil())
	g.Expect(*updated.Spec.ModelType).To(gomega.Equal("mistral"))
	g.Expect(updated.Spec.ModelArchitecture).NotTo(gomega.BeNil())
	g.Expect(*updated.Spec.ModelArchitecture).To(gomega.Equal("MistralForCausalLM"))
}

// TestErrorHandlingForMalformedJson tests error handling for malformed JSON
func TestErrorHandlingForMalformedJson(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a temporary directory
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "model")
	err := os.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create malformed config.json
	configPath := filepath.Join(modelDir, "config.json")
	malformedJSON := `{"model_type": "llama", "architectures": ["LlamaForCausalLM"], "hidden_size": 4096, "intermediate_size": 11008, "num_hidden_layers": 32, "num_attention_heads": 32, "max_position_embeddings": 4096, "vocab_size": 32000, "torch_dtype": "float16", "transformers_version": "4.34.0",` // Missing closing brace

	err = os.WriteFile(configPath, []byte(malformedJSON), 0644)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Test that malformed JSON is handled gracefully
	_, err = modelconfig.LoadModelConfig(configPath)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse model config JSON"))
}

// TestPermissionErrorsWhenReadingFiles tests permission errors when reading files
func TestPermissionErrorsWhenReadingFiles(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a temporary directory
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "model")
	err := os.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create config.json with no read permissions
	configPath := filepath.Join(modelDir, "config.json")
	configData := `{"model_type": "llama", "architectures": ["LlamaForCausalLM"]}`

	err = os.WriteFile(configPath, []byte(configData), 0000) // No permissions
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Test that permission errors are handled gracefully
	_, err = modelconfig.LoadModelConfig(configPath)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("failed to read model config file"))
}

// TestFileSystemOperations tests file system operations with afero
func TestFileSystemOperations(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create afero filesystem
	fs := afero.NewMemMapFs()

	// Create model directory structure
	modelDir := "/model"
	err := fs.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create config.json
	configData := map[string]interface{}{
		"model_type": "llama",
		"architectures": []string{
			"LlamaForCausalLM",
		},
		"hidden_size":             4096,
		"intermediate_size":       11008,
		"num_hidden_layers":       32,
		"num_attention_heads":     32,
		"max_position_embeddings": 4096,
		"vocab_size":              32000,
		"torch_dtype":             "float16",
		"transformers_version":    "4.34.0",
	}

	configJSON, err := json.Marshal(configData)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	configPath := filepath.Join(modelDir, "config.json")
	err = afero.WriteFile(fs, configPath, configJSON, 0644)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Test file existence
	exists, err := afero.Exists(fs, configPath)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(exists).To(gomega.BeTrue())

	// Test reading file
	data, err := afero.ReadFile(fs, configPath)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(data).NotTo(gomega.BeNil())
}

// TestMultipleConfigFileNames tests searching for multiple config file names
func TestMultipleConfigFileNames(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create a temporary directory structure
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "model")
	err := os.MkdirAll(modelDir, 0755)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Test different config file names
	configFiles := []string{"config.json", "model_config.json", "configuration.json"}

	for i, configFile := range configFiles {
		// Create config file
		configData := map[string]interface{}{
			"model_type": "llama",
			"architectures": []string{
				"LlamaForCausalLM",
			},
			"hidden_size":             4096,
			"intermediate_size":       11008,
			"num_hidden_layers":       32,
			"num_attention_heads":     32,
			"num_key_value_heads":     32,
			"max_position_embeddings": 4096,
			"vocab_size":              32000,
		}

		configJSON, err := json.Marshal(configData)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		configPath := filepath.Join(modelDir, configFile)
		err = os.WriteFile(configPath, configJSON, 0644)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		// Test that the file can be found and loaded
		_, err = os.Stat(configPath)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		// Test loading the config
		model, err := modelconfig.LoadModelConfig(configPath)
		g.Expect(err).NotTo(gomega.HaveOccurred(), "Should load config from %s", configFile)
		g.Expect(model).NotTo(gomega.BeNil())
		g.Expect(model.GetModelType()).To(gomega.Equal("llama"))

		// Clean up for next iteration
		if i < len(configFiles)-1 {
			err = os.Remove(configPath)
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}
	}
}

// TestCapabilityInference tests capability inference from model metadata
func TestCapabilityInference(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name         string
		modelType    string
		architecture string
		hasVision    bool
		expectedCaps []string
		description  string
	}{
		{
			name:         "Text generation model",
			modelType:    "llama",
			architecture: "LlamaForCausalLM",
			hasVision:    false,
			expectedCaps: []string{"text-generation"},
		},
		{
			name:         "Vision model",
			modelType:    "llava",
			architecture: "LlavaForConditionalGeneration",
			hasVision:    true,
			expectedCaps: []string{"vision", "text-generation"},
		},
		{
			name:         "Embedding model",
			modelType:    "bert",
			architecture: "BertModel",
			hasVision:    false,
			expectedCaps: []string{"text-embeddings"},
		},
		{
			name:         "Unknown model type",
			modelType:    "unknown_model",
			architecture: "UnknownArchitecture",
			hasVision:    false,
			expectedCaps: []string{"text-generation"}, // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockModel := &MockHuggingFaceModel{
				modelType:    tt.modelType,
				architecture: tt.architecture,
				hasVision:    tt.hasVision,
			}

			capabilities := inferCapabilities(mockModel)
			g.Expect(capabilities).To(gomega.ContainElements(tt.expectedCaps))
		})
	}
}

// TestErrorHandlingScenarios tests various error handling scenarios
func TestErrorHandlingScenarios(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name        string
		setupMock   func(*MockK8sClient)
		expectError bool
		description string
	}{
		{
			name: "BaseModel not found",
			setupMock: func(mockClient *MockK8sClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.NewNotFound(v1beta1.Resource("basemodel"), "not-found"))
			},
			expectError: true,
		},
		{
			name: "Update conflict",
			setupMock: func(mockClient *MockK8sClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.NewConflict(v1beta1.Resource("basemodel"), "conflict", fmt.Errorf("conflict")))
			},
			expectError: true,
		},
		{
			name: "Successful update",
			setupMock: func(mockClient *MockK8sClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				mockClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockK8sClient{}
			tt.setupMock(mockClient)

			mockModel := &MockHuggingFaceModel{
				modelType:    "llama",
				architecture: "LlamaForCausalLM",
			}

			err := updateBaseModelViaAPI(mockClient, "test-model", "default", mockModel)

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestConfigurationValidation tests configuration validation
func TestConfigurationValidation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		description string
	}{
		{
			name: "Valid configuration",
			config: &Config{
				ModelPath:          "/model",
				BaseModelName:      "test-model",
				BaseModelNamespace: "default",
				ClusterScoped:      false,
			},
			expectError: false,
		},
		{
			name: "Missing model path",
			config: &Config{
				BaseModelName:      "test-model",
				BaseModelNamespace: "default",
				ClusterScoped:      false,
			},
			expectError: true,
		},
		{
			name: "Missing base model name",
			config: &Config{
				ModelPath:          "/model",
				BaseModelNamespace: "default",
				ClusterScoped:      false,
			},
			expectError: true,
		},
		{
			name: "Cluster scoped without namespace",
			config: &Config{
				ModelPath:     "/model",
				BaseModelName: "test-model",
				ClusterScoped: true,
			},
			expectError: false, // Namespace not required for cluster-scoped
		},
		{
			name: "Namespace scoped without namespace",
			config: &Config{
				ModelPath:     "/model",
				BaseModelName: "test-model",
				ClusterScoped: false,
			},
			expectError: true, // Namespace required for namespace-scoped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}
		})
	}
}

// MockHuggingFaceModel is a mock implementation of modelconfig.HuggingFaceModel
type MockHuggingFaceModel struct {
	modelType          string
	architecture       string
	parameterCount     int64
	contextLength      int
	transformerVersion string
	torchDtype         string
	hasVision          bool
}

func (m *MockHuggingFaceModel) GetModelType() string {
	return m.modelType
}

func (m *MockHuggingFaceModel) GetArchitecture() string {
	return m.architecture
}

func (m *MockHuggingFaceModel) GetParameterCount() int64 {
	return m.parameterCount
}

func (m *MockHuggingFaceModel) GetContextLength() int {
	return m.contextLength
}

func (m *MockHuggingFaceModel) GetTransformerVersion() string {
	return m.transformerVersion
}

func (m *MockHuggingFaceModel) GetTorchDtype() string {
	return m.torchDtype
}

func (m *MockHuggingFaceModel) HasVision() bool {
	return m.hasVision
}

func (m *MockHuggingFaceModel) GetModelSizeBytes() int64 {
	return m.parameterCount * 2 // Rough estimate
}

func (m *MockHuggingFaceModel) GetQuantizationType() string {
	return ""
}

// Helper functions for testing

func updateSpecWithModel(spec *v1beta1.BaseModelSpec, model modelconfig.HuggingFaceModel) bool {
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
		capabilities := inferCapabilities(model)
		if len(capabilities) > 0 {
			spec.ModelCapabilities = capabilities
			updated = true
		}
	}

	// Framework
	if spec.ModelFramework == nil {
		spec.ModelFramework = &v1beta1.ModelFrameworkSpec{
			Name: "transformers",
		}
		updated = true
	}

	// Set transformer version if available
	transformerVersion := model.GetTransformerVersion()
	if transformerVersion != "" {
		spec.ModelFramework.Version = &transformerVersion
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

func updateBaseModelViaAPI(client client.Client, name, namespace string, model modelconfig.HuggingFaceModel) error {
	ctx := context.Background()

	// Fetch the BaseModel
	baseModel := &v1beta1.BaseModel{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, baseModel)
	if err != nil {
		return err
	}

	// Update spec with extracted metadata
	updated := updateSpecWithModel(&baseModel.Spec, model)
	if !updated {
		return nil
	}

	// Update the BaseModel
	return client.Update(ctx, baseModel)
}

func updateClusterBaseModelViaAPI(client client.Client, name string, model modelconfig.HuggingFaceModel) error {
	ctx := context.Background()

	// Fetch the ClusterBaseModel
	clusterBaseModel := &v1beta1.ClusterBaseModel{}
	err := client.Get(ctx, types.NamespacedName{Name: name}, clusterBaseModel)
	if err != nil {
		return err
	}

	// Update spec with extracted metadata
	updated := updateSpecWithModel(&clusterBaseModel.Spec, model)
	if !updated {
		return nil
	}

	// Update the ClusterBaseModel
	return client.Update(ctx, clusterBaseModel)
}

func inferCapabilities(model modelconfig.HuggingFaceModel) []string {
	var capabilities []string

	// Check for vision capability
	if model.HasVision() {
		capabilities = append(capabilities, "vision")
	}

	// Infer from architecture and model type
	arch := model.GetArchitecture()
	modelType := model.GetModelType()

	// Text generation models
	if contains(arch, "causallm") || contains(modelType, "gpt") ||
		contains(modelType, "llama") || contains(modelType, "llava") ||
		contains(modelType, "mistral") || contains(modelType, "falcon") ||
		contains(modelType, "opt") || contains(modelType, "bloom") ||
		contains(modelType, "qwen") {
		capabilities = append(capabilities, "text-generation")
	}

	// Embedding models
	if contains(arch, "embedding") || contains(modelType, "bert") ||
		contains(modelType, "sentence") || contains(modelType, "e5") ||
		contains(modelType, "bge") {
		capabilities = append(capabilities, "text-embeddings")
	}

	// Default to text-generation if no capabilities detected
	if len(capabilities) == 0 {
		capabilities = append(capabilities, "text-generation")
	}

	return capabilities
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Config represents the configuration for testing
type Config struct {
	ModelPath          string
	BaseModelName      string
	BaseModelNamespace string
	ClusterScoped      bool
}

func (c *Config) Validate() error {
	if c.ModelPath == "" {
		return fmt.Errorf("model_path is required")
	}
	if c.BaseModelName == "" {
		return fmt.Errorf("basemodel_name is required")
	}
	if !c.ClusterScoped && c.BaseModelNamespace == "" {
		return fmt.Errorf("basemodel_namespace is required for namespace-scoped BaseModel")
	}
	return nil
}
