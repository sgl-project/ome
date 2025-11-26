package utils

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
)

/*
GetDeploymentModeFromAnnotations extracts a valid deployment mode from annotations if present.
Returns the deployment mode and true if found and valid, otherwise returns empty string and false.
*/
func GetDeploymentModeFromAnnotations(annotations map[string]string) (constants.DeploymentModeType, bool) {
	if annotations == nil {
		return "", false
	}
	if mode, exists := annotations[constants.DeploymentMode]; exists {
		deploymentMode := constants.DeploymentModeType(mode)
		if deploymentMode.IsValid() {
			return deploymentMode, true
		}
	}
	return "", false
}

/*
GetDeploymentMode returns the current deployment mode based on annotations and config.
If a valid deployment mode is specified in annotations, it is used.
Otherwise, returns the default deployment mode from config.
*/
func GetDeploymentMode(annotations map[string]string, deployConfig *controllerconfig.DeployConfig) constants.DeploymentModeType {
	if mode, found := GetDeploymentModeFromAnnotations(annotations); found {
		return mode
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

// ResolveIngressConfig creates an effective ingress configuration by merging
// global defaults from configMap with per-service annotation overrides
func ResolveIngressConfig(baseConfig *controllerconfig.IngressConfig, annotations map[string]string) *controllerconfig.IngressConfig {
	// Start with a copy of the base config to avoid modifying the original
	resolved := &controllerconfig.IngressConfig{
		IngressGateway:             baseConfig.IngressGateway,
		IngressServiceName:         baseConfig.IngressServiceName,
		LocalGateway:               baseConfig.LocalGateway,
		LocalGatewayServiceName:    baseConfig.LocalGatewayServiceName,
		KnativeLocalGatewayService: baseConfig.KnativeLocalGatewayService,
		OmeIngressGateway:          baseConfig.OmeIngressGateway,
		IngressDomain:              baseConfig.IngressDomain,
		IngressClassName:           baseConfig.IngressClassName,
		AdditionalIngressDomains:   baseConfig.AdditionalIngressDomains,
		DomainTemplate:             baseConfig.DomainTemplate,
		UrlScheme:                  baseConfig.UrlScheme,
		DisableIstioVirtualHost:    baseConfig.DisableIstioVirtualHost,
		PathTemplate:               baseConfig.PathTemplate,
		DisableIngressCreation:     baseConfig.DisableIngressCreation,
		EnableGatewayAPI:           baseConfig.EnableGatewayAPI,
	}

	// Override with annotation values if present
	if domainTemplate, exists := annotations[constants.IngressDomainTemplate]; exists && domainTemplate != "" {
		resolved.DomainTemplate = domainTemplate
	}

	if ingressDomain, exists := annotations[constants.IngressDomain]; exists && ingressDomain != "" {
		resolved.IngressDomain = ingressDomain
	}

	if urlScheme, exists := annotations[constants.IngressURLScheme]; exists && urlScheme != "" {
		resolved.UrlScheme = urlScheme
	}

	if pathTemplate, exists := annotations[constants.IngressPathTemplate]; exists {
		resolved.PathTemplate = pathTemplate
	}

	if additionalDomains, exists := annotations[constants.IngressAdditionalDomains]; exists && additionalDomains != "" {
		// Parse comma-separated list
		domains := strings.Split(additionalDomains, ",")
		for i := range domains {
			domains[i] = strings.TrimSpace(domains[i])
		}
		resolved.AdditionalIngressDomains = &domains
	}

	// Boolean overrides
	if disableVirtualHost, exists := annotations[constants.IngressDisableIstioVirtualHost]; exists {
		resolved.DisableIstioVirtualHost = disableVirtualHost == "true"
	}

	if disableCreation, exists := annotations[constants.IngressDisableCreation]; exists {
		resolved.DisableIngressCreation = disableCreation == "true"
	}

	return resolved
}
