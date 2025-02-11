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

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"

	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
)

/*
GetDeploymentMode returns the current deployment mode based on annotations and config.
If a valid deployment mode is specified in annotations, it is used.
Otherwise, returns the default deployment mode from config.
*/
func GetDeploymentMode(annotations map[string]string, deployConfig *v1beta1.DeployConfig) constants.DeploymentModeType {
	if mode, exists := annotations[constants.DeploymentMode]; exists {
		deploymentMode := constants.DeploymentModeType(mode)
		if deploymentMode.IsValid() {
			return deploymentMode
		}
	}
	return constants.DeploymentModeType(deployConfig.DefaultDeploymentMode)
}

// IsChainsawInjectEnabled checks if the Chainsaw sidecar injection is enabled
// based on the InferenceService annotations.
func IsChainsawInjectEnabled(annotations map[string]string) bool {
	chainsawInject, ok := annotations[constants.ChainsawInject]
	return ok && chainsawInject == "enabled"
}

func IsBlockListInjectionDisabled(annotations map[string]string) bool {
	inject, ok := annotations[constants.BlockListDisableInjection]
	return ok && inject == "true"
}

func IsOriginalModelVolumnMountNecessary(annotations map[string]string) bool {
	inject, ok := annotations[constants.ModelInitInjectionKey]
	return !ok || inject != "true"
}

func LoadingMergedFinetunedWeights(finetunedWeights []*v1beta1.FineTunedWeight) bool {
	// TODO: FinetunedWeight should have a indicator about if it is a merged weights
	// TODO: The webhook should validate if only one merged weights is attached
	return len(finetunedWeights) == 1
}

func SetPodLabelsFromAnnotations(metadata *metav1.ObjectMeta) {
	// Check if the VolcanoQueue annotation exists and set the label if it does.
	if volcanoQueue, ok := metadata.Annotations[constants.VolcanoQueue]; ok {
		metadata.Labels[constants.VolcanoQueueName] = volcanoQueue
	} else if dac, ok := metadata.Annotations[constants.DedicatedAICluster]; ok {
		// If VolcanoQueue annotation does not exist, check and set DedicatedAICluster.
		metadata.Labels[constants.VolcanoQueueName] = dac
		metadata.Labels[constants.RayPrioriyClass] = constants.DedicatedAiClusterPreemptionPriorityClass
	}

	// Always set the RayScheduler label if the annotation exists.
	if _, ok := metadata.Annotations[constants.VolcanoScheduler]; ok {
		metadata.Labels[constants.RayScheduler] = constants.VolcanoScheduler
	}
}

func RemovePodAnnotations(metadata *metav1.ObjectMeta, annotationsToRemove []string) {
	for _, annotation := range annotationsToRemove {
		if _, ok := metadata.Annotations[annotation]; ok {
			delete(metadata.Annotations, annotation)
		}
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

// GetBaseModel Get the base model from the given model name.
func GetBaseModel(cl client.Client, name string, namespace string) (*v1beta1.BaseModelSpec, *metav1.ObjectMeta, error) {
	baseModel := &v1beta1.BaseModel{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, baseModel)
	if err == nil {
		return &baseModel.Spec, &baseModel.ObjectMeta, nil
	} else if !errors.IsNotFound(err) {
		return nil, nil, err
	}
	clusterBaseModel := &v1beta1.ClusterBaseModel{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterBaseModel)
	if err == nil {
		return &clusterBaseModel.Spec, &clusterBaseModel.ObjectMeta, nil
	} else if !errors.IsNotFound(err) {
		return nil, nil, err
	}
	return nil, nil, goerrors.New("No BaseModel or ClusterBaseModel with the name: " + name)
}

// GetFinetunedWeight Get the finetuned weight from the given finetuned weight name.
func GetFinetunedWeight(cl client.Client, name string, namespace string) (*v1beta1.FineTunedWeight, error) {
	finetunedWeight := &v1beta1.FineTunedWeight{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, finetunedWeight)
	if err == nil {
		return finetunedWeight, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No FinetunedWeight with the name: " + name + " in namespace: " + namespace)
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
		if len(re.FindString(image)) > 0 {
			// For TFServing/TorchServe the GPU image is tagged with suffix "-gpu", when the version is found in the tag
			// and runtimeVersion is not specified, we default to append the "-gpu" suffix to the image tag
		}
	}
}

func UpdateVolumeMounts(container *v1.Container, volumeMount *v1.VolumeMount) {
	container.VolumeMounts = append(container.VolumeMounts, *volumeMount)
}

func UpdateContainerArgs(container *v1.Container, args *[]string) {
	container.Args = append(container.Args, *args...)
}

func AppendEnvVars(container *v1.Container, envVars *[]v1.EnvVar) {
	container.Env = append(container.Env, *envVars...)
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
