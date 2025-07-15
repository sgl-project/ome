package utils

import (
	"context"
	"errors"
	"testing"

	modelVer "github.com/sgl-project/ome/pkg/modelver"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
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

func TestSortSupportedRuntime(t *testing.T) {
	baseModel := &v1beta1.BaseModelSpec{
		ModelFormat:        v1beta1.ModelFormat{Name: "test-format"},
		ModelFramework:     &v1beta1.ModelFrameworkSpec{Name: "test-framework"},
		ModelParameterSize: ptr("7B"),
	}
	modelSize := 7.0

	tests := []struct {
		name     string
		runtimes []v1beta1.SupportedRuntime
		want     []string
	}{
		{
			name: "sort by score - higher score wins",
			runtimes: []v1beta1.SupportedRuntime{
				{
					Name: "low-score-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "test-format",
									Weight: 5,
								},
								Priority: ptr(int32(1)),
							},
						},
					},
				},
				{
					Name: "high-score-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "test-format",
									Weight: 10,
								},
								ModelFramework: &v1beta1.ModelFrameworkSpec{
									Name:   "test-framework",
									Weight: 8,
								},
								Priority: ptr(int32(2)),
							},
						},
					},
				},
			},
			want: []string{"high-score-runtime", "low-score-runtime"}, // Higher score first
		},
		{
			name: "sort by model size range when scores are equal",
			runtimes: []v1beta1.SupportedRuntime{
				{
					Name: "far-size-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "test-format",
									Weight: 10,
								},
								Priority: ptr(int32(1)),
							},
						},
						ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
							Min: ptr("20B"), // Further from 7B
							Max: ptr("30B"),
						},
					},
				},
				{
					Name: "close-size-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "test-format",
									Weight: 10,
								},
								Priority: ptr(int32(1)),
							},
						},
						ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
							Min: ptr("5B"), // Closer to 7B
							Max: ptr("10B"),
						},
					},
				},
			},
			want: []string{"close-size-runtime", "far-size-runtime"}, // Closer size range first
		},
		{
			name: "no matching format - should not score",
			runtimes: []v1beta1.SupportedRuntime{
				{
					Name: "no-match-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "different-format",
									Weight: 100,
								},
								Priority: ptr(int32(10)),
							},
						},
					},
				},
				{
					Name: "matching-runtime",
					Spec: v1beta1.ServingRuntimeSpec{
						SupportedModelFormats: []v1beta1.SupportedModelFormat{
							{
								ModelFormat: &v1beta1.ModelFormat{
									Name:   "test-format",
									Weight: 5,
								},
								Priority: ptr(int32(1)),
							},
						},
					},
				},
			},
			want: []string{"matching-runtime", "no-match-runtime"}, // Matching format wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortSupportedRuntime(tt.runtimes, baseModel, modelSize)
			got := make([]string, len(tt.runtimes))
			for i, rt := range tt.runtimes {
				got[i] = rt.Name
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name          string
		runtime       v1beta1.SupportedRuntime
		baseModel     *v1beta1.BaseModelSpec
		expectedScore int64
	}{
		{
			name: "exact match with weights and priority",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:    "PyTorch",
								Version: ptr("2.0.0"),
								Weight:  10,
							},
							ModelFramework: &v1beta1.ModelFrameworkSpec{
								Name:    "transformers",
								Version: ptr("4.0.0"),
								Weight:  5,
							},
							Priority: ptr(int32(3)),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "PyTorch",
					Version: ptr("2.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "transformers",
					Version: ptr("4.0.0"),
				},
			},
			expectedScore: 45, // (10 * 3) + (5 * 3) = 30 + 15 = 45
		},
		{
			name: "format match only",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "PyTorch",
								Weight: 8,
							},
							ModelFramework: &v1beta1.ModelFrameworkSpec{
								Name:   "different-framework",
								Weight: 4,
							},
							Priority: ptr(int32(2)),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name: "transformers",
				},
			},
			expectedScore: 0, // Framework doesn't match, so no score
		},
		{
			name: "no match",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "TensorFlow",
								Weight: 10,
							},
							Priority: ptr(int32(5)),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
			},
			expectedScore: 0, // No match
		},
		{
			name: "multiple formats, best match wins",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "PyTorch",
								Weight: 5,
							},
							ModelFramework: &v1beta1.ModelFrameworkSpec{
								Name:   "transformers",
								Weight: 8,
							},
							Priority: ptr(int32(1)),
						},
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "PyTorch",
								Weight: 10,
							},
							ModelFramework: &v1beta1.ModelFrameworkSpec{
								Name:   "transformers",
								Weight: 8,
							},
							Priority: ptr(int32(2)),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name: "transformers",
				},
			},
			expectedScore: 36, // Best match: (10 * 2) + (8 * 2) = 20 + 16 = 36
		},
		{
			name: "default priority when not specified",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "PyTorch",
								Weight: 12,
							},
							// No priority specified, should default to 1
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
			},
			expectedScore: 12, // 12 * 1 (default priority) = 12
		},
		{
			name: "nil model framework in base model",
			runtime: v1beta1.SupportedRuntime{
				Name: "test-runtime",
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							ModelFormat: &v1beta1.ModelFormat{
								Name:   "PyTorch",
								Weight: 10,
							},
							ModelFramework: &v1beta1.ModelFrameworkSpec{
								Name:   "transformers",
								Weight: 5,
							},
							Priority: ptr(int32(2)),
						},
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
				// ModelFramework is nil
			},
			expectedScore: 0, // model no modelFramework, runtime has, it won't support this runtime.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualScore := score(tt.runtime, tt.baseModel)
			assert.Equal(t, tt.expectedScore, actualScore, "Score should match expected value")
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
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
				ModelParameterSize: strPtr("7B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
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
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
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
					Name:    "tensorflow",
					Version: strPtr("2.0.0"),
				},
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
						},
					},
				},
			},
			runtimeName:   "test-runtime",
			expectError:   true,
			errorContains: "model format 'mt:tensorflow:2.0.0' not in supported formats",
		},
		{
			name: "model size out of range",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: strPtr("1.0.0"),
				},
				ModelParameterSize: strPtr("70B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "pytorch",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "pytorch",
							Version: strPtr("1.0.0"),
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
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      ptrToModelQuant("fp8"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
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
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: nil,
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
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
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture: strPtr("LlamaForCausalLM"),
				Quantization:      ptrToModelQuant("fp8"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
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
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: strPtr("1B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
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
					Name:    "safetensors",
					Version: strPtr("1.0.0"),
				},
				ModelArchitecture:  strPtr("LlamaForCausalLM"),
				Quantization:       ptrToModelQuant("fp8"),
				ModelParameterSize: strPtr("13B"),
			},
			srSpec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						Name: "safetensors",
						ModelFormat: &v1beta1.ModelFormat{
							Name:    "safetensors",
							Version: strPtr("1.0.0"),
						},
						ModelArchitecture: strPtr("LlamaForCausalLM"),
						Quantization:      ptrToModelQuant("fp8"),
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
			errorContains: "model format 'mt:pytorch' not in supported formats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RuntimeSupportsModel(tt.baseModel, tt.srSpec, tt.runtimeName)

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

func TestCompareSupportedModelFormats(t *testing.T) {
	tests := []struct {
		name            string
		baseModel       *v1beta1.BaseModelSpec
		supportedFormat v1beta1.SupportedModelFormat
		expected        bool
	}{
		{
			name: "matching model format names",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			expected: true,
		},
		{
			name: "non-matching model format names",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "PyTorch",
					Version: ptrToString("1.0.0"),
				},
			},
			expected: false,
		},
		{
			name: "matching quantization",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			expected: true,
		},
		{
			name: "non-matching quantization",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptrToModelQuant("int8"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				Quantization: ptr(v1beta1.ModelQuantization("fp16")),
			},
			expected: false,
		},
		{
			name: "equal version comparison",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: true,
		},
		{
			name: "equal version comparison - not equal",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.9.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: false,
		},
		{
			name: "greater than version comparison - true",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.7.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: true,
		},
		{
			name: "greater than version comparison - false",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.7.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.7.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: false,
		},
		{
			name: "unofficial version comparison - forces equality",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.8.0-dev"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0-dev"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: true,
		},
		{
			name: "unofficial version comparison - not equal",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptr("1.8.0-dev"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.8.0-alpha"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThan))},
			},
			expected: false,
		},
		{
			name: "framework comparison - matching",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "ONNXRuntime", Version: ptr("1.10.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: true,
		},
		{
			name: "framework comparison - non-matching name",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "PyTorch", Version: ptr("1.10.0")},
			},
			expected: false,
		},
		{
			name: "framework comparison - greater than version",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.12.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "ONNXRuntime", Version: ptr("1.10.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: false,
		},
		{
			name: "framework comparison - not greater than version",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:           "test-format",
				ModelFormat:    &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "ONNXRuntime", Version: ptr("1.10.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: false,
		},
		// Test case for prefixed format names like mt:pytorch
		{
			name: "model format with prefix",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "mt:pytorch",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "pytorch", Version: ptrToString("1.0.0"), Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpEqual))},
			},
			expected: false, // Currently this will fail because we're doing exact matching
		},
		// autoselect
		{
			name: "autoselect flag explicitly false",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				AutoSelect:  ptr(false),
			},
			expected: true,
		},
		{
			name: "model architecture mismatch",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelArchitecture: ptrToString("transformer"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:              "test-format",
				ModelFormat:       &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelArchitecture: ptrToString("cnn"),
			},
			expected: false,
		},
		{
			name: "model has architecture but runtime doesn't specify",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelArchitecture: ptrToString("transformer"),
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
			},
			expected: false,
		},
		{
			name: "runtime has architecture requirement but model doesn't specify",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:              "test-format",
				ModelFormat:       &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelArchitecture: ptrToString("transformer"),
			},
			expected: false,
		},
		{
			name: "nil model format in supportedFormat",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
			},
			expected: false,
		},
		{
			name: "missing version in model format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "ONNX",
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
			},
			expected: false,
		},
		{
			name: "missing version in supported format",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX"},
			},
			expected: false,
		},
		{
			name: "greater than or equal - equal case",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:     "ONNX",
					Version:  ptrToString("1.8.0"),
					Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
				},
			},
			expected: true,
		},
		{
			name: "greater than or equal - greater than case",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:     "ONNX",
					Version:  ptrToString("1.9.0"),
					Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
				},
			},
			expected: true,
		},
		{
			name: "greater than or equal - equal to case",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:     "ONNX",
					Version:  ptrToString("1.8.0"),
					Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
				},
			},
			expected: true,
		},
		{
			name: "greater than or equal - less than case",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.8.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name: "test-format",
				ModelFormat: &v1beta1.ModelFormat{
					Name:     "ONNX",
					Version:  ptrToString("1.7.0"),
					Operator: ptrToRuntimeOp(string(v1beta1.RuntimeSelectorOpGreaterThanOrEqual)),
				},
			},
			expected: false,
		},
		{
			name: "runtime has framework requirement but model doesn't specify",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			expected: false,
		},
		{
			name: "missing framework version in model",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name: "ONNXRuntime",
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			expected: false,
		},
		{
			name: "missing framework version in supportedFormat",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "ONNX",
					Version: ptrToString("1.0.0"),
				},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "ONNXRuntime",
					Version: ptrToString("1.10.0"),
				},
			},
			supportedFormat: v1beta1.SupportedModelFormat{
				Name:        "test-format",
				ModelFormat: &v1beta1.ModelFormat{Name: "ONNX", Version: ptrToString("1.0.0")},
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name: "ONNXRuntime",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareSupportedModelFormats(tt.baseModel, tt.supportedFormat)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

func TestContainsUnofficialVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  modelVer.Version
		expected bool
	}{
		{
			name: "official version",
			version: modelVer.Version{
				Major: 1,
				Minor: 8,
				Patch: 0,
			},
			expected: false,
		},
		{
			name: "with pre-release",
			version: modelVer.Version{
				Major: 1,
				Minor: 8,
				Patch: 0,
				Pre:   []string{"beta"},
			},
			expected: true,
		},
		{
			name: "with build metadata",
			version: modelVer.Version{
				Major: 1,
				Minor: 8,
				Patch: 0,
				Build: []string{"20240707"},
			},
			expected: true,
		},
		{
			name: "with both pre-release and build",
			version: modelVer.Version{
				Major: 1,
				Minor: 8,
				Patch: 0,
				Pre:   []string{"alpha"},
				Build: []string{"20240707"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modelVer.ContainsUnofficialVersion(tt.version)
			assert.Equal(t, tt.expected, result, "Test case: %s", tt.name)
		})
	}
}

// Helper function for creating string pointers
// ptrToString is used to create string pointers for testing
func ptrToString(s string) *string {
	return &s
}

// ptrToModelQuant is used to create ModelQuantization pointers for testing
func ptrToModelQuant(s string) *v1beta1.ModelQuantization {
	mq := v1beta1.ModelQuantization(s)
	return &mq
}

// ptrToRuntimeOp is used to create RuntimeSelectorOperator pointers for testing
func ptrToRuntimeOp(s string) *v1beta1.RuntimeSelectorOperator {
	op := v1beta1.RuntimeSelectorOperator(s)
	return &op
}
