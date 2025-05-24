package components

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinode"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/knative"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinodevllm"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/raw"
	isvcutils "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"
	trainingutils "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"
	"github.com/sgl-project/sgl-ome/pkg/utils"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ Component = &Predictor{}

// Predictor reconciles resources for this component.
type Predictor struct {
	client                            client.Client
	clientset                         kubernetes.Interface
	scheme                            *runtime.Scheme
	inferenceServiceConfig            *controllerconfig.InferenceServicesConfig
	deploymentMode                    constants.DeploymentModeType
	fineTunedServing                  bool
	fineTunedServingWithMergedWeights bool
	Log                               logr.Logger
}

// NewPredictor creates a new Predictor instance.
func NewPredictor(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme,
	inferenceServiceConfig *controllerconfig.InferenceServicesConfig, deploymentMode constants.DeploymentModeType) Component {
	return &Predictor{
		client:                 client,
		clientset:              clientset,
		scheme:                 scheme,
		inferenceServiceConfig: inferenceServiceConfig,
		deploymentMode:         deploymentMode,
		Log:                    ctrl.Log.WithName("PredictorReconciler"),
	}
}

// Reconcile observes the predictor and attempts to drive the status towards the desired state.
func (p *Predictor) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	// Reconcile the base model
	baseModel, baseModelMeta, result, err := p.reconcileBaseModel(isvc)
	if err != nil {
		return result, err
	}

	// Reconcile the fine-tuned weights
	fineTunedWeights, result, err := p.reconcileFineTunedWeights(isvc)
	if err != nil {
		return result, err
	}

	// Reconcile and validate runtime
	sRuntime, runtimeName, result, err := p.getRuntime(isvc, baseModel)
	if err != nil {
		return result, err
	}

	if result, err := p.validateRuntime(isvc, sRuntime); err != nil {
		return result, err
	}

	// Reconcile object metadata and pod spec
	objectMeta, result, err := p.reconcileObjectMeta(isvc, sRuntime, runtimeName, baseModel, baseModelMeta, fineTunedWeights)
	if err != nil {
		return result, err
	}

	podSpec, result, err := p.reconcilePodSpec(isvc, sRuntime, &objectMeta, &baseModel)
	if err != nil {
		return result, err
	}

	workerPodSpec, result, err := p.reconcileWorkerPodSpec(isvc, sRuntime, &objectMeta, &baseModel)
	if err != nil {
		return result, err
	}

	p.Log.Info("Resolved podSpec for inference service",
		"inferenceServiceName", isvc.Name,
		"namespace", isvc.Namespace,
		"podSpec", podSpec)

	size := p.getWorkerSize(isvc, sRuntime)

	// Reconcile deployment based on the deployment mode
	if result, err := p.reconcileDeployment(isvc, objectMeta, &podSpec, size, &workerPodSpec); err != nil {
		return result, err
	}

	// Update the predictor status
	if err := p.updatePredictorStatus(isvc, objectMeta); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (p *Predictor) getWorkerSize(isvc *v1beta1.InferenceService, runtime v1beta1.ServingRuntimeSpec) int {
	var size int

	// Prioritize sizes in order: Predictor.Worker -> WorkerPodSpec
	switch {
	case isvc.Spec.Predictor.Worker != nil && isvc.Spec.Predictor.Worker.Size != nil:
		size = *isvc.Spec.Predictor.Worker.Size
	case runtime.WorkerPodSpec != nil && runtime.WorkerPodSpec.Size != nil:
		size = *runtime.WorkerPodSpec.Size
	default:
		size = 0 // Default value
	}

	return size
}

// reconcileDeployment manages the deployment logic for different deployment modes.
func (p *Predictor) reconcileDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec, workerSize int, workerPodSpec *v1.PodSpec) (ctrl.Result, error) {
	if p.deploymentMode == constants.RawDeployment {
		return p.reconcileRawDeployment(isvc, objectMeta, podSpec)
	}
	if p.deploymentMode == constants.MultiNodeRayVLLM {
		return p.reconcileMultiNodeVLLM(isvc, objectMeta, podSpec)
	}
	if p.deploymentMode == constants.MultiNode {
		return p.reconcileMultiNode(isvc, objectMeta, podSpec, workerSize, workerPodSpec)
	}
	if p.deploymentMode == constants.Serverless {
		return p.reconcileKnativeDeployment(isvc, objectMeta, podSpec)
	}
	p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Invalid deployment mode")
	return ctrl.Result{}, errors.New("invalid deployment mode")
}

func (p *Predictor) reconcileRawDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec) (ctrl.Result, error) {
	r, err := p.createRawKubeReconciler(isvc, objectMeta, podSpec)
	if err != nil {
		return ctrl.Result{}, err
	}
	deployment, err := r.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile predictor")
	}
	isvc.Status.PropagateRawStatus(v1beta1.PredictorComponent, deployment, r.URL)
	return ctrl.Result{}, nil
}

func (p *Predictor) reconcileMultiNodeVLLM(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec) (ctrl.Result, error) {
	p.Log.Info("PipelineParallelism is enabled, skipping raw deployment", "inference service", isvc.Name, "namespace", isvc.Namespace)
	r, err := p.createMultiNodeVllmReconciler(isvc, objectMeta, podSpec)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := p.setMultiNodeRayVLLMReferences(isvc, r); err != nil {
		return ctrl.Result{}, err
	}
	_, result, err := r.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile predictor")
	}
	isvc.Status.PropagateMultiNodeRayVLLMStatus(v1beta1.PredictorComponent, r.MultiNodeProber.Deployments, r.URL)
	return result, nil
}

func (p *Predictor) reconcileMultiNode(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, leaderPodSpec *v1.PodSpec, workerSize int, workerPodSpec *v1.PodSpec) (ctrl.Result, error) {
	p.Log.Info("Reconcile MultiNode", "inference service", isvc.Name, "namespace", isvc.Namespace)
	r, err := p.createMultiNodeReconciler(isvc, objectMeta, leaderPodSpec, workerSize, workerPodSpec)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := p.setMultiNodeMReferences(isvc, r); err != nil {
		return ctrl.Result{}, err
	}
	lws, err := r.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile predictor")
	}
	isvc.Status.PropagateMultiNodeStatus(v1beta1.PredictorComponent, lws, r.URL)
	return ctrl.Result{}, nil
}

// reconcileKnativeDeployment handles the deployment for Knative deployments.
func (p *Predictor) reconcileKnativeDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec) (ctrl.Result, error) {
	r := knative.NewKsvcReconciler(p.client, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec, podSpec, isvc.Status.Components[v1beta1.PredictorComponent])
	if err := controllerutil.SetControllerReference(isvc, r.Service, p.scheme); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to set owner reference for predictor")
	}
	status, err := r.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile predictor")
	}
	isvc.Status.PropagateStatus(v1beta1.PredictorComponent, status)
	return ctrl.Result{}, nil
}

// updatePredictorStatus updates the status of the predictor.
func (p *Predictor) updatePredictorStatus(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta) error {
	rawDeployment := p.deploymentMode == constants.RawDeployment
	statusSpec := isvc.Status.Components[v1beta1.PredictorComponent]
	podLabelKey, podLabelValue := p.getPodLabelInfo(rawDeployment, objectMeta, statusSpec)

	predictorPods, err := isvcutils.ListPodsByLabel(p.client, isvc.ObjectMeta.Namespace, podLabelKey, podLabelValue)
	if err != nil {
		return errors.Wrapf(err, "failed to list inference service pods by label")
	}
	isvc.Status.PropagateModelStatus(statusSpec, predictorPods, rawDeployment)
	return nil
}

// getPodLabelInfo returns the pod label key and value based on the deployment mode.
func (p *Predictor) getPodLabelInfo(rawDeployment bool, objectMeta metav1.ObjectMeta, statusSpec v1beta1.ComponentStatusSpec) (string, string) {
	if rawDeployment {
		return constants.RawDeploymentAppLabel, constants.GetRawServiceLabel(objectMeta.Name)
	}
	return constants.RevisionLabel, statusSpec.LatestCreatedRevision
}

// createRawKubeReconciler creates a new RawKubeReconciler instance.
func (p *Predictor) createRawKubeReconciler(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec) (*raw.RawKubeReconciler, error) {
	r, err := raw.NewRawKubeReconciler(p.client, p.clientset, p.scheme, objectMeta, &isvc.Spec, podSpec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create RawKubeReconciler for predictor")
	}
	if err := p.setRawReferences(isvc, r); err != nil {
		return nil, err
	}
	return r, nil
}

// createMultiNodeVllmReconciler creates a new MultiNodeVllmReconciler instance.
func (p *Predictor) createMultiNodeVllmReconciler(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec) (*multinodevllm.MultiNodeVllmReconciler, error) {
	r, err := multinodevllm.NewMultiNodeVllmReconciler(p.client, p.clientset, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec, podSpec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MultiNodeVllmReconciler for predictor")
	}
	return r, nil
}

func (p *Predictor) createMultiNodeReconciler(isvc *v1beta1.InferenceService,
	objectMeta metav1.ObjectMeta,
	leaderPodSpec *v1.PodSpec,
	workerSize int,
	workerPodSpec *v1.PodSpec) (*multinode.MultiNodeReconciler, error) {
	r, err := multinode.NewMultiNodeReconciler(p.client, p.clientset, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec, leaderPodSpec, workerSize, workerPodSpec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create MultiNodeReconciler for predictor")
	}
	return r, nil
}

// setRawReferences sets the necessary references for raw deployment.
func (p *Predictor) setRawReferences(isvc *v1beta1.InferenceService, r *raw.RawKubeReconciler) error {
	if err := controllerutil.SetControllerReference(isvc, r.Deployment.Deployment, p.scheme); err != nil {
		return errors.Wrapf(err, "failed to set deployment owner reference for predictor")
	}
	if err := controllerutil.SetControllerReference(isvc, r.Service.Service, p.scheme); err != nil {
		return errors.Wrapf(err, "failed to set service owner reference for predictor")
	}
	return r.Scaler.Autoscaler.SetControllerReferences(isvc, p.scheme)
}

// setMultiNodeRayVLLMReferences sets the necessary references for multi-node deployment.
func (p *Predictor) setMultiNodeRayVLLMReferences(isvc *v1beta1.InferenceService, r *multinodevllm.MultiNodeVllmReconciler) error {
	for _, ray := range r.Ray.RayClusters {
		if err := controllerutil.SetControllerReference(isvc, ray, p.scheme); err != nil {
			return errors.Wrapf(err, "failed to set ray owner reference for predictor")
		}
	}
	for _, dply := range r.MultiNodeProber.Deployments {
		if err := controllerutil.SetControllerReference(isvc, dply, p.scheme); err != nil {
			return errors.Wrapf(err, "failed to set prober owner reference for predictor")
		}
	}
	return controllerutil.SetControllerReference(isvc, r.RawMultiNodeService.Service, p.scheme)
}

func (p *Predictor) setMultiNodeMReferences(isvc *v1beta1.InferenceService, mnr *multinode.MultiNodeReconciler) error {
	err := controllerutil.SetControllerReference(isvc, mnr.LWS.LWS, p.scheme)
	if err != nil {
		return errors.Wrapf(err, "failed to set lws owner reference for leader worker set")
	}

	return controllerutil.SetControllerReference(isvc, mnr.Service.Service, p.scheme)

}

// reconcileObjectMeta reconciles the object metadata.
func (p *Predictor) reconcileObjectMeta(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	runtimeName string,
	baseModelSpec v1beta1.BaseModelSpec,
	baseModelMeta metav1.ObjectMeta,
	fineTunedWeights []*v1beta1.FineTunedWeight,
) (metav1.ObjectMeta, ctrl.Result, error) {

	annotations, err := p.processAnnotations(isvc, sRuntime, runtimeName, baseModelSpec, baseModelMeta, fineTunedWeights)
	if err != nil {
		return metav1.ObjectMeta{}, ctrl.Result{}, err
	}

	labels, err := p.processLabels(isvc, sRuntime, runtimeName, baseModelSpec, baseModelMeta, fineTunedWeights)
	if err != nil {
		return metav1.ObjectMeta{}, ctrl.Result{}, err
	}

	predictorName, err := p.determinePredictorName(isvc)
	if err != nil {
		return metav1.ObjectMeta{}, ctrl.Result{}, err
	}

	objectMeta := p.buildObjectMeta(isvc, sRuntime, predictorName, annotations, labels)
	return objectMeta, ctrl.Result{}, nil
}

// processAnnotations processes the annotations for the predictor.
func (p *Predictor) processAnnotations(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	runtimeName string,
	baseModelSpec v1beta1.BaseModelSpec,
	baseModelMeta metav1.ObjectMeta,
	fineTunedWeights []*v1beta1.FineTunedWeight,
) (map[string]string, error) {
	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	sRuntimeAnnotations := utils.Filter(sRuntime.ServingRuntimePodSpec.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	mergedAnnotations := utils.Union(sRuntimeAnnotations, annotations, isvc.Spec.Predictor.Annotations)

	err := p.processServingAnnotations(isvc, mergedAnnotations, runtimeName, baseModelSpec, baseModelMeta, fineTunedWeights)
	if err != nil {
		return mergedAnnotations, errors.Wrapf(err, "failed to process serving annotations")
	}
	return mergedAnnotations, nil
}

// processServingAnnotations processes the annotations for the predictor.
func (p *Predictor) processServingAnnotations(
	isvc *v1beta1.InferenceService,
	annotations map[string]string,
	runtimeName string,
	baseModelSpec v1beta1.BaseModelSpec,
	baseModelMeta metav1.ObjectMeta,
	fineTunedWeights []*v1beta1.FineTunedWeight,
) error {
	if p.fineTunedServing {
		// TODO: Inject serving sidecar for fine-tuned weights downloading for stacked serving case

		// Inject ft adapter for single/non-stacked fine-tuned weight downloading
		annotations[constants.FineTunedAdapterInjectionKey] = fineTunedWeights[0].Name

		// Add fine-tuned weight ft strategy, required by ft adapter & serving sidecar
		fineTunedWeightFTStrategy, err := trainingutils.GetHyperparameterValueByKey(constants.StrategyConfigKey, fineTunedWeights[0].Spec.HyperParameters)
		if err != nil || fineTunedWeightFTStrategy == nil {
			p.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", fineTunedWeights[0].Name, "namespace", isvc.Namespace)
			return err
		}

		annotations[constants.FineTunedWeightFTStrategyKey] = fineTunedWeightFTStrategy.(string)
	}

	if p.fineTunedServingWithMergedWeights {
		// For FT serving using merged FT weights, no need base model, so just delete the original model init injection
		p.Log.Info("Fine-tuned serving with merged weights, deleting model init annotation", "namespace", isvc.Namespace)
		delete(annotations, constants.ModelInitInjectionKey)

		annotations[constants.FTServingWithMergedWeightsAnnotationKey] = "true"
	} else {
		// Add model init required annotations
		baseModelDecryptionKeyName, ok := baseModelMeta.Annotations[constants.BaseModelDecryptionKeyName]
		if ok {
			annotations[constants.BaseModelDecryptionKeyName] = baseModelDecryptionKeyName
		}
		baseModelDecryptionSecretName, ok := baseModelMeta.Annotations[constants.BaseModelDecryptionSecretName]
		if ok {
			annotations[constants.BaseModelDecryptionSecretName] = baseModelDecryptionSecretName
		}
	}
	annotations[constants.BaseModelName] = baseModelMeta.Name
	annotations[constants.BaseModelVendorAnnotationKey] = isvcutils.GetBaseModelVendor(baseModelSpec)
	annotations[constants.ServingRuntimeKeyName] = runtimeName
	annotations[constants.BaseModelFormat] = baseModelSpec.ModelFormat.Name
	annotations[constants.BaseModelFormatVersion] = *baseModelSpec.ModelFormat.Version
	return nil
}

// processLabels processes the label for the predictor.
func (p *Predictor) processLabels(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	runtimeName string,
	baseModelSpec v1beta1.BaseModelSpec,
	baseModelMeta metav1.ObjectMeta,
	fineTunedWeights []*v1beta1.FineTunedWeight,
) (map[string]string, error) {
	predictorLabels := isvc.Spec.Predictor.Labels
	sRuntimeLabels := sRuntime.ServingRuntimePodSpec.Labels

	baseModelCategory, ok := baseModelMeta.Annotations[constants.ModelCategoryAnnotation]
	if !ok {
		baseModelCategory = "SMALL"
	}

	labels := utils.Union(
		sRuntimeLabels,
		isvc.Labels,
		predictorLabels,
		map[string]string{
			constants.InferenceServicePodLabelKey:           isvc.Name,
			constants.KServiceComponentLabel:                string(v1beta1.PredictorComponent),
			constants.InferenceServiceBaseModelNameLabelKey: baseModelMeta.Name,
			constants.InferenceServiceBaseModelSizeLabelKey: baseModelCategory,
			constants.BaseModelTypeLabelKey:                 string(constants.ServingBaseModel),
			constants.BaseModelVendorLabelKey:               isvcutils.GetBaseModelVendor(baseModelSpec),
			constants.ServingRuntimeLabelKey:                runtimeName,
			constants.FTServingLabelKey:                     strconv.FormatBool(p.fineTunedServing),
		},
	)

	// Conditionally add fine-tuned serving related labels
	if p.fineTunedServing {
		ftStrategyParameter, err := trainingutils.GetHyperparameterValueByKey(constants.StrategyConfigKey, fineTunedWeights[0].Spec.HyperParameters)
		if err != nil {
			p.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", fineTunedWeights[0].Name, "namespace", isvc.Namespace)
			return nil, err
		}

		fineTunedWeightFTStrategy := ""
		if ftStrategyParameter != nil {
			fineTunedWeightFTStrategy = ftStrategyParameter.(string)
		}

		labels[constants.FTServingWithMergedWeightsLabelKey] = strconv.FormatBool(p.fineTunedServingWithMergedWeights)
		labels[constants.FineTunedWeightFTStrategyLabelKey] = fineTunedWeightFTStrategy
	}

	return labels, nil
}

// determinePredictorName determines the name of the predictor.
func (p *Predictor) determinePredictorName(isvc *v1beta1.InferenceService) (string, error) {
	defaultPredictorName := constants.DefaultPredictorServiceName(isvc.Name)
	existingName := defaultPredictorName

	if p.deploymentMode == constants.RawDeployment {
		existing := &v1.Service{}
		if err := p.client.Get(context.TODO(), types.NamespacedName{Name: defaultPredictorName, Namespace: isvc.Namespace}, existing); err == nil {
			return existingName, nil
		}
	} else {
		existing := &knservingv1.Service{}
		if err := p.client.Get(context.TODO(), types.NamespacedName{Name: defaultPredictorName, Namespace: isvc.Namespace}, existing); err == nil {
			return existingName, nil
		}
	}

	return constants.PredictorServiceName(isvc.Name), nil
}

// buildObjectMeta builds the object metadata.
func (p *Predictor) buildObjectMeta(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, predictorName string, annotations, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        predictorName,
		Namespace:   isvc.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

// reconcileWorkerPodSpec reconciles the worker pod spec for the predictor.
func (p *Predictor) reconcileWorkerPodSpec(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	objectMeta *metav1.ObjectMeta,
	baseModel *v1beta1.BaseModelSpec,
) (v1.PodSpec, ctrl.Result, error) {
	// Early return if no worker specs are defined
	if sRuntime.WorkerPodSpec == nil && isvc.Spec.Predictor.Worker == nil {
		return v1.PodSpec{}, ctrl.Result{}, nil
	}

	// Find OME container indices in both runtime and isvc worker specs
	containerIndices, err := p.findWorkerContainerIndices(isvc, sRuntime)
	if err != nil {
		return v1.PodSpec{}, ctrl.Result{}, err
	}

	// If no OME container found in either spec, return empty PodSpec
	if containerIndices.bothMissing() {
		return v1.PodSpec{}, ctrl.Result{}, nil
	}

	// Create merged container and pod spec
	workerContainer, workerPodSpec, err := p.createMergedWorkerSpecs(isvc, sRuntime, containerIndices)
	if err != nil {
		return v1.PodSpec{}, ctrl.Result{}, err
	}

	// Update volume mounts and pod spec
	p.updateVolumeMounts(isvc, workerContainer, objectMeta, baseModel)
	p.updateEnvVariables(isvc, workerContainer, baseModel, objectMeta)
	p.updateWorkerPodSpec(
		isvc,
		sRuntime,
		containerIndices.isvcIndex,
		containerIndices.runtimeIndex,
		workerContainer,
		&workerPodSpec,
		baseModel,
	)

	return workerPodSpec, ctrl.Result{}, nil
}

// workerContainerIndices holds the indices of OME containers in worker specs
type workerContainerIndices struct {
	isvcIndex    int
	runtimeIndex int
}

// bothMissing returns true if no OME container was found in either spec
func (w workerContainerIndices) bothMissing() bool {
	return w.isvcIndex == -1 && w.runtimeIndex == -1
}

// findWorkerContainerIndices finds the indices of OME containers in worker specs
func (p *Predictor) findWorkerContainerIndices(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec) (workerContainerIndices, error) {
	indices := workerContainerIndices{
		isvcIndex:    -1,
		runtimeIndex: -1,
	}

	// Find OME container in runtime worker spec
	if sRuntime.WorkerPodSpec != nil && sRuntime.WorkerPodSpec.Containers != nil {
		indices.runtimeIndex = isvcutils.GetOmeContainerIndex(sRuntime.WorkerPodSpec.Containers)
	}

	// Find OME container in isvc worker spec
	if isvc.Spec.Predictor.Worker != nil && isvc.Spec.Predictor.Worker.Containers != nil {
		indices.isvcIndex = isvcutils.GetOmeContainerIndex(isvc.Spec.Predictor.Worker.Containers)
	}

	return indices, nil
}

// createMergedWorkerSpecs creates merged container and pod spec for worker
func (p *Predictor) createMergedWorkerSpecs(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	indices workerContainerIndices,
) (*v1.Container, v1.PodSpec, error) {
	// Create merged container
	workerContainer, err := p.createMergedWorkerContainer(isvc, sRuntime, indices.isvcIndex, indices.runtimeIndex)
	if err != nil {
		return nil, v1.PodSpec{}, err
	}

	// Create merged pod spec
	workerPodSpec, err := p.createMergedWorkerPodSpec(isvc, sRuntime)
	if err != nil {
		return nil, v1.PodSpec{}, err
	}

	return workerContainer, workerPodSpec, nil
}

// createMergedContainer merges the runtime and model containers.
func (p *Predictor) createMergedContainer(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, omeContainerIdx int) (*v1.Container, error) {
	container, err := isvcutils.MergeRuntimeContainers(&sRuntime.Containers[omeContainerIdx], &isvc.Spec.Predictor.Model.Container)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime container")
		return nil, errors.Wrapf(err, "failed to get runtime container")
	}

	if err = isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to replace placeholders in serving runtime Container")
		return nil, errors.Wrapf(err, "failed to replace placeholders in serving runtime Container")
	}

	isvcutils.UpdateImageTag(container, isvc.Spec.Predictor.Model.RuntimeVersion, isvc.Spec.Predictor.Model.Runtime)
	return container, nil
}

// createMergedWorkerContainer merges the runtime and model containers.
func (p *Predictor) createMergedWorkerContainer(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, isvcOmeContainerIdx int, sRuntimeOmeContainerIdx int) (*v1.Container, error) {
	var isvcOmeContainer = v1.Container{}
	if isvcOmeContainerIdx != -1 {
		isvcOmeContainer = isvc.Spec.Predictor.Worker.Containers[isvcOmeContainerIdx]
	}
	container, err := isvcutils.MergeRuntimeContainers(&sRuntime.WorkerPodSpec.Containers[sRuntimeOmeContainerIdx], &isvcOmeContainer)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime worker container")
		return nil, errors.Wrapf(err, "failed to get runtime container")
	}

	if err = isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to replace placeholders in serving runtime worker Container")
		return nil, errors.Wrapf(err, "failed to replace placeholders in serving runtime worker Container")
	}

	return container, nil
}

// createMergedPodSpec merges the runtime and model pod specs.
func (p *Predictor) createMergedPodSpec(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec) (v1.PodSpec, error) {
	mergedPodSpec, err := isvcutils.MergePodSpec(&sRuntime.ServingRuntimePodSpec, &isvc.Spec.Predictor.PodSpec)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime PodSpec")
		return v1.PodSpec{}, errors.Wrapf(err, "failed to consolidate serving runtime PodSpecs")
	}
	return *mergedPodSpec, nil
}

// createMergedWorkerPodSpec merges worker pod specs of the runtime and the inferenceService.
func (p *Predictor) createMergedWorkerPodSpec(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec) (v1.PodSpec, error) {
	var isvcWorkerPodSpec = v1beta1.PodSpec{}
	if isvc.Spec.Predictor.Worker != nil {
		isvcWorkerPodSpec = isvc.Spec.Predictor.Worker.PodSpec
	}
	mergedPodSpec, err := isvcutils.MergePodSpec(&sRuntime.WorkerPodSpec.ServingRuntimePodSpec, &isvcWorkerPodSpec)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime WorkerPodSpec")
		return v1.PodSpec{}, errors.Wrapf(err, "failed to consolidate serving runtime WorkerPodSpecs")
	}
	return *mergedPodSpec, nil
}

// updateVolumeMounts updates the volume mounts for the predictor.
func (p *Predictor) updateVolumeMounts(
	isvc *v1beta1.InferenceService,
	container *v1.Container,
	objectMeta *metav1.ObjectMeta,
	baseModel *v1beta1.BaseModelSpec,
) {
	p.Log.Info("Update volume mounts", "inference service", isvc.Name, "namespace", isvc.Namespace)

	if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
		vm := v1.VolumeMount{
			Name:      *isvc.Spec.Predictor.Model.BaseModel,
			MountPath: *baseModel.Storage.Path,
			ReadOnly:  true,
		}
		isvcutils.AppendVolumeMount(container, &vm)
	}

	if p.fineTunedServing {
		defaultModelVolumeMount := v1.VolumeMount{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: constants.ModelDefaultMountPath,
		}
		isvcutils.AppendVolumeMountIfNotExist(container, &defaultModelVolumeMount)

		if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
			// Update to have `base` sub-path in model volume mount for cohere tfew stacked serving case
			defaultModelVolumeMountWithSubPath := v1.VolumeMount{
				Name:      constants.ModelEmptyDirVolumeName,
				MountPath: filepath.Join(constants.ModelDefaultMountPath, objectMeta.Annotations[constants.BaseModelFormat]),
				SubPath:   constants.BaseModelVolumeMountSubPath,
			}
			isvcutils.UpdateVolumeMount(container, &defaultModelVolumeMountWithSubPath)

			tfewFineTunedWeightVolumeMount := v1.VolumeMount{
				Name:      constants.ModelEmptyDirVolumeName,
				MountPath: filepath.Join(constants.CohereTFewFineTunedWeightVolumeMountPath, objectMeta.Annotations[constants.BaseModelFormat]),
				ReadOnly:  true,
				SubPath:   constants.FineTunedWeightVolumeMountSubPath,
			}
			isvcutils.AppendVolumeMount(container, &tfewFineTunedWeightVolumeMount)
		}
	}

	if isvcutils.IsBlockListInjectionDisabled(objectMeta.Annotations) {
		inputBlocklistVolumeMount := v1.VolumeMount{
			Name:      constants.BlocklistConfigMapVolumeName,
			MountPath: constants.InputBlocklistMountPath,
			ReadOnly:  true,
			SubPath:   constants.InputBlocklistSubPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, inputBlocklistVolumeMount)
		outputBlocklistVolumeMount := v1.VolumeMount{
			Name:      constants.BlocklistConfigMapVolumeName,
			MountPath: constants.OutputBlocklistMountPath,
			ReadOnly:  true,
			SubPath:   constants.OutputBlocklistSubPath,
		}
		container.VolumeMounts = append(container.VolumeMounts, outputBlocklistVolumeMount)
	}
}

func (p *Predictor) updateEnvVariables(
	isvc *v1beta1.InferenceService,
	container *v1.Container,
	baseModel *v1beta1.BaseModelSpec,
	objectMeta *metav1.ObjectMeta) {
	if !p.fineTunedServing {
		if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
			p.Log.Info("Base model serving - adding Model_PATH env variable", "inference service", isvc.Name, "namespace", isvc.Namespace, "base model", isvc.Spec.Predictor.Model.Name)
			isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
				{Name: constants.ModelPathEnvVarKey, Value: *baseModel.Storage.Path},
			})
		}
	} else {
		if baseModel.Vendor == nil {
			p.Log.Info("Warning: no vendor given in base model spec - no env var added/updated")
		} else if *baseModel.Vendor == string(constants.Meta) {
			isvcutils.UpdateEnvVars(container, &v1.EnvVar{
				Name: constants.ServedModelNameEnvVarKey, Value: filepath.Join(
					constants.LLamaVllmFTServingServedModelNamePrefix,
					objectMeta.Annotations[constants.FineTunedAdapterInjectionKey])},
			)
			isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
				{Name: constants.ModelPathEnvVarKey, Value: constants.ModelDefaultMountPath},
			})
		} else if *baseModel.Vendor == string(constants.Cohere) {
			if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
				isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
					{Name: constants.TFewWeightPathEnvVarKey, Value: constants.CohereTFewFineTunedWeightDefaultPath},
				})
			}
		}
	}
}

// updatePodSpec updates the pod spec for the predictor.
func (p *Predictor) updatePodSpec(isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	omeContainerIdx int,
	container *v1.Container,
	podSpec *v1.PodSpec,
	objectMeta *metav1.ObjectMeta,
	baseModel *v1beta1.BaseModelSpec,
) {
	// Update containers by inserting the custom container and keeping the other runtime containers
	podSpec.Containers = append([]v1.Container{*container}, sRuntime.Containers[:omeContainerIdx]...)
	podSpec.Containers = append(podSpec.Containers, sRuntime.Containers[omeContainerIdx+1:]...)

	// Initialize volumes and add the main model volume
	volumes := []v1.Volume{
		{
			Name: *isvc.Spec.Predictor.Model.BaseModel,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: *baseModel.Storage.Path,
				},
			},
		},
	}

	if isvcutils.IsEmptyModelDirVolumeRequired(objectMeta.Annotations) {
		emptyModelDirVolume := v1.Volume{
			Name: constants.ModelEmptyDirVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		}
		podSpec.Volumes = utils.AppendVolumeIfNotExists(podSpec.Volumes, emptyModelDirVolume)
	}

	if isvcutils.IsBlockListInjectionDisabled(objectMeta.Annotations) {
		blockListConfigMapVolume := v1.Volume{
			Name: constants.BlocklistConfigMapVolumeName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: constants.ModelConfigName(isvc.Name),
					},
				},
			},
		}
		volumes = append(volumes, blockListConfigMapVolume)
	}

	// Append volumes to the podSpec
	podSpec.Volumes = append(podSpec.Volumes, volumes...)

	p.Log.Info("PodSpec updated", "inference service", isvc.Name, "namespace", isvc.Namespace)
}

// updateWorkerPodSpec updates the worker pod spec for the predictor.
func (p *Predictor) updateWorkerPodSpec(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, isvcOmeContainerIdx int, sRuntimeOmeContainerIdx int, container *v1.Container, podSpec *v1.PodSpec, baseModel *v1beta1.BaseModelSpec) {
	// Update containers by inserting the custom container and keeping the other runtime containers
	podSpec.Containers = append([]v1.Container{*container}, sRuntime.WorkerPodSpec.Containers[:sRuntimeOmeContainerIdx]...)
	podSpec.Containers = append(podSpec.Containers, sRuntime.WorkerPodSpec.Containers[sRuntimeOmeContainerIdx+1:]...)
	if isvcOmeContainerIdx != -1 {
		podSpec.Containers = append([]v1.Container{*container}, isvc.Spec.Predictor.Worker.Containers[:isvcOmeContainerIdx]...)
		podSpec.Containers = append(podSpec.Containers, isvc.Spec.Predictor.Worker.Containers[isvcOmeContainerIdx+1:]...)
	}

	// Initialize volumes and add the main PVC volume
	volumes := []v1.Volume{
		{
			Name: *isvc.Spec.Predictor.Model.BaseModel,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: *baseModel.Storage.Path,
				},
			},
		},
	}

	// Append volumes to the podSpec
	podSpec.Volumes = append(podSpec.Volumes, volumes...)

	p.Log.Info("PodSpec updated", "inference service", isvc.Name, "namespace", isvc.Namespace)
}

// validateRuntime validates the runtime for the predictor.
func (p *Predictor) validateRuntime(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec) (ctrl.Result, error) {
	if len(sRuntime.Containers) == 0 {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "No container configuration found in selected serving runtime")
		return ctrl.Result{}, errors.New("no container configuration found in selected serving runtime")
	}

	omeContainerIdx := isvcutils.GetOmeContainerIndex(sRuntime.Containers)
	if omeContainerIdx == -1 {
		return ctrl.Result{}, errors.New("failed to find ome-container in ServingRuntime containers")
	}
	return ctrl.Result{}, nil
}

// getRuntime retrieves the serving runtime for the predictor.
func (p *Predictor) getRuntime(isvc *v1beta1.InferenceService, baseModel v1beta1.BaseModelSpec) (v1beta1.ServingRuntimeSpec, string, ctrl.Result, error) {
	if isvc.Spec.Predictor.Model.Runtime != nil {
		runtimeSpec, result, err := p.getSpecifiedRuntime(isvc, baseModel)
		return runtimeSpec, *isvc.Spec.Predictor.Model.Runtime, result, err
	}
	return p.getSupportingRuntime(isvc, baseModel)
}

// getSupportingRuntime retrieves the supporting runtime for the predictor.
func (p *Predictor) getSupportingRuntime(isvc *v1beta1.InferenceService, baseModel v1beta1.BaseModelSpec) (v1beta1.ServingRuntimeSpec, string, ctrl.Result, error) {
	runtimes, err := isvcutils.GetSupportingRuntimes(isvc.Spec.Predictor.Model, p.client, isvc.Namespace)
	if err != nil {
		return v1beta1.ServingRuntimeSpec{}, "", ctrl.Result{}, err
	}

	if len(runtimes) == 0 {
		p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "No runtime found to support specified framework/version")
		return v1beta1.ServingRuntimeSpec{}, "", ctrl.Result{}, fmt.Errorf("no runtime found to support specified predictor with model type: %v", baseModel.ModelFormat.Name)
	}

	// Use the first supporting runtime.
	isvc.Spec.Predictor.Model.Runtime = &runtimes[0].Name
	p.Log.Info("Using first supporting runtime", "runtime", *isvc.Spec.Predictor.Model.Runtime, "inference service", isvc.Name, "namespace", isvc.Namespace)

	return runtimes[0].Spec, runtimes[0].Name, ctrl.Result{}, nil
}

// getSpecifiedRuntime retrieves the specified runtime for the predictor.
func (p *Predictor) getSpecifiedRuntime(isvc *v1beta1.InferenceService, baseModel v1beta1.BaseModelSpec) (v1beta1.ServingRuntimeSpec, ctrl.Result, error) {

	rt, err := isvcutils.GetServingRuntime(p.client, *isvc.Spec.Predictor.Model.Runtime, isvc.Namespace)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.RuntimeNotRecognized, "Waiting for runtime to become available")
		return v1beta1.ServingRuntimeSpec{}, ctrl.Result{}, err
	}

	if rt.IsDisabled() {
		p.updateModelTransitionStatus(isvc, v1beta1.RuntimeDisabled, "Specified runtime is disabled")
		return v1beta1.ServingRuntimeSpec{}, ctrl.Result{}, fmt.Errorf("specified runtime %s is disabled", *isvc.Spec.Predictor.Model.Runtime)
	}

	if !p.isProtocolVersionSupported(isvc, rt) {
		p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "Specified runtime does not support specified protocol version")
		return v1beta1.ServingRuntimeSpec{}, ctrl.Result{}, fmt.Errorf("specified runtime %s does not support specified protocol version", *isvc.Spec.Predictor.Model.Runtime)
	}

	if !isvcutils.RuntimeSupportsModel(isvc.Spec.Predictor.Model, rt, &baseModel) {
		p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "Specified runtime does not support specified framework/version")
		return v1beta1.ServingRuntimeSpec{}, ctrl.Result{}, fmt.Errorf("specified runtime %s does not support specified predictor with model type: %v", *isvc.Spec.Predictor.Model.Runtime, baseModel.ModelFormat.Name)
	}

	return *rt, ctrl.Result{}, nil
}

// isProtocolVersionSupported checks if the protocol version is supported by the runtime.
func (p *Predictor) isProtocolVersionSupported(isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec) bool {
	if isvc.Spec.Predictor.Model.ProtocolVersion == nil {
		return true
	}

	protocolVersion := isvcutils.GetProtocol(isvc.Spec.Predictor.Model)

	return runtime.IsProtocolVersionSupported(protocolVersion)
}

// reconcileBaseModel reconciles the base model for the predictor.
func (p *Predictor) reconcileBaseModel(isvc *v1beta1.InferenceService) (v1beta1.BaseModelSpec, metav1.ObjectMeta, ctrl.Result, error) {
	if isvc.Spec.Predictor.Model.BaseModel == nil {
		return v1beta1.BaseModelSpec{}, metav1.ObjectMeta{}, ctrl.Result{}, nil
	}

	baseModel, baseModelMeta, err := p.getBaseModelSpec(isvc)
	if err != nil {
		return v1beta1.BaseModelSpec{}, metav1.ObjectMeta{}, ctrl.Result{}, err
	}

	if *baseModel.Disabled {
		p.updateModelTransitionStatus(isvc, v1beta1.BaseModelDisabled, "Specified base model is disabled")
		return v1beta1.BaseModelSpec{}, metav1.ObjectMeta{}, ctrl.Result{}, fmt.Errorf("specified base model %s is disabled", *isvc.Spec.Predictor.Model.BaseModel)
	}

	return baseModel, baseModelMeta, ctrl.Result{}, nil
}

// reconcileFineTunedWeights reconciles the fine-tuned weights for the predictor.
func (p *Predictor) reconcileFineTunedWeights(isvc *v1beta1.InferenceService) ([]*v1beta1.FineTunedWeight, ctrl.Result, error) {
	numOfFineTunedWeights := len(isvc.Spec.Predictor.Model.FineTunedWeights)
	if numOfFineTunedWeights == 0 {
		return nil, ctrl.Result{}, nil
	}

	p.Log.Info("FT serving mode", "Number of fine-tuned weights", numOfFineTunedWeights)
	p.fineTunedServing = true

	// TODO: lift here when start supporting stacked FT serving
	if numOfFineTunedWeights > 1 {
		return nil, ctrl.Result{}, fmt.Errorf("stacked fine-tuned serving is not supported yet")
	}

	allFineTunedWeights := make([]*v1beta1.FineTunedWeight, 0)

	for _, fineTunedWeightName := range isvc.Spec.Predictor.Model.FineTunedWeights {
		fineTunedWeight, err := isvcutils.GetFineTunedWeight(p.client, fineTunedWeightName)
		if err != nil {
			return make([]*v1beta1.FineTunedWeight, 0), ctrl.Result{}, err
		}

		allFineTunedWeights = append(allFineTunedWeights, fineTunedWeight)
	}

	// Determine if loading a merged fine-tuned weights
	loadingMergedFineTunedWeights, err := isvcutils.LoadingMergedFineTunedWeight(allFineTunedWeights)
	if err != nil {
		p.Log.Error(err, "Failed to determine if loading merged fine-tuned weights")
		return allFineTunedWeights, ctrl.Result{}, err
	}
	p.fineTunedServingWithMergedWeights = loadingMergedFineTunedWeights

	return allFineTunedWeights, ctrl.Result{}, nil
}

// getBaseModelSpec retrieves the base model spec.
func (p *Predictor) getBaseModelSpec(isvc *v1beta1.InferenceService) (v1beta1.BaseModelSpec, metav1.ObjectMeta, error) {
	bm, bmMeta, err := isvcutils.GetBaseModel(p.client, *isvc.Spec.Predictor.Model.BaseModel, isvc.Namespace)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.BaseModelNotFound, "Waiting for base model to become available")
		return v1beta1.BaseModelSpec{}, metav1.ObjectMeta{}, err
	}
	return *bm, *bmMeta, nil
}

// updateModelTransitionStatus updates the model transition status for the predictor.
func (p *Predictor) updateModelTransitionStatus(isvc *v1beta1.InferenceService, reason v1beta1.FailureReason, message string) {
	isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
		Reason:  reason,
		Message: message,
	})
}

// reconcilePodSpec reconciles the pod spec.
func (p *Predictor) reconcilePodSpec(
	isvc *v1beta1.InferenceService,
	sRuntime v1beta1.ServingRuntimeSpec,
	objectMeta *metav1.ObjectMeta,
	baseModel *v1beta1.BaseModelSpec,
) (v1.PodSpec, ctrl.Result, error) {
	// find the OME container index, the container name must be ome-container; nothing else will be accepted
	// TODO: this is a temporary solution, we need to find a better way to identify the OME container,
	// particularly when we have multiple containers and multiple nodes in the serving runtime
	omeContainerIdx := isvcutils.GetOmeContainerIndex(sRuntime.Containers)
	container, err := p.createMergedContainer(isvc, sRuntime, omeContainerIdx)

	if err != nil {
		return v1.PodSpec{}, ctrl.Result{}, err
	}

	podSpec, err := p.createMergedPodSpec(isvc, sRuntime)
	if err != nil {
		return v1.PodSpec{}, ctrl.Result{}, err
	}

	p.updateVolumeMounts(isvc, container, objectMeta, baseModel)
	p.updateEnvVariables(isvc, container, baseModel, objectMeta)
	p.updatePodSpec(isvc, sRuntime, omeContainerIdx, container, &podSpec, objectMeta, baseModel)

	return podSpec, ctrl.Result{}, nil
}
