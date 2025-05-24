package pv

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewTrainingPVReconciler(t *testing.T) {
	var client client.Client
	var clientset kubernetes.Interface
	var scheme *runtime.Scheme

	reconciler := NewTrainingPVReconciler(client, clientset, scheme)

	if reconciler == nil {
		t.Errorf("reconciler was nil: %v", reconciler)
	}
}
