package modelmetadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/hfutil/modelconfig"
	"github.com/sgl-project/ome/pkg/logging"
)

// mockHuggingFaceModel implements the HuggingFaceModel interface for testing
type mockHuggingFaceModel struct {
	modelType          string
	architecture       string
	parameterCount     int64
	contextLength      int
	transformerVersion string
	quantizationType   string
	torchDtype         string
	modelSizeBytes     int64
	hasVision          bool
}

func (m *mockHuggingFaceModel) GetModelType() string          { return m.modelType }
func (m *mockHuggingFaceModel) GetArchitecture() string       { return m.architecture }
func (m *mockHuggingFaceModel) GetParameterCount() int64      { return m.parameterCount }
func (m *mockHuggingFaceModel) GetContextLength() int         { return m.contextLength }
func (m *mockHuggingFaceModel) GetTransformerVersion() string { return m.transformerVersion }
func (m *mockHuggingFaceModel) GetQuantizationType() string   { return m.quantizationType }
func (m *mockHuggingFaceModel) GetTorchDtype() string         { return m.torchDtype }
func (m *mockHuggingFaceModel) GetModelSizeBytes() int64      { return m.modelSizeBytes }
func (m *mockHuggingFaceModel) HasVision() bool               { return m.hasVision }

func TestMetadataExtractor_updateSpec(t *testing.T) {
	zapLogger, _ := zap.NewDevelopment()
	logger := zapLogger.Sugar()
	defer func() { _ = logger.Sync() }()

	tests := []struct {
		name           string
		initialSpec    *v1beta1.BaseModelSpec
		model          modelconfig.HuggingFaceModel
		expectedUpdate bool
		validate       func(*testing.T, *v1beta1.BaseModelSpec)
	}{
		{
			name:        "update empty spec",
			initialSpec: &v1beta1.BaseModelSpec{},
			model: &mockHuggingFaceModel{
				modelType:          "llama",
				architecture:       "LlamaForCausalLM",
				parameterCount:     7000000000, // 7B
				contextLength:      4096,
				torchDtype:         "float16",
				transformerVersion: "4.33.2",
				modelSizeBytes:     14000000000, // 14GB
			},
			expectedUpdate: true,
			validate: func(t *testing.T, spec *v1beta1.BaseModelSpec) {
				assert.Equal(t, "llama", *spec.ModelType)
				assert.Equal(t, "LlamaForCausalLM", *spec.ModelArchitecture)
				assert.Equal(t, "7B", *spec.ModelParameterSize)
				assert.Equal(t, int32(4096), *spec.MaxTokens)
				assert.Equal(t, "transformers", spec.ModelFramework.Name)
				assert.Equal(t, "float16", spec.ModelFormat.Name)
				assert.Contains(t, spec.ModelCapabilities, "text-generation")
			},
		},
		{
			name: "preserve existing values",
			initialSpec: &v1beta1.BaseModelSpec{
				ModelType:         stringPtr("custom-llama"),
				ModelArchitecture: stringPtr("CustomLlama"),
			},
			model: &mockHuggingFaceModel{
				modelType:          "llama",
				architecture:       "LlamaForCausalLM",
				parameterCount:     7000000000,
				transformerVersion: "4.33.2",
				modelSizeBytes:     14000000000,
			},
			expectedUpdate: true,
			validate: func(t *testing.T, spec *v1beta1.BaseModelSpec) {
				// Existing values should be preserved
				assert.Equal(t, "custom-llama", *spec.ModelType)
				assert.Equal(t, "CustomLlama", *spec.ModelArchitecture)
				// New values should be added
				assert.Equal(t, "7B", *spec.ModelParameterSize)
			},
		},
		{
			name:        "vision model capabilities",
			initialSpec: &v1beta1.BaseModelSpec{},
			model: &mockHuggingFaceModel{
				modelType:          "llava",
				architecture:       "LlavaForConditionalGeneration",
				hasVision:          true,
				transformerVersion: "4.33.2",
				modelSizeBytes:     14000000000,
			},
			expectedUpdate: true,
			validate: func(t *testing.T, spec *v1beta1.BaseModelSpec) {
				assert.Contains(t, spec.ModelCapabilities, "vision")
				assert.Contains(t, spec.ModelCapabilities, "text-generation")
			},
		},
		{
			name:        "embedding model capabilities",
			initialSpec: &v1beta1.BaseModelSpec{},
			model: &mockHuggingFaceModel{
				modelType:          "bge-base",
				architecture:       "BertModel",
				transformerVersion: "4.33.2",
				modelSizeBytes:     400000000,
			},
			expectedUpdate: true,
			validate: func(t *testing.T, spec *v1beta1.BaseModelSpec) {
				assert.Contains(t, spec.ModelCapabilities, "text-embeddings")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Logger: logging.Discard()}
			extractor := &MetadataExtractor{
				config: config,
				logger: config.Logger,
			}

			updated := extractor.updateSpec(tt.initialSpec, tt.model)
			assert.Equal(t, tt.expectedUpdate, updated)

			if tt.validate != nil {
				tt.validate(t, tt.initialSpec)
			}
		})
	}
}

func TestMetadataExtractor_updateBaseModel(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	baseModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: v1beta1.BaseModelSpec{},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(baseModel).
		Build()

	logger := logging.Discard()
	config := &Config{
		BaseModelName:      "test-model",
		BaseModelNamespace: "default",
		Logger:             logger,
	}

	extractor := &MetadataExtractor{
		config: config,
		client: fakeClient,
		logger: logger,
	}

	model := &mockHuggingFaceModel{
		modelType:          "llama",
		architecture:       "LlamaForCausalLM",
		parameterCount:     7000000000,
		contextLength:      4096,
		transformerVersion: "4.33.2",
		modelSizeBytes:     14000000000,
	}

	// Execute
	err := extractor.updateBaseModel(model)
	require.NoError(t, err)

	// Verify
	updatedModel := &v1beta1.BaseModel{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{
		Name:      "test-model",
		Namespace: "default",
	}, updatedModel)
	require.NoError(t, err)

	assert.Equal(t, "llama", *updatedModel.Spec.ModelType)
	assert.Equal(t, "LlamaForCausalLM", *updatedModel.Spec.ModelArchitecture)
	assert.Equal(t, "7B", *updatedModel.Spec.ModelParameterSize)
	assert.Equal(t, int32(4096), *updatedModel.Spec.MaxTokens)
}

func stringPtr(s string) *string {
	return &s
}
