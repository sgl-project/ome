package runtimeselector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Helper functions
func createFakeClient() client.Client {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func ptr[T any](v T) *T {
	return &v
}

// Basic selector tests
func TestNewSelector(t *testing.T) {
	// Create a fake client
	fakeClient := fake.NewClientBuilder().Build()

	// Create selector
	selector := New(fakeClient)

	// Verify it's not nil
	assert.NotNil(t, selector)

	// Verify it's the right type
	_, ok := selector.(*defaultSelector)
	assert.True(t, ok)
}

func TestValidateModel(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	selector := New(fakeClient).(*defaultSelector)

	tests := []struct {
		name    string
		model   *v1beta1.BaseModelSpec
		wantErr bool
		errType error
	}{
		{
			name:    "nil model",
			model:   nil,
			wantErr: true,
		},
		{
			name: "empty model format name",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "",
				},
			},
			wantErr: true,
		},
		{
			name: "valid model",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := selector.validateModel(tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, IsModelValidationError(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseModelSize(t *testing.T) {
	tests := []struct {
		name     string
		sizeStr  string
		expected float64
	}{
		{
			name:     "terabytes",
			sizeStr:  "1T",
			expected: 1_000_000_000_000,
		},
		{
			name:     "billions",
			sizeStr:  "7B",
			expected: 7_000_000_000,
		},
		{
			name:     "millions",
			sizeStr:  "350M",
			expected: 350_000_000,
		},
		{
			name:     "decimal billions",
			sizeStr:  "1.5B",
			expected: 1_500_000_000,
		},
		{
			name:     "no suffix",
			sizeStr:  "1000",
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseModelSize(tt.sizeStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetModelFormatLabel(t *testing.T) {
	version := "1.0.0"
	arch := "transformer"
	quant := v1beta1.ModelQuantization("fp16")
	frameworkVersion := "4.0.0"

	tests := []struct {
		name     string
		model    *v1beta1.BaseModelSpec
		expected string
	}{
		{
			name: "basic format",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
			},
			expected: "mt:pytorch",
		},
		{
			name: "format with version",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: &version,
				},
			},
			expected: "mt:pytorch:1.0.0",
		},
		{
			name: "full spec",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name:    "pytorch",
					Version: &version,
				},
				ModelArchitecture: &arch,
				Quantization:      &quant,
				ModelFramework: &v1beta1.ModelFrameworkSpec{
					Name:    "transformers",
					Version: &frameworkVersion,
				},
			},
			expected: "mt:pytorch:1.0.0:transformer:fp16:transformers:4.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getModelFormatLabel(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorTypes(t *testing.T) {
	// Test RuntimeCompatibilityError
	err := &RuntimeCompatibilityError{
		RuntimeName: "test-runtime",
		ModelName:   "test-model",
		ModelFormat: "pytorch",
		Reason:      "version mismatch",
	}
	assert.Contains(t, err.Error(), "test-runtime")
	assert.Contains(t, err.Error(), "test-model")
	assert.True(t, IsRuntimeCompatibilityError(err))

	// Test NoRuntimeFoundError
	noRuntimeErr := &NoRuntimeFoundError{
		ModelName:          "test-model",
		ModelFormat:        "pytorch",
		Namespace:          "default",
		TotalRuntimes:      5,
		NamespacedRuntimes: 2,
		ClusterRuntimes:    3,
	}
	assert.Contains(t, noRuntimeErr.Error(), "no runtime found")
	assert.Contains(t, noRuntimeErr.Error(), "5 runtimes")
	assert.True(t, IsNoRuntimeFoundError(noRuntimeErr))
}

// Compatibility tests ported from runtime_test.go
func TestGetSupportingRuntimes(t *testing.T) {
	// Create a fake fakeClient with our custom types registered
	fakeClient := createFakeClient()

	// Create test base models with different formats and sizes
	baseModels := []struct {
		name  string
		model *v1beta1.BaseModelSpec
	}{
		{
			name: "small-pytorch-model",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: ptr("7B"),
			},
		},
		{
			name: "medium-onnx-model",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "onnx",
				},
				ModelParameterSize: ptr("13B"),
			},
		},
		{
			name: "large-tensorflow-model",
			model: &v1beta1.BaseModelSpec{
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
							Name:   "pytorch",
							Weight: 10,
						},
						AutoSelect: ptr(true),
						Priority:   ptr(int32(2)),
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
							Name:   "onnx",
							Weight: 8,
						},
						AutoSelect: ptr(true),
						Priority:   ptr(int32(1)),
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
							Name:   "tensorflow",
							Weight: 12,
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
							Name:   "pytorch",
							Weight: 5,
						},
						AutoSelect: ptr(true),
						Priority:   ptr(int32(1)),
					},
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "onnx",
							Weight: 5,
						},
						AutoSelect: ptr(true),
						Priority:   ptr(int32(1)),
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-autoselect-rt",
				Namespace: "default",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "pytorch",
							Weight: 20,
						},
						AutoSelect: ptr(false),
					},
				},
			},
		},
	}

	// Create cluster-scoped runtimes
	clusterRuntimes := []*v1beta1.ClusterServingRuntime{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-pytorch-rt",
			},
			Spec: v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "pytorch",
							Weight: 7,
						},
						AutoSelect: ptr(true),
					},
				},
				ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
					Min: ptr("5B"),
					Max: ptr("15B"),
				},
			},
		},
	}

	// Add all runtimes to the fake client
	ctx := context.Background()
	for _, rt := range runtimes {
		assert.NoError(t, fakeClient.Create(ctx, rt))
	}
	for _, rt := range clusterRuntimes {
		assert.NoError(t, fakeClient.Create(ctx, rt))
	}

	// Create selector
	selector := New(fakeClient)

	tests := []struct {
		name                 string
		model                *v1beta1.BaseModelSpec
		expectedRuntimeNames []string
		expectedScores       []int64
		expectError          bool
	}{
		{
			name:  "small pytorch model - multiple compatible runtimes",
			model: baseModels[0].model,
			expectedRuntimeNames: []string{
				"pytorch-rt",         // score: 10 * 2 = 20 (namespace-scoped, higher priority)
				"multi-format-rt",    // score: 5 * 1 = 5 (namespace-scoped)
				"cluster-pytorch-rt", // score: 7 * 1 = 7 (cluster-scoped)
			},
			expectedScores: []int64{20, 5, 7},
		},
		{
			name:                 "medium onnx model",
			model:                baseModels[1].model,
			expectedRuntimeNames: []string{"onnx-rt", "multi-format-rt"},
			expectedScores:       []int64{8, 5},
		},
		{
			name:                 "large tensorflow model",
			model:                baseModels[2].model,
			expectedRuntimeNames: []string{"tensorflow-rt"},
			expectedScores:       []int64{12},
		},
		{
			name: "model with no compatible runtime",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "unknown-format",
				},
			},
			expectedRuntimeNames: []string{},
			expectError:          true,
		},
		{
			name: "model too large for any runtime",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "pytorch",
				},
				ModelParameterSize: ptr("200B"),
			},
			expectedRuntimeNames: []string{},
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := selector.GetCompatibleRuntimes(ctx, tt.model, "default")

			if tt.expectError {
				assert.Empty(t, matches)
				// Try SelectRuntime to get the error
				_, err := selector.SelectRuntime(ctx, tt.model, "default")
				assert.Error(t, err)
				assert.True(t, IsNoRuntimeFoundError(err))
			} else {
				assert.NoError(t, err)
				assert.Len(t, matches, len(tt.expectedRuntimeNames))

				for i, match := range matches {
					assert.Equal(t, tt.expectedRuntimeNames[i], match.Name)
					assert.Equal(t, tt.expectedScores[i], match.Score)
				}
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

	tests := []struct {
		name          string
		runtimes      []RuntimeMatch
		expectedOrder []string
	}{
		{
			name: "sort by score - higher score wins",
			runtimes: []RuntimeMatch{
				{
					RuntimeSelection: RuntimeSelection{
						Name: "low-score-runtime",
						Spec: &v1beta1.ServingRuntimeSpec{
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
						Score: 5,
					},
				},
				{
					RuntimeSelection: RuntimeSelection{
						Name: "high-score-runtime",
						Spec: &v1beta1.ServingRuntimeSpec{
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
						Score: 36, // (10 + 8) * 2
					},
				},
			},
			expectedOrder: []string{"high-score-runtime", "low-score-runtime"},
		},
		{
			name: "sort by model size range when scores are equal",
			runtimes: []RuntimeMatch{
				{
					RuntimeSelection: RuntimeSelection{
						Name: "far-size-runtime",
						Spec: &v1beta1.ServingRuntimeSpec{
							ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
								Min: ptr("20B"),
								Max: ptr("30B"),
							},
						},
						Score: 10,
					},
				},
				{
					RuntimeSelection: RuntimeSelection{
						Name: "close-size-runtime",
						Spec: &v1beta1.ServingRuntimeSpec{
							ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
								Min: ptr("5B"),
								Max: ptr("10B"),
							},
						},
						Score: 10,
					},
				},
			},
			expectedOrder: []string{"close-size-runtime", "far-size-runtime"},
		},
		{
			name: "prefer namespace-scoped over cluster-scoped when equal",
			runtimes: []RuntimeMatch{
				{
					RuntimeSelection: RuntimeSelection{
						Name:      "cluster-runtime",
						IsCluster: true,
						Score:     10,
					},
				},
				{
					RuntimeSelection: RuntimeSelection{
						Name:      "namespace-runtime",
						IsCluster: false,
						Score:     10,
					},
				},
			},
			expectedOrder: []string{"namespace-runtime", "cluster-runtime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewDefaultRuntimeScorer(NewConfig(nil))
			selector := &defaultSelector{
				scorer: scorer,
			}

			selector.sortMatches(tt.runtimes, baseModel)

			got := make([]string, len(tt.runtimes))
			for i, rt := range tt.runtimes {
				got[i] = rt.Name
			}
			assert.Equal(t, tt.expectedOrder, got)
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name          string
		runtime       *v1beta1.ServingRuntimeSpec
		baseModel     *v1beta1.BaseModelSpec
		expectedScore int64
	}{
		{
			name: "exact match with weights and priority",
			runtime: &v1beta1.ServingRuntimeSpec{
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
						Priority:   ptr(int32(3)),
						AutoSelect: ptr(true),
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
			runtime: &v1beta1.ServingRuntimeSpec{
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
						Priority:   ptr(int32(2)),
						AutoSelect: ptr(true),
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
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "TensorFlow",
							Weight: 10,
						},
						Priority:   ptr(int32(5)),
						AutoSelect: ptr(true),
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
			runtime: &v1beta1.ServingRuntimeSpec{
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
						Priority:   ptr(int32(1)),
						AutoSelect: ptr(true),
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
						Priority:   ptr(int32(2)),
						AutoSelect: ptr(true),
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
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "PyTorch",
							Weight: 12,
						},
						AutoSelect: ptr(true),
						// No priority specified, should default to 1
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
			runtime: &v1beta1.ServingRuntimeSpec{
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
						Priority:   ptr(int32(2)),
						AutoSelect: ptr(true),
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
				// ModelFramework is nil
			},
			expectedScore: 0, // model has no modelFramework, runtime has one, so they don't match
		},
		{
			name: "autoselect false - should get 0 score",
			runtime: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{
						ModelFormat: &v1beta1.ModelFormat{
							Name:   "PyTorch",
							Weight: 10,
						},
						Priority:   ptr(int32(2)),
						AutoSelect: ptr(false), // AutoSelect is false
					},
				},
			},
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "PyTorch",
				},
			},
			expectedScore: 0, // AutoSelect is false, so score is 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewDefaultRuntimeScorer(NewConfig(nil))
			actualScore, err := scorer.CalculateScore(tt.runtime, tt.baseModel)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedScore, actualScore, "Score should match expected value")
		})
	}
}
