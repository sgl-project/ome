package basemodel

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/modelagent"
)

func TestBaseModelReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name       string
		baseModel  *v1beta1.BaseModel
		setupMocks func(client.Client)
		validate   func(*testing.T, client.Client, *v1beta1.BaseModel, ctrl.Result, error)
		wantErr    bool
	}{
		{
			name: "New BaseModel gets finalizer added",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("oci://bucket/model"),
					},
				},
			},
			setupMocks: func(c client.Client) {
				// No setup needed for this test
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Fetch the updated BaseModel
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify finalizer was added
				g.Expect(updated.Finalizers).To(gomega.ContainElement(constants.BaseModelFinalizer))
			},
		},
		{
			name: "BaseModel with ConfigMap status updates to Ready",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "ready-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "pytorch",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create node
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-node-1",
					},
				}
				err = c.Create(context.TODO(), node)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create ConfigMap with Ready status
				modelEntry := modelagent.ModelEntry{
					Status: modelagent.ModelStatusReady,
					Config: &modelagent.ModelConfig{
						ModelType:         "gpt2",
						ModelArchitecture: "GPT2LMHeadModel",
						MaxTokens:         2048,
					},
				}
				entryData, _ := json.Marshal(modelEntry)

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker-node-1",
						Namespace: constants.OMENamespace,
						Labels: map[string]string{
							constants.ModelStatusConfigMapLabel: "true",
						},
					},
					Data: map[string]string{
						"default.basemodel.ready-model": string(entryData),
					},
				}
				err = c.Create(context.TODO(), configMap)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Fetch the updated BaseModel
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify status was updated
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateReady))
				g.Expect(updated.Status.NodesReady).To(gomega.ContainElement("worker-node-1"))
				g.Expect(updated.Status.NodesFailed).To(gomega.BeEmpty())

				// Verify spec was updated with config
				g.Expect(updated.Spec.ModelType).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.ModelType).To(gomega.Equal("gpt2"))
				g.Expect(updated.Spec.ModelArchitecture).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.ModelArchitecture).To(gomega.Equal("GPT2LMHeadModel"))
				g.Expect(updated.Spec.MaxTokens).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.MaxTokens).To(gomega.Equal(int32(2048)))
			},
		},
		{
			name: "BaseModel with multiple nodes - mixed status",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "mixed-status-model",
					Namespace:  "test-ns",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "onnx",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create nodes
				for _, nodeName := range []string{"node-1", "node-2", "node-3"} {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: nodeName,
						},
					}
					err := c.Create(context.TODO(), node)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Create ConfigMaps with different statuses
				statuses := map[string]modelagent.ModelStatus{
					"node-1": modelagent.ModelStatusReady,
					"node-2": modelagent.ModelStatusFailed,
					"node-3": modelagent.ModelStatusUpdating,
				}

				for nodeName, status := range statuses {
					modelEntry := modelagent.ModelEntry{
						Status: status,
					}
					entryData, _ := json.Marshal(modelEntry)

					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: constants.OMENamespace,
							Labels: map[string]string{
								constants.ModelStatusConfigMapLabel: "true",
							},
						},
						Data: map[string]string{
							"test-ns.basemodel.mixed-status-model": string(entryData),
						},
					}
					err := c.Create(context.TODO(), configMap)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Should be Ready because at least one node is ready
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateReady))
				g.Expect(updated.Status.NodesReady).To(gomega.ContainElement("node-1"))
				g.Expect(updated.Status.NodesFailed).To(gomega.ContainElement("node-2"))
				g.Expect(updated.Status.NodesReady).To(gomega.HaveLen(1))
				g.Expect(updated.Status.NodesFailed).To(gomega.HaveLen(1))
			},
		},
		{
			name: "BaseModel deletion removes finalizer when no ConfigMaps exist",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "delete-me-no-configmaps",
					Namespace:         "default",
					Finalizers:        []string{constants.BaseModelFinalizer},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "tensorflow",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace but no ConfigMaps
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)

				// The object should exist (fake client behavior) but finalizer should be removed
				if err == nil {
					// Verify finalizer was removed
					g.Expect(updated.Finalizers).NotTo(gomega.ContainElement(constants.BaseModelFinalizer))
				} else {
					// If object is not found, that's also acceptable as it means deletion completed
					g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())
				}
			},
		},
		{
			name: "BaseModel deletion waits when ConfigMap entries exist but are not deleted",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deletion-waiting-model",
					Namespace:         "default",
					Finalizers:        []string{constants.BaseModelFinalizer},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "pytorch",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create nodes
				for _, nodeName := range []string{"node-1", "node-2"} {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: nodeName,
						},
					}
					err := c.Create(context.TODO(), node)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Create ConfigMaps with entries not yet deleted
				for _, nodeName := range []string{"node-1", "node-2"} {
					modelEntry := modelagent.ModelEntry{
						Status: modelagent.ModelStatusReady, // Not marked for deletion
					}
					entryData, _ := json.Marshal(modelEntry)

					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: constants.OMENamespace,
							Labels: map[string]string{
								constants.ModelStatusConfigMapLabel: "true",
							},
						},
						Data: map[string]string{
							"default.basemodel.deletion-waiting-model": string(entryData),
						},
					}
					err := c.Create(context.TODO(), configMap)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Finalizer should still be present since deletion is waiting for ConfigMaps to be cleared
				g.Expect(updated.Finalizers).To(gomega.ContainElement(constants.BaseModelFinalizer))

				// The reconciler should have set a requeue delay when waiting for ConfigMaps to be cleared
				g.Expect(result.RequeueAfter).To(gomega.Equal(time.Second * 30))
			},
		},
		{
			name: "BaseModel deletion removes finalizer when ConfigMap entries are marked as deleted",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deletion-complete-model",
					Namespace:         "default",
					Finalizers:        []string{constants.BaseModelFinalizer},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "onnx",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create nodes
				for _, nodeName := range []string{"node-1", "node-2"} {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: nodeName,
						},
					}
					err := c.Create(context.TODO(), node)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Create ConfigMaps with entries marked as deleted
				for _, nodeName := range []string{"node-1", "node-2"} {
					modelEntry := modelagent.ModelEntry{
						Status: modelagent.ModelStatusDeleted, // Marked for deletion
					}
					entryData, _ := json.Marshal(modelEntry)

					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: constants.OMENamespace,
							Labels: map[string]string{
								constants.ModelStatusConfigMapLabel: "true",
							},
						},
						Data: map[string]string{
							"default.basemodel.deletion-complete-model": string(entryData),
						},
					}
					err := c.Create(context.TODO(), configMap)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)

				// Finalizer should be removed since all entries are marked as deleted
				if err == nil {
					g.Expect(updated.Finalizers).NotTo(gomega.ContainElement(constants.BaseModelFinalizer))
				} else {
					g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())
				}
			},
		},
		{
			name: "BaseModel deletion with mix of deleted and active entries waits",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "mixed-deletion-model",
					Namespace:         "default",
					Finalizers:        []string{constants.BaseModelFinalizer},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create nodes
				for _, nodeName := range []string{"node-1", "node-2", "node-3"} {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: nodeName,
						},
					}
					err := c.Create(context.TODO(), node)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Create ConfigMaps with mixed deletion status
				statuses := map[string]modelagent.ModelStatus{
					"node-1": modelagent.ModelStatusDeleted, // Marked for deletion
					"node-2": modelagent.ModelStatusReady,   // Not deleted
					"node-3": modelagent.ModelStatusFailed,  // Not deleted
				}

				for nodeName, status := range statuses {
					modelEntry := modelagent.ModelEntry{
						Status: status,
					}
					entryData, _ := json.Marshal(modelEntry)

					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: constants.OMENamespace,
							Labels: map[string]string{
								constants.ModelStatusConfigMapLabel: "true",
							},
						},
						Data: map[string]string{
							"default.basemodel.mixed-deletion-model": string(entryData),
						},
					}
					err := c.Create(context.TODO(), configMap)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Finalizer should still be present since not all entries are marked for deletion
				g.Expect(updated.Finalizers).To(gomega.ContainElement(constants.BaseModelFinalizer))
			},
		},
		{
			name: "BaseModel with deleted node ignores ConfigMap",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "deleted-node-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "huggingface",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ConfigMap for non-existent node
				modelEntry := modelagent.ModelEntry{
					Status: modelagent.ModelStatusReady,
				}
				entryData, _ := json.Marshal(modelEntry)

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deleted-node",
						Namespace: constants.OMENamespace,
						Labels: map[string]string{
							constants.ModelStatusConfigMapLabel: "true",
						},
					},
					Data: map[string]string{
						"default.basemodel.deleted-node-model": string(entryData),
					},
				}
				err := c.Create(context.TODO(), configMap)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				updated := &v1beta1.BaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Should remain in InTransit state (no valid nodes)
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateInTransit))
				g.Expect(updated.Status.NodesReady).To(gomega.BeEmpty())
				g.Expect(updated.Status.NodesFailed).To(gomega.BeEmpty())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create client
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.baseModel).
				WithStatusSubresource(tt.baseModel).
				Build()

			// Setup test mocks
			tt.setupMocks(c)

			// Create recorder
			recorder := record.NewFakeRecorder(10)

			// Run reconciliation
			reconciler := &BaseModelReconciler{
				Client:   c,
				Scheme:   c.Scheme(),
				Recorder: recorder,
			}

			result, err := reconciler.Reconcile(context.TODO(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: tt.baseModel.Namespace,
					Name:      tt.baseModel.Name,
				},
			})
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			// Run validation
			tt.validate(t, c, tt.baseModel, result, err)
		})
	}
}

func TestClusterBaseModelReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name             string
		clusterBaseModel *v1beta1.ClusterBaseModel
		setupMocks       func(client.Client)
		validate         func(*testing.T, client.Client, *v1beta1.ClusterBaseModel)
		wantErr          bool
	}{
		{
			name: "New ClusterBaseModel gets finalizer",
			clusterBaseModel: &v1beta1.ClusterBaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-model",
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "gguf",
					},
				},
			},
			setupMocks: func(c client.Client) {},
			validate: func(t *testing.T, c client.Client, clusterBaseModel *v1beta1.ClusterBaseModel) {
				updated := &v1beta1.ClusterBaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name: clusterBaseModel.Name,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(updated.Finalizers).To(gomega.ContainElement(constants.ClusterBaseModelFinalizer))
			},
		},
		{
			name: "ClusterBaseModel status update from multiple nodes",
			clusterBaseModel: &v1beta1.ClusterBaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "multi-node-cluster-model",
					Finalizers: []string{constants.ClusterBaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "vllm",
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create nodes
				for i := 1; i <= 3; i++ {
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("cluster-node-%d", i),
						},
					}
					err := c.Create(context.TODO(), node)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}

				// Create ConfigMaps - all ready
				for i := 1; i <= 3; i++ {
					modelEntry := modelagent.ModelEntry{
						Status: modelagent.ModelStatusReady,
						Config: &modelagent.ModelConfig{
							ModelFramework: map[string]string{
								"name":    "transformers",
								"version": "4.21.0",
							},
						},
					}
					entryData, _ := json.Marshal(modelEntry)

					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("cluster-node-%d", i),
							Namespace: constants.OMENamespace,
							Labels: map[string]string{
								constants.ModelStatusConfigMapLabel: "true",
							},
						},
						Data: map[string]string{
							"clusterbasemodel.multi-node-cluster-model": string(entryData),
						},
					}
					err := c.Create(context.TODO(), configMap)
					g.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},
			validate: func(t *testing.T, c client.Client, clusterBaseModel *v1beta1.ClusterBaseModel) {
				updated := &v1beta1.ClusterBaseModel{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name: clusterBaseModel.Name,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateReady))
				g.Expect(updated.Status.NodesReady).To(gomega.HaveLen(3))
				g.Expect(updated.Status.NodesReady).To(gomega.ContainElements("cluster-node-1", "cluster-node-2", "cluster-node-3"))

				// Verify spec updates from config
				g.Expect(updated.Spec.ModelFramework).ToNot(gomega.BeNil())
				g.Expect(updated.Spec.ModelFramework.Name).To(gomega.Equal("transformers"))
				g.Expect(updated.Spec.ModelFramework.Version).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.ModelFramework.Version).To(gomega.Equal("4.21.0"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.clusterBaseModel).
				WithStatusSubresource(tt.clusterBaseModel).
				Build()

			if tt.setupMocks != nil {
				tt.setupMocks(c)
			}

			recorder := record.NewFakeRecorder(10)
			reconciler := &ClusterBaseModelReconciler{
				Client:   c,
				Log:      ctrl.Log.WithName("test"),
				Scheme:   scheme,
				Recorder: recorder,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: tt.clusterBaseModel.Name,
				},
			}
			result, err := reconciler.Reconcile(context.TODO(), req)

			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(result).To(gomega.Equal(ctrl.Result{}))

				if tt.validate != nil {
					tt.validate(t, c, tt.clusterBaseModel)
				}
			}
		})
	}
}

func TestMapConfigMapToModelRequests(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	logger := ctrl.Log.WithName("test")

	tests := []struct {
		name          string
		configMap     *corev1.ConfigMap
		keyPrefix     string
		isNamespaced  bool
		expectedCount int
		expectedFirst *types.NamespacedName
	}{
		{
			name: "BaseModel mapping",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"default.basemodel.my-model":     `{"status":"Ready"}`,
					"test-ns.basemodel.other-model":  `{"status":"Failed"}`,
					"clusterbasemodel.cluster-model": `{"status":"Ready"}`, // Should be ignored
				},
			},
			keyPrefix:     "basemodel",
			isNamespaced:  true,
			expectedCount: 2,
			expectedFirst: nil, // Don't check specific order since map iteration is random
		},
		{
			name: "ClusterBaseModel mapping",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-node",
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"clusterbasemodel.global-model":    `{"status":"Ready"}`,
					"clusterbasemodel.multi.part.name": `{"status":"InTransit"}`,
					"basemodel.default.local-model":    `{"status":"Ready"}`, // Should be ignored
				},
			},
			keyPrefix:     "clusterbasemodel",
			isNamespaced:  false,
			expectedCount: 2,
			expectedFirst: nil, // Don't check specific order since map iteration is random
		},
		{
			name: "Invalid JSON data",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-node",
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"default.basemodel.broken-model": `{invalid json}`,
					"default.basemodel.valid-model":  `{"status":"Ready"}`,
				},
			},
			keyPrefix:     "basemodel",
			isNamespaced:  true,
			expectedCount: 1, // Only valid entry should be processed
			expectedFirst: &types.NamespacedName{
				Namespace: "default",
				Name:      "valid-model",
			},
		},
		{
			name:          "Non-ConfigMap object",
			configMap:     nil, // Will pass a different object type
			keyPrefix:     "basemodel",
			isNamespaced:  true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []reconcile.Request

			if tt.configMap != nil {
				if tt.isNamespaced {
					// Test BaseModel mapping
					reconciler := &BaseModelReconciler{Log: logger}
					requests = reconciler.mapConfigMapToBaseModels(tt.configMap)
				} else {
					// Test ClusterBaseModel mapping
					reconciler := &ClusterBaseModelReconciler{Log: logger}
					requests = reconciler.mapConfigMapToClusterBaseModels(tt.configMap)
				}
			} else {
				// Pass a non-ConfigMap object
				reconciler := &BaseModelReconciler{Log: logger}
				requests = reconciler.mapConfigMapToBaseModels(&corev1.Pod{})
			}

			g.Expect(requests).To(gomega.HaveLen(tt.expectedCount))

			if tt.expectedFirst != nil && len(requests) > 0 {
				g.Expect(requests[0].NamespacedName).To(gomega.Equal(*tt.expectedFirst))
			} else if tt.expectedCount > 0 {
				// Instead of checking order, verify that all expected requests are present
				// For BaseModel mapping case
				if tt.name == "BaseModel mapping" {
					foundDefault := false
					foundTestNs := false

					for _, req := range requests {
						if req.NamespacedName.Namespace == "default" && req.NamespacedName.Name == "my-model" {
							foundDefault = true
						}
						if req.NamespacedName.Namespace == "test-ns" && req.NamespacedName.Name == "other-model" {
							foundTestNs = true
						}
					}

					g.Expect(foundDefault).To(gomega.BeTrue(), "Should find default.my-model")
					g.Expect(foundTestNs).To(gomega.BeTrue(), "Should find test-ns.other-model")
				}
			}
		})
	}
}

func TestUpdateSpecWithConfig(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name         string
		initialSpec  *v1beta1.BaseModelSpec
		config       *modelagent.ModelConfig
		expectUpdate bool
		validateSpec func(*v1beta1.BaseModelSpec)
	}{
		{
			name: "Complete config update",
			initialSpec: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{
					Name: "", // Empty so it can be updated
				},
			},
			config: &modelagent.ModelConfig{
				ModelType:          "llama",
				ModelArchitecture:  "LlamaForCausalLM",
				ModelParameterSize: "7B",
				ModelCapabilities:  []string{"TEXT_GENERATION", "CHAT"},
				ModelFramework: map[string]string{
					"name":    "transformers",
					"version": "4.21.0",
				},
				ModelFormat: map[string]string{
					"name":    "safetensors",
					"version": "0.3.0",
				},
				MaxTokens: 4096,
			},
			expectUpdate: true,
			validateSpec: func(spec *v1beta1.BaseModelSpec) {
				g.Expect(spec.ModelType).ToNot(gomega.BeNil())
				g.Expect(*spec.ModelType).To(gomega.Equal("llama"))
				g.Expect(spec.ModelArchitecture).ToNot(gomega.BeNil())
				g.Expect(*spec.ModelArchitecture).To(gomega.Equal("LlamaForCausalLM"))
				g.Expect(spec.ModelParameterSize).ToNot(gomega.BeNil())
				g.Expect(*spec.ModelParameterSize).To(gomega.Equal("7B"))
				g.Expect(spec.ModelCapabilities).To(gomega.Equal([]string{"TEXT_GENERATION", "CHAT"}))
				g.Expect(spec.ModelFramework).ToNot(gomega.BeNil())
				g.Expect(spec.ModelFramework.Name).To(gomega.Equal("transformers"))
				g.Expect(*spec.ModelFramework.Version).To(gomega.Equal("4.21.0"))
				g.Expect(spec.ModelFormat.Name).To(gomega.Equal("safetensors"))
				g.Expect(*spec.ModelFormat.Version).To(gomega.Equal("0.3.0"))
				g.Expect(spec.MaxTokens).ToNot(gomega.BeNil())
				g.Expect(*spec.MaxTokens).To(gomega.Equal(int32(4096)))
			},
		},
		{
			name: "No update when fields already set",
			initialSpec: &v1beta1.BaseModelSpec{
				ModelType:         stringPtr("existing-type"),
				ModelArchitecture: stringPtr("existing-arch"),
				ModelFormat: v1beta1.ModelFormat{
					Name:    "existing-format",
					Version: stringPtr("1.0.0"),
				},
				MaxTokens: int32Ptr(2048),
			},
			config: &modelagent.ModelConfig{
				ModelType:         "new-type",
				ModelArchitecture: "new-arch",
				ModelFormat: map[string]string{
					"name":    "new-format",
					"version": "2.0.0",
				},
				MaxTokens: 4096,
			},
			expectUpdate: false,
			validateSpec: func(spec *v1beta1.BaseModelSpec) {
				// Values should remain unchanged
				g.Expect(*spec.ModelType).To(gomega.Equal("existing-type"))
				g.Expect(*spec.ModelArchitecture).To(gomega.Equal("existing-arch"))
				g.Expect(spec.ModelFormat.Name).To(gomega.Equal("existing-format"))
				g.Expect(*spec.ModelFormat.Version).To(gomega.Equal("1.0.0"))
				g.Expect(*spec.MaxTokens).To(gomega.Equal(int32(2048)))
			},
		},
		{
			name:         "Nil inputs return false",
			initialSpec:  nil,
			config:       &modelagent.ModelConfig{},
			expectUpdate: false,
		},
		{
			name: "Nil config returns false",
			initialSpec: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "test"},
			},
			config:       nil,
			expectUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := updateSpecWithConfig(tt.initialSpec, tt.config)
			g.Expect(updated).To(gomega.Equal(tt.expectUpdate))

			if tt.validateSpec != nil && tt.initialSpec != nil {
				tt.validateSpec(tt.initialSpec)
			}
		})
	}
}

func TestCalculateLifecycleState(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name          string
		nodesReady    []string
		nodesFailed   []string
		expectedState v1beta1.LifeCycleState
	}{
		{
			name:          "Ready state with ready nodes",
			nodesReady:    []string{"node1", "node2"},
			nodesFailed:   []string{},
			expectedState: v1beta1.LifeCycleStateReady,
		},
		{
			name:          "Ready state with mixed nodes",
			nodesReady:    []string{"node1"},
			nodesFailed:   []string{"node2"},
			expectedState: v1beta1.LifeCycleStateReady,
		},
		{
			name:          "Failed state with only failed nodes",
			nodesReady:    []string{},
			nodesFailed:   []string{"node1", "node2"},
			expectedState: v1beta1.LifeCycleStateFailed,
		},
		{
			name:          "InTransit state with no nodes",
			nodesReady:    []string{},
			nodesFailed:   []string{},
			expectedState: v1beta1.LifeCycleStateInTransit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := calculateLifecycleState(tt.nodesReady, tt.nodesFailed)
			g.Expect(state).To(gomega.Equal(tt.expectedState))
		})
	}
}

func TestCreateModelStatusConfigMapPredicate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	predicate := createModelStatusConfigMapPredicate()

	tests := []struct {
		name     string
		obj      client.Object
		expected bool
	}{
		{
			name: "Valid model status ConfigMap",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: constants.OMENamespace,
					Labels: map[string]string{
						constants.ModelStatusConfigMapLabel: "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "ConfigMap in wrong namespace",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "wrong-namespace",
					Labels: map[string]string{
						constants.ModelStatusConfigMapLabel: "true",
					},
				},
			},
			expected: false,
		},
		{
			name: "ConfigMap without label",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: constants.OMENamespace,
				},
			},
			expected: false,
		},
		{
			name: "ConfigMap with wrong label value",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: constants.OMENamespace,
					Labels: map[string]string{
						constants.ModelStatusConfigMapLabel: "false",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CreateFunc
			result := predicate.Create(event.TypedCreateEvent[client.Object]{Object: tt.obj})
			g.Expect(result).To(gomega.Equal(tt.expected))

			// Test UpdateFunc
			result = predicate.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: tt.obj})
			g.Expect(result).To(gomega.Equal(tt.expected))

			// Test DeleteFunc
			result = predicate.Delete(event.TypedDeleteEvent[client.Object]{Object: tt.obj})
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}

func TestAddToSlice(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name     string
		slice    []string
		item     string
		expected []string
	}{
		{
			name:     "Add to empty slice",
			slice:    []string{},
			item:     "item1",
			expected: []string{"item1"},
		},
		{
			name:     "Add new item",
			slice:    []string{"item1", "item2"},
			item:     "item3",
			expected: []string{"item1", "item2", "item3"},
		},
		{
			name:     "Don't add existing item",
			slice:    []string{"item1", "item2"},
			item:     "item1",
			expected: []string{"item1", "item2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addToSlice(tt.slice, tt.item)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}

func TestCreateNodeDeletionPredicate(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	pred := createNodeDeletionPredicate()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}

	// CreateFunc should return false
	createResult := pred.Create(event.TypedCreateEvent[client.Object]{Object: node})
	g.Expect(createResult).To(gomega.BeFalse(), "CreateFunc should return false")

	// UpdateFunc should return false
	updateResult := pred.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: node, ObjectOld: node})
	g.Expect(updateResult).To(gomega.BeFalse(), "UpdateFunc should return false")

	// DeleteFunc should return true
	deleteResult := pred.Delete(event.TypedDeleteEvent[client.Object]{Object: node})
	g.Expect(deleteResult).To(gomega.BeTrue(), "DeleteFunc should return true")
}

func TestHandleNodeDeletion(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name       string
		nodeName   string
		setupMocks func(client.Client)
		validate   func(*testing.T, client.Client, string)
	}{
		{
			name:     "Node deletion cleans up associated ConfigMap",
			nodeName: "node-with-configmap",
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create a model status ConfigMap for this node
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-with-configmap",
						Namespace: constants.OMENamespace,
						Labels: map[string]string{
							constants.ModelStatusConfigMapLabel: "true",
						},
					},
					Data: map[string]string{
						"clusterbasemodel.test-model": `{"status":"Ready"}`,
					},
				}
				err = c.Create(context.TODO(), configMap)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, nodeName string) {
				// ConfigMap should be deleted
				configMap := &corev1.ConfigMap{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Namespace: constants.OMENamespace,
					Name:      nodeName,
				}, configMap)
				g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue(), "ConfigMap should be deleted")
			},
		},
		{
			name:     "Node deletion with no ConfigMap does nothing",
			nodeName: "node-without-configmap",
			setupMocks: func(c client.Client) {
				// Create ome namespace but no ConfigMap
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, nodeName string) {
				// No ConfigMap to check - just ensure no error occurred
				// The function should silently skip
			},
		},
		{
			name:     "Node deletion skips non-model-status ConfigMap",
			nodeName: "node-with-other-configmap",
			setupMocks: func(c client.Client) {
				// Create ome namespace
				omeNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: constants.OMENamespace,
					},
				}
				err := c.Create(context.TODO(), omeNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create a ConfigMap without model status label
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-with-other-configmap",
						Namespace: constants.OMENamespace,
						// No model status label
					},
					Data: map[string]string{
						"some-key": "some-value",
					},
				}
				err = c.Create(context.TODO(), configMap)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, nodeName string) {
				// ConfigMap should NOT be deleted (it's not a model status ConfigMap)
				configMap := &corev1.ConfigMap{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Namespace: constants.OMENamespace,
					Name:      nodeName,
				}, configMap)
				g.Expect(err).NotTo(gomega.HaveOccurred(), "ConfigMap should still exist")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			tt.setupMocks(c)

			log := ctrl.Log.WithName("test")

			// Create the node object that was "deleted"
			deletedNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.nodeName,
				},
			}

			// Call handleNodeDeletion (shared function)
			requests := handleNodeDeletion(context.TODO(), c, log, deletedNode)

			// Should return nil (no reconcile requests needed)
			g.Expect(requests).To(gomega.BeNil())

			// Validate the result
			tt.validate(t, c, tt.nodeName)
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}
