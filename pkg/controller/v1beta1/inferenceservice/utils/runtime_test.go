package utils

import (
	"context"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
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
