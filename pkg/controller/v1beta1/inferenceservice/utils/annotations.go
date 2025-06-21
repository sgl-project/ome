package utils

import (
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
GetDeploymentMode returns the current deployment mode based on annotations and config.
If a valid deployment mode is specified in annotations, it is used.
Otherwise, returns the default deployment mode from config.
*/
func GetDeploymentMode(annotations map[string]string, deployConfig *controllerconfig.DeployConfig) constants.DeploymentModeType {
	if mode, exists := annotations[constants.DeploymentMode]; exists {
		deploymentMode := constants.DeploymentModeType(mode)
		if deploymentMode.IsValid() {
			return deploymentMode
		}
	}
	return constants.DeploymentModeType(deployConfig.DefaultDeploymentMode)
}

func IsBlockListInjectionDisabled(annotations map[string]string) bool {
	inject, ok := annotations[constants.BlockListDisableInjection]
	return ok && inject == "true"
}

func IsOriginalModelVolumeMountNecessary(annotations map[string]string) bool {
	return annotations[constants.ModelInitInjectionKey] != "true" &&
		annotations[constants.FTServingWithMergedWeightsAnnotationKey] != "true"
}

// IsEntrypointRouter checks if the InferenceService has the
// entrypoint-component annotation set to router.
// Returns true if the annotation is set to router, false otherwise.
func IsEntrypointRouter(annotations map[string]string) bool {
	componentValue, hasAnnotation := annotations[constants.EntrypointComponent]
	if !hasAnnotation {
		return false
	}

	// Validate against known component types
	switch v1beta1.ComponentType(componentValue) {
	case v1beta1.RouterComponent:
		return true
	default:
		// Invalid component type
		return false
	}
}

func IsEmptyModelDirVolumeRequired(annotations map[string]string) bool {
	modelInitInject := annotations[constants.ModelInitInjectionKey]
	fineTunedAdapterInject := annotations[constants.FineTunedAdapterInjectionKey]

	return modelInitInject == "true" || len(fineTunedAdapterInject) > 0
}

func IsCohereCommand1TFewFTServing(servingPodObjectMeta *metav1.ObjectMeta) bool {
	if servingPodObjectMeta.Annotations[constants.BaseModelVendorAnnotationKey] == string(constants.Cohere) &&
		servingPodObjectMeta.Annotations[constants.FineTunedWeightFTStrategyKey] == string(constants.TFewTrainingStrategy) &&
		servingPodObjectMeta.Annotations[constants.FTServingWithMergedWeightsAnnotationKey] != "true" {
		return true
	}
	return false
}

func SetPodLabelsFromAnnotations(metadata *metav1.ObjectMeta) {
	// Check if the VolcanoQueue annotation exists and set the label if it does.
	if volcanoQueue, ok := metadata.Annotations[constants.VolcanoQueue]; ok {
		metadata.Labels[constants.VolcanoQueueName] = volcanoQueue
		// If VolcanoQueue annotation does not exist, check and set to DedicatedAICluster name
	} else if dac, ok := metadata.Annotations[constants.DedicatedAICluster]; ok {
		if _, ok = metadata.Annotations[constants.KueueEnabledLabelKey]; ok {
			// Kueue case
			metadata.Labels[constants.KueueQueueLabelKey] = dac
			metadata.Labels[constants.KueueWorkloadPriorityClassLabelKey] = constants.DedicatedAiClusterPreemptionWorkloadPriorityClass
		} else {
			// Volcano case
			metadata.Labels[constants.VolcanoQueueName] = dac
			metadata.Labels[constants.RayPrioriyClass] = constants.DedicatedAiClusterPreemptionPriorityClass
		}
	}

	// Always set the RayScheduler label if the annotation exists.
	if _, ok := metadata.Annotations[constants.VolcanoScheduler]; ok {
		metadata.Labels[constants.RayScheduler] = constants.VolcanoScheduler
	}
}

func RemovePodAnnotations(metadata *metav1.ObjectMeta, annotationsToRemove []string) {
	for _, annotation := range annotationsToRemove {
		delete(metadata.Annotations, annotation)
	}
}
