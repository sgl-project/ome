package reconcilers

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("JobReconciler")

// K8sJobReconciler reconciles the raw Kubernetes Job resource
type JobReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	Job    *batchv1.Job
}

func NewJobReconciler(client client.Client, scheme *runtime.Scheme, podSpec *corev1.PodSpec, replicas int32, objectMeta metav1.ObjectMeta) *JobReconciler {
	return &JobReconciler{
		client: client,
		scheme: scheme,
		Job: &batchv1.Job{
			ObjectMeta: objectMeta,
			Spec: batchv1.JobSpec{
				Completions: &replicas,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: objectMeta,
					Spec:       *podSpec,
				},
			},
		},
	}
}

func (jr *JobReconciler) Reconcile() (*batchv1.Job, error) {
	checkResult, job, err := jr.checkJobExist(jr.client)
	if err != nil {
		return nil, err
	}
	log.Info("Training k8s job reconcile", "checkResult", checkResult, "k8s_job", jr.Job.Name)

	switch checkResult {
	case constants.CheckResultCreate:
		err = jr.client.Create(context.TODO(), jr.Job)
		if err != nil {
			log.Info("Failed to create training k8s job", "k8s_job", jr.Job.Name)
			return nil, err
		} else {
			return jr.Job, nil
		}
	case constants.CheckResultUpdate:
		err = jr.client.Update(context.TODO(), jr.Job)
		if err != nil {
			log.Info("Failed to update training k8s job", "k8s_job", jr.Job.Name)
			return nil, err
		} else {
			return jr.Job, nil
		}
	default:
		return job, nil
	}
}

func (jr *JobReconciler) checkJobExist(client client.Client) (constants.CheckResultType, *batchv1.Job, error) {
	existingJob := &batchv1.Job{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: jr.Job.ObjectMeta.Namespace,
		Name:      jr.Job.ObjectMeta.Name,
	}, existingJob)
	if err != nil {
		if apierr.IsNotFound(err) {
			log.Info("K8s training job not found, will be created", "k8s_job", jr.Job.Name)
			return constants.CheckResultCreate, nil, nil
		}
		log.Error(err, "Failed to get k8s training job", "k8s_job", jr.Job.Name)
		return constants.CheckResultUnknown, nil, err
	}
	return constants.CheckResultUpdate, existingJob, nil
}
