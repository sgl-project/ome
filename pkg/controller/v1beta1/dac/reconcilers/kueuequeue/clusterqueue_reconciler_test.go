package kueuequeue

import (
	"context"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func TestClusterQueueReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = kueuev1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		queueName      string
		resources      *corev1.ResourceRequirements
		count          int
		existingQueue  *kueuev1beta1.ClusterQueue
		wantErr        bool
		expectedQueue  *kueuev1beta1.ClusterQueue
		expectedUpdate bool
	}{
		{
			name:      "successfully create new ClusterQueue",
			queueName: "test-queue",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30"),
					corev1.ResourceMemory: resource.MustParse("120Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("8"),
				},
			},
			count:          1,
			wantErr:        false,
			expectedQueue:  createTestClusterQueue("test-queue", "60", "240Gi", "16"),
			expectedUpdate: false,
		},
		{
			name:      "update existing ClusterQueue",
			queueName: "existing-queue",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("120Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("2"),
				},
			},
			count: 3,
			existingQueue: &kueuev1beta1.ClusterQueue{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-queue",
				},
				Spec: kueuev1beta1.ClusterQueueSpec{
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         constants.CPUResourceType,
											NominalQuota: resource.MustParse("2"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedQueue:  createTestClusterQueue("existing-queue", "80", "480Gi", "8"),
			expectedUpdate: true,
		},
		{
			name:      "clusterQueue already exists with same spec",
			queueName: "existing-same-queue",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("150Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("4"),
				},
			},
			count:          2,
			existingQueue:  createTestClusterQueue("existing-same-queue", "30", "450Gi", "12"),
			wantErr:        false,
			expectedQueue:  createTestClusterQueue("existing-same-queue", "30", "450Gi", "12"),
			expectedUpdate: false, // Should not update since specs match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fakectrl.NewClientBuilder().WithScheme(scheme).Build()

			// If testing existing queue case, create it first
			if tt.existingQueue != nil {
				err := fakeClient.Create(context.TODO(), tt.existingQueue)
				assert.NoError(t, err)
			}

			// Create reconciler
			reconciler := NewClusterQueueReconciler(fakeClient, scheme, tt.queueName, tt.resources, tt.count)
			// Perform reconciliation
			result, err := reconciler.Reconcile()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify ClusterQueue was created/updated correctly
			createdQueue := &kueuev1beta1.ClusterQueue{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: tt.queueName}, createdQueue)
			assert.NoError(t, err)

			if tt.expectedQueue != nil {
				// Verify specific fields for new creation
				assert.Equal(t, tt.expectedQueue.Name, createdQueue.Name)
				assert.Equal(t, tt.expectedQueue.Spec.NamespaceSelector, createdQueue.Spec.NamespaceSelector)
				assert.Equal(t, tt.expectedQueue.Spec.ResourceGroups[0].CoveredResources,
					createdQueue.Spec.ResourceGroups[0].CoveredResources)

				// Verify resource quotas
				expectedResources := tt.expectedQueue.Spec.ResourceGroups[0].Flavors[0].Resources
				actualResources := createdQueue.Spec.ResourceGroups[0].Flavors[0].Resources
				for i, resource := range expectedResources {
					assert.Equal(t, resource.Name, actualResources[i].Name)
					assert.True(t, resource.NominalQuota.Equal(actualResources[i].NominalQuota))
				}
			}

			if tt.expectedUpdate {
				// Verify the queue was updated
				assert.NotEqual(t, tt.existingQueue.Spec, createdQueue.Spec)
			}
			if !tt.expectedUpdate && tt.existingQueue != nil {
				// Verify for the case clusterqueue already exist with desired spec
				assert.Equal(t, tt.existingQueue.Spec, createdQueue.Spec)
			}
		})
	}
}

func createTestClusterQueue(clusterQueueName string, cpuRequest string, memoryRequest string, gpuRequest string) *kueuev1beta1.ClusterQueue {
	return &kueuev1beta1.ClusterQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterQueueName,
		},
		Spec: kueuev1beta1.ClusterQueueSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					corev1.LabelMetadataName: clusterQueueName,
				},
			},
			ResourceGroups: []kueuev1beta1.ResourceGroup{
				{
					CoveredResources: []corev1.ResourceName{
						constants.NvidiaGPUResourceType,
						constants.CPUResourceType,
						constants.MemoryResourceType,
					},
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "default-flavor",
							Resources: []kueuev1beta1.ResourceQuota{
								{
									Name:         constants.NvidiaGPUResourceType,
									NominalQuota: resource.MustParse(gpuRequest),
								},
								{
									Name:         constants.CPUResourceType,
									NominalQuota: resource.MustParse(cpuRequest),
								},
								{
									Name:         constants.MemoryResourceType,
									NominalQuota: resource.MustParse(memoryRequest),
								},
							},
						},
					},
				},
			},
			QueueingStrategy:  constants.DefaultQueueingStrategy,
			FlavorFungibility: &constants.DefaultFlavorFungibility,
			Preemption:        &constants.DefaultPreemptionConfig,
			StopPolicy:        &constants.DefaultStopPolicy,
		},
	}
}
