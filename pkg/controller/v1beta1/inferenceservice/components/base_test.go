package components

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func TestUpdatePodSpecNodeSelector(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                              string
		baseModel                         *v1beta1.BaseModelSpec
		baseModelMeta                     *metav1.ObjectMeta
		fineTunedServingWithMergedWeights bool
		existingNodeSelector              map[string]string
		expectedLabelKey                  string
		expectNodeSelector                bool
	}{
		{
			name: "BaseModel with namespace adds node selector",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "llama-3-8b",
				Namespace: "default",
			},
			expectedLabelKey:   "models.ome.io/default.basemodel.llama-3-8b",
			expectNodeSelector: true,
		},
		{
			name: "ClusterBaseModel without namespace adds node selector",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "mixtral-8x7b",
				// No namespace for ClusterBaseModel
			},
			expectedLabelKey:   "models.ome.io/clusterbasemodel.mixtral-8x7b",
			expectNodeSelector: true,
		},
		{
			name: "Existing node selector should be preserved",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "model-1",
				Namespace: "test-ns",
			},
			existingNodeSelector: map[string]string{
				"existing-key": "existing-value",
			},
			expectedLabelKey:   "models.ome.io/test-ns.basemodel.model-1",
			expectNodeSelector: true,
		},
		{
			name: "Skip node selector for merged fine-tuned weights",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "base-model",
				Namespace: "default",
			},
			fineTunedServingWithMergedWeights: true,
			expectNodeSelector:                false,
		},
		{
			name:               "No base model",
			baseModel:          nil,
			baseModelMeta:      nil,
			expectNodeSelector: false,
		},
		{
			name: "Long model names should be handled",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "very-long-model-name-that-exceeds-normal-length-limits-and-should-be-truncated",
				Namespace: "long-namespace-name",
			},
			expectedLabelKey:   constants.GetBaseModelLabel("long-namespace-name", "very-long-model-name-that-exceeds-normal-length-limits-and-should-be-truncated"),
			expectNodeSelector: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create BaseComponentFields
			b := &BaseComponentFields{
				BaseModel:                         tt.baseModel,
				BaseModelMeta:                     tt.baseModelMeta,
				FineTunedServingWithMergedWeights: tt.fineTunedServingWithMergedWeights,
				Log:                               ctrl.Log.WithName("test"),
			}

			// Create pod spec with existing node selector if provided
			podSpec := &v1.PodSpec{}
			if tt.existingNodeSelector != nil {
				podSpec.NodeSelector = make(map[string]string)
				for k, v := range tt.existingNodeSelector {
					podSpec.NodeSelector[k] = v
				}
			}

			// Create inference service
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			}

			// Call the function
			UpdatePodSpecNodeSelector(b, isvc, podSpec, "")

			// Verify the result
			if !tt.expectNodeSelector {
				// Should not have added any node selector for model
				if podSpec.NodeSelector == nil {
					return // OK - no node selector added
				}
				// Make sure we didn't add model node selector
				for key := range podSpec.NodeSelector {
					g.Expect(key).NotTo(gomega.HavePrefix("models.ome.io/"))
				}
				return
			}

			// Should have node selector
			g.Expect(podSpec.NodeSelector).NotTo(gomega.BeNil())

			// Check that the expected label key exists with value "Ready"
			value, found := podSpec.NodeSelector[tt.expectedLabelKey]
			g.Expect(found).To(gomega.BeTrue(), "Model node selector label not found: %s", tt.expectedLabelKey)
			g.Expect(value).To(gomega.Equal("Ready"))

			// If there was existing node selector, verify it's preserved
			if tt.existingNodeSelector != nil {
				for k, v := range tt.existingNodeSelector {
					existingValue, existingFound := podSpec.NodeSelector[k]
					g.Expect(existingFound).To(gomega.BeTrue(), "Existing node selector should be preserved")
					g.Expect(existingValue).To(gomega.Equal(v))
				}
				// Should have existing labels + new model label
				g.Expect(podSpec.NodeSelector).To(gomega.HaveLen(len(tt.existingNodeSelector) + 1))
			}
		})
	}
}

func TestProcessBaseLabels(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Test that ProcessBaseLabels adds the correct labels
	b := &BaseComponentFields{
		BaseModel: &v1beta1.BaseModelSpec{
			ModelExtensionSpec: v1beta1.ModelExtensionSpec{
				Vendor: stringPtr("meta"),
			},
		},
		BaseModelMeta: &metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
			Annotations: map[string]string{
				constants.ModelCategoryAnnotation: "LARGE",
			},
		},
		RuntimeName:      "test-runtime",
		FineTunedServing: true,
		Log:              logr.Discard(),
	}

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-isvc",
			Namespace: "default",
		},
	}

	existingLabels := map[string]string{
		"custom-label": "custom-value",
	}

	labels, err := ProcessBaseLabels(b, isvc, v1beta1.EngineComponent, existingLabels)
	g.Expect(err).To(gomega.BeNil())

	// Check expected labels
	g.Expect(labels).To(gomega.HaveKeyWithValue("custom-label", "custom-value"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.InferenceServicePodLabelKey, "test-isvc"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.OMEComponentLabel, "engine"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.ServingRuntimeLabelKey, "test-runtime"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.FTServingLabelKey, "true"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.InferenceServiceBaseModelNameLabelKey, "test-model"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.InferenceServiceBaseModelSizeLabelKey, "LARGE"))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.BaseModelTypeLabelKey, string(constants.ServingBaseModel)))
	g.Expect(labels).To(gomega.HaveKeyWithValue(constants.BaseModelVendorLabelKey, "meta"))
}
