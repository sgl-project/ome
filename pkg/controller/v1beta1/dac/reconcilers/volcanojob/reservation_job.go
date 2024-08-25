package volcanojob

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/dac/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	volbatchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

var log = logf.Log.WithName("VolcanoJobReconciler")

const (
	mainTaskName = "reservation"
)

type ReservationJobConfig struct {
	Image                                string         `json:"image"`  
	CreationFailedTimeThresholdSecond    int            `json:"creationFailedTimeThresholdSecond"`
}

type ReservationJobReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	CreationFailedTimeThreshold time.Duration
	ReservationJob *volbatchv1alpha1.Job
}

func NewReservationJobReconciler(client client.Client, scheme *runtime.Scheme, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*ReservationJobReconciler, error) {
	jobName := namespace
	reservationJob, creationFailedTimeThresholdSecond, err := createReservationJob(client, jobName, namespace, resources, affinity, count)
	if err != nil {
		return nil, err
	}

	return &ReservationJobReconciler{
		client: client,
		scheme: scheme,
		CreationFailedTimeThreshold: time.Duration(creationFailedTimeThresholdSecond) * time.Second,
		ReservationJob: reservationJob,
	}, nil
}

func createReservationJob(client client.Client, jobName string, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*volbatchv1alpha1.Job, int, error) {
	configMap, err := utils.GetDedicatedAIClausterConfigMap(client)
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

	var terminationGracePeriodSeconds int64 = 5
	return &volbatchv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Namespace: namespace,
		},
		Spec: volbatchv1alpha1.JobSpec{
			SchedulerName: constants.VolcanoScheduler,
			PriorityClassName: constants.DedicatedAiClusterReservationPriorityClass,
			MinAvailable: int32(count),
			MaxRetry: 3,
			Queue: namespace,
			Tasks: []volbatchv1alpha1.TaskSpec{
				{
					Replicas: int32(count),
					Name: mainTaskName,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								constants.VolcanoPreemptable: "true",
							},
						},
						Spec: corev1.PodSpec{
							TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
							RestartPolicy: corev1.RestartPolicyAlways,
							Containers: []corev1.Container{
								{
									ImagePullPolicy: corev1.PullIfNotPresent,
									Image: reservationJobConfig.Image,
									Command: []string{
										"/bin/bash",
									},
									Args: []string{
										"-c",
										"trap \"echo Shutting down; exit 0\" SIGTERM; /bin/sleep infinity & wait",
									},
									Name: mainTaskName,
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											"nvidia.com/gpu": resources.Requests[corev1.ResourceName("nvidia.com/gpu")],
										},
										Requests: corev1.ResourceList{
											"nvidia.com/gpu": resources.Requests[corev1.ResourceName("nvidia.com/gpu")],
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

	if semanticJobEquals(r.ReservationJob, existingRjob) {
		return constants.CheckResultExisted, existingRjob, nil
	} 
	return constants.CheckResultUpdate, existingRjob, nil
}

func semanticJobEquals(desired, existing *volbatchv1alpha1.Job) bool {
	return equality.Semantic.DeepEqual(desired.Spec.SchedulerName, existing.Spec.SchedulerName) &&
	equality.Semantic.DeepEqual(desired.Spec.PriorityClassName, existing.Spec.PriorityClassName) &&
	equality.Semantic.DeepEqual(desired.Spec.MinAvailable, existing.Spec.MinAvailable) &&
	equality.Semantic.DeepEqual(desired.Spec.Queue, existing.Spec.Queue) &&
	equality.Semantic.DeepEqual(len(desired.Spec.Tasks), len(existing.Spec.Tasks)) &&
	equality.Semantic.DeepEqual(desired.Spec.Tasks[0].Replicas, existing.Spec.Tasks[0].Replicas) &&
	equality.Semantic.DeepEqual(desired.Spec.Tasks[0].Template.ObjectMeta, existing.Spec.Tasks[0].Template.ObjectMeta) &&
	equality.Semantic.DeepEqual(desired.Spec.Tasks[0].Template.Spec.Containers[0].Resources, existing.Spec.Tasks[0].Template.Spec.Containers[0].Resources) &&
	equality.Semantic.DeepEqual(desired.Spec.Tasks[0].Template.Spec.Affinity, existing.Spec.Tasks[0].Template.Spec.Affinity)
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
		r.ReservationJob.SetResourceVersion(reservationJob.GetResourceVersion())
		opErr = r.client.Update(context.TODO(), r.ReservationJob)
	default:
		return reservationJob, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.ReservationJob, nil
}
