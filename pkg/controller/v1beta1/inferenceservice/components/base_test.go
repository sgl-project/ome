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
		expectedNodeSelector              map[string]string
	}{
		{
			name: "BaseModel with namespace",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name:      "llama-3-8b",
				Namespace: "default",
			},
			expectedNodeSelector: map[string]string{
				"models.ome.io/default.basemodel.llama-3-8b": "Ready",
			},
		},
		{
			name: "ClusterBaseModel without namespace",
			baseModel: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "safetensors",
				},
			},
			baseModelMeta: &metav1.ObjectMeta{
				Name: "mixtral-8x7b",
				// No namespace for ClusterBaseModel
			},
			expectedNodeSelector: map[string]string{
				"models.ome.io/clusterbasemodel.mixtral-8x7b": "Ready",
			},
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
				"custom-label": "custom-value",
			},
			expectedNodeSelector: map[string]string{
				"custom-label": "custom-value",
				"models.ome.io/test-ns.basemodel.model-1": "Ready",
			},
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
			expectedNodeSelector:              nil, // No node selector should be added
		},
		{
			name:                 "No base model",
			baseModel:            nil,
			baseModelMeta:        nil,
			expectedNodeSelector: nil,
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
			expectedNodeSelector: map[string]string{
				constants.GetBaseModelLabel("long-namespace-name", "very-long-model-name-that-exceeds-normal-length-limits-and-should-be-truncated"): "Ready",
			},
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
			UpdatePodSpecNodeSelector(b, isvc, podSpec)

			// Verify the result
			g.Expect(podSpec.NodeSelector).To(gomega.Equal(tt.expectedNodeSelector))
		})
	}
}

// TestUpdatePodSpecNodeSelector_PVCStorage tests that node selector is skipped for PVC storage
func TestUpdatePodSpecNodeSelector_PVCStorage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name                 string
		storageUri           string
		expectedNodeSelector map[string]string
		description          string
	}{
		{
			name:                 "PVC storage - skip node selector",
			storageUri:           "pvc://model-pvc/models/llama",
			expectedNodeSelector: nil,
			description:          "PVC storage should not add node selector",
		},
		{
			name:                 "PVC storage with namespace - skip node selector",
			storageUri:           "pvc://models:model-pvc/llama-2",
			expectedNodeSelector: nil,
			description:          "PVC storage with namespace should not add node selector",
		},
		{
			name:       "S3 storage - add node selector",
			storageUri: "s3://bucket/model",
			expectedNodeSelector: map[string]string{
				"models.ome.io/default.basemodel.test-model": "Ready",
			},
			description: "Non-PVC storage should add node selector",
		},
		{
			name:       "OCI storage - add node selector",
			storageUri: "oci://registry/model:latest",
			expectedNodeSelector: map[string]string{
				"models.ome.io/default.basemodel.test-model": "Ready",
			},
			description: "OCI storage should add node selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BaseComponentFields{
				BaseModel: &v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: &tt.storageUri,
					},
				},
				BaseModelMeta: &metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Log: ctrl.Log.WithName("test"),
			}

			podSpec := &v1.PodSpec{}
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			}

			UpdatePodSpecNodeSelector(b, isvc, podSpec)

			g.Expect(podSpec.NodeSelector).To(gomega.Equal(tt.expectedNodeSelector), tt.description)
		})
	}
}

// TestUpdatePodSpecVolumes_PVCStorage tests PVC volume creation
func TestUpdatePodSpecVolumes_PVCStorage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name               string
		storageUri         string
		expectedVolumeType string
		expectedPVCName    string
		expectedReadOnly   bool
		description        string
	}{
		{
			name:               "PVC storage - create PVC volume",
			storageUri:         "pvc://model-data-pvc/models/llama-2",
			expectedVolumeType: "pvc",
			expectedPVCName:    "model-data-pvc",
			expectedReadOnly:   true,
			description:        "PVC storage should create PersistentVolumeClaim volume",
		},
		{
			name:               "PVC storage with namespace",
			storageUri:         "pvc://models:shared-pvc/mistral",
			expectedVolumeType: "pvc",
			expectedPVCName:    "shared-pvc",
			expectedReadOnly:   true,
			description:        "PVC with namespace should create PVC volume",
		},
		{
			name:               "HostPath storage for non-PVC",
			storageUri:         "s3://bucket/model",
			expectedVolumeType: "hostpath",
			description:        "Non-PVC storage should create HostPath volume",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelPath := "/mnt/models"
			b := &BaseComponentFields{
				BaseModel: &v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: &tt.storageUri,
						Path:       &modelPath, // Only set for non-PVC
					},
				},
				BaseModelMeta: &metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Log: ctrl.Log.WithName("test"),
			}

			podSpec := &v1.PodSpec{}
			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-isvc",
				},
			}

			UpdatePodSpecVolumes(b, isvc, podSpec, &isvc.ObjectMeta)

			if tt.expectedVolumeType == "pvc" {
				g.Expect(podSpec.Volumes).To(gomega.HaveLen(1), "Should have one volume")
				volume := podSpec.Volumes[0]
				g.Expect(volume.Name).To(gomega.Equal("test-model"))
				g.Expect(volume.PersistentVolumeClaim).ToNot(gomega.BeNil(), tt.description)
				g.Expect(volume.PersistentVolumeClaim.ClaimName).To(gomega.Equal(tt.expectedPVCName))
				g.Expect(volume.PersistentVolumeClaim.ReadOnly).To(gomega.Equal(tt.expectedReadOnly))
				g.Expect(volume.HostPath).To(gomega.BeNil(), "Should not have HostPath for PVC")
			} else if tt.expectedVolumeType == "hostpath" {
				if b.BaseModel.Storage.Path != nil {
					g.Expect(podSpec.Volumes).To(gomega.HaveLen(1), "Should have one volume")
					volume := podSpec.Volumes[0]
					g.Expect(volume.HostPath).ToNot(gomega.BeNil(), tt.description)
					g.Expect(volume.PersistentVolumeClaim).To(gomega.BeNil(), "Should not have PVC for HostPath")
				}
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

	labels := ProcessBaseLabels(b, isvc, v1beta1.EngineComponent, existingLabels)

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
