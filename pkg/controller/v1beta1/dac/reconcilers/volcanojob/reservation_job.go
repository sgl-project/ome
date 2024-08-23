package volcanojob

import (
	"encoding/json"
	"fmt"
	"context"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/dac/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"github.com/google/go-cmp/cmp/cmpopts"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	volbatchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

var log = logf.Log.WithName("VolcanoJobReconciler")

const (
	mainTaskName = "reservation"
)

type ReservationJobConfig struct {
	Image                                string `json:"image"` 
}

type ReservationJobReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	ReservationJob *volbatchv1alpha1.Job
}

func NewReservationJobReconciler(client client.Client, scheme *runtime.Scheme, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*ReservationJobReconciler, error) {
	jobName := namespace
	reservationJob, err := createReservationJob(client, jobName, namespace, resources, affinity, count)
	if err != nil {
		return nil, err
	}

	return &ReservationJobReconciler{
		client: client,
		scheme: scheme,
		ReservationJob: reservationJob,
	}, nil
}

func createReservationJob(client client.Client, jobName string, namespace string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*volbatchv1alpha1.Job, error) {
	configMap, err := utils.GetDedicatedAIClausterConfigMap(client)
	if err != nil {
		return nil, err
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
							RestartPolicy: corev1.RestartPolicyNever,
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
											"cpu":            resources.Requests[corev1.ResourceCPU],
											"memory":         resources.Requests[corev1.ResourceMemory],
											"nvidia.com/gpu": resources.Requests[corev1.ResourceName("nvidia.com/gpu")],
										},
										Requests: corev1.ResourceList{
											"cpu":            resources.Requests[corev1.ResourceCPU],
											"memory":         resources.Requests[corev1.ResourceMemory],
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
	}, nil
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

	ignoreFields := cmpopts.IgnoreFields(corev1.Container{}, "Image")
	
	if err := r.client.Update(context.TODO(), r.ReservationJob, client.DryRunAll); err != nil {
		log.Error(err, "Failed to perform dry-run update of reservationJob", "VolcanoJob", r.ReservationJob.Name)
		return constants.CheckResultUnknown, nil, err
	}

	if diff, err := kmp.SafeDiff(r.ReservationJob.Spec, existingRjob.Spec, ignoreFields); err != nil {
		return constants.CheckResultUnknown, nil, err
	} else if diff != "" {
		log.Info("VolcanoJob Updated", "Diff", diff)
		return constants.CheckResultUpdate, existingRjob, nil
	}
	return constants.CheckResultUpdate, existingRjob, nil
}

func (r *ReservationJobReconciler) Reconcile() (*volbatchv1alpha1.Job, error) {
	existingRjob := &volbatchv1alpha1.Job{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.ReservationJob.Name, Namespace: r.ReservationJob.Namespace}, existingRjob)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), r.ReservationJob)
			if err != nil {
				return nil, err
			}
			return r.ReservationJob, nil
		}
		return nil, err
	}
	return existingRjob, nil
}
