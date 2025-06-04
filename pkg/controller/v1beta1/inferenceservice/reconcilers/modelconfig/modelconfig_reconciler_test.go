package multimodelconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"

	v1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestModelConfigReconciler_Reconcile(t *testing.T) {
	logf.SetLogger(logr.Discard())
	g := gomega.NewWithT(t)

	// Scheme
	s := runtime.NewScheme()
	g.Expect(scheme.AddToScheme(s)).To(gomega.Succeed())
	g.Expect(v1beta1.AddToScheme(s)).To(gomega.Succeed())
	g.Expect(corev1.AddToScheme(s)).To(gomega.Succeed())

	// Common objects
	isvcName := "test-isvc"
	isvcNamespace := "test-ns"
	modelConfigMapName := constants.ModelConfigName(isvcName)

	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		existingObjects []client.Object
		validate        func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error)
		expectError     bool
	}{
		{
			name: "InferenceService with no model spec",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: isvcName, Namespace: isvcNamespace, UID: "isvc-uid"},
				Spec:       v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}}, // No Model field
			},
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				cm := &corev1.ConfigMap{}
				errGet := c.Get(ctx, types.NamespacedName{Name: modelConfigMapName, Namespace: isvcNamespace}, cm)
				g.Expect(errGet).To(gomega.HaveOccurred()) // Expect not found
				g.Expect(client.IgnoreNotFound(errGet)).To(gomega.Succeed())
			},
		},
		{
			name: "Create ConfigMap for BaseModel only",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: isvcName, Namespace: isvcNamespace, UID: "isvc-uid-bm"},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "my-base-model",
						Kind: func(s string) *string { return &s }("BaseModel"),
					},
				},
			},
			existingObjects: []client.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "my-base-model", Namespace: isvcNamespace},
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("oci://bucket/base-model/"),
							Path:       func(s string) *string { return &s }("model.onnx"),
						},
						ModelFormat:    v1beta1.ModelFormat{Name: "onnx"},
						ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "pytorch"},
						ModelType:      func(s string) *string { return &s }("classification"),
					},
				},
			},
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				cm := &corev1.ConfigMap{}
				g.Expect(c.Get(ctx, types.NamespacedName{Name: modelConfigMapName, Namespace: isvcNamespace}, cm)).To(gomega.Succeed())

				g.Expect(cm.OwnerReferences).To(gomega.HaveLen(1))
				g.Expect(cm.OwnerReferences[0].APIVersion).To(gomega.Equal(v1beta1.SchemeGroupVersion.String()))
				g.Expect(cm.OwnerReferences[0].Kind).To(gomega.Equal("InferenceService"))
				g.Expect(cm.OwnerReferences[0].Name).To(gomega.Equal(isvc.Name))
				g.Expect(cm.OwnerReferences[0].UID).To(gomega.Equal(isvc.UID))
				controller := true
				g.Expect(cm.OwnerReferences[0].Controller).To(gomega.Equal(&controller))

				var entries []ModelConfigEntry
				g.Expect(json.Unmarshal([]byte(cm.Data[constants.ModelConfigKey]), &entries)).To(gomega.Succeed())
				g.Expect(entries).To(gomega.HaveLen(1))
				expectedEntry := ModelConfigEntry{
					ModelName: "my-base-model",
					ModelSpec: ModelSpecInfo{
						StorageURI:         func(s string) *string { return &s }("oci://bucket/base-model/"),
						Path:               func(s string) *string { return &s }("model.onnx"),
						ModelFormatName:    func(s string) *string { return &s }("onnx"),
						ModelFrameworkName: func(s string) *string { return &s }("pytorch"),
						ModelType:          func(s string) *string { return &s }("classification"),
					},
				}
				g.Expect(entries[0]).To(gomega.Equal(expectedEntry))
			},
		},
		{
			name: "Create ConfigMap for ClusterBaseModel only",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: isvcName, Namespace: isvcNamespace, UID: "isvc-uid-cbm"},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "my-cluster-base-model",
						Kind: func(s string) *string { return &s }("ClusterBaseModel"), // Or nil, as it defaults to ClusterBaseModel
					},
				},
			},
			existingObjects: []client.Object{
				&v1beta1.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-base-model"}, // No namespace for Cluster CRs
					Spec: v1beta1.BaseModelSpec{
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("s3://cluster-bucket/cluster-model/"),
							Path:       func(s string) *string { return &s }("model.tar.gz"),
							Parameters: &map[string]string{"region": "us-west-2"},
						},
						ModelFormat:    v1beta1.ModelFormat{Name: "tensorflow", Version: func(s string) *string { return &s }("2.7")},
						ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "tfserving", Version: func(s string) *string { return &s }("latest")},
						ModelType:      func(s string) *string { return &s }("regression"),
					},
				},
			},
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				cm := &corev1.ConfigMap{}
				g.Expect(c.Get(ctx, types.NamespacedName{Name: modelConfigMapName, Namespace: isvcNamespace}, cm)).To(gomega.Succeed())

				g.Expect(cm.OwnerReferences).To(gomega.HaveLen(1))
				g.Expect(cm.OwnerReferences[0].APIVersion).To(gomega.Equal(v1beta1.SchemeGroupVersion.String()))
				g.Expect(cm.OwnerReferences[0].Name).To(gomega.Equal(isvc.Name))

				var entries []ModelConfigEntry
				g.Expect(json.Unmarshal([]byte(cm.Data[constants.ModelConfigKey]), &entries)).To(gomega.Succeed())
				g.Expect(entries).To(gomega.HaveLen(1))
				expectedEntry := ModelConfigEntry{
					ModelName: "my-cluster-base-model",
					ModelSpec: ModelSpecInfo{
						StorageURI:            func(s string) *string { return &s }("s3://cluster-bucket/cluster-model/"),
						Path:                  func(s string) *string { return &s }("model.tar.gz"),
						Parameters:            map[string]string{"region": "us-west-2"},
						ModelFormatName:       func(s string) *string { return &s }("tensorflow"),
						ModelFormatVersion:    func(s string) *string { return &s }("2.7"),
						ModelFrameworkName:    func(s string) *string { return &s }("tfserving"),
						ModelFrameworkVersion: func(s string) *string { return &s }("latest"),
						ModelType:             func(s string) *string { return &s }("regression"),
					},
				}
				g.Expect(entries[0]).To(gomega.Equal(expectedEntry))
			},
		},
		{
			name: "Update_ConfigMap_when_BaseModel_changes",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-isvc-update", Namespace: isvcNamespace},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "initial-base-model",
						Kind: func(s string) *string { return &s }("BaseModel"), // Explicitly set Kind
					},
				},
			},
			existingObjects: []client.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "initial-base-model", Namespace: isvcNamespace},
					Spec: v1beta1.BaseModelSpec{
						ModelType:      func(s string) *string { return &s }("INITIAL_MODEL_TYPE"),
						ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "InitialFramework", Version: func(s string) *string { return &s }("1.0")},
						ModelFormat:    v1beta1.ModelFormat{Name: "InitialFormat", Version: func(s string) *string { return &s }("1.0")},
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("oci://initial-bucket/initial-path"),
							SchemaPath: func(s string) *string { return &s }("oci://initial-bucket/initial-schema"),
							Parameters: func(m map[string]string) *map[string]string { return &m }(map[string]string{"initial_param": "initial_value"}),
							StorageKey: func(s string) *string { return &s }("initial-storage-key"),
						},
					},
				},
			},
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, errPassed error) {
				g.Expect(errPassed).NotTo(gomega.HaveOccurred()) // Initial reconcile should succeed
				modelConfigMapName := constants.ModelConfigName(isvc.Name)
				cm := &corev1.ConfigMap{}

				// At this point, the main test loop has already run Reconcile once.
				// Verify initial ConfigMap creation
				g.Expect(c.Get(ctx, types.NamespacedName{Name: modelConfigMapName, Namespace: isvc.Namespace}, cm)).To(gomega.Succeed())

				var entriesInitial []ModelConfigEntry
				g.Expect(json.Unmarshal([]byte(cm.Data[constants.ModelConfigKey]), &entriesInitial)).To(gomega.Succeed())
				g.Expect(entriesInitial).To(gomega.HaveLen(1))
				expectedInitialEntry := ModelConfigEntry{
					ModelName: "initial-base-model",
					ModelSpec: ModelSpecInfo{
						ModelType:             func(s string) *string { return &s }("INITIAL_MODEL_TYPE"),
						StorageURI:            func(s string) *string { return &s }("oci://initial-bucket/initial-path"),
						ModelFrameworkName:    func(s string) *string { return &s }("InitialFramework"),
						ModelFrameworkVersion: func(s string) *string { return &s }("1.0"),
						ModelFormatName:       func(s string) *string { return &s }("InitialFormat"),
						ModelFormatVersion:    func(s string) *string { return &s }("1.0"),
						SchemaPath:            func(s string) *string { return &s }("oci://initial-bucket/initial-schema"),
						Parameters:            map[string]string{"initial_param": "initial_value"},
						StorageKey:            func(s string) *string { return &s }("initial-storage-key"),
					},
				}
				g.Expect(entriesInitial[0]).To(gomega.Equal(expectedInitialEntry))

				// Prepare for update: Create the new BaseModel that ISVC will point to
				updatedBaseModel := &v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "updated-base-model", Namespace: isvcNamespace},
					Spec: v1beta1.BaseModelSpec{
						ModelType:      func(s string) *string { return &s }("UPDATED_MODEL_TYPE"),
						Storage:        &v1beta1.StorageSpec{StorageUri: func(s string) *string { return &s }("oci://updated-bucket/updated-path")},
						ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "UpdatedFramework", Version: func(s string) *string { return &s }("2.0")},
					},
				}
				g.Expect(c.Create(ctx, updatedBaseModel)).To(gomega.Succeed())

				// Fetch the ISVC to update it
				isvcToUpdate := &v1beta1.InferenceService{}
				g.Expect(c.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, isvcToUpdate)).To(gomega.Succeed())
				isvcToUpdate.Spec.Model.Name = "updated-base-model"
				g.Expect(c.Update(ctx, isvcToUpdate)).To(gomega.Succeed())

				// Get the reconciler from the test setup (it's not directly passed to validate func)
				kubeClient := kubefake.NewSimpleClientset() // This is the clientset, not controller-runtime client
				r := NewModelConfigReconciler(c, kubeClient, scheme.Scheme)

				// Second reconcile: This should trigger an update to the ConfigMap
				_, err := r.Reconcile(ctx, isvcToUpdate)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				updatedCm := &corev1.ConfigMap{}
				g.Expect(c.Get(ctx, types.NamespacedName{Name: modelConfigMapName, Namespace: isvc.Namespace}, updatedCm)).To(gomega.Succeed())
				g.Expect(updatedCm.UID).To(gomega.Equal(cm.UID)) // Ensure it's an update, not delete/create, by checking UID

				var entriesUpdated []ModelConfigEntry
				g.Expect(json.Unmarshal([]byte(updatedCm.Data[constants.ModelConfigKey]), &entriesUpdated)).To(gomega.Succeed())
				g.Expect(entriesUpdated).To(gomega.HaveLen(1))
				expectedUpdatedEntry := ModelConfigEntry{
					ModelName: "updated-base-model",
					ModelSpec: ModelSpecInfo{
						ModelType:             func(s string) *string { return &s }("UPDATED_MODEL_TYPE"),
						StorageURI:            func(s string) *string { return &s }("oci://updated-bucket/updated-path"),
						ModelFrameworkName:    func(s string) *string { return &s }("UpdatedFramework"),
						ModelFrameworkVersion: func(s string) *string { return &s }("2.0"),
						Path:                  nil,
						SchemaPath:            nil,
						Parameters:            nil,
						StorageKey:            nil,
						ModelFormatName:       func(s string) *string { return &s }(""),
						ModelFormatVersion:    nil,
					},
				}
				fmt.Printf("DEBUG: entriesUpdated[0].ModelSpec.ModelFormatName = %#v\n", entriesUpdated[0].ModelSpec.ModelFormatName)
				fmt.Printf("DEBUG: expectedUpdatedEntry.ModelSpec.ModelFormatName = %#v\n", expectedUpdatedEntry.ModelSpec.ModelFormatName)
				g.Expect(entriesUpdated[0]).To(gomega.Equal(expectedUpdatedEntry))
			},
		},

		{
			name: "Create_ConfigMap_with_FineTunedWeight",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "fine-tuned-isvc", Namespace: isvcNamespace},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name:             "reference-base-model",
						Kind:             func(s string) *string { return &s }("BaseModel"),
						FineTunedWeights: []string{"my-fine-tuned-weight"},
					},
				},
			},
			existingObjects: []client.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "reference-base-model", Namespace: isvcNamespace},
					Spec: v1beta1.BaseModelSpec{
						ModelType:      func(s string) *string { return &s }("REFERENCE_MODEL_TYPE"),
						ModelFormat:    v1beta1.ModelFormat{Name: "ReferenceFormat", Version: func(s string) *string { return &s }("1.0")},
						ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "ReferenceFramework", Version: func(s string) *string { return &s }("1.0")},
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("s3://base-bucket/base-path"),
						},
					},
				},
				&v1beta1.FineTunedWeight{
					ObjectMeta: metav1.ObjectMeta{Name: "my-fine-tuned-weight"}, // Cluster-scoped resource
					Spec: v1beta1.FineTunedWeightSpec{
						BaseModelRef: v1beta1.ObjectReference{
							Name: func(s string) *string { return &s }("reference-base-model"),
						},
						ModelType: func(s string) *string { return &s }("LoRA"),
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("s3://weights-bucket/weights-path"),
							SchemaPath: func(s string) *string { return &s }("s3://weights-bucket/schema.json"),
							Parameters: func(m map[string]string) *map[string]string { return &m }(map[string]string{"region": "us-east-1"}),
						},
					},
				},
			},
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())

				cm := &corev1.ConfigMap{}
				g.Expect(c.Get(ctx, types.NamespacedName{Name: constants.ModelConfigName(isvc.Name), Namespace: isvcNamespace}, cm)).To(gomega.Succeed())

				var entries []ModelConfigEntry
				g.Expect(json.Unmarshal([]byte(cm.Data[constants.ModelConfigKey]), &entries)).To(gomega.Succeed())
				g.Expect(entries).To(gomega.HaveLen(1))

				expectedEntry := ModelConfigEntry{
					ModelName: "reference-base-model",
					ModelSpec: ModelSpecInfo{
						ModelType:             func(s string) *string { return &s }("REFERENCE_MODEL_TYPE"),
						StorageURI:            func(s string) *string { return &s }("s3://base-bucket/base-path"),
						ModelFormatName:       func(s string) *string { return &s }("ReferenceFormat"),
						ModelFormatVersion:    func(s string) *string { return &s }("1.0"),
						ModelFrameworkName:    func(s string) *string { return &s }("ReferenceFramework"),
						ModelFrameworkVersion: func(s string) *string { return &s }("1.0"),
					},
					FineTunedWeightSpec: &ModelSpecInfo{
						StorageURI: func(s string) *string { return &s }("s3://weights-bucket/weights-path"),
						SchemaPath: func(s string) *string { return &s }("s3://weights-bucket/schema.json"),
						Parameters: map[string]string{"region": "us-east-1"},
						ModelType:  func(s string) *string { return &s }("LoRA"),
					},
				}
				g.Expect(entries[0]).To(gomega.Equal(expectedEntry))
			},
		},
		// Add test for error cases
		{
			name: "Error_when_BaseModel_not_found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "missing-model-isvc", Namespace: isvcNamespace},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name: "non-existent-model",
						Kind: func(s string) *string { return &s }("BaseModel"),
					},
				},
			},
			expectError: true,
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring("failed to get base model spec"))

				// Verify ConfigMap wasn't created
				cm := &corev1.ConfigMap{}
				err = c.Get(ctx, types.NamespacedName{Name: constants.ModelConfigName(isvc.Name), Namespace: isvcNamespace}, cm)
				g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
			},
		},
		{
			name: "Error_when_FineTunedWeight_not_found",
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{Name: "missing-ftw-isvc", Namespace: isvcNamespace},
				Spec: v1beta1.InferenceServiceSpec{
					Model: &v1beta1.ModelRef{
						Name:             "base-model-2",
						Kind:             func(s string) *string { return &s }("BaseModel"),
						FineTunedWeights: []string{"missing-weight"},
					},
				},
			},
			existingObjects: []client.Object{
				&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{Name: "base-model-2", Namespace: isvcNamespace},
					Spec: v1beta1.BaseModelSpec{
						ModelType:   func(s string) *string { return &s }("MODEL_TYPE"),
						ModelFormat: v1beta1.ModelFormat{Name: "Format"},
						Storage: &v1beta1.StorageSpec{
							StorageUri: func(s string) *string { return &s }("s3://base-bucket/path"),
						},
					},
				},
			},
			expectError: true,
			validate: func(ctx context.Context, t *testing.T, g *gomega.WithT, c client.Client, isvc *v1beta1.InferenceService, err error) {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring("failed to get fine tuned weight spec"))

				// Verify ConfigMap wasn't created
				cm := &corev1.ConfigMap{}
				err = c.Get(ctx, types.NamespacedName{Name: constants.ModelConfigName(isvc.Name), Namespace: isvcNamespace}, cm)
				g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			// Create fake client
			clBuilder := fake.NewClientBuilder().WithScheme(s)
			if len(tc.existingObjects) > 0 {
				clBuilder.WithObjects(tc.existingObjects...)
			}
			// Add the ISVC itself to the client for OwnerReference resolution and fetching by reconciler
			clBuilder.WithObjects(tc.isvc)
			c := clBuilder.Build()

			// Create reconciler
			kubeClient := kubefake.NewSimpleClientset()
			reconciler := NewModelConfigReconciler(c, kubeClient, s)

			// Reconcile
			_, err := reconciler.Reconcile(ctx, tc.isvc)

			// Validate
			tc.validate(ctx, t, g, c, tc.isvc, err)
			if tc.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}
		})
	}
}
