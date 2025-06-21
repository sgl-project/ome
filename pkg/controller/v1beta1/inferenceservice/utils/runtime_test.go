package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func createFakeClient() client.Client {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestGetProtocol(t *testing.T) {
	tests := []struct {
		name      string
		modelSpec *v1beta1.ModelSpec
		want      constants.InferenceServiceProtocol
	}{
		{
			name: "with protocol version",
			modelSpec: &v1beta1.ModelSpec{
				PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
					ProtocolVersion: ptr(constants.OpenInferenceProtocolV1),
				},
			},
			want: constants.OpenInferenceProtocolV1,
		},
		{
			name: "without protocol version",
			modelSpec: &v1beta1.ModelSpec{
				PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
					ProtocolVersion: nil,
				},
			},
			want: constants.OpenInferenceProtocolV2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProtocol(tt.modelSpec)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetBaseModel(t *testing.T) {
	// Create a fake fakeClient with our custom types registered
	fakeClient := createFakeClient()

	// Create test base models
	baseModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "test-format",
			},
		},
	}

	clusterBaseModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-model",
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "test-cluster-format",
			},
		},
	}

	// Add the models to the fake fakeClient
	_ = fakeClient.Create(context.Background(), baseModel)
	_ = fakeClient.Create(context.Background(), clusterBaseModel)

	tests := []struct {
		name      string
		modelName string
		namespace string
		wantErr   bool
	}{
		{
			name:      "existing namespace model",
			modelName: "test-model",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "existing cluster model",
			modelName: "test-cluster-model",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "non-existent model",
			modelName: "non-existent",
			namespace: "default",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, meta, err := GetBaseModel(fakeClient, tt.modelName, tt.namespace)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, spec)
				assert.Nil(t, meta)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, spec)
				assert.NotNil(t, meta)
				assert.Equal(t, tt.modelName, meta.Name)
			}
		})
	}
}

func TestGetSupportingRuntimes(t *testing.T) {
	// Create a fake fakeClient with our custom types registered
	fakeClient := createFakeClient()

	// Create test base models with different formats and sizes
	baseModels := []*v1beta1.BaseModel{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "small-model",
				Namespace: "default",
			},
			Spec: v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: ptr("7B"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "medium-model",
				Namespace: "default",
			},
			Spec: v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "onnx",
				},
				ModelParameterSize: ptr("13B"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "large-model",
				Namespace: "default",
			},
			Spec: v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "tensorflow",
				},
				ModelParameterSize: ptr("70B"),
			},
		},
	}

	// Create test serving runtimes with different capabilities
	runtimes := []*v1beta1.ServingRuntime{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pytorch-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
						AutoSelect: ptr(true),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("1B"),
					Max: ptr("10B"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "onnx-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "onnx",
						},
						AutoSelect: ptr(true),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("10B"),
					Max: ptr("20B"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tensorflow-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "tensorflow",
						},
						AutoSelect: ptr(true),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("50B"),
					Max: ptr("100B"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-format-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
						AutoSelect: ptr(true),
					},
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "onnx",
						},
						AutoSelect: ptr(true),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("1B"),
					Max: ptr("20B"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "disabled-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				Disabled: ptr(true),
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
						AutoSelect: ptr(true),
					},
				},
			},
		},
	}

	// Add all models and runtimes to the fake fakeClient
	for _, model := range baseModels {
		_ = fakeClient.Create(context.Background(), model)
	}
	for _, rt := range runtimes {
		_ = fakeClient.Create(context.Background(), rt)
	}

	tests := []struct {
		name      string
		modelSpec *v1beta1.ModelSpec
		wantCount int
		wantNames []string
		wantErr   bool
	}{
		{
			name: "small pytorch model",
			modelSpec: &v1beta1.ModelSpec{
				BaseModel: ptr("small-model"),
			},
			wantCount: 2,
			wantNames: []string{"pytorch-rt", "multi-format-rt"},
			wantErr:   false,
		},
		{
			name: "medium onnx model",
			modelSpec: &v1beta1.ModelSpec{
				BaseModel: ptr("medium-model"),
			},
			wantCount: 2,
			wantNames: []string{"onnx-rt", "multi-format-rt"},
			wantErr:   false,
		},
		{
			name: "large tensorflow model",
			modelSpec: &v1beta1.ModelSpec{
				BaseModel: ptr("large-model"),
			},
			wantCount: 1,
			wantNames: []string{"tensorflow-rt"},
			wantErr:   false,
		},
		{
			name: "non-existent model",
			modelSpec: &v1beta1.ModelSpec{
				BaseModel: ptr("non-existent"),
			},
			wantCount: 0,
			wantNames: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimes, err := GetSupportingRuntimes(tt.modelSpec, fakeClient, "default")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, runtimes, tt.wantCount)
				if tt.wantNames != nil {
					gotNames := make([]string, len(runtimes))
					for i, rt := range runtimes {
						gotNames[i] = rt.Name
					}
					assert.ElementsMatch(t, tt.wantNames, gotNames)
				}
			}
		})
	}
}

func TestRuntimeSupportsModel(t *testing.T) {
	tests := []struct {
		name      string
		modelSpec *v1beta1.ModelSpec
		runtime   *v1beta1.ServingRuntimeSpec
		baseModel *v1beta1.BaseModelSpec
		want      bool
	}{
		{
			name: "supported model format and size",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "test-format",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("1B"),
					Max: ptr("10B"),
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
				ModelParameterSize: ptr("7B"),
			},
			want: true,
		},
		{
			name: "unsupported model format",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "different-format",
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
			},
			want: false,
		},
		{
			name: "model size out of range",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "test-format",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("1B"),
					Max: ptr("10B"),
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
				ModelParameterSize: ptr("20B"),
			},
			want: false,
		},
		{
			name: "multiple supported formats",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "format1",
						},
					},
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "format2",
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "format2",
				},
			},
			want: true,
		},
		{
			name: "model with architecture",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "test-format",
						},
						ModelArchitecture: ptr("gpu"),
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
				ModelArchitecture: ptr("gpu"),
			},
			want: true,
		},
		{
			name: "model with quantization",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "test-format",
						},
						Quantization: ptr(v1beta1.ModelQuantizationFP8),
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
				Quantization: ptr(v1beta1.ModelQuantizationFP8),
			},
			want: true,
		},
		{
			name: "model with framework",
			modelSpec: &v1beta1.ModelSpec{
				Runtime: ptr("test-runtime"),
			},
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name: "test-format",
						},
						ModelFramework: &v1beta1.ModelFrameworkSpec{
							Name: "pytorch",
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "test-format",
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name: "pytorch",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RuntimeSupportsModel(tt.modelSpec, tt.runtime, tt.baseModel)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseModelSize(t *testing.T) {
	tests := []struct {
		name    string
		sizeStr string
		want    float64
		wantErr bool
	}{
		{
			name:    "terabytes",
			sizeStr: "1T",
			want:    1_000_000_000_000,
			wantErr: false,
		},
		{
			name:    "billions",
			sizeStr: "7B",
			want:    7_000_000_000,
			wantErr: false,
		},
		{
			name:    "millions",
			sizeStr: "13M",
			want:    13_000_000,
			wantErr: false,
		},
		{
			name:    "no suffix",
			sizeStr: "42",
			want:    42,
			wantErr: false,
		},
		{
			name:    "invalid format",
			sizeStr: "invalid",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseModelSize(tt.sizeStr)
			if tt.wantErr {
				assert.Equal(t, float64(0), got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSortSupportedRuntimeByPriority(t *testing.T) {
	modelFormat := v1beta1.ModelFormat{Name: "test-format"}
	modelSize := 7.0

	tests := []struct {
		name     string
		runtimes []v1beta1.SupportedRuntime
		want     []string
	}{
		{
			name: "sort by size range match",
			runtimes: []v1beta1.SupportedRuntime{
				{
					Name: "runtime1",
					Spec: v1beta1.ServingRuntimeSpec{
						ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
							Min: ptr("1B"),
							Max: ptr("10B"),
						},
					},
				},
				{
					Name: "runtime2",
					Spec: v1beta1.ServingRuntimeSpec{
						ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
							Min: ptr("20B"),
							Max: ptr("30B"),
						},
					},
				},
			},
			want: []string{"runtime1", "runtime2"},
		},
		{
			name: "sort by auto select",
			runtimes: []v1beta1.SupportedRuntime{
				{
					Name: "runtime1",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name: "test-format",
								},
								AutoSelect: ptr(true),
							},
						},
					},
				},
				{
					Name: "runtime2",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name: "test-format",
								},
								AutoSelect: ptr(false),
							},
						},
					},
				},
			},
			want: []string{"runtime1", "runtime2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortSupportedRuntimeByPriority(tt.runtimes, modelFormat, modelSize)
			got := make([]string, len(tt.runtimes))
			for i, rt := range tt.runtimes {
				got[i] = rt.Name
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create a pointer to a value
func ptr[T any](v T) *T {
	return &v
}

// MockClient is a mock implementation of client.Client for testing
type mockClient struct {
	client.Client
	listFunc func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listFunc != nil {
		return m.listFunc(ctx, list, opts...)
	}
	return nil
}

// Tests for New Architecture functions

func TestRuntimeCompatibilityError(t *testing.T) {
	tests := []struct {
		name     string
		err      *RuntimeCompatibilityError
		expected string
	}{
		{
			name: "error without detailed error",
			err: &RuntimeCompatibilityError{
				RuntimeName: "test-runtime",
				ModelName:   "test-model",
				ModelFormat: "pytorch",
				Reason:      "incompatible format",
			},
			expected: "runtime test-runtime does not support model test-model: incompatible format",
		},
		{
			name: "error with detailed error",
			err: &RuntimeCompatibilityError{
				RuntimeName:   "test-runtime",
				ModelName:     "test-model",
				ModelFormat:   "pytorch",
				Reason:        "size mismatch",
				DetailedError: errors.New("size 70B > max 13B"),
			},
			expected: "runtime test-runtime does not support model test-model: size mismatch (size 70B > max 13B)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestFormatToString(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		format   v1beta1.SupportedModelFormat
		expected string
	}{
		{
			name: "format with only name",
			format: v1beta1.SupportedModelFormat{
				Name: "simple-format",
			},
			expected: "simple-format",
		},
		{
			name: "format with model format no version",
			format: v1beta1.SupportedModelFormat{
				Name: "ignored",
				ModelFormat: &v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			expected: "pytorch",
		},
		{
			name: "format with model format and version",
			format: v1beta1.SupportedModelFormat{
				Name: "ignored",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("2.0"),
				},
			},
			expected: "pytorch:2.0",
		},
		{
			name: "format with architecture",
			format: v1beta1.SupportedModelFormat{
				Name: "ignored",
				ModelFormat: &v1beta1.ModelFormat{
					Name: "safetensors",
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
			},
			expected: "safetensors/LlamaForCausalLM",
		},
		{
			name: "format with quantization",
			format: v1beta1.SupportedModelFormat{
				Name: "ignored",
				ModelFormat: &v1beta1.ModelFormat{
					Name: "onnx",
				},
				Quantization: (*v1beta1.ModelQuantization)(strPtr("fp8")),
			},
			expected: "onnx/fp8",
		},
		{
			name: "format with all fields",
			format: v1beta1.SupportedModelFormat{
				Name: "ignored",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("2.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      (*v1beta1.ModelQuantization)(strPtr("fp8")),
			},
			expected: "pytorch:2.0/LlamaForCausalLM/fp8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToString(tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRuntimeSupportsModelNewArchitecture(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name          string
		baseModel     *v1beta1.BaseModelSpec
		srSpec        *v1beta1.ServingRuntimeSpec
		runtimeName   string
		expectError   bool
		errorContains string
	}{
		{
			name: "supported model format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "supported model format with all attributes matching",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
						AutoSelect: boolPtr(true),
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "unsupported model format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "tensorflow",
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model format 'mt:tensorflow' not in supported formats",
		},
		{
			name: "model size out of range",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("70B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model size 70B is outside supported range [1B, 13B]",
		},
		{
			name: "model with architecture and quantization match",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "safetensors",
					Version: strPtr("1.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      (*v1beta1.ModelQuantization)(strPtr("fp8")),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      (*v1beta1.ModelQuantization)(strPtr("fp8")),
					},
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model with nil parameter size",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: nil,
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "runtime with nil size range",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
				ModelSizeRange: nil,
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model size at minimum boundary",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("1B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "model size at maximum boundary",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("13B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name: "pytorch",
						},
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: strPtr("1B"),
					Max: strPtr("13B"),
				},
			},
			runtimeName: "test-runtime",
			expectError: false,
		},
		{
			name: "empty supported formats",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model format 'mt:pytorch' not in supported formats []",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RuntimeSupportsModelNewArchitecture(tt.baseModel, tt.srSpec, tt.runtimeName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				// Verify it's a RuntimeCompatibilityError
				var compatErr *RuntimeCompatibilityError
				assert.True(t, errors.As(err, &compatErr))
				assert.Equal(t, tt.runtimeName, compatErr.RuntimeName)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSupportingRuntimesNewArchitecture(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }
	int32Ptr := func(i int32) *int32 { return &i }

	tests := []struct {
		name             string
		baseModel        *v1beta1.BaseModelSpec
		namespace        string
		setupClient      func() client.Client
		expectedCount    int
		expectedFirst    string
		expectedExcluded map[string]string // runtime name -> expected error substring
		expectError      bool
	}{
		{
			name: "find runtimes supporting pytorch model",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "pytorch-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
												Priority:   int32Ptr(10),
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "multi-format-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
												Priority:   int32Ptr(5),
											},
											{
												Name: "onnx",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "onnx",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "no-autoselect-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(false),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 2,
			expectedFirst: "pytorch-runtime", // Higher priority
			expectedExcluded: map[string]string{
				"no-autoselect-runtime": "runtime does not have auto-select enabled",
			},
			expectError: false,
		},
		{
			name: "no runtimes support the model",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "custom-format",
				},
				ModelParameterSize: strPtr("100B"),
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "pytorch-only",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectedExcluded: map[string]string{
				"pytorch-only": "model format 'mt:custom-format' not in supported formats",
			},
			expectError: false,
		},
		{
			name: "runtime is disabled",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "disabled-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										Disabled: boolPtr(true),
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectedExcluded: map[string]string{
				"disabled-runtime": "runtime is disabled",
			},
			expectError: false,
		},
		{
			name: "error listing namespace runtimes",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if _, ok := list.(*v1beta1.ServingRuntimeList); ok {
							return errors.New("failed to list namespace runtimes")
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectError:   true,
		},
		{
			name: "error listing cluster runtimes",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if _, ok := list.(*v1beta1.ServingRuntimeList); ok {
							// Return empty namespace list
							return nil
						}
						if _, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							return errors.New("failed to list cluster runtimes")
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectError:   true,
		},
		{
			name: "mix of namespace and cluster runtimes",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: strPtr("7B"),
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "namespace-runtime",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "cluster-runtime",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
											},
										},
									},
								},
							}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 2,
			expectedFirst: "namespace-runtime", // Namespace runtimes come first
			expectError:   false,
		},
		{
			name: "model without parameter size",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: nil, // No size specified
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "runtime-with-priority",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
												Priority:   int32Ptr(10),
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "runtime-lower-priority",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(true),
												Priority:   int32Ptr(5),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 2,
			expectedFirst: "runtime-with-priority",
			expectError:   false,
		},
		{
			name: "runtime with nil auto-select defaults to false",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "runtime-nil-autoselect",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: nil, // nil should be treated as false
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectedExcluded: map[string]string{
				"runtime-nil-autoselect": "runtime does not have auto-select enabled",
			},
			expectError: false,
		},
		{
			name: "runtime supports model but has all formats with autoselect false",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			namespace: "test-namespace",
			setupClient: func() client.Client {
				return &mockClient{
					listFunc: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						if runtimeList, ok := list.(*v1beta1.ServingRuntimeList); ok {
							runtimeList.Items = []v1beta1.ServingRuntime{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name:      "runtime-mixed-autoselect",
										Namespace: "test-namespace",
									},
									Spec: v1beta1.ServingRuntimeSpec{
										SupportedModelFormats: []v1beta1.SupportedModelFormat{
											{
												Name: "pytorch",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "pytorch",
												},
												AutoSelect: boolPtr(false),
											},
											{
												Name: "onnx",
												ModelFormat: &v1beta1.ModelFormat{
													Name: "onnx",
												},
												AutoSelect: boolPtr(false),
											},
										},
									},
								},
							}
							return nil
						}
						if clusterRuntimeList, ok := list.(*v1beta1.ClusterServingRuntimeList); ok {
							clusterRuntimeList.Items = []v1beta1.ClusterServingRuntime{}
							return nil
						}
						return nil
					},
				}
			},
			expectedCount: 0,
			expectedExcluded: map[string]string{
				"runtime-mixed-autoselect": "runtime does not have auto-select enabled",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := tt.setupClient()
			result, excludedRuntimes, err := GetSupportingRuntimesNewArchitecture(tt.baseModel, cl, tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
				if tt.expectedCount > 0 {
					assert.Equal(t, tt.expectedFirst, result[0].Name)
				}

				// Check excluded runtimes
				for name, expectedErr := range tt.expectedExcluded {
					if err, exists := excludedRuntimes[name]; exists {
						assert.Contains(t, err.Error(), expectedErr)
					} else {
						t.Errorf("Expected runtime %s to be excluded but it wasn't", name)
					}
				}
			}
		})
	}
}
