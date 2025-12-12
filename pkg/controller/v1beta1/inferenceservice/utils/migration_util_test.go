package utils

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta2 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func TestIsPredictorUsed(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name     string
		isvc     *v1beta2.InferenceService
		expected bool
	}{
		{
			name: "Predictor with model and base model",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Predictor with min replicas",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						ComponentExtensionSpec: v1beta2.ComponentExtensionSpec{
							MinReplicas: intPtr(2),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Predictor with containers",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						PodSpec: v1beta2.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Predictor with worker spec",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Worker: &v1beta2.WorkerSpec{
							Size: intPtr(3),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Empty predictor",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{},
				},
			},
			expected: false,
		},
		{
			name: "Predictor with empty model",
			isvc: &v1beta2.InferenceService{
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{},
					},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsPredictorUsed(test.isvc)
			g.Expect(result).To(gomega.Equal(test.expected))
		})
	}
}

func TestMigratePredictor(t *testing.T) {
	tests := []struct {
		name        string
		isvc        *v1beta2.InferenceService
		validate    func(*testing.T, *v1beta2.InferenceService)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Basic predictor with model",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel:        stringPtr("test-model"),
							FineTunedWeights: []string{"weight1", "weight2"},
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Model).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Model.Name).To(gomega.Equal("test-model"))
				g.Expect(isvc.Spec.Model.FineTunedWeights).To(gomega.Equal([]string{"weight1", "weight2"}))
				g.Expect(*isvc.Spec.Model.Kind).To(gomega.Equal("ClusterBaseModel"))
				g.Expect(*isvc.Spec.Model.APIGroup).To(gomega.Equal("ome.io"))
				g.Expect(isvc.Spec.Engine).NotTo(gomega.BeNil())
			},
		},
		{
			name: "Predictor with runtime",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
							Runtime:   stringPtr("test-runtime"),
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Runtime).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Runtime.Name).To(gomega.Equal("test-runtime"))
				g.Expect(*isvc.Spec.Runtime.Kind).To(gomega.Equal("ClusterServingRuntime"))
				g.Expect(*isvc.Spec.Runtime.APIGroup).To(gomega.Equal("ome.io"))
			},
		},
		{
			name: "Predictor with containers - ome-container",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						PodSpec: v1beta2.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "ome-container",
									Image: "ome-image:latest",
								},
								{
									Name:  "sidecar",
									Image: "sidecar:latest",
								},
							},
						},
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
							PredictorExtensionSpec: v1beta2.PredictorExtensionSpec{
								StorageUri:      stringPtr("gs://bucket/model"),
								ProtocolVersion: protocolVersionPtr(constants.OpenInferenceProtocolV2),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Engine).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Engine.Runner).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Engine.Runner.Name).To(gomega.Equal("ome-container"))
				g.Expect(isvc.Spec.Engine.Runner.Image).To(gomega.Equal("ome-image:latest"))

				// Check environment variables
				g.Expect(isvc.Spec.Engine.Runner.Env).To(gomega.ContainElement(v1.EnvVar{
					Name:  "STORAGE_URI",
					Value: "gs://bucket/model",
				}))
				g.Expect(isvc.Spec.Engine.Runner.Env).To(gomega.ContainElement(v1.EnvVar{
					Name:  "PROTOCOL_VERSION",
					Value: "openInference-v2",
				}))

				// Check sidecar container
				g.Expect(isvc.Spec.Engine.Containers).To(gomega.HaveLen(1))
				g.Expect(isvc.Spec.Engine.Containers[0].Name).To(gomega.Equal("sidecar"))
			},
		},
		{
			name: "Predictor with containers - first container as runner",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						PodSpec: v1beta2.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "predictor",
									Image: "predictor:latest",
								},
								{
									Name:  "sidecar",
									Image: "sidecar:latest",
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Engine.Runner).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Engine.Runner.Name).To(gomega.Equal("predictor"))
				g.Expect(isvc.Spec.Engine.Containers).To(gomega.HaveLen(1))
				g.Expect(isvc.Spec.Engine.Containers[0].Name).To(gomega.Equal("sidecar"))
			},
		},
		{
			name: "Predictor with model container spec",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
							PredictorExtensionSpec: v1beta2.PredictorExtensionSpec{
								Container: v1.Container{
									Name:  "model-container",
									Image: "model:latest",
								},
								StorageUri:      stringPtr("s3://bucket/model"),
								ProtocolVersion: protocolVersionPtr(constants.OpenInferenceProtocolV1),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Engine.Runner).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Engine.Runner.Name).To(gomega.Equal("model-container"))
				g.Expect(isvc.Spec.Engine.Runner.Image).To(gomega.Equal("model:latest"))
				g.Expect(isvc.Spec.Engine.Runner.Env).To(gomega.ContainElement(v1.EnvVar{
					Name:  "STORAGE_URI",
					Value: "s3://bucket/model",
				}))
				g.Expect(isvc.Spec.Engine.Runner.Env).To(gomega.ContainElement(v1.EnvVar{
					Name:  "PROTOCOL_VERSION",
					Value: "openInference-v1",
				}))
			},
		},
		{
			name: "Predictor with worker spec",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
						Worker: &v1beta2.WorkerSpec{
							Size: intPtr(3),
							PodSpec: v1beta2.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "worker",
										Image: "worker:latest",
									},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Engine.Worker).NotTo(gomega.BeNil())
				g.Expect(*isvc.Spec.Engine.Worker.Size).To(gomega.Equal(3))
				g.Expect(isvc.Spec.Engine.Worker.Containers).To(gomega.HaveLen(1))
				g.Expect(isvc.Spec.Engine.Worker.Containers[0].Name).To(gomega.Equal("worker"))
			},
		},
		{
			name: "Predictor with component extension spec",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
						ComponentExtensionSpec: v1beta2.ComponentExtensionSpec{
							MinReplicas: intPtr(2),
							MaxReplicas: 5,
						},
					},
				},
			},
			validate: func(t *testing.T, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				g.Expect(isvc.Spec.Engine.MinReplicas).NotTo(gomega.BeNil())
				g.Expect(*isvc.Spec.Engine.MinReplicas).To(gomega.Equal(2))
				g.Expect(isvc.Spec.Engine.MaxReplicas).To(gomega.Equal(5))
			},
		},
		{
			name: "Predictor with model but model not found",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("non-existent-model"),
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "neither ClusterBaseModel nor BaseModel found with name non-existent-model",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			// Create a fake client with a ClusterBaseModel for testing
			scheme := runtime.NewScheme()
			_ = v1beta2.AddToScheme(scheme)

			// Create a ClusterBaseModel that will be found by DetermineModelKind
			// Only create it if the test ISVC has a model reference AND we don't expect an error
			var fakeClient client.Client
			if !test.expectError && test.isvc.Spec.Predictor.Model != nil && test.isvc.Spec.Predictor.Model.BaseModel != nil {
				clusterBaseModel := &v1beta2.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: *test.isvc.Spec.Predictor.Model.BaseModel,
					},
				}
				fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterBaseModel).Build()
			} else {
				fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			}

			ctx := context.Background()

			err := MigratePredictor(ctx, fakeClient, test.isvc)
			if test.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
				if test.errorMsg != "" {
					g.Expect(err.Error()).To(gomega.ContainSubstring(test.errorMsg))
				}
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				test.validate(t, test.isvc)
			}
		})
	}
}

func TestMigratePredictorToNewArchitecture(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta2.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	tests := []struct {
		name         string
		isvc         *v1beta2.InferenceService
		existingObjs []client.Object
		validate     func(*testing.T, client.Client, *v1beta2.InferenceService)
		expectError  bool
	}{
		{
			name: "Full migration with spec transformation",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
						PodSpec: v1beta2.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "predictor",
									Image: "predictor:latest",
								},
							},
						},
					},
				},
			},
			existingObjs: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc",
						Namespace: "default",
					},
				},
				&v1beta2.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
					},
				},
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				// Check that migration happened
				g.Expect(isvc.Spec.Model).NotTo(gomega.BeNil())
				g.Expect(isvc.Spec.Engine).NotTo(gomega.BeNil())

				// Check deprecation warning
				g.Expect(isvc.Annotations).NotTo(gomega.BeNil())
				g.Expect(isvc.Annotations[constants.DeprecationWarning]).To(gomega.ContainSubstring("deprecated"))

				// Note: Old deployment deletion is now handled by cleanupOldPredictorDeployment
				// in the controller after new component deployments are ready.
				// The migration function only transforms the spec.
				deployment := &appsv1.Deployment{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "test-isvc",
					Namespace: "default",
				}, deployment)
				// Deployment should still exist after migration
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
		},
		{
			name: "No migration when engine already exists",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{
						Model: &v1beta2.ModelSpec{
							BaseModel: stringPtr("test-model"),
						},
					},
					Engine: &v1beta2.EngineSpec{},
				},
			},
			existingObjs: []client.Object{
				&v1beta2.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
					},
				},
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				// Should not have added deprecation warning
				g.Expect(isvc.Annotations).To(gomega.BeNil())
			},
		},
		{
			name: "No migration when predictor not used",
			isvc: &v1beta2.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta2.InferenceServiceSpec{
					Predictor: v1beta2.PredictorSpec{},
				},
			},
			validate: func(t *testing.T, c client.Client, isvc *v1beta2.InferenceService) {
				g := gomega.NewGomegaWithT(t)
				// Should not have any migration
				g.Expect(isvc.Spec.Model).To(gomega.BeNil())
				g.Expect(isvc.Spec.Engine).To(gomega.BeNil())
				g.Expect(isvc.Annotations).To(gomega.BeNil())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			// Create fake client with existing objects
			objs := append(test.existingObjs, test.isvc)
			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Run migration
			logger := logr.Discard()
			err := MigratePredictorToNewArchitecture(context.TODO(), c, logger, test.isvc)

			if test.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				test.validate(t, c, test.isvc)
			}
		})
	}
}

func TestDetermineModelKind(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta2.AddToScheme(scheme)

	tests := []struct {
		name         string
		modelName    string
		namespace    string
		existingObjs []client.Object
		expectedKind string
		expectError  bool
		errorMsg     string
	}{
		{
			name:      "ClusterBaseModel found",
			modelName: "test-model",
			namespace: "default",
			existingObjs: []client.Object{
				&v1beta2.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-model",
					},
				},
			},
			expectedKind: "ClusterBaseModel",
			expectError:  false,
		},
		{
			name:      "BaseModel found",
			modelName: "test-model",
			namespace: "test-ns",
			existingObjs: []client.Object{
				&v1beta2.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "test-ns",
					},
				},
			},
			expectedKind: "BaseModel",
			expectError:  false,
		},
		{
			name:      "Neither ClusterBaseModel nor BaseModel found",
			modelName: "test-model",
			namespace: "default",
			existingObjs: []client.Object{
				&v1beta2.ClusterBaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-embed-model",
					},
				},
			},
			expectError: true,
			errorMsg:    "neither ClusterBaseModel nor BaseModel found with name test-model",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(test.existingObjs...).Build()
			ctx := context.Background()

			kind, err := DetermineModelKind(ctx, fakeClient, test.modelName, test.namespace)

			if test.expectError {
				g.Expect(err).To(gomega.HaveOccurred())
				if test.errorMsg != "" {
					g.Expect(err.Error()).To(gomega.ContainSubstring(test.errorMsg))
				}
				g.Expect(kind).To(gomega.BeEmpty())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(kind).To(gomega.Equal(test.expectedKind))
			}
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func protocolVersionPtr(p constants.InferenceServiceProtocol) *constants.InferenceServiceProtocol {
	return &p
}
