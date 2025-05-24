package capacityreservation

import (
	"time"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/capacityreservation/utils"

	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

// TODO: add interface for clusterQueue and use github.com/golang/mock/gomock

var (
	SmallResourceGroups = []kueuev1beta1.ResourceGroup{
		{
			CoveredResources: []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory, constants.NvidiaGPUResourceType},
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "bm-gpu-h100-8",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         v1.ResourceCPU,
							NominalQuota: resource.MustParse("10"),
						},
						{
							Name:         v1.ResourceMemory,
							NominalQuota: resource.MustParse("10Gi"),
						},
						{
							Name:         constants.NvidiaGPUResourceType,
							NominalQuota: resource.MustParse("2"),
						},
					},
				},
			},
		},
	}
	LargeResourceGroups = []kueuev1beta1.ResourceGroup{
		{
			CoveredResources: []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory, constants.NvidiaGPUResourceType},
			Flavors: []kueuev1beta1.FlavorQuotas{
				{
					Name: "bm-gpu-h100-8",
					Resources: []kueuev1beta1.ResourceQuota{
						{
							Name:         v1.ResourceCPU,
							NominalQuota: resource.MustParse("1000"),
						},
						{
							Name:         v1.ResourceMemory,
							NominalQuota: resource.MustParse("10Ti"),
						},
						{
							Name:         constants.NvidiaGPUResourceType,
							NominalQuota: resource.MustParse("2000"),
						},
					},
				},
			},
		},
	}
)

func TestCapacityReservationReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = omev1beta1.AddToScheme(scheme)
	_ = kueuev1beta1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	tests := []struct {
		name                       string
		clusterCapacityReservation *omev1beta1.ClusterCapacityReservation
		configMap                  *v1.ConfigMap
		expectedResult             ctrl.Result
		expectedError              bool
		expectedState              omev1beta1.CapacityReservationLifecycleState
	}{
		{
			name:                       "ClusterCapacityReservation not found",
			clusterCapacityReservation: nil,
			configMap:                  nil,
			expectedResult:             ctrl.Result{},
			expectedError:              false,
			expectedState:              "",
		},
		{
			name:                       "ClusterCapacityReservation with deletion timestamp",
			clusterCapacityReservation: createTestClusterCapacityReservation("testreservation", &metav1.Time{Time: time.Now()}, SmallResourceGroups),
			configMap:                  nil,
			expectedResult:             ctrl.Result{},
			expectedError:              false,
			expectedState:              "",
		},
		{
			name:                       "ClusterCapacityReservation creation fails with sufficient resources",
			clusterCapacityReservation: createTestClusterCapacityReservation("testreservation", nil, SmallResourceGroups),
			configMap:                  readConfigMapFromFile(t, scheme),
			expectedResult:             ctrl.Result{},
			expectedError:              false,
			expectedState:              omev1beta1.CapacityReservationFailed,
		},
		{
			name:                       "ClusterCapacityReservation creation fails with insufficient resources",
			clusterCapacityReservation: createTestClusterCapacityReservation("testreservation", nil, LargeResourceGroups),
			configMap:                  readConfigMapFromFile(t, scheme),
			expectedResult:             ctrl.Result{Requeue: false, RequeueAfter: 0},
			expectedError:              false,
			expectedState:              omev1beta1.CapacityReservationFailed,
		},
		{
			name:                       "ClusterCapacityReservation update processing with sufficient resources",
			clusterCapacityReservation: createTestClusterCapacityReservationFromExisting("testreservation", nil, SmallResourceGroups),
			configMap:                  readConfigMapFromFile(t, scheme),
			expectedResult:             ctrl.Result{Requeue: true, RequeueAfter: 0},
			expectedError:              false,
			expectedState:              omev1beta1.CapacityReservationUpdating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := cfake.NewClientBuilder().WithScheme(scheme)
			if tt.clusterCapacityReservation != nil {
				clientBuilder = clientBuilder.WithObjects(tt.clusterCapacityReservation)
			}
			if tt.configMap != nil {
				clientBuilder = clientBuilder.WithObjects(tt.configMap)
			}
			if tt.clusterCapacityReservation != nil {
				clientBuilder = clientBuilder.WithStatusSubresource(tt.clusterCapacityReservation)
			}
			fakeClient := clientBuilder.Build()

			reconciler := &CapacityReservationReconciler{
				Client:                             fakeClient,
				CapacityReservationReconcilePolicy: &controllerconfig.CapacityReservationReconcilePolicyConfig{ReconcileFailedLifecycleState: false},
				ClientConfig:                       &rest.Config{},
				Clientset:                          kubernetes.Interface(nil),
				Log:                                zap.New(),
				Scheme:                             scheme,
				Recorder:                           nil,
			}

			ctx := context.Background()
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: "testreservation",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			if tt.clusterCapacityReservation != nil {
				updatedCcr := &omev1beta1.ClusterCapacityReservation{}
				err = fakeClient.Get(ctx, req.NamespacedName, updatedCcr)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, updatedCcr.Status.CapacityReservationLifecycleState)
			}
		})
	}
}

func createTestClusterCapacityReservation(name string, deletionTimestamp *metav1.Time, resourceGroups []kueuev1beta1.ResourceGroup) *omev1beta1.ClusterCapacityReservation {
	return &omev1beta1.ClusterCapacityReservation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			DeletionTimestamp: deletionTimestamp,
			Finalizers:        []string{"capacityreservation.finalizers"},
		},
		Spec: omev1beta1.CapacityReservationSpec{
			ResourceGroups: resourceGroups,
		},
	}
}

func createTestClusterCapacityReservationFromExisting(name string, deletionTimestamp *metav1.Time, resourceGroups []kueuev1beta1.ResourceGroup) *omev1beta1.ClusterCapacityReservation {
	return &omev1beta1.ClusterCapacityReservation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			DeletionTimestamp: deletionTimestamp,
			Finalizers:        []string{"capacityreservation.finalizers"},
		},
		Spec: omev1beta1.CapacityReservationSpec{
			ResourceGroups: resourceGroups,
		},
		Status: omev1beta1.CapacityReservationStatus{
			Capacity:    utils.ConvertResourceGroupsToFlavorUsage(resourceGroups),
			Allocatable: utils.ConvertResourceGroupsToFlavorUsage(resourceGroups),
		},
	}
}

func readConfigMapFromFile(t *testing.T, scheme *runtime.Scheme) *v1.ConfigMap {
	configMapPath := filepath.Join("..", "..", "..", "..", "config", "configmap", "capacityreservation.yaml")
	yamlFile, err := os.ReadFile(configMapPath)
	if err != nil {
		t.Fatalf("failed to read YAML file: %v", err)
	}
	configMap := &v1.ConfigMap{}
	if err := runtime.DecodeInto(serializer.NewCodecFactory(scheme).UniversalDecoder(), yamlFile, configMap); err != nil {
		t.Fatalf("failed to decode ConfigMap: %v", err)
	}

	return configMap
}
