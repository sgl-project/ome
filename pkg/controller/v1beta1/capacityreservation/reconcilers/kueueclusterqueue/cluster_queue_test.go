package kueueclusterqueue

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
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func TestClusterQueueReconciler_Reconcile(t *testing.T) {
	testScheme := runtime.NewScheme()
	_ = kueuev1beta1.AddToScheme(testScheme)
	_ = corev1.AddToScheme(testScheme)

	tests := []struct {
		name           string
		configMap      *corev1.ConfigMap
		existingCQ     *kueuev1beta1.ClusterQueue
		expectedResult *kueuev1beta1.ClusterQueue
		expectedError  bool
	}{
		{
			name: "Create new ClusterQueue",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.CapacityReservationConfigMapName,
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"clusterQueue": `{"creationFailedTimeThresholdSecond": 30}`,
				},
			},
			existingCQ:     nil,
			expectedResult: &kueuev1beta1.ClusterQueue{ObjectMeta: metav1.ObjectMeta{Name: "test-cq"}},
			expectedError:  false,
		},
		{
			name: "Update existing ClusterQueue",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.CapacityReservationConfigMapName,
					Namespace: constants.OMENamespace,
				},
				Data: map[string]string{
					"clusterQueue": `{"creationFailedTimeThresholdSecond": 30}`,
				},
			},
			existingCQ: &kueuev1beta1.ClusterQueue{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cq",
				},
				Spec: kueuev1beta1.ClusterQueueSpec{
					ResourceGroups: []kueuev1beta1.ResourceGroup{
						{
							CoveredResources: []corev1.ResourceName{},
							Flavors: []kueuev1beta1.FlavorQuotas{
								{
									Name: "flavor1",
									Resources: []kueuev1beta1.ResourceQuota{
										{
											Name:         corev1.ResourceCPU,
											NominalQuota: resource.MustParse("1"),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: &kueuev1beta1.ClusterQueue{ObjectMeta: metav1.ObjectMeta{Name: "test-cq"}},
			expectedError:  false,
		},
		{
			name: "ConfigMap not found",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-name",
					Namespace: constants.OMENamespace,
				},
			},
			existingCQ:     nil,
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := cfake.NewClientBuilder().WithScheme(testScheme)
			if tt.configMap != nil {
				clientBuilder = clientBuilder.WithObjects(tt.configMap)
			}
			if tt.existingCQ != nil {
				clientBuilder = clientBuilder.WithObjects(tt.existingCQ)
			}
			fakeClient := clientBuilder.Build()

			reconciler, err := NewClusterQueueReconciler(
				fakeClient,
				testScheme,
				"test-cq",
				[]kueuev1beta1.ResourceGroup{
					{
						CoveredResources: []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory},
						Flavors: []kueuev1beta1.FlavorQuotas{
							{
								Name: "flavor1",
								Resources: []kueuev1beta1.ResourceQuota{
									{
										Name:         corev1.ResourceCPU,
										NominalQuota: resource.MustParse("1"),
									},
									{
										Name:         corev1.ResourceMemory,
										NominalQuota: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
				constants.DedicatedServingCohort,
				&constants.DefaultPreemptionConfig,
			)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Reconcile
			result, err := reconciler.Reconcile()
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify the result
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedResult.Name, result.Name)

			// Verify the ClusterQueue in the fake client
			clusterQueue := &kueuev1beta1.ClusterQueue{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test-cq"}, clusterQueue)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult.Name, clusterQueue.Name)
		})
	}
}
