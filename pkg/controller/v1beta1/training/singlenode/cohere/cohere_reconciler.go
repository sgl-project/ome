package cohere

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/singlenode"
	trainingJobUtils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var (
	_ singlenode.SinglePodTrainingReconciler = &CohereTrainingReconciler{}
)

// CohereTrainingReconciler reconciles a PeftTrainingJob object
type CohereTrainingReconciler struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
}

func NewCohereTrainingReconciler(
	client client.Client,
	scheme *runtime.Scheme) *CohereTrainingReconciler {
	return &CohereTrainingReconciler{
		client: client,
		scheme: scheme,
		log:    ctrl.Log.WithName("PeftTrainingReconciler"),
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//		the TrainingJob object against the actual cluster state, and then
//		perform operations to make the cluster state reflect the state specified by
//		the user.
//	 Currently, we only support single pod training job (only launcher job). We may support multi-pod training (launcher-worker) in the future.
func (r *CohereTrainingReconciler) Reconcile(trainingJob *v1beta1.TrainingJob) (ctrl.Result, error) {
	var launcherRuntimeName = ""
	var launcherPodSpec *v1.PodSpec
	var launcherRuntime *v1beta1.TrainingRuntimeSpec = nil
	var launcherRuntimeLabels map[string]string
	var launcherRuntimeAnnotations map[string]string
	var err error

	cohereJobSpec := &v1beta1.CohereTrainingJobSpec{
		TrainingJobSpec: trainingJob.Spec,
		ReplicaSpecs:    nil,
	}

	if trainingJob.Spec.TrainingFramework.IsRuntimeSpecified() {
		launcherRuntimeName = *cohereJobSpec.TrainingFramework.Runtimes[v1beta1.CohereLauncher]
		launcherRuntime, err = trainingJobUtils.GetTrainingRuntime(r.client, launcherRuntimeName, trainingJob.ObjectMeta.Namespace)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting launcher runtime %s, error: %s", launcherRuntimeName, err)
		}
		if launcherRuntime.IsDisabled() {
			return ctrl.Result{}, fmt.Errorf("specified launcher runtime %s is disabled", launcherRuntimeName)
		}
		// Verify if given runtime supports the given framework
		if launcherRuntime.IsTrainingFrameworkSupported(*cohereJobSpec.TrainingFramework) {
			return ctrl.Result{}, fmt.Errorf("specified launcher runtime %s does not support specified framework/version %+v", launcherRuntimeName, cohereJobSpec.TrainingFramework)
		}
	} else {
		// Default to get the most recently created runtime that supports the training framework
		launcherRuntime, err := cohereJobSpec.TrainingFramework.GetMostRecentSupportedRuntime(r.client, trainingJob.ObjectMeta.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if launcherRuntime == nil {
			return ctrl.Result{}, fmt.Errorf("no launcher runtime found for trainer with framework: %s", cohereJobSpec.TrainingFramework.FrameworkType)
		}
	}

	if len(launcherRuntime.ReplicaSpec.Template.Spec.Containers) == 0 {
		return ctrl.Result{}, fmt.Errorf("no container configuration found in launcher runtime")
	}

	genaiContainerIdx := trainingJobUtils.GetContainerIndex(constants.TrainingJobContainerName, launcherRuntime.ReplicaSpec.Template.Spec.Containers)
	if genaiContainerIdx == -1 {
		return ctrl.Result{}, fmt.Errorf("failed to find genai-container in TrainingRuntime")
	}

	launcherContainer, err := trainingJobUtils.MergeRuntimeContainers(&launcherRuntime.ReplicaSpec.Template.Spec.Containers[genaiContainerIdx], cohereJobSpec.GetLauncherContainer())
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get launcher container")
	}
	launcherPodSpec, err = trainingJobUtils.MergePodSpec(&launcherRuntime.Template.Spec, &cohereJobSpec.GetLauncherReplicaSpec().Template.Spec)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get launcher pod spec")
	}

	launcherPodSpec.Containers = []v1.Container{
		*launcherContainer,
	}
	launcherRuntimeLabels = launcherRuntime.Labels
	launcherRuntimeAnnotations = launcherRuntime.Annotations

	launcherPodSpec.RestartPolicy = r.resolveLauncherRestartPolicy(cohereJobSpec, launcherRuntime)
	launcherReplicas := r.resolveLauncherReplicas(cohereJobSpec, launcherRuntime)

	result, err := r.reconcileLauncher(trainingJob, cohereJobSpec, trainingJob.ObjectMeta, launcherRuntimeName, launcherReplicas, launcherPodSpec, launcherRuntimeLabels,
		launcherRuntimeAnnotations, cohereJobSpec.GetDatasets(), cohereJobSpec.GetModelStorage())
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *CohereTrainingReconciler) reconcileLauncher(
	trainingJob *v1beta1.TrainingJob,
	jobSpec *v1beta1.CohereTrainingJobSpec,
	tjobObjectMeta metav1.ObjectMeta,
	launcherRuntimeName string,
	launcherReplicas int32,
	launcherPodSpec *v1.PodSpec,
	launcherRuntimeLabels map[string]string,
	launcherRuntimeAnnotations map[string]string,
	datasets *map[constants.DatasetType]*v1beta1.Storage,
	modelStorage *v1beta1.Storage) (ctrl.Result, error) {

	launcherPodConfig := &singlenode.LauncherPodConfig{
		Namespace:           tjobObjectMeta.Namespace,
		LauncherRuntimeName: launcherRuntimeName,
		TrainingJobName:     tjobObjectMeta.Name,
		Datasets:            *datasets,
		ModelStorage:        modelStorage,
	}

	// Get runtime specific variables
	var err error
	baseModel, err := jobSpec.GetBaseModel(r.client, tjobObjectMeta.Name, tjobObjectMeta.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	hyperparameters := jobSpec.GetHyperparameters()

	err = NewCohereLauncherPodConfig(launcherPodConfig, r.client, launcherPodSpec, baseModel, hyperparameters)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Labels and annotations priority: additional key-value pairs > training job > runtime
	// Labels and annotations from high priority will overwrite that from low priority
	launcherMeta := metav1.ObjectMeta{
		Name:      tjobObjectMeta.Name,
		Namespace: tjobObjectMeta.Namespace,
		Labels: utils.Union(
			launcherRuntimeLabels,
			tjobObjectMeta.Labels,
			map[string]string{
				constants.TrainingJobPodLabelKey:         tjobObjectMeta.Name,
				constants.TrainingFineTunedModelLabelKey: trainingJobUtils.GetFineTunedModelName(tjobObjectMeta.Name),
				constants.TrainingReplicaTypeLabelKey:    string(v1beta1.PeftFinetuningReplicaTypeLauncher),
				constants.TrainingReplicaNumLabelKey:     strconv.Itoa(int(launcherReplicas)),
				constants.TrainingBaseModelNameLabelKey:  *baseModel.Spec.DisplayName,
				constants.TrainingBaseModelSizeLabelKey:  *baseModel.Spec.ModelParameterSize,
			},
		),
		Annotations: utils.Union(
			launcherRuntimeAnnotations,
			tjobObjectMeta.Annotations,
		),
	}

	res, err := singlenode.ReconcileJob(trainingJob, r.client, r.scheme, launcherPodSpec, launcherReplicas, launcherMeta)
	if err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (r *CohereTrainingReconciler) resolveLauncherRestartPolicy(jobSpec *v1beta1.CohereTrainingJobSpec, runtimeSpec *v1beta1.TrainingRuntimeSpec) v1.RestartPolicy {
	// RestartPolicy priority: training job > runtime > default
	if jobSpec.GetLauncherReplicaSpec().RestartPolicy != "" {
		return jobSpec.GetLauncherReplicaSpec().RestartPolicy
	}
	if runtimeSpec != nil && runtimeSpec.RestartPolicy != "" {
		return runtimeSpec.RestartPolicy
	}
	return v1.RestartPolicyNever
}

// Replicas priority: training job > runtime > default
func (r *CohereTrainingReconciler) resolveLauncherReplicas(jobSpec *v1beta1.CohereTrainingJobSpec, runtimeSpec *v1beta1.TrainingRuntimeSpec) int32 {
	if jobSpec.GetLauncherReplicaSpec().ReplicaCount != nil {
		return *jobSpec.GetLauncherReplicaSpec().ReplicaCount
	}
	if runtimeSpec != nil && runtimeSpec.ReplicaCount != nil {
		return *runtimeSpec.ReplicaCount
	}
	return constants.DefaultTrainingLauncherReplicas
}
