package replicationjob

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	jobutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/benchmark/reconcilers/job"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/replicationjob/utils"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	replicaCommand         = "replica"
	omeAgentConfigFilePath = "/ome-agent.yaml"
)

var totalModelSizeLogPattern = regexp.MustCompile(`Total model size:\s*([0-9]+)\s+bytes`)

// +kubebuilder:rbac:groups=ome.io,resources=replicationjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=replicationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=replicationjobs/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/log,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=get;update;patch

// ReplicationJobReconciler reconciles a ReplicationJob object.
type ReplicationJobReconciler struct {
	client.Client
	Clientset    kubernetes.Interface
	PodLogReader func(ctx context.Context, namespace, podName, containerName string) (string, error)
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
}

// Reconcile is the entry point for the reconciliation logic.
func (r *ReplicationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("replicationjob", req.NamespacedName)

	replicationJob, err := r.fetchReplicationJob(ctx, req)
	if err != nil {
		if apierr.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Reconciling ReplicationJob", "name", replicationJob.Name, "namespace", replicationJob.Namespace)

	// Deletion handling
	if !replicationJob.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, replicationJob)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(replicationJob, constants.ReplicationJobFinalizer) {
		controllerutil.AddFinalizer(replicationJob, constants.ReplicationJobFinalizer)
		if err := r.Update(ctx, replicationJob); err != nil {
			r.Log.Error(err, "Failed to add finalizer to ReplicationJob")
			return ctrl.Result{}, err
		}
	}

	// Validate input
	log.Info("Validating ReplicationJob", "name", replicationJob.Name, "namespace", replicationJob.Namespace)
	err = utils.ValidateStorageUris(replicationJob.Spec.Source, replicationJob.Spec.Destination)
	if err != nil {
		return ctrl.Result{}, r.updateFailedStatus(ctx, replicationJob, log, err)
	}

	// Update status
	log.Info("Updating ReplicationJob Status", "name", replicationJob.Name, "namespace", replicationJob.Namespace)
	if err = r.updateStatus(ctx, replicationJob, log); err != nil {
		r.Recorder.Eventf(replicationJob, v1.EventTypeWarning, "StatusUpdateFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Build config and pod spec
	config, err := controllerconfig.NewReplicationJobConfig(r.Clientset)
	if err != nil {
		return ctrl.Result{}, err
	}

	meta := r.buildMetadata(replicationJob)
	_, podSpec, err := r.reconcilePodSpec(replicationJob, config)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Job
	log.Info("Reconciling job", "name", replicationJob.Name)
	if _, err := r.reconcileJob(replicationJob, podSpec, meta); err != nil {
		// Attempt status update on failure
		if uErr := r.updateStatus(ctx, replicationJob, log); uErr != nil {
			return ctrl.Result{}, uErr
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// fetchReplicationJob retrieves the ReplicationJob resource from the cluster.
func (r *ReplicationJobReconciler) fetchReplicationJob(ctx context.Context, req ctrl.Request) (*v1beta1.ReplicationJob, error) {
	replicationJob := &v1beta1.ReplicationJob{}
	if err := r.Get(ctx, req.NamespacedName, replicationJob); err != nil {
		return nil, err
	}
	return replicationJob, nil
}

// handleDeletion performs cleanup steps when the ReplicationJob is being deleted.
func (r *ReplicationJobReconciler) handleDeletion(ctx context.Context, replicationJob *v1beta1.ReplicationJob) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(replicationJob, constants.ReplicationJobFinalizer) {
		// Perform cleanup logic here
		controllerutil.RemoveFinalizer(replicationJob, constants.ReplicationJobFinalizer)
		if err := r.Update(ctx, replicationJob); err != nil {
			r.Log.Error(err, "Failed to remove finalizer from ReplicationJob")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileJob creates or updates the Job resource associated with the ReplicationJob.
func (r *ReplicationJobReconciler) reconcileJob(
	replicationJob *v1beta1.ReplicationJob,
	podSpec *v1.PodSpec,
	meta metav1.ObjectMeta,
) (ctrl.Result, error) {

	jobReconciler := jobutils.NewJobReconciler(r.Client, r.Scheme, meta, podSpec)
	if err := controllerutil.SetControllerReference(replicationJob, jobReconciler.Job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if _, err := jobReconciler.Reconcile(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile benchmark job")
	}
	return ctrl.Result{}, nil
}

// buildMetadata creates the ObjectMeta for associated resources.
func (r *ReplicationJobReconciler) buildMetadata(replicationJob *v1beta1.ReplicationJob) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      replicationJob.Name,
		Namespace: replicationJob.Namespace,
		Labels: map[string]string{
			constants.ReplicationJobLabel: replicationJob.Name,
			"logging-forward":             "enabled",
		},
	}
}

// reconcilePodSpec creates the final PodSpec by merging defaults and overrides.
func (r *ReplicationJobReconciler) reconcilePodSpec(replicationJob *v1beta1.ReplicationJob, config *controllerconfig.ReplicationJobConfig) (ctrl.Result, *v1.PodSpec, error) {
	podSpec, err := r.createPodSpec(replicationJob, config)
	if err != nil {
		return ctrl.Result{}, nil, err
	}
	return ctrl.Result{}, podSpec, nil
}

// createPodSpec creates a PodSpec for the ReplicationJob by combining defaults with any user overrides
func (r *ReplicationJobReconciler) createPodSpec(
	replicationJob *v1beta1.ReplicationJob,
	config *controllerconfig.ReplicationJobConfig,
) (*v1.PodSpec, error) {

	args := r.buildJobArgs()

	env, err := utils.BuildEnvVars(replicationJob.Spec, config)
	if err != nil {
		return nil, err
	}

	// Create base container
	defaultContainer := v1.Container{
		Name:      replicationJob.Name,
		Image:     config.PodConfig.Image,
		Resources: *config.PodConfig.Resources,
		Env:       env,
		Args:      args,
	}

	if replicationJob.Spec.ContainerOverride == nil {
		return r.createPodSpecWithContainer(defaultContainer), nil
	}

	mergedContainer, err := r.patchContainer(defaultContainer, *replicationJob.Spec.ContainerOverride)
	if err != nil {
		return nil, err
	}
	return r.createPodSpecWithContainer(mergedContainer), nil
}

func (r *ReplicationJobReconciler) createPodSpecWithContainer(container v1.Container) *v1.PodSpec {
	return &v1.PodSpec{
		Containers: []v1.Container{container},
		NodeSelector: map[string]string{
			// GPU has sufficient CPU and memory to support the replication job.
			constants.NvidiaGPUResourceType: "true",
		},
		Tolerations: []v1.Toleration{
			{
				Key:      constants.NvidiaGPUResourceType,
				Operator: v1.TolerationOpExists,
				Effect:   v1.TaintEffectNoSchedule,
			},
		},
		Affinity: &v1.Affinity{
			NodeAffinity: &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "node.kubernetes.io/instance-type",
									Operator: v1.NodeSelectorOpNotIn,
									Values: []string{
										"BM.GPU.A10.4", // EXCLUDE A10 to avoid ephemeral storage pressure
									},
								},
							},
						},
					},
				},
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
	}
}

func (r *ReplicationJobReconciler) patchContainer(base, override v1.Container) (v1.Container, error) {
	defaultContainerJSON, err := json.Marshal(base)
	if err != nil {
		return v1.Container{}, err
	}

	overrideContainerJSON, err := json.Marshal(override)
	if err != nil {
		return v1.Container{}, err
	}

	mergedContainerJSON, err := strategicpatch.StrategicMergePatch(defaultContainerJSON, overrideContainerJSON, v1.Container{})
	if err != nil {
		return v1.Container{}, err
	}

	var mergedContainer v1.Container
	if err = json.Unmarshal(mergedContainerJSON, &mergedContainer); err != nil {
		return v1.Container{}, err
	}
	return mergedContainer, nil
}

// buildJobArgs constructs the command line arguments for the ome-agent replica container.
func (r *ReplicationJobReconciler) buildJobArgs() []string {
	var args []string
	args = append(args, replicaCommand)
	args = append(args, "--config", omeAgentConfigFilePath, "--debug")
	return args
}

// updateStatus updates the status of the ReplicationJob. Currently, no additional logic.
func (r *ReplicationJobReconciler) updateStatus(
	ctx context.Context,
	replicationJob *v1beta1.ReplicationJob,
	log logr.Logger,
) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      replicationJob.Name,
		Namespace: replicationJob.Namespace,
	}, job)

	now := metav1.Now()
	status := &replicationJob.Status

	if err != nil && apierr.IsNotFound(err) {
		// If the Job does not exist, consider the ReplicationJob Pending.
		log.Info("ReplicationJob doesn't exist",
			"name", replicationJob.Name, "namespace", replicationJob.Namespace)
		if status.Status != v1beta1.ReplicationJobPending {
			status.Status = v1beta1.ReplicationJobPending
			status.StartTime = nil
			status.CompletionTime = nil
			status.Message = ""
			status.LastReconcileTime = &now
		}
	} else if err != nil {
		// If there's another error fetching the Job, we can't update status meaningfully.
		log.Error(err, "error when fetch job",
			"name", replicationJob.Name, "namespace", replicationJob.Namespace)
		return err
	} else {
		// The Job exists. Determine its status from conditions.
		var isComplete, isFailed bool
		var completionTime *metav1.Time

		// Retrieve detailed error from pod (exit code and termination log)
		rawMsg := r.getPodErrorInfo(ctx, replicationJob.Namespace, job.Name, log)
		observedStatus := r.getObservedStatus(ctx, replicationJob.Namespace, job.Name, log)
		if observedStatus != nil {
			status.Observed = observedStatus
		}

		if job.Status.CompletionTime != nil && rawMsg == "" {
			isComplete = true
			completionTime = job.Status.CompletionTime
		}

		// A job is considered failed if it either reaches its backoffLimit or has a failure condition.
		// Here, backoffLimit is set to 0 because retry behavior is managed by ome-agent.
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == v1.ConditionTrue ||
				strings.Contains(rawMsg, v1.PodReasonTerminationByKubelet) {

				isFailed = true
				completionTime = &cond.LastTransitionTime

				formattedMsg, errStatus := utils.FormatClientErrorAndStatus(rawMsg)
				status.Message = formattedMsg

				// Determine condition type and reason
				condType := constants.StateReasonError
				reason := constants.StateReasonError
				if errStatus != "" {
					condType = constants.ClientError
					reason = errStatus
				} else if strings.Contains(rawMsg, v1.PodReasonTerminationByKubelet) {
					condType = constants.NodeError
					reason = v1.PodReasonTerminationByKubelet
				}

				errorCondition := metav1.Condition{
					Type:               condType,
					Status:             metav1.ConditionTrue,
					Reason:             reason,
					Message:            formattedMsg,
					LastTransitionTime: metav1.Now(),
				}
				status.Conditions = setOrUpdateCondition(status.Conditions, errorCondition)
				break
			}
		}

		switch {
		case isComplete:
			log.Info("ReplicationJob is completed",
				"name", replicationJob.Name, "namespace", replicationJob.Namespace)

			if status.Status != v1beta1.ReplicationJobCompleted {
				status.Status = v1beta1.ReplicationJobCompleted
				if status.StartTime == nil && job.Status.StartTime != nil {
					status.StartTime = job.Status.StartTime
				}
				status.CompletionTime = completionTime
				status.LastReconcileTime = &now
			}
		case isFailed:
			log.Info("ReplicationJob failed", "name", replicationJob.Name,
				"namespace", replicationJob.Namespace, "reason", status.Message)
			if status.Status != v1beta1.ReplicationJobFailed {
				status.Status = v1beta1.ReplicationJobFailed
				if status.StartTime == nil && job.Status.StartTime != nil {
					status.StartTime = job.Status.StartTime
				}
				status.CompletionTime = completionTime
				status.LastReconcileTime = &now
			}
		default:
			// TODO: handle job suspended status
			// Running in progress
			var durationStr string
			if job.Status.StartTime != nil {
				duration := now.Sub(job.Status.StartTime.Time)
				durationStr = duration.String()
			} else {
				durationStr = "unknown"
			}
			log.Info("ReplicationJob is running", "name", replicationJob.Name,
				"namespace", replicationJob.Namespace, "duration", durationStr)

			if status.Status != v1beta1.ReplicationJobRunning {
				status.Status = v1beta1.ReplicationJobRunning
				if status.StartTime == nil && job.Status.StartTime != nil {
					status.StartTime = job.Status.StartTime
				}
				status.CompletionTime = nil
				// Reset message
				status.Message = ""
				status.LastReconcileTime = &now
			}
		}
	}

	// Update the ReplicationJob status on the cluster if changed.
	// This ensures the status is stored and reflected in the resource.
	return r.Status().Update(ctx, replicationJob)
}

func setOrUpdateCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for i, cond := range conditions {
		if cond.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}

func (r *ReplicationJobReconciler) updateFailedStatus(ctx context.Context, replicationJob *v1beta1.ReplicationJob, log logr.Logger, err error) error {
	replicationJob.Status.Status = v1beta1.ReplicationJobFailed
	if replicationJob.Status.Message == "" {
		replicationJob.Status.Message = err.Error()
	}
	if err := r.Status().Update(ctx, replicationJob); err != nil {
		log.Error(err, "Failed to update replicationJob failed status")
		return err
	}
	return nil
}

// getPodErrorInfo retrieves error information from the pod associated with the job.
// It checks container termination messages and exit codes.
func (r *ReplicationJobReconciler) getPodErrorInfo(ctx context.Context, namespace, jobName string, log logr.Logger) string {
	// List pods with the job name as label selector
	podList := &v1.PodList{}
	labelSelector := client.MatchingLabels{
		"job-name": jobName,
	}
	err := r.List(ctx, podList, client.InNamespace(namespace), labelSelector)
	if err != nil {
		msg := "Failed to list pods for job"
		log.Error(err, msg, "job", jobName)
		return msg
	}

	// Find the failed pod (usually the first one if job failed)
	for _, pod := range podList.Items {
		// 1. Check Pod conditions (for evictions, node resource pressure, and abnormal reasons)
		for _, cond := range pod.Status.Conditions {
			// Check for PodDisruption conditions (introduced in Kubernetes v1.29+).
			// These disruptions (e.g., DisruptionTarget=True) may not be reflected in the Job status,
			// and affected Jobs might still appear as completed (Succeeded) despite being interrupted.
			if cond.Type == v1.DisruptionTarget &&
				cond.Status == v1.ConditionTrue &&
				cond.Reason == v1.PodReasonTerminationByKubelet {
				msg := fmt.Sprintf(
					"Job '%s': Condition Type='%s', Reason='%s', Message='%s'",
					jobName, cond.Type, cond.Reason, cond.Message)
				log.Info(msg)
				return msg
			}
		}
		// 2. Check container statuses
		// 2.1 Check container termination status for exit code and termination message
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil {
				terminated := containerStatus.State.Terminated
				if terminated.ExitCode != 0 {
					if terminated.Message != "" {
						log.Info("Found error in pod termination message", "pod", pod.Name, "container", containerStatus.Name, "exitCode", terminated.ExitCode)
						log.Error(errors.New(terminated.Message), "pod terminated with error message", "pod", pod.Name, "job", jobName)
						return terminated.Message
					}
					// Check termination reason as fallback
					if terminated.Reason != "" {
						return terminated.Reason
					}
				}
			}
			// 2.2 Check waiting state for errors
			if containerStatus.State.Waiting != nil {
				waiting := containerStatus.State.Waiting
				if waiting.Reason == constants.StateReasonError || waiting.Reason == constants.StateReasonCrashLoopBackOff {
					if waiting.Message != "" {
						return waiting.Message
					}
					return waiting.Reason
				}
			}
		}
	}

	return ""
}

// getObservedStatus retrieves observed metadata from successful replica pod logs.
func (r *ReplicationJobReconciler) getObservedStatus(ctx context.Context, namespace, jobName string, log logr.Logger) *v1beta1.ReplicationJobObservedStatus {
	podList := &v1.PodList{}
	labelSelector := client.MatchingLabels{
		"job-name": jobName,
	}
	if err := r.List(ctx, podList, client.InNamespace(namespace), labelSelector); err != nil {
		log.Error(err, "Failed to list pods for observed replication status", "job", jobName)
		return nil
	}

	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			terminated := containerStatus.State.Terminated
			if terminated == nil || terminated.ExitCode != 0 {
				continue
			}

			podLogs, err := r.readPodLogs(ctx, namespace, pod.Name, containerStatus.Name)
			if err != nil {
				log.Error(err, "Failed to read pod logs for observed replication status", "pod", pod.Name, "container", containerStatus.Name)
				continue
			}
			sourceArtifactSizeBytes, err := parseSourceArtifactSizeBytesFromLog(podLogs)
			if err != nil {
				log.V(1).Info("Unable to parse source artifact size from pod logs", "pod", pod.Name, "container", containerStatus.Name)
				continue
			}

			return &v1beta1.ReplicationJobObservedStatus{
				SourceArtifactSizeBytes: sourceArtifactSizeBytes,
			}
		}
	}

	return nil
}

func (r *ReplicationJobReconciler) readPodLogs(ctx context.Context, namespace, podName, containerName string) (string, error) {
	if r.PodLogReader != nil {
		return r.PodLogReader(ctx, namespace, podName, containerName)
	}
	if r.Clientset == nil {
		return "", errors.New("kubernetes clientset is not configured for pod log reads")
	}

	stream, err := r.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
	}).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	logBytes, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(logBytes), nil
}

func parseSourceArtifactSizeBytesFromLog(logContent string) (*int64, error) {
	matches := totalModelSizeLogPattern.FindAllStringSubmatch(logContent, -1)
	if len(matches) == 0 {
		return nil, errors.New("source artifact size not found in pod logs")
	}

	sizeStr := matches[len(matches)-1][1]
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil, err
	}
	return &size, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicationJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ReplicationJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
