package utils

import (
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/constants"
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

func AppendEnvVarsIfNotExist(container *v1.Container, envVars *[]v1.EnvVar) {
	if envVars == nil {
		return
	}

	for _, envVar := range *envVars {
		var exists bool
		for i := range container.Env {
			if container.Env[i].Name == envVar.Name {
				exists = true
				break
			}
		}
		if !exists {
			container.Env = append(container.Env, envVar)
		}
	}
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

// GetGpuCountFromContainer extracts the accelerator count from container
// resources: it walks acceleratorResources in order and returns the value of
// the first name present, checking that name's Limits before its Requests.
//
// acceleratorResources is the operator-configured list of extended resource
// names that count as an accelerator (inferenceservice-config ConfigMap's
// "acceleratorResources" key — see
// controllerconfig.InferenceServicesConfig.AcceleratorResourceNames). There is
// NO in-code default list: an empty/nil acceleratorResources means the config
// is absent, and this falls back to recognizing only
// constants.NvidiaGPUResourceType, so an unconfigured cluster keeps
// NVIDIA-only counting.
func GetGpuCountFromContainer(container *v1.Container, acceleratorResources []string) int {
	if container == nil {
		return 0
	}
	names := acceleratorResources
	if len(names) == 0 {
		names = []string{constants.NvidiaGPUResourceType}
	}

	for _, name := range names {
		resourceName := v1.ResourceName(name)
		if quantity, ok := container.Resources.Limits[resourceName]; ok {
			return int(quantity.Value())
		}
		if quantity, ok := container.Resources.Requests[resourceName]; ok {
			return int(quantity.Value())
		}
	}
	return 0
}
