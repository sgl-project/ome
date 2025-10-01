package utils

import (
	v1 "k8s.io/api/core/v1"

	"github.com/sgl-project/ome/pkg/constants"
)

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

func AppendEnvVarIfNotExist(container *v1.Container, envVar *v1.EnvVar) {
	for i := range container.Env {
		if container.Env[i].Name == envVar.Name {
			return
		}
	}
	container.Env = append(container.Env, *envVar)
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
