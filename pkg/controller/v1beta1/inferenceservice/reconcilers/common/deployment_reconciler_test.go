package common

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	lwsapi "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/lws"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinode"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/podmonitor"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
)

// newMultiNodeReconcilerForRefTest assembles a MultiNodeReconciler whose
// sub-reconcilers hold only the exported child objects
// setMultiNodeReferences touches.
func newMultiNodeReconcilerForRefTest(meta metav1.ObjectMeta) *multinode.MultiNodeReconciler {
	return &multinode.MultiNodeReconciler{
		LWS: &lws.LWSReconciler{
			LWS: &lwsapi.LeaderWorkerSet{ObjectMeta: meta},
		},
		Service: &service.ServiceReconciler{
			Service: &corev1.Service{ObjectMeta: meta},
		},
		PodMonitor: &podmonitor.PodMonitorReconciler{
			PodMonitor: &monitoringv1.PodMonitor{ObjectMeta: meta},
		},
	}
}

func ownsISVC(refs []metav1.OwnerReference, isvc *v1beta1.InferenceService) bool {
	for _, ref := range refs {
		if ref.Kind == "InferenceService" && ref.Name == isvc.Name &&
			ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

// TestSetMultiNodeReferences pins the owner-ref contract of the MultiNode
// path: every child object (LWS, Service, PodMonitor) must carry a
// controller owner-ref pointing at the ISVC so it is garbage-collected when
// the ISVC is deleted.
func TestSetMultiNodeReferences(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
	}
	meta := metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"}

	r := &DeploymentReconciler{
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	mnr := newMultiNodeReconcilerForRefTest(meta)
	require.NoError(t, r.setMultiNodeReferences(isvc, mnr))
	assert.True(t, ownsISVC(mnr.LWS.LWS.OwnerReferences, isvc), "LWS owner-ref missing")
	assert.True(t, ownsISVC(mnr.Service.Service.OwnerReferences, isvc), "Service owner-ref missing")
	assert.True(t, ownsISVC(mnr.PodMonitor.PodMonitor.OwnerReferences, isvc), "PodMonitor owner-ref missing")
}
