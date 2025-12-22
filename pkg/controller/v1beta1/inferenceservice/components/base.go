package components

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

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
	"github.com/sgl-project/ome/pkg/utils/storage"
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
	AcceleratorClass                  *v1beta1.AcceleratorClassSpec
	AcceleratorClassName              string
	FineTunedServing                  bool
	FineTunedServingWithMergedWeights bool
	FineTunedWeights                  []*v1beta1.FineTunedWeight
	StatusManager                     *status.StatusReconciler
	Log                               logr.Logger
	SupportedModelFormat              *v1beta1.SupportedModelFormat
}

// Common methods as functions that operate on BaseComponentFields

// PVCVolumeInfo contains parsed PVC volume information for mounting
// PVCName is the name of the PVC to mount
// Namespace is the namespace where the PVC exists (required for cross-namespace validation)
// SubPath is the path within the PVC to mount (from the URI)
// MountPath is where to mount the volume in the container
type PVCVolumeInfo struct {
	PVCName   string
	Namespace string
	SubPath   string
	MountPath string
}

// GetPVCVolumeInfo extracts PVC volume information from a storage spec
// Returns nil if the storage is not PVC-based or if parsing fails
func GetPVCVolumeInfo(storageSpec *v1beta1.StorageSpec, defaultNamespace string) *PVCVolumeInfo {
	if storageSpec == nil || storageSpec.StorageUri == nil {
		return nil
	}

	uri := *storageSpec.StorageUri
	if !strings.HasPrefix(uri, storage.PVCStoragePrefix) {
		return nil
	}

	pvcComponents, err := storage.ParsePVCStorageURI(uri)
	if err != nil {
		// Log the error for debugging - invalid URI format
		// Callers should validate URIs before reaching this point
		return nil
	}

	// Use the namespace from URI if specified, otherwise use the default namespace
	namespace := pvcComponents.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	// Determine mount path: use explicit Path if provided, otherwise use default
	mountPath := constants.DefaultModelLocalMountPath
	if storageSpec.Path != nil && *storageSpec.Path != "" {
		mountPath = *storageSpec.Path
	}

	return &PVCVolumeInfo{
		PVCName:   pvcComponents.PVCName,
		Namespace: namespace,
		SubPath:   pvcComponents.SubPath,
		MountPath: mountPath,
	}
}

// ValidatePVCNamespace validates that the PVC namespace matches the expected namespace
// Returns an error if there is a namespace mismatch
// Note: Kubernetes PVCs can only be mounted by pods in the same namespace,
// so the InferenceService namespace must match the PVC namespace
func ValidatePVCNamespace(pvcInfo *PVCVolumeInfo, podNamespace string) error {
	if pvcInfo == nil {
		return nil
	}
	if pvcInfo.Namespace != "" && pvcInfo.Namespace != podNamespace {
		return fmt.Errorf("PVC namespace mismatch: PVC is in namespace %q but InferenceService is in namespace %q; Kubernetes requires PVCs to be in the same namespace as the pod",
			pvcInfo.Namespace, podNamespace)
	}
	return nil
}

// sanitizeVolumeName ensures the volume name is valid for Kubernetes (max 63 chars, DNS label format)
func sanitizeVolumeName(name string) string {
	const maxLen = 63
	if len(name) <= maxLen {
		return name
	}
	// Truncate and add a suffix to indicate truncation
	// Use first 55 chars + "-" + last 7 chars to maintain uniqueness
	return name[:55] + "-" + name[len(name)-7:]
}

// GetModelMountPath returns the mount path for the model storage
// It checks both explicit Path and PVC storage URI
func GetModelMountPath(storageSpec *v1beta1.StorageSpec, defaultNamespace string) string {
	if storageSpec == nil {
		return ""
	}

	// If explicit path is provided, use it
	if storageSpec.Path != nil && *storageSpec.Path != "" {
		return *storageSpec.Path
	}

	// Check if this is a PVC storage URI
	pvcInfo := GetPVCVolumeInfo(storageSpec, defaultNamespace)
	if pvcInfo != nil {
		return pvcInfo.MountPath
	}

	return ""
}

// IsPVCStorage checks if the storage spec uses PVC storage
func IsPVCStorage(storageSpec *v1beta1.StorageSpec) bool {
	if storageSpec == nil || storageSpec.StorageUri == nil {
		return false
	}
	return strings.HasPrefix(*storageSpec.StorageUri, storage.PVCStoragePrefix)
}

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
	if b.BaseModel != nil && b.BaseModel.Storage != nil && b.BaseModelMeta != nil {
		if isvcutils.IsOriginalModelVolumeMountNecessary(objectMeta.Annotations) {
			defaultNamespace := isvc.Namespace
			// Use sanitized volume name to match the volume created in UpdatePodSpecVolumes
			volumeName := sanitizeVolumeName(b.BaseModelMeta.Name)

			// Check if this is PVC storage
			if IsPVCStorage(b.BaseModel.Storage) {
				pvcInfo := GetPVCVolumeInfo(b.BaseModel.Storage, defaultNamespace)
				if pvcInfo != nil {
					// Skip if PVC namespace doesn't match (volume wasn't created)
					if err := ValidatePVCNamespace(pvcInfo, isvc.Namespace); err != nil {
						return
					}

					vm := corev1.VolumeMount{
						Name:      volumeName,
						MountPath: pvcInfo.MountPath,
						SubPath:   pvcInfo.SubPath,
						ReadOnly:  true,
					}
					isvcutils.AppendVolumeMount(container, &vm)
					b.Log.Info("Added PVC volume mount for model",
						"mountPath", pvcInfo.MountPath,
						"subPath", pvcInfo.SubPath,
						"volumeName", volumeName,
						"modelName", b.BaseModelMeta.Name)
				}
			} else if b.BaseModel.Storage.Path != nil {
				// Use explicit path for non-PVC storage
				vm := corev1.VolumeMount{
					Name:      volumeName,
					MountPath: *b.BaseModel.Storage.Path,
					ReadOnly:  true,
				}
				isvcutils.AppendVolumeMount(container, &vm)
			}
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
			if b.BaseModel != nil && b.BaseModel.Storage != nil {
				defaultNamespace := isvc.Namespace
				modelPath := GetModelMountPath(b.BaseModel.Storage, defaultNamespace)
				if modelPath != "" {
					b.Log.Info("Base model serving - adding MODEL_PATH env variable if not provided",
						"inference service", isvc.Name,
						"namespace", isvc.Namespace,
						"modelPath", modelPath)
					isvcutils.AppendEnvVarsIfNotExist(container, &[]corev1.EnvVar{
						{Name: constants.ModelPathEnvVarKey, Value: modelPath},
					})
				}
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
				isvcutils.AppendEnvVarsIfNotExist(container, &[]corev1.EnvVar{
					{Name: constants.ModelPathEnvVarKey, Value: constants.ModelDefaultMountPath},
				})
			} else if *b.BaseModel.Vendor == string(constants.Cohere) {
				// Cohere vendor specific env vars
				if isvcutils.IsCohereCommand1TFewFTServing(objectMeta) {
					isvcutils.AppendEnvVarsIfNotExist(container, &[]corev1.EnvVar{
						{Name: constants.TFewWeightPathEnvVarKey, Value: constants.CohereTFewFineTunedWeightDefaultPath},
					})
				}
			}
		} else {
			b.Log.Info("Warning: no vendor given in base model spec - no env var added/updated")
		}
	}

	// append env var from runtime spec if it is specified.
	// runner container is user values, it takes precedence over runtime values.
	// if the env exists, update its value.
	// if the env does not exist, append it to the list.
	if b.SupportedModelFormat != nil && b.SupportedModelFormat.AcceleratorConfig != nil && b.AcceleratorClassName != "" {
		acceleratorConfig := b.SupportedModelFormat.GetAcceleratorConfig(b.AcceleratorClassName)
		if acceleratorConfig != nil {
			envOverride := acceleratorConfig.EnvironmentOverride
			for envName, envVar := range envOverride {
				isvcutils.UpdateEnvVars(container, &corev1.EnvVar{
					Name: envName, Value: envVar})
			}
		}
	}
}

// UpdatePodSpecNodeSelector updates pod spec with node selector for model scheduling
func UpdatePodSpecNodeSelector(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec, componentType v1beta1.ComponentType) {
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

	// Add preferred node affinity for model readiness using the shared utility function
	isvcutils.AddPreferredNodeAffinityForModel(podSpec, b.BaseModelMeta)

	// Add node selector merged from AcceleratorClass if applicable
	// Only add mergedNodeSelector to engine and decoder component.
	mergedNodeSelector := isvcutils.MergeNodeSelector(b.Runtime, b.AcceleratorClass, isvc, componentType)
	if len(mergedNodeSelector) > 0 {
		if podSpec.NodeSelector == nil {
			podSpec.NodeSelector = make(map[string]string)
		}
		for k, v := range mergedNodeSelector {
			podSpec.NodeSelector[k] = v
		}
	}

	b.Log.Info("Added preferred node affinity for model scheduling",
		"modelName", b.BaseModelMeta.Name,
		"namespace", b.BaseModelMeta.Namespace,
		"inferenceService", isvc.Name)
}

// UpdatePodSpecVolumes updates pod spec with common volumes
func UpdatePodSpecVolumes(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec, objectMeta *metav1.ObjectMeta) {
	// Add model volume if base model is specified
	if b.BaseModel != nil && b.BaseModel.Storage != nil && b.BaseModelMeta != nil {
		// Determine the namespace for PVC (use isvc namespace for namespaced resources)
		defaultNamespace := isvc.Namespace

		// Sanitize volume name to ensure it's valid for Kubernetes (max 63 chars)
		volumeName := sanitizeVolumeName(b.BaseModelMeta.Name)

		// Check if this is PVC storage
		if IsPVCStorage(b.BaseModel.Storage) {
			pvcInfo := GetPVCVolumeInfo(b.BaseModel.Storage, defaultNamespace)
			if pvcInfo != nil {
				// Validate that PVC namespace matches the pod namespace
				// Kubernetes requires PVCs to be in the same namespace as the pod
				if err := ValidatePVCNamespace(pvcInfo, isvc.Namespace); err != nil {
					b.Log.Error(err, "PVC namespace validation failed",
						"pvcName", pvcInfo.PVCName,
						"pvcNamespace", pvcInfo.Namespace,
						"isvcNamespace", isvc.Namespace,
						"modelName", b.BaseModelMeta.Name)
					// Skip adding this volume - the pod would fail to start anyway
					// A status condition should be added by the caller
					return
				}

				modelVolume := corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcInfo.PVCName,
							ReadOnly:  true,
						},
					},
				}
				podSpec.Volumes = append(podSpec.Volumes, modelVolume)
				b.Log.Info("Added PVC volume for model",
					"pvcName", pvcInfo.PVCName,
					"pvcNamespace", pvcInfo.Namespace,
					"subPath", pvcInfo.SubPath,
					"mountPath", pvcInfo.MountPath,
					"volumeName", volumeName,
					"modelName", b.BaseModelMeta.Name)
			}
		} else if b.BaseModel.Storage.Path != nil {
			// Use HostPath for non-PVC storage with explicit path
			modelVolume := corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: *b.BaseModel.Storage.Path,
					},
				},
			}
			podSpec.Volumes = append(podSpec.Volumes, modelVolume)
		}
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

// MergeRuntimeArgumentsOverride merges runtime argument overrides according AcceleratorClass into the container args
func MergeRuntimeArgumentsOverride(b *BaseComponentFields, container *corev1.Container) {
	// append arg var from runtime spec if it is specified
	if b.SupportedModelFormat != nil && b.SupportedModelFormat.AcceleratorConfig != nil && b.AcceleratorClassName != "" {
		acceleratorModelConfig := b.SupportedModelFormat.GetAcceleratorConfig(b.AcceleratorClassName)
		argsOverride := acceleratorModelConfig.RuntimeArgsOverride
		container.Args = isvcutils.MergeMultilineArgs(container.Args, argsOverride)

		// if runtime argument override has TensorParallelism, update the args accordingly
		if acceleratorModelConfig.TensorParallelismOverride != nil {
			tensorParallelismConfig := acceleratorModelConfig.TensorParallelismOverride

			// Override tensor parallel size if specified
			if tensorParallelismConfig.TensorParallelSize != nil && *tensorParallelismConfig.TensorParallelSize > 0 {
				var updated bool
				// Check --tp-size first, it is the parameter used in sglang
				container.Args, updated = isvcutils.OverrideIntParam(container.Args, "--tp-size", *tensorParallelismConfig.TensorParallelSize)
				if !updated {
					// Check --tp next, it is the alias of --tp-size in sglang
					container.Args, updated = isvcutils.OverrideIntParam(container.Args, "--tp", *tensorParallelismConfig.TensorParallelSize)
				}
				if !updated {
					// If --tp-size doesn't exist, check --tensor-parallel-size, it is the parameter used in vllm
					container.Args, _ = isvcutils.OverrideIntParam(container.Args, "--tensor-parallel-size", *tensorParallelismConfig.TensorParallelSize)
				}
			}

			// Override pipeline parallel size if specified
			if tensorParallelismConfig.PipelineParallelSize != nil && *tensorParallelismConfig.PipelineParallelSize > 0 {
				var updated bool
				// Check --pp-size first, it is the parameter used in sglang
				container.Args, updated = isvcutils.OverrideIntParam(container.Args, "--pp-size", *tensorParallelismConfig.PipelineParallelSize)
				if !updated {
					// Check --pp next, it is the alias of --pp-size in sglang
					container.Args, updated = isvcutils.OverrideIntParam(container.Args, "--pp", *tensorParallelismConfig.PipelineParallelSize)
				}
				if !updated {
					// If --pp-size doesn't exist, check --pipeline-parallel-size
					container.Args, _ = isvcutils.OverrideIntParam(container.Args, "--pipeline-parallel-size", *tensorParallelismConfig.PipelineParallelSize)
				}
			}
		}
	}

}

// isResourcesUnspecified checks if the resource requirements are unspecified
func isResourcesUnspecified(resources corev1.ResourceRequirements) bool {
	return resources.Limits == nil && resources.Requests == nil && len(resources.Claims) == 0
}

// MergeResources merges resource requests and limits from the runtime and accelerator class into the container
func MergeResources(b *BaseComponentFields, container *corev1.Container) {
	isvcutils.MergeResource(container, b.AcceleratorClass, b.Runtime)
}

// MergeEngineResources merges resource requests and limits for the engine container.
// It only merges resources from the runtime and accelerator class when the user has not
// explicitly specified resources in the InferenceService spec. This ensures user-specified
// resources are respected and not overridden, while providing sensible defaults from the
// runtime and accelerator class when resources are not specified.
func MergeEngineResources(b *BaseComponentFields, isvc *v1beta1.InferenceService, container *corev1.Container) {
	if isvc.Spec.Engine != nil &&
		(isvc.Spec.Engine.Runner == nil ||
			isResourcesUnspecified(isvc.Spec.Engine.Runner.Container.Resources)) {
		b.Log.Info("Merging resources for engine container as user did not specify resources in InferenceService")
		MergeResources(b, container)
	}
}

// MergeDecoderResources merges resource requests and limits for the decoder container.
// It only merges resources from the runtime and accelerator class when the user has not
// explicitly specified resources in the InferenceService spec. This ensures user-specified
// resources are respected and not overridden, while providing sensible defaults from the
// runtime and accelerator class when resources are not specified.
func MergeDecoderResources(b *BaseComponentFields, isvc *v1beta1.InferenceService, container *corev1.Container) {
	if isvc.Spec.Decoder != nil &&
		(isvc.Spec.Decoder.Runner == nil ||
			isResourcesUnspecified(isvc.Spec.Decoder.Runner.Container.Resources)) {
		b.Log.Info("Merging resources for decoder container as user did not specify resources in InferenceService")
		MergeResources(b, container)
	}
}

// UpdateEngineAffinity merges affinity from the accelerator class into the pod spec
// It only merges when customer didn't specify affinity in the inference service
func UpdateEngineAffinity(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec) {
	if isvc.Spec.Engine != nil &&
		isvc.Spec.Engine.PodSpec.Affinity == nil {
		if b.AcceleratorClass != nil && b.AcceleratorClass.Discovery.Affinity != nil {
			b.Log.Info("Merging affinity from accelerator class into engine pod spec as user did not specify affinity in InferenceService")
			podSpec.Affinity = b.AcceleratorClass.Discovery.Affinity
		}
	}
}

// UpdateDecoderAffinity merges affinity from the accelerator class into the pod spec
// It only merges when customer didn't specify affinity in the inference service
func UpdateDecoderAffinity(b *BaseComponentFields, isvc *v1beta1.InferenceService, podSpec *corev1.PodSpec) {
	if isvc.Spec.Decoder != nil &&
		isvc.Spec.Decoder.PodSpec.Affinity == nil {
		if b.AcceleratorClass != nil && b.AcceleratorClass.Discovery.Affinity != nil {
			b.Log.Info("Merging affinity from accelerator class into decoder pod spec as user did not specify affinity in InferenceService")
			podSpec.Affinity = b.AcceleratorClass.Discovery.Affinity
		}
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
func ProcessBaseLabels(b *BaseComponentFields, isvc *v1beta1.InferenceService, componentType v1beta1.ComponentType, labels map[string]string) (map[string]string, error) {
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
			return nil, err
		}

		fineTunedWeightFTStrategy := ""
		if ftStrategyParameter != nil {
			fineTunedWeightFTStrategy = ftStrategyParameter.(string)
		}
		labels[constants.FineTunedWeightFTStrategyLabelKey] = fineTunedWeightFTStrategy

		labels[constants.FTServingWithMergedWeightsLabelKey] = strconv.FormatBool(b.FineTunedServingWithMergedWeights)
	}

	return labels, nil
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
