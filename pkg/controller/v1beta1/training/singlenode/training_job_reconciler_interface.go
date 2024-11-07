package singlenode

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SinglePodTrainingReconciler interface {
	Reconcile(trainingJob *v1beta1.TrainingJob) (ctrl.Result, error)
}

func ReconcileJob(trainingJob *v1beta1.TrainingJob, client client.Client, scheme *runtime.Scheme, podSpec *v1.PodSpec, replicaCount int32, objectMeta metav1.ObjectMeta) (ctrl.Result, error) {
	// New reconciler for launcher raw k8s resources
	jobReconciler := NewJobReconciler(client, scheme, podSpec, replicaCount, objectMeta)

	// Set owner reference for launcher raw kube resources
	res, err := setLauncherOwnerReference(trainingJob, jobReconciler, scheme)
	if err != nil {
		return res, err
	}
	_, err = jobReconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "fails to reconcile raw kubernetes resources for Launcher")
	}

	return ctrl.Result{}, nil
}
func setLauncherOwnerReference(tjob *v1beta1.TrainingJob, k8sReconciler *JobReconciler, scheme *runtime.Scheme) (ctrl.Result, error) {
	if k8sReconciler.Job != nil {
		if err := controllerutil.SetControllerReference(tjob, k8sReconciler.Job, scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "fails to set job owner reference for trainer")
		}
	}
	return ctrl.Result{}, nil
}
