package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/utils"
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

func LoadingMergedFineTunedWeight(fineTunedWeights []*v1beta1.FineTunedWeight) (bool, error) {
	mergedFineTunedWeights, err := IsMergedFineTunedWeight(fineTunedWeights[0])
	if err != nil {
		return false, err
	}
	return len(fineTunedWeights) == 1 && mergedFineTunedWeights, nil
}

func IsMergedFineTunedWeight(fineTunedWeight *v1beta1.FineTunedWeight) (bool, error) {
	if fineTunedWeight != nil {
		var configMap map[string]interface{}
		if err := json.Unmarshal(fineTunedWeight.Spec.Configuration.Raw, &configMap); err != nil {
			return false, err
		}
		if mergedWeights, exists := configMap[constants.FineTunedWeightMergedWeightsConfigKey]; exists && mergedWeights == true {
			return true, nil
		}
	}
	return false, nil
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

// MergeRuntimeContainers Merge the predictor Container struct with the runtime Container struct, allowing users
// to override runtime container settings from the predictor spec.
func MergeRuntimeContainers(runtimeContainer *v1.Container, predictorContainer *v1.Container) (*v1.Container, error) {
	// Save runtime container name, as the name can be overridden as empty string during the Unmarshal below
	// since the Name field does not have the 'omitempty' struct tag.
	runtimeContainerName := runtimeContainer.Name

	// Use JSON Marshal/Unmarshal to merge Container structs using strategic merge patch
	runtimeContainerJson, err := json.Marshal(runtimeContainer)
	if err != nil {
		return nil, err
	}

	overrides, err := json.Marshal(predictorContainer)
	if err != nil {
		return nil, err
	}

	mergedContainer := v1.Container{}
	jsonResult, err := strategicpatch.StrategicMergePatch(runtimeContainerJson, overrides, mergedContainer)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &mergedContainer); err != nil {
		return nil, err
	}

	if mergedContainer.Name == "" {
		mergedContainer.Name = runtimeContainerName
	}

	// Strategic merge patch will replace args but more useful behaviour here is to concatenate
	mergedContainer.Args = append(append([]string{}, runtimeContainer.Args...), predictorContainer.Args...)

	return &mergedContainer, nil
}

// MergePodSpec Merge the predictor PodSpec struct with the runtime PodSpec struct, allowing users
// to override runtime PodSpec settings from the predictor spec.
func MergePodSpec(runtimePodSpec *v1beta1.ServingRuntimePodSpec, predictorPodSpec *v1beta1.PodSpec) (*v1.PodSpec, error) {
	runtimePodSpecJson, err := json.Marshal(v1.PodSpec{
		NodeSelector:     runtimePodSpec.NodeSelector,
		Affinity:         runtimePodSpec.Affinity,
		Tolerations:      runtimePodSpec.Tolerations,
		Volumes:          runtimePodSpec.Volumes,
		ImagePullSecrets: runtimePodSpec.ImagePullSecrets,
		DNSPolicy:        runtimePodSpec.DNSPolicy,
		HostNetwork:      runtimePodSpec.HostNetwork,
		SchedulerName:    runtimePodSpec.SchedulerName,
	})
	if err != nil {
		return nil, err
	}

	// Use JSON Marshal/Unmarshal to merge PodSpec structs.
	overrides, err := json.Marshal(predictorPodSpec)
	if err != nil {
		return nil, err
	}

	corePodSpec := v1.PodSpec{}
	jsonResult, err := strategicpatch.StrategicMergePatch(runtimePodSpecJson, overrides, corePodSpec)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &corePodSpec); err != nil {
		return nil, err
	}

	return &corePodSpec, nil
}

// GetServingRuntime Get a ServingRuntime by name. First, ServingRuntimes in the given namespace will be checked.
// If a resource of the specified name is not found, then ClusterServingRuntimes will be checked.
func GetServingRuntime(cl client.Client, name string, namespace string) (*v1beta1.ServingRuntimeSpec, error) {
	runtime := &v1beta1.ServingRuntime{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, runtime)
	if err == nil {
		return &runtime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}

	clusterRuntime := &v1beta1.ClusterServingRuntime{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterRuntime)
	if err == nil {
		return &clusterRuntime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No ServingRuntimes or ClusterServingRuntimes with the name: " + name)
}

// GetFineTunedWeight Get the fine-tuned weight from the given fine-tuned weight name.
func GetFineTunedWeight(cl client.Client, name string) (*v1beta1.FineTunedWeight, error) {
	fineTunedWeight := &v1beta1.FineTunedWeight{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name}, fineTunedWeight)
	if err == nil {
		return fineTunedWeight, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No FineTunedWeight with the name: " + name)
}

// ReplacePlaceholders Replace placeholders in runtime container by values from inferenceservice metadata
func ReplacePlaceholders(container *v1.Container, meta metav1.ObjectMeta) error {
	data, _ := json.Marshal(container)
	tmpl, err := template.New("container-tmpl").Parse(string(data))
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, meta)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), container)
}

// UpdateImageTag Update image tag if GPU is enabled or runtime version is provided
func UpdateImageTag(container *v1.Container, runtimeVersion *string, servingRuntime *string) {
	image := container.Image
	if runtimeVersion != nil {
		re := regexp.MustCompile(`(:([\w.\-_]*))$`)
		if len(re.FindString(image)) == 0 {
			container.Image = image + ":" + *runtimeVersion
		} else {
			container.Image = re.ReplaceAllString(image, ":"+*runtimeVersion)
		}
	} else if utils.IsGPUEnabled(container.Resources) && len(strings.Split(image, ":")) > 0 {
		re := regexp.MustCompile(`(:([\w.\-_]*))$`)
		// For TFServing/TorchServe the GPU image is tagged with suffix "-gpu", when the version is found in the tag
		// and runtimeVersion is not specified, we default to append the "-gpu" suffix to the image tag
		if len(re.FindString(image)) > 0 {
			// TODO: RuntimeVersion is not passed at this moment and also the image tagged with "-gpu" is not ready as well, so comment these 2 lines for now.
			//tag := re.FindStringSubmatch(image)[2]
			//container.Image = re.ReplaceAllString(image, ":"+tag+"-gpu")
			container.Image = image
		}
	}
}

func AppendVolumeMount(container *v1.Container, volumeMount *v1.VolumeMount) {
	container.VolumeMounts = append(container.VolumeMounts, *volumeMount)
}

func UpdateVolumeMount(container *v1.Container, volumeMount *v1.VolumeMount) {
	if volumeMount == nil {
		return
	}
	var updated bool
	for i, vm := range container.VolumeMounts {
		if vm.Name == volumeMount.Name {
			container.VolumeMounts[i].MountPath = volumeMount.MountPath
			container.VolumeMounts[i].SubPath = volumeMount.SubPath
			container.VolumeMounts[i].ReadOnly = volumeMount.ReadOnly
			updated = true
			break
		}
	}

	// If the volume mount does not exist, append it to the list.
	if !updated {
		container.VolumeMounts = append(container.VolumeMounts, *volumeMount)
	}
}

func AppendVolumeMountIfNotExist(container *v1.Container, volumeMount *v1.VolumeMount) {
	for i := range container.VolumeMounts {
		if container.VolumeMounts[i].Name == volumeMount.Name {
			return
		}
	}
	container.VolumeMounts = append(container.VolumeMounts, *volumeMount)
}

func AppendContainerArgs(container *v1.Container, args *[]string) {
	container.Args = append(container.Args, *args...)
}

func AppendEnvVars(container *v1.Container, envVars *[]v1.EnvVar) {
	container.Env = append(container.Env, *envVars...)
}

func UpdateEnvVars(container *v1.Container, envVar *v1.EnvVar) {
	var updated bool
	for i, existingEnvVar := range container.Env {
		if existingEnvVar.Name == envVar.Name {
			// If it exists, update its value.
			container.Env[i].Value = envVar.Value
			updated = true
			break
		}
	}
	// If the environment variable does not exist, append it to the list.
	if !updated {
		container.Env = append(container.Env, *envVar)
	}
}

// ListPodsByLabel Get a PodList by label.
func ListPodsByLabel(cl client.Client, namespace string, labelKey string, labelVal string) (*v1.PodList, error) {
	podList := &v1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{labelKey: labelVal},
	}
	err := cl.List(context.TODO(), podList, opts...)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	sortPodsByCreatedTimestampDesc(podList)
	return podList, nil
}

func sortPodsByCreatedTimestampDesc(pods *v1.PodList) {
	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[j].ObjectMeta.CreationTimestamp.Before(&pods.Items[i].ObjectMeta.CreationTimestamp)
	})
}

// function to get generate scaledObject name
func GetScaledObjectName(isvcName string) string {
	const (
		prefix     = "scaledobject-"
		maxNameLen = 50
	)
	if len(isvcName) > maxNameLen {
		isvcName = isvcName[len(isvcName)-maxNameLen:]
	}
	return fmt.Sprintf("%s%s", prefix, isvcName)
}

// GetOmeContainerIndex returns the index of the OME container in the runtime containers.
func GetOmeContainerIndex(containers []v1.Container) int {
	for i, container := range containers {
		if container.Name == constants.MainContainerName {
			return i
		}
	}
	return -1
}

func GetBaseModelVendor(baseModel v1beta1.BaseModelSpec) string {
	baseModelVendor := "Unknown"
	if baseModel.Vendor != nil {
		baseModelVendor = *baseModel.Vendor
	}
	return baseModelVendor
}

// GetValueFromRawExtension extracts a value by key from a JSON-encoded runtime.RawExtension.
// It returns nil if the key does not exist or the data is not a map.
func GetValueFromRawExtension(raw runtime.RawExtension, key string) (interface{}, error) {
	if len(raw.Raw) == 0 {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw.Raw, &data); err != nil {
		return nil, err
	}

	val, ok := data[key]
	if !ok {
		return nil, nil // or optionally return an error if key must exist
	}

	return val, nil
}
