package volcanojob

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	volbatchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

var log = logf.Log.WithName("VolcanoJobReconciler")

type ReservationJobConfig struct {
	Image                             string `json:"image"`
	CreationFailedTimeThresholdSecond int    `json:"creationFailedTimeThresholdSecond"`
}

type ReservationJobReconciler struct {
	client                      client.Client
	scheme                      *runtime.Scheme
	CreationFailedTimeThreshold time.Duration
	ReservationJob              *volbatchv1alpha1.Job
}

func NewReservationJobReconciler(client client.Client, scheme *runtime.Scheme, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*ReservationJobReconciler, error) {
	jobName := namespace
	reservationJob, creationFailedTimeThresholdSecond, err := createReservationJob(client, jobName, namespace, resources, affinity, count)
	if err != nil {
		return nil, err
	}

	return &ReservationJobReconciler{
		client:                      client,
		scheme:                      scheme,
		CreationFailedTimeThreshold: time.Duration(creationFailedTimeThresholdSecond) * time.Second,
		ReservationJob:              reservationJob,
	}, nil
}

func createReservationJob(client client.Client, jobName string, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*volbatchv1alpha1.Job, int, error) {
	configMap, err := utils.GetDedicatedAIClusterConfigMap(client)
	if err != nil {
		return nil, 0, err
	}

	reservationJobConfig := &ReservationJobConfig{}
	if rjConfig, ok := configMap.Data["reservationJob"]; ok {
		err := json.Unmarshal([]byte(rjConfig), &reservationJobConfig)
		if err != nil {
			panic(fmt.Errorf("unable to unmarshall %v json string due to %v ", "reservationJob", err))
		}
	} else {
		panic(fmt.Errorf("missing the %v json config in the dedicatedaicluster-config ConfigMap", "reservationJob"))
	}

	return &volbatchv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: volbatchv1alpha1.JobSpec{
			SchedulerName:     constants.VolcanoScheduler,
			PriorityClassName: constants.DedicatedAiClusterReservationPriorityClass,
			MinAvailable:      int32(count),
			MaxRetry:          3,
			Queue:             namespace,
			Tasks: []volbatchv1alpha1.TaskSpec{
				{
					Replicas: int32(count),
					Name:     constants.DACMainTaskName,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								constants.VolcanoPreemptable: "true",
							},
						},
						Spec: corev1.PodSpec{
							TerminationGracePeriodSeconds: &constants.DACReservationJobTerminationGracePeriodSeconds,
							RestartPolicy:                 corev1.RestartPolicyAlways,
							Containers: []corev1.Container{
								{
									ImagePullPolicy: corev1.PullIfNotPresent,
									Image:           reservationJobConfig.Image,
									Command: []string{
										"/bin/bash",
									},
									Args: []string{
										"-c",
										"trap \"echo Shutting down; exit 0\" SIGTERM; /bin/sleep infinity & wait",
									},
									Name: constants.DACMainTaskName,
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											constants.NvidiaGPUResourceType: resources.Requests[corev1.ResourceName(constants.NvidiaGPUResourceType)],
										},
										Requests: corev1.ResourceList{
											constants.NvidiaGPUResourceType: resources.Requests[corev1.ResourceName(constants.NvidiaGPUResourceType)],
										},
									},
								},
							},
							Affinity: affinity,
						},
					},
				},
			},
		},
	}, reservationJobConfig.CreationFailedTimeThresholdSecond, nil
}

func (r *ReservationJobReconciler) checkJobExist() (constants.CheckResultType, *volbatchv1alpha1.Job, error) {
	existingRjob := &volbatchv1alpha1.Job{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.ReservationJob.Name, Namespace: r.ReservationJob.Namespace}, existingRjob)
	if err != nil {
		if errors.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	r.mergeVolcanoJobSpecAndStatus(r.ReservationJob, existingRjob)
	r.ReservationJob.SetResourceVersion(existingRjob.GetResourceVersion())
	if !semanticJobEquals(r.ReservationJob, existingRjob) {
		return constants.CheckResultUpdate, existingRjob, nil
	}

	return constants.CheckResultExisted, existingRjob, nil
}

func (r *ReservationJobReconciler) mergeVolcanoJobSpecAndStatus(desired, existing *volbatchv1alpha1.Job) {
	// Merge the Spec fields that are not allowed to be updated
	desired.Spec.Queue = existing.Spec.Queue
	desired.Spec.Policies = existing.Spec.Policies
	desired.Spec.Plugins = existing.Spec.Plugins
	desired.Spec.PriorityClassName = existing.Spec.PriorityClassName
	desired.Spec.MaxRetry = existing.Spec.MaxRetry
	desired.Spec.SchedulerName = existing.Spec.SchedulerName

	// Merge the tasks (excluding replicas)
	for i := range desired.Spec.Tasks {
		if i < len(existing.Spec.Tasks) {
			desired.Spec.Tasks[i].Name = existing.Spec.Tasks[i].Name
			desired.Spec.Tasks[i].Template = existing.Spec.Tasks[i].Template
			desired.Spec.Tasks[i].Policies = existing.Spec.Tasks[i].Policies
			desired.Spec.Tasks[i].MaxRetry = existing.Spec.Tasks[i].MaxRetry
		}
	}

	// Merge the Status fields
	desired.Status.State = existing.Status.State
	desired.Status.Pending = existing.Status.Pending
	desired.Status.Running = existing.Status.Running
	desired.Status.Succeeded = existing.Status.Succeeded
	desired.Status.Failed = existing.Status.Failed
	desired.Status.Terminating = existing.Status.Terminating
}

func semanticJobEquals(desired, existing *volbatchv1alpha1.Job) bool {
	// Check if MinAvailable is equal
	if !equality.Semantic.DeepEqual(desired.Spec.MinAvailable, existing.Spec.MinAvailable) {
		return false
	}

	// Check if the number of tasks in the desired job is greater or equal to the existing job
	if len(desired.Spec.Tasks) < len(existing.Spec.Tasks) {
		return false
	}

	// Compare only the `Replicas` field in each task, ignoring other fields
	for i := range existing.Spec.Tasks {
		if !equality.Semantic.DeepEqual(desired.Spec.Tasks[i].Replicas, existing.Spec.Tasks[i].Replicas) {
			return false
		}
	}

	// If all checks pass, the jobs are considered equal for the purpose of reconciliation
	return true
}

func (r *ReservationJobReconciler) Reconcile() (*volbatchv1alpha1.Job, error) {
	checkResult, reservationJob, err := r.checkJobExist()
	if err != nil {
		return nil, err
	}
	log.Info("reservation job reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.ReservationJob)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.ReservationJob)
	default:
		return reservationJob, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.ReservationJob, nil
}
