package utils

import (
	"regexp"
	"strings"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/utils"
	v1 "k8s.io/api/core/v1"
)

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

// GetContainerIndex returns the index of the container in the runtime containers.
func GetContainerIndex(containers []v1.Container, containerName string) int {
	for i, container := range containers {
		if container.Name == containerName {
			return i
		}
	}
	return -1
}

// GetGpuCountFromContainer extracts the GPU count from container resources.
// It checks both Limits and Requests, preferring Limits.
func GetGpuCountFromContainer(container *v1.Container) int {
	if container == nil {
		return 0
	}
	var gpuCount int
	resourceName := v1.ResourceName(constants.NvidiaGPUResourceType)

	if quantity, ok := container.Resources.Limits[resourceName]; ok {
		gpuCount = int(quantity.Value())
	} else if quantity, ok := container.Resources.Requests[resourceName]; ok {
		gpuCount = int(quantity.Value())
	}
	return gpuCount
}
