package pvc

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewTrainingPVCReconciler(t *testing.T) {
	var client client.Client
	var clientset kubernetes.Interface
	var scheme *runtime.Scheme

	reconciler := NewTrainingPVCReconciler(client, clientset, scheme)

	if reconciler == nil {
		t.Errorf("reconciler was nil: %v", reconciler)
	}
}
