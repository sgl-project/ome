package pod

import (
	"encoding/json"
	"fmt"
	"strings"

	isvcutils "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	modelInitConfigMapKeyName = "modelInit"
)

// ModelInitInjector represents configuration parameters for the Model Init container.
type ModelInitInjector struct {
	Image         string `json:"image" validate:"required"`
	MemoryRequest string `json:"memoryRequest"`
	MemoryLimit   string `json:"memoryLimit"`
	CpuRequest    string `json:"cpuRequest"`
	CpuLimit      string `json:"cpuLimit"`
	CompartmentId string `json:"compartmentId" validate:"required"`
	AuthType      string `json:"authType" validate:"required"`
	VaultId       string `json:"vaultId" validate:"required"`
	Region        string `json:"region"`
}

// newModelInitInjector initializes a ModelInitInjector from a ConfigMap.
func newModelInitInjector(configMap *v1.ConfigMap) *ModelInitInjector {
	modelInitInjector := &ModelInitInjector{}
	if modelInitConfigVal, ok := configMap.Data[modelInitConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(modelInitConfigVal), modelInitInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", modelInitConfigMapKeyName, err))
		}
	}
	return modelInitInjector
}

// InjectModelInit injects the model initialization container into the pod if necessary.
func (mi *ModelInitInjector) InjectModelInit(pod *v1.Pod) error {
	if enableModelInit, ok := pod.ObjectMeta.Annotations[constants.ModelInitInjectionKey]; ok && enableModelInit == "true" {
		return mi.injectModelInit(pod)
	}
	return nil
}

// injectModelInit adds the Model Init container and its configurations if it doesn’t already exist in the pod.
func (mi *ModelInitInjector) injectModelInit(pod *v1.Pod) error {
	if mi.containerExists(pod) {
		return nil
	}

	// general validation
	if err := mi.validate(); err != nil {
		return err
	}

	// validate specially for auth type
	if err := mi.validateAuth(pod); err != nil {
		return err
	}

	modelInitMounts := mi.getVolumeMounts(pod)
	initEnvs, err := mi.getModelInitEnvs(pod)
	if err != nil {
		return err
	}

	securityContext, err := mi.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	initContainer := mi.createInitContainer(initEnvs, modelInitMounts, securityContext)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)
	return nil
}

// containerExists checks if the Model Init container is already in the pod.
func (mi *ModelInitInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == constants.ModelInitContainerName {
			return true
		}
	}
	return false
}

// validateAuth checks if the correct authentication type is set for the Model Init container.
func (mi *ModelInitInjector) validateAuth(pod *v1.Pod) error {
	if mi.AuthType == constants.AuthtypeOKEWorkloadIdentity && len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}

	if mi.AuthType == constants.AuthtypeOKEWorkloadIdentity {
		automount := true
		pod.Spec.AutomountServiceAccountToken = &automount
	}
	return nil
}

func (mi *ModelInitInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(mi); err != nil {
		return fmt.Errorf("failed to validate ModelInitInjector: %w", err)
	}
	return nil
}

// getVolumeMounts defines and returns volume mounts for the Model Init container.
func (mi *ModelInitInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	baseModelName := pod.ObjectMeta.Annotations[constants.BaseModelName]
	return []v1.VolumeMount{
		{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: constants.ModelDefaultMountPath,
			SubPath:   mi.getBaseModelVolumeMountSubPath(pod),
		},
		{
			Name:      baseModelName,
			MountPath: constants.ModelDefaultSourcePath,
		},
	}
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (mi *ModelInitInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName || container.Name == constants.TrainingMainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s or %s specified", constants.MainContainerName, constants.TrainingMainContainerName)
}

// createInitContainer constructs the init container configuration.
func (mi *ModelInitInjector) createInitContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	return &v1.Container{
		Name:                     constants.ModelInitContainerName,
		Image:                    mi.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"enigma", "--config", "/ome-agent.yaml", "--debug"},
		Resources: v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(mi.CpuLimit),
				v1.ResourceMemory: resource.MustParse(mi.MemoryLimit),
			},
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(mi.CpuRequest),
				v1.ResourceMemory: resource.MustParse(mi.MemoryRequest),
			},
		},
		SecurityContext: securityContext,
	}
}

// getModelInitEnvs generates environment variables for the Model Init container.
func (mi *ModelInitInjector) getModelInitEnvs(pod *v1.Pod) ([]v1.EnvVar, error) {
	envVars := []v1.EnvVar{
		{Name: constants.AgentAuthTypeEnvVarKey, Value: mi.AuthType},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: mi.CompartmentId},
		{Name: constants.AgentVaultIDEnvVarKey, Value: mi.VaultId},
		{Name: constants.AgentModelNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelName]},
		{Name: constants.AgentKeyNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelDecryptionKeyName]},
		{Name: constants.AgentSecretNameEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelDecryptionSecretName]},
		{Name: constants.AgentDisableModelDecryptionEnvVarKey, Value: mi.getAnnotationOrDefault(pod, constants.DisableModelDecryption, "false")},
		{Name: constants.AgentBaseModelTypeEnvVarKey, Value: mi.getLabelOrDefault(pod, constants.BaseModelTypeLabelKey, string(constants.ServingBaseModel))},
		{Name: constants.AgentLocalPathEnvVarKey, Value: constants.ModelDefaultSourcePath},
		{Name: constants.AgentModelStoreDirectoryEnvVarKey, Value: constants.ModelDefaultMountPath},
		{Name: constants.AgentRegionEnvVarKey, Value: mi.Region},
	}

	modelFormat := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(pod.ObjectMeta.Annotations[constants.BaseModelFormat], "_", ""), "-", ""))
	if modelFormat == strings.ToLower(strings.ReplaceAll(constants.TensorRTLLM, "_", "")) {
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentModelFrameworkEnvVarKey, Value: constants.TensorRTLLM})
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentTensorRTLLMVersionsEnvVarKey, Value: pod.ObjectMeta.Annotations[constants.BaseModelFormatVersion]})
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentNumOfGPUEnvVarKey, Value: mi.getGPUCount(pod)})
	} else {
		envVars = append(envVars, v1.EnvVar{Name: constants.AgentModelFrameworkEnvVarKey, Value: modelFormat})
	}

	return envVars, nil
}

// getGPUCount retrieves the GPU count for the main container.
func (mi *ModelInitInjector) getGPUCount(pod *v1.Pod) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName {
			if gpus, exists := container.Resources.Limits[constants.NvidiaGPUResourceType]; exists {
				return gpus.String()
			}
		}
	}
	panic("NVIDIA GPU resource not set for main container")
}

// getAnnotationOrDefault retrieves the value from the pod's annotations if it exists;
// otherwise, it returns the provided default value.
func (mi *ModelInitInjector) getAnnotationOrDefault(pod *v1.Pod, key, defaultValue string) string {
	if value, exists := pod.ObjectMeta.Annotations[key]; exists {
		return value
	}
	return defaultValue
}

// getLabelOrDefault retrieves the value from the pod's labels if it exists;
// otherwise, it returns the provided default value.
func (mi *ModelInitInjector) getLabelOrDefault(pod *v1.Pod, key, defaultValue string) string {
	if value, exists := pod.ObjectMeta.Labels[key]; exists {
		return value
	}
	return defaultValue
}

func (mi *ModelInitInjector) getBaseModelVolumeMountSubPath(pod *v1.Pod) string {
	if isvcutils.IsCohereCommand1TFewFTServing(&pod.ObjectMeta) {
		return constants.BaseModelVolumeMountSubPath
	}
	return ""
}
