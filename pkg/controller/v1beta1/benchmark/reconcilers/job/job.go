package job

import (
	"context"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/constants"
)

// JobReconciler reconciles batch/v1 Job resources for benchmarks
type JobReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	log    logr.Logger
	Job    *batchv1.Job
}

// NewJobReconciler creates a new JobReconciler with the given parameters
func NewJobReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	objMeta metav1.ObjectMeta,
	podSpec *corev1.PodSpec,
) *JobReconciler {
	return &JobReconciler{
		client: client,
		scheme: scheme,
		log:    logf.Log.WithName("JobReconciler").WithValues("name", objMeta.Name, "namespace", objMeta.Namespace),
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

// Reconcile creates the Job if it doesn't exist. Jobs are immutable after creation,
// so updates are not supported - the job must be deleted and recreated if changes are needed.
func (r *JobReconciler) Reconcile(ctx context.Context) error {
	exists, err := r.jobExists(ctx)
	if err != nil {
		return err
	}

	if !exists {
		r.log.Info("Creating Job")
		if err := r.client.Create(ctx, r.Job); err != nil {
			r.log.Error(err, "Failed to create Job")
			return err
		}
	}

	return nil
}

// jobExists checks if the job already exists in the cluster
func (r *JobReconciler) jobExists(ctx context.Context) (bool, error) {
	err := r.client.Get(ctx, types.NamespacedName{
		Namespace: r.Job.Namespace,
		Name:      r.Job.Name,
	}, &batchv1.Job{})

	if apierr.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CheckResult returns the reconciliation action needed for backward compatibility
func (r *JobReconciler) CheckResult(ctx context.Context) (constants.CheckResultType, error) {
	exists, err := r.jobExists(ctx)
	if err != nil {
		return constants.CheckResultUnknown, err
	}
	if exists {
		return constants.CheckResultExisted, nil
	}
	return constants.CheckResultCreate, nil
}
