package job

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/constants"
)

var log = logf.Log.WithName("JobReconciler")

type JobReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	Job    *batchv1.Job
}

func NewJobReconciler(client client.Client,
	scheme *runtime.Scheme,
	objMeta metav1.ObjectMeta,
	podSpec *corev1.PodSpec,
) *JobReconciler {

	return &JobReconciler{
		client: client,
		scheme: scheme,
		Job:    createJob(podSpec, objMeta),
	}
}

func createJob(podSpec *corev1.PodSpec, objMeta metav1.ObjectMeta) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: objMeta,
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objMeta,
				Spec:       *podSpec,
			},
		},
	}
}

// Reconcile handles the reconciliation of BenchmarkJob resources
func (r *JobReconciler) Reconcile() (ctrl.Result, error) {
	log.Info("Reconciling Job", "name", r.Job.Name, "namespace", r.Job.Namespace)
	checkResult, _, err := r.checkJobExist()
	if err != nil {
		return ctrl.Result{}, err
	}

	if checkResult == constants.CheckResultCreate {
		if err := r.client.Create(context.TODO(), r.Job); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *JobReconciler) checkJobExist() (constants.CheckResultType, *batchv1.Job, error) {
	existingJob := &batchv1.Job{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Job.ObjectMeta.Namespace,
		Name:      r.Job.ObjectMeta.Name,
	}, existingJob)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	return constants.CheckResultExisted, existingJob, nil
}
