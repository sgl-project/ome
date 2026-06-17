package dac

import (
	"context"
	"testing"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	controllerconfig "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	opensourcev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestGetDesiredReservationReplicaCount(t *testing.T) {
	tests := []struct {
		name             string
		dac              *omev1beta1.DedicatedAICluster
		profile          *omev1beta1.DedicatedAIClusterProfile
		isvcs            []opensourcev1beta1.InferenceService
		reservationCount int
		expectedCount    int
	}{
		{
			name: "restores full reservation count when no isvcs remain for two-node profile",
			dac: &omev1beta1.DedicatedAICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dac",
				},
				Spec: omev1beta1.DedicatedAIClusterSpec{
					Profile: "h100-x16",
				},
			},
			profile: &omev1beta1.DedicatedAIClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h100-x16",
				},
				Spec: omev1beta1.DedicatedAIClusterProfileSpec{
					Count: 2,
				},
			},
			reservationCount: 2,
			expectedCount:    2,
		},
		{
			name: "subtracts one full two-node reservation for a single isvc replica",
			dac: &omev1beta1.DedicatedAICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dac",
				},
				Spec: omev1beta1.DedicatedAIClusterSpec{
					Profile: "h100-x16",
				},
			},
			profile: &omev1beta1.DedicatedAIClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h100-x16",
				},
				Spec: omev1beta1.DedicatedAIClusterProfileSpec{
					Count: 2,
				},
			},
			isvcs: []opensourcev1beta1.InferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "isvc-0",
						Namespace: "test-dac",
					},
					Spec: opensourcev1beta1.InferenceServiceSpec{
						Engine: &opensourcev1beta1.EngineSpec{
							ComponentExtensionSpec: opensourcev1beta1.ComponentExtensionSpec{
								MaxReplicas: 1,
							},
						},
					},
				},
			},
			reservationCount: 2,
			expectedCount:    0,
		},
		{
			name: "multiplies two-node profile count by isvc max replicas",
			dac: &omev1beta1.DedicatedAICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dac",
				},
				Spec: omev1beta1.DedicatedAIClusterSpec{
					Profile: "h100-x16",
				},
			},
			profile: &omev1beta1.DedicatedAIClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h100-x16",
				},
				Spec: omev1beta1.DedicatedAIClusterProfileSpec{
					Count: 2,
				},
			},
			isvcs: []opensourcev1beta1.InferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "isvc-0",
						Namespace: "test-dac",
					},
					Spec: opensourcev1beta1.InferenceServiceSpec{
						Engine: &opensourcev1beta1.EngineSpec{
							ComponentExtensionSpec: opensourcev1beta1.ComponentExtensionSpec{
								MaxReplicas: 2,
							},
						},
					},
				},
			},
			reservationCount: 4,
			expectedCount:    0,
		},
		{
			name: "uses profile count for larger multi-node shapes",
			dac: &omev1beta1.DedicatedAICluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-dac",
				},
				Spec: omev1beta1.DedicatedAIClusterSpec{
					Profile: "h100-x32",
				},
			},
			profile: &omev1beta1.DedicatedAIClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "h100-x32",
				},
				Spec: omev1beta1.DedicatedAIClusterProfileSpec{
					Count: 4,
				},
			},
			isvcs: []opensourcev1beta1.InferenceService{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "isvc-0",
						Namespace: "test-dac",
					},
					Spec: opensourcev1beta1.InferenceServiceSpec{
						Engine: &opensourcev1beta1.EngineSpec{
							ComponentExtensionSpec: opensourcev1beta1.ComponentExtensionSpec{
								MaxReplicas: 1,
							},
						},
					},
				},
			},
			reservationCount: 4,
			expectedCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, omev1beta1.AddToScheme(scheme))
			require.NoError(t, opensourcev1beta1.AddToScheme(scheme))

			objects := []runtime.Object{tt.profile}
			for i := range tt.isvcs {
				objects = append(objects, &tt.isvcs[i])
			}

			reconciler := &DedicatedAIClusterReconciler{
				Client: ctrlclientfake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(objects...).
					Build(),
				DacReconcilePolicy: &controllerconfig.DacReconcilePolicyConfig{},
				Scheme:             scheme,
			}

			replicaCount, err := reconciler.GetDesiredReservationReplicaCount(tt.dac, tt.reservationCount, true)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, replicaCount)
		})
	}
}

func TestGetDesiredReservationReplicaCount_RestoresCapacityAfterISVCDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, omev1beta1.AddToScheme(scheme))
	require.NoError(t, opensourcev1beta1.AddToScheme(scheme))

	dac := &omev1beta1.DedicatedAICluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dac",
		},
		Spec: omev1beta1.DedicatedAIClusterSpec{
			Profile: "h100-x16",
		},
	}
	profile := &omev1beta1.DedicatedAIClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "h100-x16",
		},
		Spec: omev1beta1.DedicatedAIClusterProfileSpec{
			Count: 2,
		},
	}
	isvc := &opensourcev1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "isvc-0",
			Namespace: "test-dac",
		},
		Spec: opensourcev1beta1.InferenceServiceSpec{
			Engine: &opensourcev1beta1.EngineSpec{
				ComponentExtensionSpec: opensourcev1beta1.ComponentExtensionSpec{
					MaxReplicas: 1,
				},
			},
		},
	}

	reconciler := &DedicatedAIClusterReconciler{
		Client: ctrlclientfake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(profile, isvc).
			Build(),
		DacReconcilePolicy: &controllerconfig.DacReconcilePolicyConfig{},
		Scheme:             scheme,
	}

	replicaCount, err := reconciler.GetDesiredReservationReplicaCount(dac, 2, true)
	require.NoError(t, err)
	assert.Equal(t, 0, replicaCount)

	require.NoError(t, reconciler.Delete(context.Background(), isvc))

	replicaCount, err = reconciler.GetDesiredReservationReplicaCount(dac, 2, true)
	require.NoError(t, err)
	assert.Equal(t, 2, replicaCount)
}
