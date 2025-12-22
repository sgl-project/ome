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

func TestGetPVCVolumeInfo(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name             string
		storageSpec      *v1beta1.StorageSpec
		defaultNamespace string
		expectedPVCInfo  *PVCVolumeInfo
	}{
		{
			name:             "nil storage spec",
			storageSpec:      nil,
			defaultNamespace: "default",
			expectedPVCInfo:  nil,
		},
		{
			name: "nil storage uri",
			storageSpec: &v1beta1.StorageSpec{
				Path: stringPtr("/mnt/models"),
			},
			defaultNamespace: "default",
			expectedPVCInfo:  nil,
		},
		{
			name: "non-pvc storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("hf://meta-llama/Llama-3-8B"),
			},
			defaultNamespace: "default",
			expectedPVCInfo:  nil,
		},
		{
			name: "pvc storage uri without namespace",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models/llama"),
			},
			defaultNamespace: "default",
			expectedPVCInfo: &PVCVolumeInfo{
				PVCName:   "my-pvc",
				Namespace: "default",
				SubPath:   "models/llama",
				MountPath: constants.DefaultModelLocalMountPath,
			},
		},
		{
			name: "pvc storage uri with namespace",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://ome:hdml-consumer-pvc/data/hub/models--nvidia--DeepSeek"),
			},
			defaultNamespace: "default",
			expectedPVCInfo: &PVCVolumeInfo{
				PVCName:   "hdml-consumer-pvc",
				Namespace: "ome",
				SubPath:   "data/hub/models--nvidia--DeepSeek",
				MountPath: constants.DefaultModelLocalMountPath,
			},
		},
		{
			name: "pvc storage uri with explicit path",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://ome:my-pvc/models/llama"),
				Path:       stringPtr("/opt/ml/model"),
			},
			defaultNamespace: "default",
			expectedPVCInfo: &PVCVolumeInfo{
				PVCName:   "my-pvc",
				Namespace: "ome",
				SubPath:   "models/llama",
				MountPath: "/opt/ml/model",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPVCVolumeInfo(tt.storageSpec, tt.defaultNamespace)
			if tt.expectedPVCInfo == nil {
				g.Expect(result).To(gomega.BeNil())
			} else {
				g.Expect(result).NotTo(gomega.BeNil())
				g.Expect(result.PVCName).To(gomega.Equal(tt.expectedPVCInfo.PVCName))
				g.Expect(result.Namespace).To(gomega.Equal(tt.expectedPVCInfo.Namespace))
				g.Expect(result.SubPath).To(gomega.Equal(tt.expectedPVCInfo.SubPath))
				g.Expect(result.MountPath).To(gomega.Equal(tt.expectedPVCInfo.MountPath))
			}
		})
	}
}

func TestIsPVCStorage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name        string
		storageSpec *v1beta1.StorageSpec
		expected    bool
	}{
		{
			name:        "nil storage spec",
			storageSpec: nil,
			expected:    false,
		},
		{
			name: "nil storage uri",
			storageSpec: &v1beta1.StorageSpec{
				Path: stringPtr("/mnt/models"),
			},
			expected: false,
		},
		{
			name: "pvc storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models"),
			},
			expected: true,
		},
		{
			name: "hf storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("hf://meta-llama/Llama-3-8B"),
			},
			expected: false,
		},
		{
			name: "oci storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("oci://n/namespace/b/bucket/o/path"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPVCStorage(tt.storageSpec)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}

func TestGetModelMountPath(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name             string
		storageSpec      *v1beta1.StorageSpec
		defaultNamespace string
		expected         string
	}{
		{
			name:             "nil storage spec",
			storageSpec:      nil,
			defaultNamespace: "default",
			expected:         "",
		},
		{
			name: "explicit path takes precedence",
			storageSpec: &v1beta1.StorageSpec{
				Path:       stringPtr("/custom/path"),
				StorageUri: stringPtr("pvc://my-pvc/models"),
			},
			defaultNamespace: "default",
			expected:         "/custom/path",
		},
		{
			name: "pvc storage uses default mount path",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models"),
			},
			defaultNamespace: "default",
			expected:         constants.DefaultModelLocalMountPath,
		},
		{
			name: "non-pvc storage without path returns empty",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("hf://meta-llama/Llama-3-8B"),
			},
			defaultNamespace: "default",
			expected:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelMountPath(tt.storageSpec, tt.defaultNamespace)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}

func TestUpdatePodSpecVolumes_PVC(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name           string
		storageSpec    *v1beta1.StorageSpec
		isvcNamespace  string
		expectedVolume *v1.Volume
	}{
		{
			name: "PVC storage creates PVC volume (same namespace)",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://default:my-pvc/models/llama"),
			},
			isvcNamespace: "default",
			expectedVolume: &v1.Volume{
				Name: "test-model",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "my-pvc",
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "PVC storage without namespace in URI (defaults to isvc namespace)",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models/llama"),
			},
			isvcNamespace: "default",
			expectedVolume: &v1.Volume{
				Name: "test-model",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "my-pvc",
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "HostPath storage creates HostPath volume",
			storageSpec: &v1beta1.StorageSpec{
				Path: stringPtr("/mnt/models/llama"),
			},
			isvcNamespace: "default",
			expectedVolume: &v1.Volume{
				Name: "test-model",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: "/mnt/models/llama",
					},
				},
			},
		},
		{
			name: "No volume for non-PVC storage without path",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: stringPtr("hf://meta-llama/Llama-3-8B"),
			},
			isvcNamespace:  "default",
			expectedVolume: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BaseComponentFields{
				BaseModel: &v1beta1.BaseModelSpec{
					Storage: tt.storageSpec,
				},
				BaseModelMeta: &metav1.ObjectMeta{
					Name: "test-model",
				},
				Log: ctrl.Log.WithName("test"),
			}

			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: tt.isvcNamespace,
				},
			}

			podSpec := &v1.PodSpec{}
			objectMeta := &metav1.ObjectMeta{}

			UpdatePodSpecVolumes(b, isvc, podSpec, objectMeta)

			if tt.expectedVolume == nil {
				// Check that no model volume was added
				for _, vol := range podSpec.Volumes {
					g.Expect(vol.Name).NotTo(gomega.Equal("test-model"))
				}
			} else {
				// Find the model volume
				var foundVolume *v1.Volume
				for i := range podSpec.Volumes {
					if podSpec.Volumes[i].Name == tt.expectedVolume.Name {
						foundVolume = &podSpec.Volumes[i]
						break
					}
				}
				g.Expect(foundVolume).NotTo(gomega.BeNil(), "Model volume not found")
				g.Expect(foundVolume.Name).To(gomega.Equal(tt.expectedVolume.Name))

				if tt.expectedVolume.PersistentVolumeClaim != nil {
					g.Expect(foundVolume.PersistentVolumeClaim).NotTo(gomega.BeNil())
					g.Expect(foundVolume.PersistentVolumeClaim.ClaimName).To(gomega.Equal(tt.expectedVolume.PersistentVolumeClaim.ClaimName))
					g.Expect(foundVolume.PersistentVolumeClaim.ReadOnly).To(gomega.Equal(tt.expectedVolume.PersistentVolumeClaim.ReadOnly))
				}

				if tt.expectedVolume.HostPath != nil {
					g.Expect(foundVolume.HostPath).NotTo(gomega.BeNil())
					g.Expect(foundVolume.HostPath.Path).To(gomega.Equal(tt.expectedVolume.HostPath.Path))
				}
			}
		})
	}
}

func TestValidatePVCNamespace(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name         string
		pvcInfo      *PVCVolumeInfo
		podNamespace string
		expectError  bool
	}{
		{
			name:         "nil pvc info",
			pvcInfo:      nil,
			podNamespace: "default",
			expectError:  false,
		},
		{
			name: "matching namespace",
			pvcInfo: &PVCVolumeInfo{
				PVCName:   "my-pvc",
				Namespace: "default",
			},
			podNamespace: "default",
			expectError:  false,
		},
		{
			name: "empty pvc namespace (defaults to pod namespace)",
			pvcInfo: &PVCVolumeInfo{
				PVCName:   "my-pvc",
				Namespace: "",
			},
			podNamespace: "default",
			expectError:  false,
		},
		{
			name: "mismatched namespace",
			pvcInfo: &PVCVolumeInfo{
				PVCName:   "my-pvc",
				Namespace: "other-namespace",
			},
			podNamespace: "default",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePVCNamespace(tt.pvcInfo, tt.podNamespace)
			if tt.expectError {
				g.Expect(err).NotTo(gomega.BeNil())
				g.Expect(err.Error()).To(gomega.ContainSubstring("namespace mismatch"))
			} else {
				g.Expect(err).To(gomega.BeNil())
			}
		})
	}
}

func TestSanitizeVolumeName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short name unchanged",
			input:    "my-model",
			expected: "my-model",
		},
		{
			name:     "exactly 63 chars unchanged",
			input:    "model-name-that-is-exactly-sixty-three-characters-long-exactly",
			expected: "model-name-that-is-exactly-sixty-three-characters-long-exactly",
		},
		{
			name:     "long name truncated",
			input:    "very-long-model-name-that-exceeds-the-kubernetes-volume-name-limit-of-63-characters",
			expected: "very-long-model-name-that-exceeds-the-kubernetes-volume-racters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeVolumeName(tt.input)
			g.Expect(result).To(gomega.Equal(tt.expected))
			g.Expect(len(result)).To(gomega.BeNumerically("<=", 63))
		})
	}
}

func TestUpdatePodSpecVolumes_PVC_NamespaceMismatch(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Test that PVC volume is NOT added when namespace doesn't match
	b := &BaseComponentFields{
		BaseModel: &v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://other-namespace:my-pvc/models/llama"),
			},
		},
		BaseModelMeta: &metav1.ObjectMeta{
			Name: "test-model",
		},
		Log: ctrl.Log.WithName("test"),
	}

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-isvc",
			Namespace: "default", // Different from PVC namespace
		},
	}

	podSpec := &v1.PodSpec{}
	objectMeta := &metav1.ObjectMeta{}

	UpdatePodSpecVolumes(b, isvc, podSpec, objectMeta)

	// No volume should be added due to namespace mismatch
	g.Expect(podSpec.Volumes).To(gomega.BeEmpty())
}
