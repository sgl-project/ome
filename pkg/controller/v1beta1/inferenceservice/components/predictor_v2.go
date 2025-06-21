package components

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/common"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/status"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Component = &PredictorV2{}

// PredictorV2 is the refactored predictor component
type PredictorV2 struct {
	BaseComponentFields
	deploymentReconciler *common.DeploymentReconciler
	podSpecReconciler    *common.PodSpecReconciler
}

// NewPredictorV2 creates a new refactored Predictor instance
func NewPredictorV2(
	client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	inferenceServiceConfig *controllerconfig.InferenceServicesConfig,
	deploymentMode constants.DeploymentModeType,
) Component {
	base := BaseComponentFields{
		Client:                 client,
		Clientset:              clientset,
		Scheme:                 scheme,
		InferenceServiceConfig: inferenceServiceConfig,
		DeploymentMode:         deploymentMode,
		StatusManager:          status.NewStatusReconciler(),
		Log:                    ctrl.Log.WithName("PredictorReconcilerV2"),
	}

	return &PredictorV2{
		BaseComponentFields: base,
		deploymentReconciler: &common.DeploymentReconciler{
			Client:        client,
			Clientset:     clientset,
			Scheme:        scheme,
			StatusManager: base.StatusManager,
			Log:           base.Log,
		},
		podSpecReconciler: &common.PodSpecReconciler{
			Log: base.Log,
		},
	}
}

// Reconcile implements the Component interface for PredictorV2
func (p *PredictorV2) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	p.Log.Info("Reconciling predictor component", "inferenceService", isvc.Name, "namespace", isvc.Namespace)

	// Validate predictor spec
	if isvc.Spec.Predictor.Model == nil {
		return ctrl.Result{}, errors.New("predictor model spec is nil")
	}

	// Reconcile base model
	if err := p.reconcileBaseModel(isvc); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile base model")
	}

	// Reconcile fine-tuned weights if specified
	if err := p.reconcileFineTunedWeights(isvc); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile fine-tuned weights")
	}

	// Get runtime
	runtime, runtimeName, err := p.getRuntime(isvc)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get runtime")
	}

	// Validate runtime
	if err := p.validateRuntime(runtime); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to validate runtime")
	}

	// Update BaseComponentFields with runtime info
	p.Runtime = &runtime
	p.RuntimeName = runtimeName

	// Reconcile object metadata
	objectMeta, err := p.reconcileObjectMeta(isvc)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile object metadata")
	}

	// Reconcile pod spec
	podSpec, err := p.reconcilePodSpec(isvc, runtime, &objectMeta)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile pod spec")
	}

	// Reconcile worker pod spec if needed
	workerPodSpec, err := p.reconcileWorkerPodSpec(isvc, runtime, &objectMeta)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile worker pod spec")
	}

	// Get worker size
	size := p.getWorkerSize(isvc, runtime)

	// Reconcile deployment based on deployment mode
	if result, err := p.reconcileDeployment(isvc, objectMeta, podSpec, size, workerPodSpec); err != nil {
		return result, err
	}

	// Update predictor status
	if err := p.updatePredictorStatus(isvc, objectMeta); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileBaseModel reconciles the base model for the predictor
func (p *PredictorV2) reconcileBaseModel(isvc *v1beta1.InferenceService) error {
	if isvc.Spec.Predictor.Model.BaseModel == nil {
		return nil
	}

	baseModel, baseModelMeta, err := isvcutils.GetBaseModel(p.Client, *isvc.Spec.Predictor.Model.BaseModel, isvc.Namespace)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.BaseModelNotFound, "Waiting for base model to become available")
		return err
	}

	// Check if base model is disabled
	if baseModel.Disabled != nil && *baseModel.Disabled {
		p.updateModelTransitionStatus(isvc, v1beta1.BaseModelDisabled, "Specified base model is disabled")
		return errors.Errorf("specified base model %s is disabled", *isvc.Spec.Predictor.Model.BaseModel)
	}

	p.BaseModel = baseModel
	p.BaseModelMeta = baseModelMeta
	return nil
}

// reconcileFineTunedWeights reconciles the fine-tuned weights for the predictor
func (p *PredictorV2) reconcileFineTunedWeights(isvc *v1beta1.InferenceService) error {
	numOfFineTunedWeights := len(isvc.Spec.Predictor.Model.FineTunedWeights)
	if numOfFineTunedWeights == 0 {
		return nil
	}

	p.Log.Info("FT serving mode", "Number of fine-tuned weights", numOfFineTunedWeights)
	p.FineTunedServing = true

	// TODO: lift here when start supporting stacked FT serving
	if numOfFineTunedWeights > 1 {
		return errors.New("stacked fine-tuned serving is not supported yet")
	}

	allFineTunedWeights := make([]*v1beta1.FineTunedWeight, 0)

	for _, fineTunedWeightName := range isvc.Spec.Predictor.Model.FineTunedWeights {
		fineTunedWeight, err := isvcutils.GetFineTunedWeight(p.Client, fineTunedWeightName)
		if err != nil {
			return err
		}
		allFineTunedWeights = append(allFineTunedWeights, fineTunedWeight)
	}

	// Determine if loading merged fine-tuned weights
	loadingMergedFineTunedWeights, err := isvcutils.LoadingMergedFineTunedWeight(allFineTunedWeights)
	if err != nil {
		p.Log.Error(err, "Failed to determine if loading merged fine-tuned weights")
		return err
	}
	p.FineTunedServingWithMergedWeights = loadingMergedFineTunedWeights
	p.FineTunedWeights = allFineTunedWeights

	return nil
}

// getRuntime retrieves the runtime for the predictor
func (p *PredictorV2) getRuntime(isvc *v1beta1.InferenceService) (v1beta1.ServingRuntimeSpec, string, error) {
	if isvc.Spec.Predictor.Model.Runtime != nil {
		// Use specified runtime
		rt, err := isvcutils.GetServingRuntime(p.Client, *isvc.Spec.Predictor.Model.Runtime, isvc.Namespace)
		if err != nil {
			p.updateModelTransitionStatus(isvc, v1beta1.RuntimeNotRecognized, "Waiting for runtime to become available")
			return v1beta1.ServingRuntimeSpec{}, "", err
		}

		if rt.IsDisabled() {
			p.updateModelTransitionStatus(isvc, v1beta1.RuntimeDisabled, "Specified runtime is disabled")
			return v1beta1.ServingRuntimeSpec{}, "", errors.Errorf("specified runtime %s is disabled", *isvc.Spec.Predictor.Model.Runtime)
		}

		// Check protocol version support
		if !p.isProtocolVersionSupported(isvc, rt) {
			p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "Specified runtime does not support specified protocol version")
			return v1beta1.ServingRuntimeSpec{}, "", errors.Errorf("specified runtime %s does not support specified protocol version", *isvc.Spec.Predictor.Model.Runtime)
		}

		// Verify runtime supports the model
		if p.BaseModel != nil && !isvcutils.RuntimeSupportsModel(isvc.Spec.Predictor.Model, rt, p.BaseModel) {
			p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "Specified runtime does not support specified framework/version")
			return v1beta1.ServingRuntimeSpec{}, "", errors.Errorf("specified runtime %s does not support predictor with model type: %v",
				*isvc.Spec.Predictor.Model.Runtime, p.BaseModel.ModelFormat.Name)
		}

		return *rt, *isvc.Spec.Predictor.Model.Runtime, nil
	}

	// Auto-select runtime
	runtimes, err := isvcutils.GetSupportingRuntimes(isvc.Spec.Predictor.Model, p.Client, isvc.Namespace)
	if err != nil {
		return v1beta1.ServingRuntimeSpec{}, "", err
	}

	if len(runtimes) == 0 {
		p.updateModelTransitionStatus(isvc, v1beta1.NoSupportingRuntime, "No runtime found to support specified framework/version")
		return v1beta1.ServingRuntimeSpec{}, "", errors.New("no runtime found to support specified predictor")
	}

	// Use the first supporting runtime
	selectedRuntime := &runtimes[0]
	isvc.Spec.Predictor.Model.Runtime = &selectedRuntime.Name
	p.Log.Info("Auto-selected runtime", "runtime", selectedRuntime.Name, "inferenceService", isvc.Name)

	return selectedRuntime.Spec, selectedRuntime.Name, nil
}

// isProtocolVersionSupported checks if the protocol version is supported by the runtime
func (p *PredictorV2) isProtocolVersionSupported(isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec) bool {
	if isvc.Spec.Predictor.Model.ProtocolVersion == nil {
		return true
	}

	protocolVersion := isvcutils.GetProtocol(isvc.Spec.Predictor.Model)
	return runtime.IsProtocolVersionSupported(protocolVersion)
}

// validateRuntime validates the runtime
func (p *PredictorV2) validateRuntime(runtime v1beta1.ServingRuntimeSpec) error {
	if len(runtime.Containers) == 0 {
		p.Log.Error(nil, "No containers in runtime", "runtime", p.RuntimeName)
		return errors.New("no container configuration found in selected serving runtime")
	}

	omeContainerIdx := isvcutils.GetContainerIndex(runtime.Containers, constants.MainContainerName)
	if omeContainerIdx == -1 {
		return errors.New("failed to find ome-container in ServingRuntime containers")
	}

	return nil
}

// getWorkerSize returns the worker size for multi-node deployments
func (p *PredictorV2) getWorkerSize(isvc *v1beta1.InferenceService, runtime v1beta1.ServingRuntimeSpec) int {
	// Prioritize sizes in order: Predictor.Worker -> WorkerPodSpec
	if isvc.Spec.Predictor.Worker != nil && isvc.Spec.Predictor.Worker.Size != nil {
		return *isvc.Spec.Predictor.Worker.Size
	}
	if runtime.WorkerPodSpec != nil && runtime.WorkerPodSpec.Size != nil {
		return *runtime.WorkerPodSpec.Size
	}
	return 0 // Default value
}

// reconcileDeployment reconciles the deployment based on the deployment mode
func (p *PredictorV2) reconcileDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *v1.PodSpec, workerSize int, workerPodSpec *v1.PodSpec) (ctrl.Result, error) {
	switch p.DeploymentMode {
	case constants.RawDeployment:
		return p.deploymentReconciler.ReconcileRawDeployment(isvc, objectMeta, podSpec, &isvc.Spec.Predictor.ComponentExtensionSpec, v1beta1.PredictorComponent)
	case constants.MultiNode:
		return p.deploymentReconciler.ReconcileMultiNodeDeployment(isvc, objectMeta, podSpec, workerSize, workerPodSpec, &isvc.Spec.Predictor.ComponentExtensionSpec, v1beta1.PredictorComponent)
	case constants.Serverless:
		return p.deploymentReconciler.ReconcileKnativeDeployment(isvc, objectMeta, podSpec, &isvc.Spec.Predictor.ComponentExtensionSpec, v1beta1.PredictorComponent)
	case constants.MultiNodeRayVLLM:
		return p.deploymentReconciler.ReconcileMultiNodeRayVLLMDeployment(isvc, objectMeta, podSpec, &isvc.Spec.Predictor.ComponentExtensionSpec, v1beta1.PredictorComponent)
	default:
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Invalid deployment mode")
		return ctrl.Result{}, errors.New("invalid deployment mode for predictor")
	}
}

// updatePredictorStatus updates the status of the predictor
func (p *PredictorV2) updatePredictorStatus(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta) error {
	rawDeployment := p.DeploymentMode == constants.RawDeployment
	statusSpec := isvc.Status.Components[v1beta1.PredictorComponent]
	podLabelKey, podLabelValue := p.getPodLabelInfo(rawDeployment, objectMeta, statusSpec)

	predictorPods, err := isvcutils.ListPodsByLabel(p.Client, isvc.ObjectMeta.Namespace, podLabelKey, podLabelValue)
	if err != nil {
		return errors.Wrapf(err, "failed to list predictor pods by label")
	}
	p.StatusManager.PropagateModelStatus(&isvc.Status, statusSpec, predictorPods, rawDeployment)
	return nil
}

// getPodLabelInfo returns the pod label key and value based on the deployment mode
func (p *PredictorV2) getPodLabelInfo(rawDeployment bool, objectMeta metav1.ObjectMeta, statusSpec v1beta1.ComponentStatusSpec) (string, string) {
	if rawDeployment {
		return constants.RawDeploymentAppLabel, constants.GetRawServiceLabel(objectMeta.Name)
	}
	return constants.RevisionLabel, statusSpec.LatestCreatedRevision
}

// reconcileObjectMeta creates the object metadata for the predictor component
func (p *PredictorV2) reconcileObjectMeta(isvc *v1beta1.InferenceService) (metav1.ObjectMeta, error) {
	annotations, err := p.processAnnotations(isvc)
	if err != nil {
		return metav1.ObjectMeta{}, err
	}

	labels := p.processLabels(isvc)

	predictorName, err := p.determinePredictorName(isvc)
	if err != nil {
		return metav1.ObjectMeta{}, err
	}

	return metav1.ObjectMeta{
		Name:        predictorName,
		Namespace:   isvc.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}, nil
}

// processAnnotations processes the annotations for the predictor
func (p *PredictorV2) processAnnotations(isvc *v1beta1.InferenceService) (map[string]string, error) {
	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	// Merge with runtime annotations if available
	if p.Runtime != nil {
		runtimeAnnotations := utils.Filter(p.Runtime.ServingRuntimePodSpec.Annotations, func(key string) bool {
			return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
		})
		annotations = utils.Union(runtimeAnnotations, annotations)
	}

	// Merge with predictor annotations
	mergedAnnotations := utils.Union(annotations, isvc.Spec.Predictor.Annotations)

	// Process serving-specific annotations
	if err := p.processServingAnnotations(isvc, mergedAnnotations); err != nil {
		return nil, errors.Wrap(err, "failed to process serving annotations")
	}

	return mergedAnnotations, nil
}

// processServingAnnotations processes the serving-specific annotations for the predictor
func (p *PredictorV2) processServingAnnotations(isvc *v1beta1.InferenceService, annotations map[string]string) error {
	if p.FineTunedServing && len(p.FineTunedWeights) > 0 {
		// TODO: Inject serving sidecar for fine-tuned weights downloading for stacked serving case

		// Inject ft adapter for single/non-stacked fine-tuned weight downloading
		annotations[constants.FineTunedAdapterInjectionKey] = p.FineTunedWeights[0].Name

		// Add fine-tuned weight ft strategy, required by ft adapter & serving sidecar
		fineTunedWeightFTStrategy, err := isvcutils.GetValueFromRawExtension(p.FineTunedWeights[0].Spec.HyperParameters, constants.StrategyConfigKey)
		if err != nil || fineTunedWeightFTStrategy == nil {
			p.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", p.FineTunedWeights[0].Name, "namespace", isvc.Namespace)
			return err
		}

		annotations[constants.FineTunedWeightFTStrategyKey] = fineTunedWeightFTStrategy.(string)
	}

	if p.FineTunedServingWithMergedWeights {
		// For FT serving using merged FT weights, no need base model, so just delete the original model init injection
		p.Log.Info("Fine-tuned serving with merged weights, deleting model init annotation", "namespace", isvc.Namespace)
		delete(annotations, constants.ModelInitInjectionKey)

		annotations[constants.FTServingWithMergedWeightsAnnotationKey] = "true"
	} else {
		// Add model init required annotations
		if p.BaseModelMeta != nil {
			baseModelDecryptionKeyName, ok := p.BaseModelMeta.Annotations[constants.BaseModelDecryptionKeyName]
			if ok {
				annotations[constants.BaseModelDecryptionKeyName] = baseModelDecryptionKeyName
			}
			baseModelDecryptionSecretName, ok := p.BaseModelMeta.Annotations[constants.BaseModelDecryptionSecretName]
			if ok {
				annotations[constants.BaseModelDecryptionSecretName] = baseModelDecryptionSecretName
			}
		}
	}

	if p.BaseModelMeta != nil {
		annotations[constants.BaseModelName] = p.BaseModelMeta.Name
	}
	if p.BaseModel != nil {
		annotations[constants.BaseModelVendorAnnotationKey] = isvcutils.GetBaseModelVendor(*p.BaseModel)
		annotations[constants.BaseModelFormat] = p.BaseModel.ModelFormat.Name
		if p.BaseModel.ModelFormat.Version != nil {
			annotations[constants.BaseModelFormatVersion] = *p.BaseModel.ModelFormat.Version
		}
	}
	annotations[constants.ServingRuntimeKeyName] = p.RuntimeName

	return nil
}

// processLabels processes the labels for the predictor
func (p *PredictorV2) processLabels(isvc *v1beta1.InferenceService) map[string]string {
	predictorLabels := isvc.Spec.Predictor.Labels

	// Start with runtime labels if available
	labels := map[string]string{}
	if p.Runtime != nil {
		labels = utils.Union(labels, p.Runtime.ServingRuntimePodSpec.Labels)
	}

	// Get base model category
	baseModelCategory := "SMALL"
	if p.BaseModelMeta != nil {
		if category, ok := p.BaseModelMeta.Annotations[constants.ModelCategoryAnnotation]; ok {
			baseModelCategory = category
		}
	}

	// Create base labels
	baseLabels := map[string]string{
		constants.InferenceServicePodLabelKey: isvc.Name,
		constants.KServiceComponentLabel:      string(v1beta1.PredictorComponent),
		constants.FTServingLabelKey:           strconv.FormatBool(p.FineTunedServing),
	}

	// Add base model labels
	if p.BaseModelMeta != nil {
		baseLabels[constants.InferenceServiceBaseModelNameLabelKey] = p.BaseModelMeta.Name
		baseLabels[constants.InferenceServiceBaseModelSizeLabelKey] = baseModelCategory
		baseLabels[constants.BaseModelTypeLabelKey] = string(constants.ServingBaseModel)
	}

	// Add vendor label
	if p.BaseModel != nil {
		baseLabels[constants.BaseModelVendorLabelKey] = isvcutils.GetBaseModelVendor(*p.BaseModel)
	}

	// Add runtime label
	if p.RuntimeName != "" {
		baseLabels[constants.ServingRuntimeLabelKey] = p.RuntimeName
	}

	// Conditionally add fine-tuned serving related labels
	if p.FineTunedServing && len(p.FineTunedWeights) > 0 {
		ftStrategyParameter, err := isvcutils.GetValueFromRawExtension(p.FineTunedWeights[0].Spec.HyperParameters, constants.StrategyConfigKey)
		if err != nil {
			p.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", p.FineTunedWeights[0].Name, "namespace", isvc.Namespace)
		}

		fineTunedWeightFTStrategy := ""
		if ftStrategyParameter != nil {
			fineTunedWeightFTStrategy = ftStrategyParameter.(string)
		}

		baseLabels[constants.FTServingWithMergedWeightsLabelKey] = strconv.FormatBool(p.FineTunedServingWithMergedWeights)
		baseLabels[constants.FineTunedWeightFTStrategyLabelKey] = fineTunedWeightFTStrategy
	}

	// Merge labels in the correct order: runtime labels, isvc labels, predictor labels, base labels
	labels = utils.Union(labels, isvc.Labels, predictorLabels, baseLabels)

	return labels
}

// determinePredictorName determines the name of the predictor service
func (p *PredictorV2) determinePredictorName(isvc *v1beta1.InferenceService) (string, error) {
	defaultPredictorName := constants.DefaultPredictorServiceName(isvc.Name)

	if p.DeploymentMode == constants.RawDeployment {
		existing := &v1.Service{}
		if err := p.Client.Get(context.TODO(), types.NamespacedName{Name: defaultPredictorName, Namespace: isvc.Namespace}, existing); err == nil {
			return defaultPredictorName, nil
		}
	} else if p.DeploymentMode == constants.Serverless {
		existing := &knservingv1.Service{}
		if err := p.Client.Get(context.TODO(), types.NamespacedName{Name: defaultPredictorName, Namespace: isvc.Namespace}, existing); err == nil {
			return defaultPredictorName, nil
		}
	}

	// Try shorter name
	return constants.PredictorServiceName(isvc.Name), nil
}

// reconcilePodSpec creates the pod spec for the predictor component
func (p *PredictorV2) reconcilePodSpec(isvc *v1beta1.InferenceService, runtime v1beta1.ServingRuntimeSpec, objectMeta *metav1.ObjectMeta) (*v1.PodSpec, error) {
	// Find the ome-container index
	omeContainerIdx := isvcutils.GetContainerIndex(runtime.Containers, constants.MainContainerName)
	if omeContainerIdx == -1 {
		return nil, errors.New("failed to find ome-container in ServingRuntime containers")
	}

	// Create merged container
	container, err := isvcutils.MergeRuntimeContainers(&runtime.Containers[omeContainerIdx], &isvc.Spec.Predictor.Model.Container)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime container")
		return nil, errors.Wrap(err, "failed to get runtime container")
	}

	// Replace placeholders
	if err := isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to replace placeholders in serving runtime Container")
		return nil, errors.Wrap(err, "failed to replace placeholders in serving runtime Container")
	}

	// Update image tag
	isvcutils.UpdateImageTag(container, isvc.Spec.Predictor.Model.RuntimeVersion, isvc.Spec.Predictor.Model.Runtime)

	// Create merged pod spec
	podSpec, err := isvcutils.MergePodSpec(&runtime.ServingRuntimePodSpec, &isvc.Spec.Predictor.PodSpec)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime PodSpec")
		return nil, errors.Wrap(err, "failed to consolidate serving runtime PodSpecs")
	}

	// Update volume mounts
	p.updateVolumeMounts(isvc, container, objectMeta)

	// Update environment variables
	p.updateEnvVariables(isvc, container, objectMeta)

	// Update containers by inserting the custom container and keeping the other runtime containers
	podSpec.Containers = append([]v1.Container{*container}, runtime.Containers[:omeContainerIdx]...)
	podSpec.Containers = append(podSpec.Containers, runtime.Containers[omeContainerIdx+1:]...)

	// Update pod spec with volumes
	p.updatePodSpecVolumes(isvc, podSpec, objectMeta)

	p.Log.Info("PodSpec updated", "inference service", isvc.Name, "namespace", isvc.Namespace)
	return podSpec, nil
}

// updateVolumeMounts updates the volume mounts for the predictor
func (p *PredictorV2) updateVolumeMounts(isvc *v1beta1.InferenceService, container *v1.Container, objectMeta *metav1.ObjectMeta) {
	p.Log.Info("Update volume mounts", "inference service", isvc.Name, "namespace", isvc.Namespace)

	if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
		vm := v1.VolumeMount{
			Name:      *isvc.Spec.Predictor.Model.BaseModel,
			MountPath: *p.BaseModel.Storage.Path,
			ReadOnly:  true,
		}
		isvcutils.AppendVolumeMount(container, &vm)
	}

	if p.FineTunedServing {
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

// updateEnvVariables updates environment variables for the predictor
func (p *PredictorV2) updateEnvVariables(isvc *v1beta1.InferenceService, container *v1.Container, objectMeta *metav1.ObjectMeta) {
	if !p.FineTunedServing {
		if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
			p.Log.Info("Base model serving - adding MODEL_PATH env variable", "inference service", isvc.Name, "namespace", isvc.Namespace, "base model", isvc.Spec.Predictor.Model.Name)
			isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
				{Name: constants.ModelPathEnvVarKey, Value: *p.BaseModel.Storage.Path},
			})
		}
	} else {
		if p.BaseModel.Vendor == nil {
			p.Log.Info("Warning: no vendor given in base model spec - no env var added/updated")
		} else if *p.BaseModel.Vendor == string(constants.Meta) {
			isvcutils.UpdateEnvVars(container, &v1.EnvVar{
				Name: constants.ServedModelNameEnvVarKey, Value: filepath.Join(
					constants.LLamaVllmFTServingServedModelNamePrefix,
					objectMeta.Annotations[constants.FineTunedAdapterInjectionKey])},
			)
			isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
				{Name: constants.ModelPathEnvVarKey, Value: constants.ModelDefaultMountPath},
			})
		} else if *p.BaseModel.Vendor == string(constants.Cohere) {
			if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
				isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
					{Name: constants.TFewWeightPathEnvVarKey, Value: constants.CohereTFewFineTunedWeightDefaultPath},
				})
			}
		}
	}
}

// updatePodSpecVolumes updates the pod spec with necessary volumes
func (p *PredictorV2) updatePodSpecVolumes(isvc *v1beta1.InferenceService, podSpec *v1.PodSpec, objectMeta *metav1.ObjectMeta) {
	// Initialize volumes and add the main model volume
	volumes := []v1.Volume{
		{
			Name: *isvc.Spec.Predictor.Model.BaseModel,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: *p.BaseModel.Storage.Path,
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
}

// reconcileWorkerPodSpec reconciles the worker pod spec for multi-node deployments
func (p *PredictorV2) reconcileWorkerPodSpec(isvc *v1beta1.InferenceService, runtime v1beta1.ServingRuntimeSpec, objectMeta *metav1.ObjectMeta) (*v1.PodSpec, error) {
	// Early return if no worker specs are defined
	if runtime.WorkerPodSpec == nil && isvc.Spec.Predictor.Worker == nil {
		return nil, nil
	}

	// Find OME container indices in both runtime and isvc worker specs
	runtimeOmeIdx := -1
	isvcOmeIdx := -1

	if runtime.WorkerPodSpec != nil && runtime.WorkerPodSpec.Containers != nil {
		runtimeOmeIdx = isvcutils.GetContainerIndex(runtime.WorkerPodSpec.Containers, constants.MainContainerName)
	}

	if isvc.Spec.Predictor.Worker != nil && isvc.Spec.Predictor.Worker.Containers != nil {
		isvcOmeIdx = isvcutils.GetContainerIndex(isvc.Spec.Predictor.Worker.Containers, constants.MainContainerName)
	}

	// If no OME container found in either spec, return empty PodSpec
	if runtimeOmeIdx == -1 && isvcOmeIdx == -1 {
		return nil, nil
	}

	// Create merged container
	var workerContainer *v1.Container
	if runtimeOmeIdx != -1 {
		isvcOmeContainer := v1.Container{}
		if isvcOmeIdx != -1 {
			isvcOmeContainer = isvc.Spec.Predictor.Worker.Containers[isvcOmeIdx]
		}

		container, err := isvcutils.MergeRuntimeContainers(&runtime.WorkerPodSpec.Containers[runtimeOmeIdx], &isvcOmeContainer)
		if err != nil {
			p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime worker container")
			return nil, errors.Wrap(err, "failed to get runtime container")
		}

		if err := isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
			p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to replace placeholders in serving runtime worker Container")
			return nil, errors.Wrap(err, "failed to replace placeholders in serving runtime worker Container")
		}

		workerContainer = container
	}

	// Create merged pod spec using MergePodSpec
	var isvcWorkerPodSpec *v1beta1.PodSpec
	if isvc.Spec.Predictor.Worker != nil {
		isvcWorkerPodSpec = &isvc.Spec.Predictor.Worker.PodSpec
	}

	mergedPodSpec, err := isvcutils.MergePodSpec(&runtime.WorkerPodSpec.ServingRuntimePodSpec, isvcWorkerPodSpec)
	if err != nil {
		p.updateModelTransitionStatus(isvc, v1beta1.InvalidPredictorSpec, "Failed to get runtime WorkerPodSpec")
		return nil, errors.Wrap(err, "failed to consolidate serving runtime WorkerPodSpecs")
	}

	// Update worker pod spec with container and volumes
	if workerContainer != nil && mergedPodSpec != nil {
		p.updateVolumeMounts(isvc, workerContainer, objectMeta)
		p.updateEnvVariables(isvc, workerContainer, objectMeta)
		p.updateWorkerPodSpec(isvc, runtime, isvcOmeIdx, runtimeOmeIdx, workerContainer, mergedPodSpec)
	}

	p.Log.Info("WorkerPodSpec updated", "inference service", isvc.Name, "namespace", isvc.Namespace)
	return mergedPodSpec, nil
}

// updateWorkerPodSpec updates the worker pod spec for the predictor
func (p *PredictorV2) updateWorkerPodSpec(isvc *v1beta1.InferenceService, runtime v1beta1.ServingRuntimeSpec, isvcOmeIdx int, runtimeOmeIdx int, container *v1.Container, podSpec *v1.PodSpec) {
	// Update containers by inserting the custom container and keeping the other runtime containers
	podSpec.Containers = append([]v1.Container{*container}, runtime.WorkerPodSpec.Containers[:runtimeOmeIdx]...)
	podSpec.Containers = append(podSpec.Containers, runtime.WorkerPodSpec.Containers[runtimeOmeIdx+1:]...)
	if isvcOmeIdx != -1 {
		podSpec.Containers = append([]v1.Container{*container}, isvc.Spec.Predictor.Worker.Containers[:isvcOmeIdx]...)
		podSpec.Containers = append(podSpec.Containers, isvc.Spec.Predictor.Worker.Containers[isvcOmeIdx+1:]...)
	}

	// Initialize volumes and add the main PVC volume
	volumes := []v1.Volume{
		{
			Name: *isvc.Spec.Predictor.Model.BaseModel,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: *p.BaseModel.Storage.Path,
				},
			},
		},
	}

	// Append volumes to the podSpec
	podSpec.Volumes = append(podSpec.Volumes, volumes...)
}

// updateModelTransitionStatus updates the model transition status
func (p *PredictorV2) updateModelTransitionStatus(isvc *v1beta1.InferenceService, reason v1beta1.FailureReason, message string) {
	p.StatusManager.UpdateModelTransitionStatus(&isvc.Status, v1beta1.InvalidSpec, &v1beta1.FailureInfo{
		Reason:  reason,
		Message: message,
	})
}
