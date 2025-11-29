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
		existingAffinity                  *v1.Affinity
		expectedLabelKey                  string
		expectAffinity                    bool
	}{
		{
			name: "BaseModel with namespace adds preferred affinity",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "llama-3-8b",
				Namespace: "default",
			},
			expectedLabelKey: "models.ome.io/default.basemodel.llama-3-8b",
			expectAffinity:   true,
		},
		{
			name: "ClusterBaseModel without namespace adds preferred affinity",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "mixtral-8x7b",
				// No namespace for ClusterBaseModel
			},
			expectedLabelKey: "models.ome.io/clusterbasemodel.mixtral-8x7b",
			expectAffinity:   true,
		},
		{
			name: "Existing affinity should be preserved",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "model-1",
				Namespace: "test-ns",
			},
			existingAffinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
						{
							Weight: 50,
							Preference: v1.NodeSelectorTerm{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "existing-key",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{"existing-value"},
									},
								},
							},
						},
					},
				},
			},
			expectedLabelKey: "models.ome.io/test-ns.basemodel.model-1",
			expectAffinity:   true,
		},
		{
			name: "Skip affinity for merged fine-tuned weights",
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
			expectAffinity:                    false,
		},
		{
			name:           "No base model",
			baseModel:      nil,
			baseModelMeta:  nil,
			expectAffinity: false,
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
			expectedLabelKey: constants.GetBaseModelLabel("long-namespace-name", "very-long-model-name-that-exceeds-normal-length-limits-and-should-be-truncated"),
			expectAffinity:   true,
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

			// Create pod spec with existing affinity if provided
			podSpec := &v1.PodSpec{}
			if tt.existingAffinity != nil {
				podSpec.Affinity = tt.existingAffinity.DeepCopy()
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
			if !tt.expectAffinity {
				// Should not have added any affinity for model
				if podSpec.Affinity == nil {
					return // OK - no affinity added
				}
				if podSpec.Affinity.NodeAffinity == nil {
					return // OK - no node affinity added
				}
				// If there's existing affinity, make sure we didn't add model affinity
				for _, term := range podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
					for _, expr := range term.Preference.MatchExpressions {
						g.Expect(expr.Key).NotTo(gomega.HavePrefix("models.ome.io/"))
					}
				}
				return
			}

			// Should have preferred node affinity
			g.Expect(podSpec.Affinity).NotTo(gomega.BeNil())
			g.Expect(podSpec.Affinity.NodeAffinity).NotTo(gomega.BeNil())
			g.Expect(podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).NotTo(gomega.BeEmpty())

			// Find the model affinity term
			var foundModelTerm *v1.PreferredSchedulingTerm
			for i := range podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				term := &podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[i]
				for _, expr := range term.Preference.MatchExpressions {
					if expr.Key == tt.expectedLabelKey {
						foundModelTerm = term
						break
					}
				}
				if foundModelTerm != nil {
					break
				}
			}

			g.Expect(foundModelTerm).NotTo(gomega.BeNil(), "Model affinity term not found")
			g.Expect(foundModelTerm.Weight).To(gomega.Equal(int32(100)))
			g.Expect(foundModelTerm.Preference.MatchExpressions).To(gomega.HaveLen(1))
			g.Expect(foundModelTerm.Preference.MatchExpressions[0].Key).To(gomega.Equal(tt.expectedLabelKey))
			g.Expect(foundModelTerm.Preference.MatchExpressions[0].Operator).To(gomega.Equal(v1.NodeSelectorOpIn))
			g.Expect(foundModelTerm.Preference.MatchExpressions[0].Values).To(gomega.Equal([]string{"Ready"}))

			// If there was existing affinity, verify it's preserved
			if tt.existingAffinity != nil {
				existingTermCount := len(tt.existingAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
				// Should have existing terms + new model term
				g.Expect(podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(gomega.HaveLen(existingTermCount + 1))
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
