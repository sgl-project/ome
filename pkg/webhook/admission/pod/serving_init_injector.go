package pod

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	ServingInitConfigMapKeyName = "servingInit"
)

// ServingInitInjector represents configuration parameters for the Serving Init container.
type ServingInitInjector struct {
	Image         string `json:"image"`
	MemoryRequest string `json:"memoryRequest"`
	MemoryLimit   string `json:"memoryLimit"`
	CpuRequest    string `json:"cpuRequest"`
	CpuLimit      string `json:"cpuLimit"`
	CompartmentId string `json:"compartmentId"`
	AuthType      string `json:"authType"`
	VaultId       string `json:"vaultId"`
	Region        string `json:"region"`
}

// newServingInitInjector initializes a ServingInitInjector from a ConfigMap.
func newServingInitInjector(configMap *v1.ConfigMap) *ServingInitInjector {
	ci := &ServingInitInjector{}
	if ciConfigVal, ok := configMap.Data[ServingInitConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(ciConfigVal), ci); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", ServingInitConfigMapKeyName, err))
		}
	}
	return ci
}

// InjectServingInit injects the serving initialization container into the pod if necessary.
func (si *ServingInitInjector) InjectServingInit(pod *v1.Pod) error {
	if enableServingInit, ok := pod.ObjectMeta.Annotations[constants.SevingInitInjectionKey]; ok && enableServingInit == "true" {
		return si.injectServingInit(pod)
	}
	return nil
}

// injectServingInit adds the Serving Init container and its configurations if it doesn’t already exist in the pod.
func (si *ServingInitInjector) injectServingInit(pod *v1.Pod) error {
	if si.containerExists(pod) {
		return nil
	}

	if err := si.validateAuth(pod); err != nil {
		return err
	}

	servingInitMounts := si.getVolumeMounts(pod)
	initEnvs, err := si.getServingInitEnvs(pod)
	if err != nil {
		return err
	}

	securityContext, err := si.getMainServingContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	initContainer := si.createInitContainer(initEnvs, servingInitMounts, securityContext)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)
	return nil
}

// containerExists checks if the Serving Init container is already in the pod.
func (si *ServingInitInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == constants.ServingInitContainerName {
			return true
		}
	}
	return false
}

// validateAuth checks if the correct authentication type is set for the Serving Init container.
func (si *ServingInitInjector) validateAuth(pod *v1.Pod) error {
	if si.AuthType == constants.AuthtypeOKEWorkloadIdentity && len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}
	automount := true
	pod.Spec.AutomountServiceAccountToken = &automount
	return nil
}

// getVolumeMounts defines and returns volume mounts for the Serving Init container.
func (si *ServingInitInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	baseModelName := pod.ObjectMeta.Annotations[constants.BaseModelName]
	return []v1.VolumeMount{
		{
			Name:      constants.EmptyDirVolumeSourceName,
			MountPath: constants.InitContainerModelFinalDefaultPath,
		},
		{
			Name:      baseModelName,
			MountPath: constants.InitContainerModelSourceDefaultPath,
		},
	}
}

// getMainServingContainerSecurityContext finds and returns the security context of the main serving container.
func (si *ServingInitInjector) getMainServingContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.InferenceServiceContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main serving container specified")
}

// createInitContainer constructs the init container configuration.
func (si *ServingInitInjector) createInitContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	return &v1.Container{
		Name:                     constants.ServingInitContainerName,
		Image:                    si.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"enigma", "--config", "/ome-agent.yaml", "--debug"},
		Resources: v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(si.CpuLimit),
				v1.ResourceMemory: resource.MustParse(si.MemoryLimit),
			},
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(si.CpuRequest),
				v1.ResourceMemory: resource.MustParse(si.MemoryRequest),
			},
		},
		SecurityContext: securityContext,
	}
}

// getServingInitEnvs generates environment variables for the Serving Init container.
func (si *ServingInitInjector) getServingInitEnvs(pod *v1.Pod) ([]v1.EnvVar, error) {
	envVars := []v1.EnvVar{

		{Name: constants.AgentAuthTypeEnvVarKey, Value: si.getAnnotationOrDefault(pod, constants.EncryptionAuthType, si.AuthType)},
		{Name: constants.AgentLocalPathEnvVarKey, Value: constants.InitContainerModelSourceDefaultPath},
		{Name: constants.AgentModelNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelName]},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: si.getAnnotationOrDefault(pod, constants.BaseModelDecryptionCompartmentID, si.CompartmentId)},
		{Name: constants.AgentVaultIDEnvVarKey, Value: si.getAnnotationOrDefault(pod, constants.BaseModelDecryptionVaultID, si.VaultId)},
		{Name: constants.AgentSecretNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelDecryptionSecretName]},
		{Name: constants.AgentKeyNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelDecryptionKeyName]},
		{Name: constants.AgentDisableModelDecryptionEnvVarKey, Value: si.getAnnotationOrDefault(pod, constants.DisableModelDecryption, "false")},
		{Name: constants.AgentModelStoreDirectoryEnvVarKey, Value: constants.InitContainerModelFinalDefaultPath},
		{Name: constants.AgentNumOfGPUEnvVarKey, Value: si.getGPUCount(pod)},
	}

	if modelFormat := pod.ObjectMeta.Annotations[constants.BaseModelFormat]; modelFormat == "tensorrt_llm" {
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentModelFrameworkEnvVarKey, Value: "tensorrtllm"})
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentTensorRTLLMVersionsEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelFormatVersion]})
	} else {
		envVars = append(envVars, v1.EnvVar{Name: "OME_AGENT_IS_TENSORRT_MODEL", Value: "false"})
	}

	return envVars, nil
}

// getGPUCount retrieves the GPU count for the main serving container.
func (si *ServingInitInjector) getGPUCount(pod *v1.Pod) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.InferenceServiceContainerName {
			if gpus, exists := container.Resources.Limits[constants.NvidiaGPUResourceType]; exists {
				return gpus.String()
			}
		}
	}
	panic("NVIDIA GPU resource not set for serving container")
}

// getAnnotationOrDefault retrieves the value from the pod's annotations if it exists;
// otherwise, it returns the provided default value.
func (si *ServingInitInjector) getAnnotationOrDefault(pod *v1.Pod, key, defaultValue string) string {
	if value, exists := pod.ObjectMeta.Annotations[key]; exists {
		return value
	}
	return defaultValue
}
