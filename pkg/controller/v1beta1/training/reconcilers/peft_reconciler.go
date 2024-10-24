package reconcilers

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PeftTrainingReconciler reconciles a PeftTrainingJob object
type PeftTrainingReconciler struct {
	client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewPeftTrainingReconciler(
	client client.Client,
	scheme *runtime.Scheme) *PeftTrainingReconciler {
	return &PeftTrainingReconciler{
		client: client,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("TrainerReconciler"),
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//	the TrainingJob object against the actual cluster state, and then
//	perform operations to make the cluster state reflect the state specified by
//	the user.
func (r *PeftTrainingReconciler) Reconcile(jobSpec *v1beta1.PeftTrainingJobSpec) (ctrl.Result, error) {
	// Todo: Implement reconciliation logic for peft framework
	return ctrl.Result{}, nil
}
