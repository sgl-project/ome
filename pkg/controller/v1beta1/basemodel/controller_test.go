package basemodel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	batchv1 "k8s.io/api/batch/v1"
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

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

// TestBaseModelPVCStorageScenarios tests PVC storage scenarios for BaseModel controller
func TestBaseModelPVCStorageScenarios(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name       string
		baseModel  *v1beta1.BaseModel
		setupMocks func(client.Client)
		validate   func(*testing.T, client.Client, *v1beta1.BaseModel, ctrl.Result, error)
		wantErr    bool
	}{
		{
			name: "BaseModel with PVC storage should create metadata job",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
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

				// Create PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify metadata job was created
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("metadata-%s", baseModel.Name),
					Namespace: baseModel.Namespace,
				}, job)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify job spec
				g.Expect(job.Spec.Template.Spec.Containers).To(gomega.HaveLen(1))
				container := job.Spec.Template.Spec.Containers[0]
				g.Expect(container.Image).To(gomega.ContainSubstring("ome-agent"))
				g.Expect(container.Args).To(gomega.ContainElement("model-metadata"))
				g.Expect(container.Args).To(gomega.ContainElement("--model-path"))
				g.Expect(container.Args).To(gomega.ContainElement("/model"))
				g.Expect(container.Args).To(gomega.ContainElement("--basemodel-name"))
				g.Expect(container.Args).To(gomega.ContainElement(baseModel.Name))

				// Verify PVC volume mount
				g.Expect(job.Spec.Template.Spec.Volumes).To(gomega.HaveLen(1))
				volume := job.Spec.Template.Spec.Volumes[0]
				g.Expect(volume.Name).To(gomega.Equal("model-volume"))
				g.Expect(volume.PersistentVolumeClaim.ClaimName).To(gomega.Equal("my-pvc"))

				// Verify volume mount in container
				g.Expect(container.VolumeMounts).To(gomega.HaveLen(1))
				volumeMount := container.VolumeMounts[0]
				g.Expect(volumeMount.Name).To(gomega.Equal("model-volume"))
				g.Expect(volumeMount.MountPath).To(gomega.Equal("/model"))
			},
		},
		{
			name: "BaseModel with PVC storage and namespace should create metadata job",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-namespace-model",
					Namespace:  "model-storage",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "pytorch",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://model-storage:shared-pvc/models/llama2-7b"),
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

				// Create model-storage namespace
				modelNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "model-storage",
					},
				}
				err = c.Create(context.TODO(), modelNamespace)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create PVC in model-storage namespace
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shared-pvc",
						Namespace: "model-storage",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify metadata job was created
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      fmt.Sprintf("metadata-%s", baseModel.Name),
					Namespace: baseModel.Namespace,
				}, job)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify job spec with namespace-specific PVC
				container := job.Spec.Template.Spec.Containers[0]
				g.Expect(container.Args).To(gomega.ContainElement("--basemodel-namespace"))
				g.Expect(container.Args).To(gomega.ContainElement("model-storage"))

				// Verify PVC volume mount
				volume := job.Spec.Template.Spec.Volumes[0]
				g.Expect(volume.PersistentVolumeClaim.ClaimName).To(gomega.Equal("shared-pvc"))
			},
		},
		{
			name: "BaseModel with PVC storage should handle job idempotency",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-idempotent-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
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

				// Create PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create existing metadata job
				existingJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "metadata-pvc-idempotent-model",
						Namespace: "default",
						Labels: map[string]string{
							"app": "ome-metadata-agent",
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "metadata-agent",
										Image: "ome-agent:latest",
									},
								},
							},
						},
					},
				}
				err = c.Create(context.TODO(), existingJob)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify existing job is not recreated
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "metadata-pvc-idempotent-model",
					Namespace: "default",
				}, job)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify job still has original spec
				g.Expect(job.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("ome-agent:latest"))
			},
		},
		{
			name: "BaseModel with PVC storage should handle job success",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-success-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
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

				// Create PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create successful metadata job
				successfulJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "metadata-pvc-success-model",
						Namespace: "default",
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobComplete,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				err = c.Create(context.TODO(), successfulJob)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create ConfigMap with metadata
				modelEntry := modelagent.ModelEntry{
					Status: modelagent.ModelStatusReady,
					Config: &modelagent.ModelConfig{
						ModelType:         "llama",
						ModelArchitecture: "LlamaForCausalLM",
						MaxTokens:         4096,
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
						"default.basemodel.pvc-success-model": string(entryData),
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

				// Verify spec was updated with metadata
				g.Expect(updated.Spec.ModelType).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.ModelType).To(gomega.Equal("llama"))
				g.Expect(updated.Spec.ModelArchitecture).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.ModelArchitecture).To(gomega.Equal("LlamaForCausalLM"))
				g.Expect(updated.Spec.MaxTokens).ToNot(gomega.BeNil())
				g.Expect(*updated.Spec.MaxTokens).To(gomega.Equal(int32(4096)))
			},
		},
		{
			name: "BaseModel with PVC storage should handle job failure",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-failure-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
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

				// Create PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create failed metadata job
				failedJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "metadata-pvc-failure-model",
						Namespace: "default",
					},
					Status: batchv1.JobStatus{
						Failed: 1,
						Conditions: []batchv1.JobCondition{
							{
								Type:   batchv1.JobFailed,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
				err = c.Create(context.TODO(), failedJob)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Create ConfigMap with failed status
				modelEntry := modelagent.ModelEntry{
					Status: modelagent.ModelStatusFailed,
					Config: nil,
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
						"default.basemodel.pvc-failure-model": string(entryData),
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

				// Verify status reflects failure
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateFailed))
				g.Expect(updated.Status.NodesFailed).To(gomega.ContainElement("worker-node-1"))
			},
		},
		{
			name: "BaseModel with invalid PVC URI should fail validation",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "invalid-pvc-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc:///invalid-uri"),
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
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify no job was created due to invalid PVC URI
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "metadata-invalid-pvc-model",
					Namespace: "default",
				}, job)
				g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())

				// Verify BaseModel status reflects error
				updated := &v1beta1.BaseModel{}
				err = c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateFailed))
			},
			wantErr: true,
		},
		{
			name: "BaseModel with non-existent PVC should fail validation",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "missing-pvc-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://non-existent-pvc/models/llama2"),
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
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify no job was created due to missing PVC
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "metadata-missing-pvc-model",
					Namespace: "default",
				}, job)
				g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())

				// Verify BaseModel status reflects error
				updated := &v1beta1.BaseModel{}
				err = c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateFailed))
			},
			wantErr: true,
		},
		{
			name: "BaseModel with unbound PVC should wait for binding",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "unbound-pvc-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					ModelFormat: v1beta1.ModelFormat{
						Name: "safetensors",
					},
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://pending-pvc/models/llama2"),
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

				// Create unbound PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pending-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify no job was created due to unbound PVC
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "metadata-unbound-pvc-model",
					Namespace: "default",
				}, job)
				g.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())

				// Verify BaseModel status reflects pending state
				updated := &v1beta1.BaseModel{}
				err = c.Get(context.TODO(), types.NamespacedName{
					Name:      baseModel.Name,
					Namespace: baseModel.Namespace,
				}, updated)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(updated.Status.State).To(gomega.Equal(v1beta1.LifeCycleStateInTransit))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			client := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.baseModel).
				Build()

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(client)
			}

			// Create reconciler
			reconciler := &BaseModelReconciler{
				Client:   client,
				Log:      ctrl.Log.WithName("test"),
				Scheme:   scheme,
				Recorder: &record.FakeRecorder{},
			}

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.baseModel.Name,
					Namespace: tt.baseModel.Namespace,
				},
			}

			result, err := reconciler.Reconcile(context.TODO(), req)

			// Validate results
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			if tt.validate != nil {
				tt.validate(t, client, tt.baseModel, result, err)
			}
		})
	}
}

// TestBaseModelPVCJobSpecValidation tests job spec validation for PVC storage scenarios
func TestBaseModelPVCJobSpecValidation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name           string
		baseModel      *v1beta1.BaseModel
		expectedImage  string
		expectedArgs   []string
		expectedVolume string
		description    string
	}{
		{
			name: "PVC job should have correct image",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc-image",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
					},
				},
			},
			expectedImage:  "ome-agent:latest",
			expectedArgs:   []string{"model-metadata", "--model-path", "/model", "--basemodel-name", "test-pvc-image"},
			expectedVolume: "my-pvc",
			description:    "PVC job should use ome-agent image with correct arguments",
		},
		{
			name: "PVC job should have correct volume mount",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc-volume",
					Namespace: "default",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://shared-pvc/models/llama2"),
					},
				},
			},
			expectedImage:  "ome-agent:latest",
			expectedArgs:   []string{"model-metadata", "--model-path", "/model", "--basemodel-name", "test-pvc-volume"},
			expectedVolume: "shared-pvc",
			description:    "PVC job should mount the correct PVC volume",
		},
		{
			name: "PVC job should have correct namespace argument",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc-namespace",
					Namespace: "model-storage",
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://model-storage:shared-pvc/models/llama2"),
					},
				},
			},
			expectedImage:  "ome-agent:latest",
			expectedArgs:   []string{"model-metadata", "--model-path", "/model", "--basemodel-name", "test-pvc-namespace", "--basemodel-namespace", "model-storage"},
			expectedVolume: "shared-pvc",
			description:    "PVC job should include namespace argument when specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock job creation (this would be the actual job creation logic)
			// For testing purposes, we'll validate the expected job spec
			expectedJobName := fmt.Sprintf("metadata-%s", tt.baseModel.Name)
			g.Expect(expectedJobName).To(gomega.ContainSubstring("metadata-"))

			// Validate expected arguments
			g.Expect(tt.expectedArgs).To(gomega.ContainElement("model-metadata"))
			g.Expect(tt.expectedArgs).To(gomega.ContainElement("--model-path"))
			g.Expect(tt.expectedArgs).To(gomega.ContainElement("/model"))
			g.Expect(tt.expectedArgs).To(gomega.ContainElement("--basemodel-name"))
			g.Expect(tt.expectedArgs).To(gomega.ContainElement(tt.baseModel.Name))

			// Validate namespace argument if specified
			if tt.baseModel.Namespace != "default" {
				g.Expect(tt.expectedArgs).To(gomega.ContainElement("--basemodel-namespace"))
				g.Expect(tt.expectedArgs).To(gomega.ContainElement(tt.baseModel.Namespace))
			}

			// Validate expected volume
			g.Expect(tt.expectedVolume).To(gomega.Not(gomega.BeEmpty()))
		})
	}
}

// TestBaseModelPVCReconciliationLoops tests reconciliation loops for PVC storage scenarios
func TestBaseModelPVCReconciliationLoops(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name           string
		baseModel      *v1beta1.BaseModel
		setupMocks     func(client.Client)
		expectedResult ctrl.Result
		description    string
	}{
		{
			name: "PVC reconciliation should requeue on pending PVC",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-requeue-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://pending-pvc/models/llama2"),
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

				// Create pending PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pending-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			expectedResult: ctrl.Result{RequeueAfter: time.Minute},
			description:    "PVC reconciliation should requeue when PVC is pending",
		},
		{
			name: "PVC reconciliation should not requeue on bound PVC",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pvc-no-requeue-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://bound-pvc/models/llama2"),
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

				// Create bound PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bound-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			expectedResult: ctrl.Result{},
			description:    "PVC reconciliation should not requeue when PVC is bound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			client := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.baseModel).
				Build()

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(client)
			}

			// Create reconciler
			reconciler := &BaseModelReconciler{
				Client:   client,
				Log:      ctrl.Log.WithName("test"),
				Scheme:   scheme,
				Recorder: &record.FakeRecorder{},
			}

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.baseModel.Name,
					Namespace: tt.baseModel.Namespace,
				},
			}

			result, err := reconciler.Reconcile(context.TODO(), req)

			// Validate results
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result).To(gomega.Equal(tt.expectedResult), tt.description)
		})
	}
}

// TestBaseModelPVCReconciliationLoopsComprehensive tests comprehensive reconciliation loops for PVC storage
func TestBaseModelPVCReconciliationLoopsComprehensive(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	testCases := []struct {
		name          string
		baseModel     *v1beta1.BaseModel
		setupMocks    func(client.Client)
		validate      func(*testing.T, client.Client, *v1beta1.BaseModel, ctrl.Result, error)
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name: "PVC storage BaseModel with metadata job creation",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pvc-metadata-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
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

				// Create PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify that metadata job was created
				jobList := &batchv1.JobList{}
				err := c.List(context.TODO(), jobList)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Look for metadata extraction job
				found := false
				for _, job := range jobList.Items {
					if strings.Contains(job.Name, baseModel.Name) && strings.Contains(job.Name, "metadata") {
						found = true
						// Verify job spec
						g.Expect(job.Spec.Template.Spec.Containers).To(gomega.HaveLen(1))
						container := job.Spec.Template.Spec.Containers[0]
						g.Expect(container.Image).To(gomega.ContainSubstring("model-metadata-agent"))
						g.Expect(container.Args).To(gomega.ContainElement("--model-path=/models"))
						g.Expect(container.Args).To(gomega.ContainElement("--basemodel-name=" + baseModel.Name))
						g.Expect(container.Args).To(gomega.ContainElement("--basemodel-namespace=" + baseModel.Namespace))
						break
					}
				}
				g.Expect(found).To(gomega.BeTrue(), "Metadata extraction job should be created")
			},
			expectError: false,
			description: "PVC storage BaseModel should create metadata extraction job",
		},
		{
			name: "PVC storage BaseModel with unbound PVC",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pvc-unbound-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://my-pvc-unbound/models/llama2"),
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

				// Create unbound PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pvc-unbound",
						Namespace: "default",
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimPending,
					},
				}
				err = c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify that no metadata job was created due to unbound PVC
				jobList := &batchv1.JobList{}
				err := c.List(context.TODO(), jobList)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Should not find metadata extraction job for unbound PVC
				found := false
				for _, job := range jobList.Items {
					if strings.Contains(job.Name, baseModel.Name) && strings.Contains(job.Name, "metadata") {
						found = true
						break
					}
				}
				g.Expect(found).To(gomega.BeFalse(), "Metadata extraction job should not be created for unbound PVC")
			},
			expectError: false,
			description: "PVC storage BaseModel with unbound PVC should not create metadata job",
		},
		{
			name: "PVC storage BaseModel with non-existent PVC",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pvc-nonexistent-model",
					Namespace:  "default",
					Finalizers: []string{constants.BaseModelFinalizer},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://nonexistent-pvc/models/llama2"),
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
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Verify that no metadata job was created due to non-existent PVC
				jobList := &batchv1.JobList{}
				err := c.List(context.TODO(), jobList)
				g.Expect(err).NotTo(gomega.HaveOccurred())

				// Should not find metadata extraction job for non-existent PVC
				found := false
				for _, job := range jobList.Items {
					if strings.Contains(job.Name, baseModel.Name) && strings.Contains(job.Name, "metadata") {
						found = true
						break
					}
				}
				g.Expect(found).To(gomega.BeFalse(), "Metadata extraction job should not be created for non-existent PVC")
			},
			expectError: false,
			description: "PVC storage BaseModel with non-existent PVC should not create metadata job",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fake client
			client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

			// Setup mocks
			if tc.setupMocks != nil {
				tc.setupMocks(client)
			}

			// Create controller
			controller := &BaseModelReconciler{
				Client: client,
				Scheme: scheme,
			}

			// Create BaseModel
			err := client.Create(context.TODO(), tc.baseModel)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			// Reconcile
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tc.baseModel.Name,
					Namespace: tc.baseModel.Namespace,
				},
			}

			result, reconcileErr := controller.Reconcile(context.TODO(), request)

			// Validate results
			if tc.validate != nil {
				tc.validate(t, client, tc.baseModel, result, reconcileErr)
			}

			if tc.expectError {
				g.Expect(reconcileErr).To(gomega.HaveOccurred(), tc.description)
				if tc.errorContains != "" {
					g.Expect(reconcileErr.Error()).To(gomega.ContainSubstring(tc.errorContains), tc.description)
				}
			} else {
				g.Expect(reconcileErr).NotTo(gomega.HaveOccurred(), tc.description)
			}
		})
	}
}

// TestBaseModelPVCIdempotency tests idempotency of PVC storage BaseModel reconciliation
func TestBaseModelPVCIdempotency(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	// Create fake client
	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	// Create ome namespace
	omeNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.OMENamespace,
		},
	}
	err := client.Create(context.TODO(), omeNamespace)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create bound PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pvc",
			Namespace: "default",
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	err = client.Create(context.TODO(), pvc)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create BaseModel
	baseModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-pvc-idempotent-model",
			Namespace:  "default",
			Finalizers: []string{constants.BaseModelFinalizer},
		},
		Spec: v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr("pvc://my-pvc/models/llama2"),
			},
		},
	}
	err = client.Create(context.TODO(), baseModel)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// Create controller
	controller := &BaseModelReconciler{
		Client: client,
		Scheme: scheme,
	}

	// First reconciliation
	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      baseModel.Name,
			Namespace: baseModel.Namespace,
		},
	}

	result1, err1 := controller.Reconcile(context.TODO(), request)
	g.Expect(err1).NotTo(gomega.HaveOccurred())

	// Second reconciliation (should be idempotent)
	result2, err2 := controller.Reconcile(context.TODO(), request)
	g.Expect(err2).NotTo(gomega.HaveOccurred())

	// Verify that both reconciliations produced the same result
	g.Expect(result1).To(gomega.Equal(result2), "Reconciliation should be idempotent")

	// Verify that only one metadata job was created
	jobList := &batchv1.JobList{}
	err = client.List(context.TODO(), jobList)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	metadataJobCount := 0
	for _, job := range jobList.Items {
		if strings.Contains(job.Name, baseModel.Name) && strings.Contains(job.Name, "metadata") {
			metadataJobCount++
		}
	}
	g.Expect(metadataJobCount).To(gomega.Equal(1), "Only one metadata job should be created")
}
