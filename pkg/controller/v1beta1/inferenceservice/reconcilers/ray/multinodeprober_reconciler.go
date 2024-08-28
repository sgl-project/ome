package raycluster

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	knapis "knative.dev/pkg/apis"
	volcanobatch "volcano.sh/apis/pkg/apis/batch/v1alpha1"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MultiNodeProberReconciler reconciles the raw kubernetes deployment resource for multi node prober
type MultiNodeProberReconciler struct {
	client       kclient.Client
	scheme       *runtime.Scheme
	VolcanoJobs  *volcanobatch.Job
	componentExt *v1beta1.ComponentExtensionSpec
	URL          *knapis.URL
}

func NewMultiNodeProberReconciler(client kclient.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	multiNodeProberConfig *v1beta1.MultiNodeProberConfig) *MultiNodeProberReconciler {
	return &MultiNodeProberReconciler{
		client:       client,
		scheme:       scheme,
		componentExt: componentExt,
		VolcanoJobs:  createVolcanoJob(componentMeta, componentExt, multiNodeProberConfig),
	}
}

func createVolcanoJob(componentMeta metav1.ObjectMeta, componentExt *v1beta1.ComponentExtensionSpec, multiNodeProberConfig *v1beta1.MultiNodeProberConfig) *volcanobatch.Job {
	jobMetadata := componentMeta
	jobMetadata.Labels["app"] = constants.GetRawServiceLabel(componentMeta.Name)
	utils.SetPodLabelsFromAnnotations(&jobMetadata)
	jobSpec := getDefaultJobSpec(componentMeta, componentExt, multiNodeProberConfig)
	volcanoJob := &volcanobatch.Job{
		ObjectMeta: jobMetadata,
		Spec:       *jobSpec,
	}
	return volcanoJob
}

func getDefaultJobSpec(componentMeta metav1.ObjectMeta, componentExt *v1beta1.ComponentExtensionSpec, multiNodeProberConfig *v1beta1.MultiNodeProberConfig) *volcanobatch.JobSpec {

	return &volcanobatch.JobSpec{
		Plugins: map[string][]string{
			"env": {"[]"},
			"ssh": {"[]"},
		},
		MinAvailable:  int32(*componentExt.MinReplicas),
		SchedulerName: constants.VolcanoScheduler,
		MaxRetry:      3,
		Queue:         "default",

		Tasks: []volcanobatch.TaskSpec{
			{
				Replicas: int32(*componentExt.MinReplicas),
				Name:     constants.MultiNodeProberContainerName,
				Template: corev1.PodTemplateSpec{
					Spec: *getPodSpec(componentMeta, multiNodeProberConfig),
				},
			},
		},
	}
}

func getPodSpec(componentMeta metav1.ObjectMeta, multiNodeProberConfig *v1beta1.MultiNodeProberConfig) *corev1.PodSpec {

	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            constants.MultiNodeProberContainerName,
				Image:           multiNodeProberConfig.ContainerImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPULimit),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryLimit),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPURequest),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryRequest),
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/healthz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/readyz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/startupz",
						},
					},
					TimeoutSeconds:      multiNodeProberConfig.StartupTimeoutSeconds,
					PeriodSeconds:       multiNodeProberConfig.StartupPeriodSeconds,
					SuccessThreshold:    1,
					FailureThreshold:    multiNodeProberConfig.StartupFailureThreshold,
					InitialDelaySeconds: multiNodeProberConfig.StartupInitialDelaySeconds,
				},
				Env: []corev1.EnvVar{
					{
						Name:  "RAY_ADDRESS",
						Value: componentMeta.Name,
					},
					{
						Name:  "NAMESPACE",
						Value: componentMeta.Namespace,
					},
				},

				Command: []string{
					"/bin/bash",
					"-lc",
					"--",
				},
				Args: []string{
					`/multinode-prober \
					--vllm-endpoint http://$RAY_ADDRESS-$VC_TASK_INDEX.$NAMESPACE.svc.cluster.local:8080 \
					--addr 0.0.0.0:8080`},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: constants.MultiNodeProberContainerPort,
					},
				},
			},
		},
	}
}

func (r *MultiNodeProberReconciler) checkVolcanoJobExist(client kclient.Client) (constants.CheckResultType, *volcanobatch.Job, error) {
	// get the job
	existingJob := &volcanobatch.Job{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.VolcanoJobs.ObjectMeta.Namespace,
		Name:      r.VolcanoJobs.ObjectMeta.Name,
	}, existingJob)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	r.mergeVolcanoJobSpecAndStatus(r.VolcanoJobs, existingJob)
	r.VolcanoJobs.SetResourceVersion(existingJob.GetResourceVersion())

	if !semanticJobEquals(r.VolcanoJobs, existingJob) {
		return constants.CheckResultUpdate, existingJob, nil
	}
	return constants.CheckResultExisted, existingJob, nil
}

func (r *MultiNodeProberReconciler) mergeVolcanoJobSpecAndStatus(desired, existing *volcanobatch.Job) {
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

func semanticJobEquals(desired, existing *volcanobatch.Job) bool {
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

func getDefaultPodSpec(multiNodeProberConfig *v1beta1.MultiNodeProberConfig, url *knapis.URL) *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            constants.MultiNodeProberContainerName,
				Image:           multiNodeProberConfig.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPULimit),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryLimit),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(multiNodeProberConfig.CPURequest),
						corev1.ResourceMemory: resource.MustParse(multiNodeProberConfig.MemoryRequest),
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/healthz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/readyz",
						},
					},
					TimeoutSeconds:   5,
					PeriodSeconds:    30,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.IntOrString{
								IntVal: constants.MultiNodeProberContainerPort,
							},
							Path: "/startupz",
						},
					},
					TimeoutSeconds:      multiNodeProberConfig.StartupTimeoutSeconds,
					PeriodSeconds:       multiNodeProberConfig.StartupPeriodSeconds,
					SuccessThreshold:    1,
					FailureThreshold:    multiNodeProberConfig.StartupFailureThreshold,
					InitialDelaySeconds: multiNodeProberConfig.StartupInitialDelaySeconds,
				},
				Args: []string{
					"--vllm-endpoint",
					fmt.Sprintf("%s:%s", url.String(), constants.InferenceServiceDefaultHttpPort),
					"--addr",
					"0.0.0.0:8080",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: constants.MultiNodeProberContainerPort,
					},
				},
			},
		},
	}
}

func (r *MultiNodeProberReconciler) Reconcile() (*volcanobatch.Job, error) {
	// Reconcile Volcano Job
	checkResult, vcJob, err := r.checkVolcanoJobExist(r.client)
	if err != nil {
		return nil, err
	}
	log.Info("Volcano job reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.VolcanoJobs)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.VolcanoJobs)
	default:
		return vcJob, nil
	}

	if opErr != nil {
		return nil, opErr
	}
	return r.VolcanoJobs, nil
}
