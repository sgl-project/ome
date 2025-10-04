package components

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/status"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils"
	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// BaseComponentFields contains common fields for all components
type BaseComponentFields struct {
	Client                            client.Client
	Clientset                         kubernetes.Interface
	Scheme                            *runtime.Scheme
	InferenceServiceConfig            *controllerconfig.InferenceServicesConfig
	DeploymentMode                    constants.DeploymentModeType
	BaseModel                         *v1beta1.BaseModelSpec
	BaseModelMeta                     *metav1.ObjectMeta
	Runtime                           *v1beta1.ServingRuntimeSpec
	RuntimeName                       string
	FineTunedServing                  bool
	FineTunedServingWithMergedWeights bool
	FineTunedWeights                  []*v1beta1.FineTunedWeight
	StatusManager                     *status.StatusReconciler
	Log                               logr.Logger
}

// Common methods as functions that operate on BaseComponentFields

// ReconcileFineTunedWeights reconciles fine-tuned weights for any component
func ReconcileFineTunedWeights(b *BaseComponentFields, isvc *v1beta1.InferenceService) error {
	numOfFineTunedWeights := len(isvc.Spec.Model.FineTunedWeights)
	if numOfFineTunedWeights == 0 {
		return nil
	}

	b.Log.Info("FT serving mode", "Number of fine-tuned weights", numOfFineTunedWeights)
	b.FineTunedServing = true

	// TODO: lift here when start supporting stacked FT serving
	if numOfFineTunedWeights > 1 {
		return fmt.Errorf("stacked fine-tuned serving is not supported yet")
	}

	allFineTunedWeights := make([]*v1beta1.FineTunedWeight, 0)

	for _, fineTunedWeightName := range isvc.Spec.Model.FineTunedWeights {
		fineTunedWeight, err := isvcutils.GetFineTunedWeight(b.Client, fineTunedWeightName)
		if err != nil {
			return err
		}
		allFineTunedWeights = append(allFineTunedWeights, fineTunedWeight)
	}

	// Determine if loading merged fine-tuned weights
	loadingMergedFineTunedWeights, err := isvcutils.LoadingMergedFineTunedWeight(allFineTunedWeights)
	if err != nil {
		b.Log.Error(err, "Failed to determine if loading merged fine-tuned weights")
		return err
	}
	b.FineTunedServingWithMergedWeights = loadingMergedFineTunedWeights
	b.FineTunedWeights = allFineTunedWeights

	return nil
}

// UpdateVolumeMounts updates volume mounts for the container
func UpdateVolumeMounts(b *BaseComponentFields, isvc *v1beta1.InferenceService, container *corev1.Container, objectMeta *metav1.ObjectMeta) {
	if container == nil {
		b.Log.Error(errors.New("container is nil"), "UpdateVolumeMounts: container is nil")
		return
	}

	// Add model volume mount if base model is specified and it's necessary
	if b.BaseModel != nil && b.BaseModel.Storage != nil && b.BaseModel.Storage.Path != nil && b.BaseModelMeta != nil {
		if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
			vm := corev1.VolumeMount{
				Name:      b.BaseModelMeta.Name,
				MountPath: *b.BaseModel.Storage.Path,
				ReadOnly:  true,
			}
			isvcutils.AppendVolumeMount(container, &vm)
		}
	}

	// Add fine-tuned serving volume mounts
	if b.FineTunedServing {
		defaultModelVolumeMount := corev1.VolumeMount{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: constants.ModelDefaultMountPath,
		}
		isvcutils.AppendVolumeMountIfNotExist(container, &defaultModelVolumeMount)

		if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
			// Update to have `base` sub-path in model volume mount for cohere tfew stacked serving case
			defaultModelVolumeMountWithSubPath := corev1.VolumeMount{
				Name:      constants.ModelEmptyDirVolumeName,
				MountPath: filepath.Join(constants.ModelDefaultMountPath, objectMeta.Annotations[constants.BaseModelFormat]),
				SubPath:   constants.BaseModelVolumeMountSubPath,
			}
			isvcutils.UpdateVolumeMount(container, &defaultModelVolumeMountWithSubPath)

			tfewFineTunedWeightVolumeMount := corev1.VolumeMount{
				Name:      constants.ModelEmptyDirVolumeName,
				MountPath: filepath.Join(constants.CohereTFewFineTunedWeightVolumeMountPath, objectMeta.Annotations[constants.BaseModelFormat]),
				ReadOnly:  true,
				SubPath:   constants.FineTunedWeightVolumeMountSubPath,
			}
			isvcutils.AppendVolumeMount(container, &tfewFineTunedWeightVolumeMount)
		}
	}

	// Add blocklist volume mounts if enabled
	if isvcutils.IsBlockListInjectionDisabled(objectMeta.Annotations) {
		inputBlocklistVolumeMount := corev1.VolumeMount{
			Name:      constants.BlocklistConfigMapVolumeName,
			MountPath: constants.InputBlocklistMountPath,
			ReadOnly:  true,
			SubPath:   constants.InputBlocklistSubPath,
		}
		isvcutils.AppendVolumeMount(container, &inputBlocklistVolumeMount)

		outputBlocklistVolumeMount := corev1.VolumeMount{
			Name:      constants.BlocklistConfigMapVolumeName,
			MountPath: constants.OutputBlocklistMountPath,
			ReadOnly:  true,
			SubPath:   constants.OutputBlocklistSubPath,
		}
		isvcutils.AppendVolumeMount(container, &outputBlocklistVolumeMount)
	}
}

// UpdateEnvVariables updates environment variables for the container
func UpdateEnvVariables(b *BaseComponentFields, isvc *v1beta1.InferenceService, container *corev1.Container, objectMeta *metav1.ObjectMeta) {
	if container == nil {
		b.Log.Error(errors.New("container is nil"), "UpdateEnvVariables: container is nil")
		return
	}

	if !b.FineTunedServing {
		// Base model serving - add MODEL_PATH env variable if necessary
		if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
			if b.BaseModel != nil && b.BaseModel.Storage != nil && b.BaseModel.Storage.Path != nil {
				b.Log.Info("Base model serving - adding MODEL_PATH env variable", "inference service", isvc.Name, "namespace", isvc.Namespace)
				isvcutils.AppendEnvVars(container, &[]corev1.EnvVar{
					{Name: constants.ModelPathEnvVarKey, Value: *b.BaseModel.Storage.Path},
				})
			}
		}
	} else {
		// Fine-tuned serving - add vendor-specific environment variables
		if b.BaseModel != nil && b.BaseModel.Vendor != nil {
			if *b.BaseModel.Vendor == string(constants.Meta) {
				// Llama/Meta vendor specific env vars
				isvcutils.UpdateEnvVars(container, &corev1.EnvVar{
					Name: constants.ServedModelNameEnvVarKey,
					Value: filepath.Join(
						constants.LLamaVllmFTServingServedModelNamePrefix,
						objectMeta.Annotations[constants.FineTunedAdapterInjectionKey],
					),
				})
				isvcutils.AppendEnvVars(container, &[]corev1.EnvVar{
					{Name: constants.ModelPathEnvVarKey, Value: constants.ModelDefaultMountPath},
				})
			} else if *b.BaseModel.Vendor == string(constants.Cohere) {
				// Cohere vendor specific env vars
				if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
					isvcutils.AppendEnvVars(container, &[]corev1.EnvVar{
						{Name: constants.TFewWeightPathEnvVarKey, Value: constants.CohereTFewFineTunedWeightDefaultPath},
					})
				}
			}
		} else {
			b.Log.Info("Warning: no vendor given in base model spec - no env var added/updated")
		}
	}
}

// UpdatePodSpecNodeSelector updates pod spec with node selector for model scheduling
func UpdatePodSpecNodeSelector(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec) {
	// Only add node selector if we have a base model
	if b.BaseModel == nil || b.BaseModelMeta == nil {
		return
	}

	// Skip node selector for fine-tuned serving with merged weights
	// as they don't need the base model on the node
	if b.FineTunedServingWithMergedWeights {
		b.Log.Info("Skipping node selector for fine-tuned serving with merged weights",
			"inferenceService", isvc.Name, "namespace", isvc.Namespace)
		return
	}

	// Skip node selector for PVC storage (model accessible from any node)
	if b.BaseModel.Storage != nil && b.BaseModel.Storage.StorageUri != nil {
		storageType, err := utilstorage.GetStorageType(*b.BaseModel.Storage.StorageUri)
		if err == nil && storageType == utilstorage.StorageTypePVC {
			b.Log.Info("Skipping node selector for PVC storage",
				"inferenceService", isvc.Name, "namespace", isvc.Namespace,
				"storageUri", *b.BaseModel.Storage.StorageUri)
			return
		}
	}

	// Determine if this is a ClusterBaseModel or BaseModel
	var labelKey string
	isClusterScoped := b.BaseModelMeta.Namespace == ""

	if isClusterScoped {
		// ClusterBaseModel
		labelKey = constants.GetClusterBaseModelLabel(b.BaseModelMeta.Name)
	} else {
		// BaseModel (namespace-scoped)
		labelKey = constants.GetBaseModelLabel(b.BaseModelMeta.Namespace, b.BaseModelMeta.Name)
	}

	// Initialize node selector if nil
	if podSpec.NodeSelector == nil {
		podSpec.NodeSelector = make(map[string]string)
	}

	// Add node selector for model with "Ready" status
	podSpec.NodeSelector[labelKey] = "Ready"

	b.Log.Info("Added node selector for model scheduling",
		"labelKey", labelKey,
		"modelName", b.BaseModelMeta.Name,
		"namespace", b.BaseModelMeta.Namespace,
		"inferenceService", isvc.Name)
}

// UpdatePodSpecVolumes updates pod spec with common volumes
func UpdatePodSpecVolumes(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec, objectMeta *metav1.ObjectMeta) {
	// Add model volume if base model is specified
	if b.BaseModel != nil && b.BaseModel.Storage != nil && b.BaseModelMeta != nil {
		var volumeSource corev1.VolumeSource

		// Check if this is PVC storage
		if b.BaseModel.Storage.StorageUri != nil {
			storageType, err := utilstorage.GetStorageType(*b.BaseModel.Storage.StorageUri)
			if err == nil && storageType == utilstorage.StorageTypePVC {
				// Parse PVC URI to get PVC name
				pvcComponents, err := utilstorage.ParsePVCStorageURI(*b.BaseModel.Storage.StorageUri)
				if err != nil {
					b.Log.Error(err, "Failed to parse PVC storage URI", "storageUri", *b.BaseModel.Storage.StorageUri)
					return
				}

				// Create PVC volume source
				volumeSource = corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcComponents.PVCName,
						ReadOnly:  true,
					},
				}

				b.Log.Info("Created PVC volume for model",
					"pvcName", pvcComponents.PVCName,
					"modelName", b.BaseModelMeta.Name)
			} else if b.BaseModel.Storage.Path != nil {
				// Use HostPath for non-PVC storage types
				volumeSource = corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: *b.BaseModel.Storage.Path,
					},
				}
			} else {
				// No valid storage configuration
				return
			}
		} else if b.BaseModel.Storage.Path != nil {
			// Legacy case: only Path specified (no StorageUri)
			volumeSource = corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: *b.BaseModel.Storage.Path,
				},
			}
		} else {
			// No valid storage configuration
			return
		}

		modelVolume := corev1.Volume{
			Name:         b.BaseModelMeta.Name,
			VolumeSource: volumeSource,
		}
		podSpec.Volumes = append(podSpec.Volumes, modelVolume)
	}

	// Add empty model directory volume if required for fine-tuned serving
	if isvcutils.IsEmptyModelDirVolumeRequired(objectMeta.Annotations) {
		emptyModelDirVolume := corev1.Volume{
			Name: constants.ModelEmptyDirVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
		podSpec.Volumes = utils.AppendVolumeIfNotExists(podSpec.Volumes, emptyModelDirVolume)
	}

	// Add blocklist configmap volume if enabled
	if isvcutils.IsBlockListInjectionDisabled(objectMeta.Annotations) {
		blockListConfigMapVolume := corev1.Volume{
			Name: constants.BlocklistConfigMapVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: constants.ModelConfigName(isvc.Name),
					},
				},
			},
		}
		podSpec.Volumes = append(podSpec.Volumes, blockListConfigMapVolume)
	}
}

// ProcessBaseAnnotations processes common annotations
func ProcessBaseAnnotations(b *BaseComponentFields, isvc *v1beta1.InferenceService, annotations map[string]string) (map[string]string, error) {
	// Add fine-tuned weight annotations if applicable
	if b.FineTunedServing && len(b.FineTunedWeights) > 0 {
		// Inject ft adapter for single/non-stacked fine-tuned weight downloading
		annotations[constants.FineTunedAdapterInjectionKey] = b.FineTunedWeights[0].Name

		// Add fine-tuned weight ft strategy
		fineTunedWeightFTStrategy, err := isvcutils.GetValueFromRawExtension(b.FineTunedWeights[0].Spec.HyperParameters, constants.StrategyConfigKey)
		if err != nil || fineTunedWeightFTStrategy == nil {
			b.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", b.FineTunedWeights[0].Name, "namespace", isvc.Namespace)
			return nil, err
		}
		annotations[constants.FineTunedWeightFTStrategyKey] = fineTunedWeightFTStrategy.(string)
	}

	if b.FineTunedServingWithMergedWeights {
		// For FT serving using merged FT weights, no need base model
		b.Log.Info("Fine-tuned serving with merged weights", "namespace", isvc.Namespace)
		annotations[constants.FTServingWithMergedWeightsAnnotationKey] = "true"
	} else if b.BaseModelMeta != nil {
		// Add model init required annotations (for non-merged FT or regular serving)
		baseModelDecryptionKeyName, ok := b.BaseModelMeta.Annotations[constants.BaseModelDecryptionKeyName]
		if ok {
			annotations[constants.BaseModelDecryptionKeyName] = baseModelDecryptionKeyName
		}
		baseModelDecryptionSecretName, ok := b.BaseModelMeta.Annotations[constants.BaseModelDecryptionSecretName]
		if ok {
			annotations[constants.BaseModelDecryptionSecretName] = baseModelDecryptionSecretName
		}
	}

	// Add base model specific annotations
	if b.BaseModel != nil && b.BaseModelMeta != nil {
		annotations[constants.BaseModelName] = b.BaseModelMeta.Name
		if b.BaseModel.Vendor != nil {
			annotations[constants.BaseModelVendorAnnotationKey] = *b.BaseModel.Vendor
		}
		annotations[constants.BaseModelFormat] = b.BaseModel.ModelFormat.Name
		if b.BaseModel.ModelFormat.Version != nil {
			annotations[constants.BaseModelFormatVersion] = *b.BaseModel.ModelFormat.Version
		}
	}

	if b.RuntimeName != "" {
		annotations[constants.ServingRuntimeKeyName] = b.RuntimeName
	}

	return annotations, nil
}

// ProcessBaseLabels processes common labels
func ProcessBaseLabels(b *BaseComponentFields, isvc *v1beta1.InferenceService, componentType v1beta1.ComponentType, labels map[string]string) map[string]string {
	baseModelCategory := "SMALL"
	if b.BaseModelMeta != nil {
		if category, ok := b.BaseModelMeta.Annotations[constants.ModelCategoryAnnotation]; ok {
			baseModelCategory = category
		}
	}

	baseLabels := map[string]string{
		constants.InferenceServicePodLabelKey: isvc.Name,
		constants.OMEComponentLabel:           string(componentType),
		constants.ServingRuntimeLabelKey:      b.RuntimeName,
		constants.FTServingLabelKey:           strconv.FormatBool(b.FineTunedServing),
	}

	// Merge with provided labels
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range baseLabels {
		labels[k] = v
	}

	if b.BaseModelMeta != nil {
		labels[constants.InferenceServiceBaseModelNameLabelKey] = b.BaseModelMeta.Name
		labels[constants.InferenceServiceBaseModelSizeLabelKey] = baseModelCategory
		labels[constants.BaseModelTypeLabelKey] = string(constants.ServingBaseModel)
	}

	if b.BaseModel != nil && b.BaseModel.Vendor != nil {
		labels[constants.BaseModelVendorLabelKey] = *b.BaseModel.Vendor
	}

	// Add fine-tuned serving related labels
	if b.FineTunedServing && len(b.FineTunedWeights) > 0 {
		ftStrategyParameter, err := isvcutils.GetValueFromRawExtension(b.FineTunedWeights[0].Spec.HyperParameters, constants.StrategyConfigKey)
		if err != nil {
			b.Log.Error(err, "Error getting hyper-parameter strategy from FineTunedWeight", "FineTunedWeight", b.FineTunedWeights[0].Name, "namespace", isvc.Namespace)
		} else {
			fineTunedWeightFTStrategy := ""
			if ftStrategyParameter != nil {
				fineTunedWeightFTStrategy = ftStrategyParameter.(string)
			}
			labels[constants.FineTunedWeightFTStrategyLabelKey] = fineTunedWeightFTStrategy
		}
		labels[constants.FTServingWithMergedWeightsLabelKey] = strconv.FormatBool(b.FineTunedServingWithMergedWeights)
	}

	return labels
}

// UpdateComponentStatus updates component status based on deployment mode
// This method provides a systematic way to handle status updates across all components
func UpdateComponentStatus(b *BaseComponentFields, isvc *v1beta1.InferenceService, componentType v1beta1.ComponentType, objectMeta metav1.ObjectMeta, getPodLabelInfo func(bool, metav1.ObjectMeta, v1beta1.ComponentStatusSpec) (string, string)) error {
	// Always initialize the component ready condition to ensure it's visible from the start
	// The deployment reconciler will update the condition based on the actual deployment status:
	// - MultiNode: Updates when LWS becomes available
	// - RawDeployment: Updates when Deployment becomes available
	// - Serverless: Updates when Knative Service becomes ready
	b.StatusManager.InitializeComponentCondition(&isvc.Status, componentType)

	// Update model status for all deployment modes based on actual pod information
	rawDeployment := b.DeploymentMode == constants.RawDeployment
	statusSpec := isvc.Status.Components[componentType]
	podLabelKey, podLabelValue := getPodLabelInfo(rawDeployment, objectMeta, statusSpec)

	pods, err := isvcutils.ListPodsByLabel(b.Client, isvc.ObjectMeta.Namespace, podLabelKey, podLabelValue)
	if err != nil {
		return errors.Wrapf(err, "failed to list %s pods by label", componentType)
	}
	b.StatusManager.PropagateModelStatus(&isvc.Status, statusSpec, pods, rawDeployment)

	return nil
}
