package components

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Component can be reconciled to create underlying resources for an InferenceService
type Component interface {
	Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error)
}
