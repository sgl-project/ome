package components

import (
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Component can be reconciled to create underlying resources for an InferenceService
type Component interface {
	Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error)
}
