package training

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	"github.com/go-logr/logr"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrainingJobReconciler reconciles a TrainingJob object
type TrainingJobReconciler struct {
	client   client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//	the TrainingJob object against the actual cluster state, and then
//	perform operations to make the cluster state reflect the state specified by
//	the user.
func (r *TrainingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var trainJob v1beta1.TrainingJob
	if err := r.client.Get(ctx, req.NamespacedName, &trainJob); err != nil {
		if apierr.IsNotFound(err) {
			r.Log.Error(err, "TrainingJob not found", "namespace", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Error getting TrainingJob", "namespace", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling training job", "namespace", req.NamespacedName)
	if isTrainJobFinished(&trainJob) {
		r.Log.Info("TrainJob has already been finished", "namespace", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Todo: Get runtime specified in training job spec
	// Todo: Reconcile objects
	// Todo: Update training job conditions

	return ctrl.Result{}, nil
}

func isTrainJobFinished(trainJob *v1beta1.TrainingJob) bool {
	return meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobComplete) ||
		meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobFailed)
}
