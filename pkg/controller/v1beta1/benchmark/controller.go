package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/benchmark/reconcilers/job"
	benchmarkutils "github.com/sgl-project/ome/pkg/controller/v1beta1/benchmark/utils"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	finalizerName = "benchmarkjob.finalizers"
)

// +kubebuilder:rbac:groups=ome.io,resources=benchmarkjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=benchmarkjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=benchmarkjobs/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=get;update;patch

// BenchmarkJobReconciler reconciles a BenchmarkJob object.
type BenchmarkJobReconciler struct {
	client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// Reconcile is the entry point for the reconciliation logic.
func (r *BenchmarkJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("benchmarkjob", req.NamespacedName)

	benchmarkJob, err := r.fetchBenchmarkJob(ctx, req)
	if err != nil {
		if apierr.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Reconciling BenchmarkJob", "name", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)

	// Finalizer handling
	if !benchmarkJob.DeletionTimestamp.IsZero() {
		// Object is being deleted
		return r.handleDeletion(ctx, benchmarkJob)
	}

	// Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(benchmarkJob, finalizerName) {
		controllerutil.AddFinalizer(benchmarkJob, finalizerName)
		if err := r.Update(ctx, benchmarkJob); err != nil {
			r.Log.Error(err, "Failed to add finalizer to BenchmarkJob")
			return ctrl.Result{}, err
		}
	}

	// Update status
	if err := r.updateStatus(ctx, benchmarkJob); err != nil {
		r.Recorder.Eventf(benchmarkJob, v1.EventTypeWarning, "StatusUpdateFailed", err.Error())
		return ctrl.Result{}, err
	}

	var isvcRef *v1beta1.InferenceService
	if benchmarkJob.Spec.Endpoint.InferenceService != nil {
		isvcRef, err = benchmarkutils.GetInferenceService(r.Client, benchmarkJob.Spec.Endpoint.InferenceService)
		if err != nil {
			return ctrl.Result{}, err
		}
		isReady := isvcRef.Status.IsReady()
		if !isReady {
			log.Info("InferenceService is not ready, re-queuing", "name", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Minute,
			}, nil
		}
	}

	// Build config and pod spec
	config, err := controllerconfig.NewBenchmarkJobConfig(r.Clientset)
	if err != nil {
		return ctrl.Result{}, err
	}

	meta := r.buildMetadata(benchmarkJob)
	_, podSpec, err := r.reconcilePodSpec(benchmarkJob, config)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Job
	if _, err := r.reconcileJob(benchmarkJob, podSpec, meta); err != nil {
		// Attempt status update on failure
		if uErr := r.updateStatus(ctx, benchmarkJob); uErr != nil {
			return ctrl.Result{}, uErr
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// fetchBenchmarkJob retrieves the BenchmarkJob resource from the cluster.
func (r *BenchmarkJobReconciler) fetchBenchmarkJob(ctx context.Context, req ctrl.Request) (*v1beta1.BenchmarkJob, error) {
	benchmarkJob := &v1beta1.BenchmarkJob{}
	if err := r.Get(ctx, req.NamespacedName, benchmarkJob); err != nil {
		return nil, err
	}
	return benchmarkJob, nil
}

// handleDeletion performs cleanup steps when the BenchmarkJob is being deleted.
func (r *BenchmarkJobReconciler) handleDeletion(ctx context.Context, benchmarkJob *v1beta1.BenchmarkJob) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(benchmarkJob, finalizerName) {
		// Perform cleanup logic here
		controllerutil.RemoveFinalizer(benchmarkJob, finalizerName)
		if err := r.Update(ctx, benchmarkJob); err != nil {
			r.Log.Error(err, "Failed to remove finalizer from BenchmarkJob")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileJob creates or updates the Job resource associated with the BenchmarkJob.
func (r *BenchmarkJobReconciler) reconcileJob(benchmarkJob *v1beta1.BenchmarkJob, podSpec *v1.PodSpec, meta metav1.ObjectMeta) (ctrl.Result, error) {
	jobReconciler := job.NewJobReconciler(r.Client, r.Scheme, meta, podSpec)
	if err := controllerutil.SetControllerReference(benchmarkJob, jobReconciler.Job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if _, err := jobReconciler.Reconcile(); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile benchmark job")
	}
	return ctrl.Result{}, nil
}

// reconcilePodSpec creates the final PodSpec by merging defaults and overrides.
func (r *BenchmarkJobReconciler) reconcilePodSpec(benchmarkJob *v1beta1.BenchmarkJob, benchmarkConfig *controllerconfig.BenchmarkJobConfig) (ctrl.Result, *v1.PodSpec, error) {
	podSpec, err := r.createPodSpec(benchmarkJob, benchmarkConfig)
	if err != nil {
		return ctrl.Result{}, nil, err
	}
	return ctrl.Result{}, podSpec, nil
}

// buildMetadata creates the ObjectMeta for associated resources.
func (r *BenchmarkJobReconciler) buildMetadata(benchmarkJob *v1beta1.BenchmarkJob) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      benchmarkJob.Name,
		Namespace: benchmarkJob.Namespace,
		Labels: map[string]string{
			"benchmark": benchmarkJob.Name,
		},
		Annotations: map[string]string{
			"logging-forward": "true",
		},
	}
}

// createPodSpec creates a PodSpec for the BenchmarkJob by combining defaults with any user overrides
func (r *BenchmarkJobReconciler) createPodSpec(benchmarkJob *v1beta1.BenchmarkJob, benchmarkConfig *controllerconfig.BenchmarkJobConfig) (*v1.PodSpec, error) {
	// Build default container spec
	resources := v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(benchmarkConfig.PodConfig.CPURequest),
			v1.ResourceMemory: resource.MustParse(benchmarkConfig.PodConfig.MemoryRequest),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(benchmarkConfig.PodConfig.CPULimit),
			v1.ResourceMemory: resource.MustParse(benchmarkConfig.PodConfig.MemoryLimit),
		},
	}

	env := []v1.EnvVar{
		{
			Name:  "ENABLE_UI",
			Value: "false",
		},
	}
	if benchmarkJob.Spec.HuggingFaceSecretReference != nil && benchmarkJob.Spec.HuggingFaceSecretReference.Name != "" {
		env = append(env, v1.EnvVar{
			Name: "HUGGINGFACE_API_KEY",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: benchmarkJob.Spec.HuggingFaceSecretReference.Name,
					},
					Key: "HUGGINGFACE_API_KEY",
				},
			},
		})
	}

	cmd, args, err := r.buildBenchmarkCommand(benchmarkJob)
	if err != nil {
		return nil, err
	}

	// Create base container
	defaultContainer := v1.Container{
		Name:      benchmarkJob.Name,
		Image:     benchmarkConfig.PodConfig.Image,
		Resources: resources,
		Env:       env,
		Command:   cmd,
		Args:      args,
	}

	// Create volumes if InferenceService is specified
	var volumes []v1.Volume
	if benchmarkJob.Spec.Endpoint.InferenceService != nil {
		inferenceService, err := benchmarkutils.GetInferenceService(r.Client, benchmarkJob.Spec.Endpoint.InferenceService)
		if err != nil {
			return nil, err
		}
		var baseModelName string
		if inferenceService.Spec.Predictor.Model != nil &&
			inferenceService.Spec.Predictor.Model.BaseModel != nil {
			baseModelName = *inferenceService.Spec.Predictor.Model.BaseModel
		} else if inferenceService.Spec.Model != nil {
			baseModelName = inferenceService.Spec.Model.Name
		}
		if baseModelName == "" {
			return nil, fmt.Errorf("InferenceService %s/%s has no Model defined", inferenceService.Name, inferenceService.Namespace)
		}
		baseModel, _, err := isvcutils.GetBaseModel(r.Client, baseModelName, inferenceService.Namespace)
		if err != nil {
			return nil, err
		}
		benchmarkutils.UpdateVolumeMounts(inferenceService, &defaultContainer, baseModel)

		volumes = append(volumes, v1.Volume{
			Name: baseModelName,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: *baseModel.Storage.Path,
				},
			},
		})
	}

	// Handle storage PVC if specified
	storageType, err := storage.GetStorageType(*benchmarkJob.Spec.OutputLocation.StorageUri)
	if err != nil {
		return nil, fmt.Errorf("error determining storage type: %v", err)
	}

	if storageType == storage.StorageTypePVC {
		components, err := storage.ParsePVCStorageURI(*benchmarkJob.Spec.OutputLocation.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("error parsing PVC storage URI: %v", err)
		}

		// Check if PVC exists
		pvc := &v1.PersistentVolumeClaim{}
		if err := r.Client.Get(context.Background(), types.NamespacedName{
			Name:      components.PVCName,
			Namespace: benchmarkJob.Namespace,
		}, pvc); err != nil {
			return nil, fmt.Errorf("PVC %s not found: %v", components.PVCName, err)
		}

		// Add volume for PVC
		volumes = append(volumes, v1.Volume{
			Name: "benchmark-output-storage",
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: components.PVCName,
				},
			},
		})

		// Add volume mount to container
		defaultContainer.VolumeMounts = append(defaultContainer.VolumeMounts, v1.VolumeMount{
			Name:      "benchmark-output-storage",
			MountPath: "/" + components.SubPath,
			SubPath:   components.SubPath,
		})
	}

	// If no overrides, return default spec
	if benchmarkJob.Spec.PodOverride == nil {
		return &v1.PodSpec{
			Containers: []v1.Container{defaultContainer},
			Volumes:    volumes,
			NodeSelector: map[string]string{
				"nvidia.com/gpu": "true",
			},
			Tolerations: []v1.Toleration{
				{
					Key:      "nvidia.com/gpu",
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoSchedule,
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		}, nil
	}

	// First merge container specs
	defaultContainerJSON, err := json.Marshal(defaultContainer)
	if err != nil {
		return nil, err
	}

	overrideContainer := v1.Container{
		Name:         "genai-bench",
		Image:        benchmarkJob.Spec.PodOverride.Image,
		Env:          benchmarkJob.Spec.PodOverride.Env,
		EnvFrom:      benchmarkJob.Spec.PodOverride.EnvFrom,
		VolumeMounts: benchmarkJob.Spec.PodOverride.VolumeMounts,
	}
	if benchmarkJob.Spec.PodOverride.Resources != nil {
		overrideContainer.Resources = *benchmarkJob.Spec.PodOverride.Resources
	}

	overrideContainerJSON, err := json.Marshal(overrideContainer)
	if err != nil {
		return nil, err
	}

	mergedContainerJSON, err := strategicpatch.StrategicMergePatch(defaultContainerJSON, overrideContainerJSON, v1.Container{})
	if err != nil {
		return nil, err
	}

	var mergedContainer v1.Container
	if err := json.Unmarshal(mergedContainerJSON, &mergedContainer); err != nil {
		return nil, err
	}

	// Create and merge pod specs
	defaultPodSpec := &v1.PodSpec{
		Containers: []v1.Container{mergedContainer},
		Volumes:    volumes,
		NodeSelector: map[string]string{
			"nvidia.com/gpu": "true",
		},
		Tolerations: []v1.Toleration{
			{
				Key:      "nvidia.com/gpu",
				Operator: v1.TolerationOpExists,
				Effect:   v1.TaintEffectNoSchedule,
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
	}

	defaultPodJSON, err := json.Marshal(defaultPodSpec)
	if err != nil {
		return nil, err
	}

	overridePodSpec := &v1.PodSpec{
		Volumes:      benchmarkJob.Spec.PodOverride.Volumes,
		Affinity:     benchmarkJob.Spec.PodOverride.Affinity,
		NodeSelector: benchmarkJob.Spec.PodOverride.NodeSelector,
		Tolerations:  benchmarkJob.Spec.PodOverride.Tolerations,
	}

	overridePodJSON, err := json.Marshal(overridePodSpec)
	if err != nil {
		return nil, err
	}

	mergedPodJSON, err := strategicpatch.StrategicMergePatch(defaultPodJSON, overridePodJSON, v1.PodSpec{})
	if err != nil {
		return nil, err
	}

	var mergedPodSpec v1.PodSpec
	if err := json.Unmarshal(mergedPodJSON, &mergedPodSpec); err != nil {
		return nil, err
	}
	// Update container, since this will be the only container
	mergedPodSpec.Containers = []v1.Container{mergedContainer}

	return &mergedPodSpec, nil
}

// buildBenchmarkCommand constructs the command line arguments for the benchmark container.
func (r *BenchmarkJobReconciler) buildBenchmarkCommand(benchmarkJob *v1beta1.BenchmarkJob) ([]string, []string, error) {
	command := []string{"genai-bench"}

	inferenceArgs, err := benchmarkutils.BuildInferenceServiceArgs(r.Client, benchmarkJob.Spec.Endpoint, benchmarkJob.Namespace)
	if err != nil {
		return nil, nil, err
	}

	args := []string{
		"benchmark",
		"--api-backend", inferenceArgs["--api-backend"],
		"--api-base", inferenceArgs["--api-base"],
		"--api-key", inferenceArgs["--api-key"],
		"--api-model-name", inferenceArgs["--api-model-name"],
		"--task", benchmarkJob.Spec.Task,
		"--max-time-per-run", strconv.Itoa(*benchmarkJob.Spec.MaxTimePerIteration),
		"--max-requests-per-run", strconv.Itoa(*benchmarkJob.Spec.MaxRequestsPerIteration),
		"--model-tokenizer", inferenceArgs["--model-tokenizer"],
	}

	// Add traffic scenarios
	for _, scenario := range benchmarkJob.Spec.TrafficScenarios {
		args = append(args, "--traffic-scenario", scenario)
	}

	// Add concurrency levels
	for _, concurrency := range benchmarkJob.Spec.NumConcurrency {
		args = append(args, "--num-concurrency", fmt.Sprintf("%d", concurrency))
	}

	// Add experiment folder name
	if benchmarkJob.Spec.ResultFolderName != nil {
		args = append(args, "--experiment-folder-name", *benchmarkJob.Spec.ResultFolderName)
	}

	// Add server metadata
	if benchmarkJob.Spec.ServiceMetadata != nil {
		args = append(args,
			"--server-engine", benchmarkJob.Spec.ServiceMetadata.Engine,
			"--server-gpu-type", benchmarkJob.Spec.ServiceMetadata.GpuType,
			"--server-version", benchmarkJob.Spec.ServiceMetadata.Version,
			"--server-gpu-count", fmt.Sprintf("%d", benchmarkJob.Spec.ServiceMetadata.GpuCount),
		)
	}

	storageArgs, _ := benchmarkutils.BuildStorageArgs(benchmarkJob.Spec.OutputLocation)
	args = append(args, storageArgs...)

	return command, args, nil
}

// updateStatus updates the status of the BenchmarkJob. Currently, no additional logic.
func (r *BenchmarkJobReconciler) updateStatus(ctx context.Context, benchmarkJob *v1beta1.BenchmarkJob) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: benchmarkJob.Namespace,
		Name:      benchmarkJob.Name,
	}, job)

	now := metav1.Now()
	benchmarkStatus := &benchmarkJob.Status

	if err != nil && apierr.IsNotFound(err) {
		// If the Job does not exist, consider the BenchmarkJob Pending.
		if benchmarkStatus.State != "Pending" {
			benchmarkStatus.State = "Pending"
			benchmarkStatus.StartTime = nil
			benchmarkStatus.CompletionTime = nil
			benchmarkStatus.FailureMessage = ""
			benchmarkStatus.LastReconcileTime = &now
		}
	} else if err != nil {
		// If there's another error fetching the Job, we can't update status meaningfully.
		return err
	} else {
		// The Job exists. Determine its status from conditions.
		// Possible conditions for a Job include:
		// - Complete
		// - Failed
		var isComplete, isFailed bool
		var completionTime *metav1.Time

		// A job is considered completed if it has a completion time or a condition that indicates completion.
		if job.Status.CompletionTime != nil {
			isComplete = true
			completionTime = job.Status.CompletionTime
		} else {
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == v1.ConditionTrue {
					isComplete = true
					completionTime = &cond.LastTransitionTime
					break
				}
			}
		}

		// A job is considered failed if it has backoffLimit reached or conditions show failure.
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == v1.ConditionTrue {
				isFailed = true
				completionTime = &cond.LastTransitionTime
				benchmarkStatus.FailureMessage = cond.Message
				break
			}
		}

		// Map job conditions to benchmark job states
		switch {
		case isFailed:
			// Failed - check this first as a job can be both complete and failed
			if benchmarkStatus.State != "Failed" {
				benchmarkStatus.State = "Failed"
				if benchmarkStatus.StartTime == nil && job.Status.StartTime != nil {
					benchmarkStatus.StartTime = job.Status.StartTime
				}
				benchmarkStatus.CompletionTime = completionTime
				benchmarkStatus.LastReconcileTime = &now
			}
		case isComplete:
			// Completed - only if not failed
			if benchmarkStatus.State != "Completed" {
				benchmarkStatus.State = "Completed"
				if benchmarkStatus.StartTime == nil && job.Status.StartTime != nil {
					benchmarkStatus.StartTime = job.Status.StartTime
				}
				benchmarkStatus.CompletionTime = completionTime
				benchmarkStatus.LastReconcileTime = &now
			}
		default:
			// If not complete or failed, consider it running.
			if benchmarkStatus.State != "Running" {
				benchmarkStatus.State = "Running"
				if benchmarkStatus.StartTime == nil && job.Status.StartTime != nil {
					benchmarkStatus.StartTime = job.Status.StartTime
				}
				benchmarkStatus.CompletionTime = nil
				benchmarkStatus.FailureMessage = ""
				benchmarkStatus.LastReconcileTime = &now
			}
		}
	}

	// Update the BenchmarkJob status on the cluster if changed.
	// This ensures the status is stored and reflected in the resource.
	return r.Status().Update(ctx, benchmarkJob)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.BenchmarkJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
