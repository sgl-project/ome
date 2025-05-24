package pod

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	servingSidecarConfigMapKeyName = "servingSidecar"
)

// ServingSidecarInjector represents configuration parameters for the Serving sidecar container.
type ServingSidecarInjector struct {
	Image                string `json:"image" validate:"required"`
	MemoryRequest        string `json:"memoryRequest"`
	MemoryLimit          string `json:"memoryLimit"`
	CpuRequest           string `json:"cpuRequest"`
	CpuLimit             string `json:"cpuLimit"`
	CompartmentId        string `json:"compartmentId" validate:"required"`
	AuthType             string `json:"authType" validate:"required"`
	Region               string `json:"region"`
	RealmDomainComponent string `json:"realmDomainComponent"`
}

// newServingSidecarInjector initializes a ServingSidecarInjector from a ConfigMap.
func newServingSidecarInjector(configMap *v1.ConfigMap) *ServingSidecarInjector {
	servingSidecarInjector := &ServingSidecarInjector{}
	if servingSidecarConfigVal, ok := configMap.Data[servingSidecarConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(servingSidecarConfigVal), servingSidecarInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", servingSidecarConfigMapKeyName, err))
		}
	}
	return servingSidecarInjector
}

// InjectServingSidecar injects the serving sidecar container into the pod if necessary.
func (ss *ServingSidecarInjector) InjectServingSidecar(pod *v1.Pod) error {
	if enableServingSidecar, ok := pod.ObjectMeta.Annotations[constants.ServingSidecarInjectionKey]; ok && enableServingSidecar == "true" {
		return ss.injectServingSidecar(pod)
	}
	return nil
}

// njectServingSidecar adds the serving sidecar container and its configurations if it doesn’t already exist in the pod.
func (ss *ServingSidecarInjector) injectServingSidecar(pod *v1.Pod) error {
	if ss.containerExists(pod) {
		return nil
	}

	// general validation
	if err := ss.validate(); err != nil {
		return err
	}

	// validate specially for auth type
	if err := ss.validateAuth(pod); err != nil {
		return err
	}

	fineTunedWeightFTStrategy, err := ss.getFineTunedWeightFTStrategy(pod)
	if err != nil {
		return err
	}

	servingSidecarMounts := ss.getVolumeMounts(pod, fineTunedWeightFTStrategy)
	initEnvs := ss.getServingSidecarEnvs(fineTunedWeightFTStrategy)

	securityContext, err := ss.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	sidecarContainer := ss.createServingSidecarContainer(initEnvs, servingSidecarMounts, securityContext)
	pod.Spec.Containers = append(pod.Spec.Containers, *sidecarContainer)
	return nil
}

// containerExists checks if the Serving Sidecar container is already in the pod.
func (ss *ServingSidecarInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.ServingSidecarContainerName {
			return true
		}
	}
	return false
}

func (ss *ServingSidecarInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(ss); err != nil {
		return fmt.Errorf("failed to validate ServingSidecarInjector: %w", err)
	}
	return nil
}

// validateAuth checks if the correct authentication type is set for the Serving Sidecar container.
func (ss *ServingSidecarInjector) validateAuth(pod *v1.Pod) error {
	if ss.AuthType == constants.AuthtypeOKEWorkloadIdentity && len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}

	if ss.AuthType == constants.AuthtypeOKEWorkloadIdentity {
		automount := true
		pod.Spec.AutomountServiceAccountToken = &automount
	}
	return nil
}

// getVolumeMounts defines and returns volume mounts for the Model Init container.
func (ss *ServingSidecarInjector) getFineTunedWeightFTStrategy(pod *v1.Pod) (string, error) {
	if fineTunedWeightFTStrategy, ok := pod.ObjectMeta.Annotations[constants.FineTunedWeightFTStrategyKey]; ok {
		return fineTunedWeightFTStrategy, nil
	}
	return "", fmt.Errorf("failed to get the fine-tuned weight FT strategy for the serving sidecar")
}

// getVolumeMounts defines and returns volume mounts for the Model Init container.
func (ss *ServingSidecarInjector) getVolumeMounts(pod *v1.Pod, fineTunedWeightFTStrategy string) []v1.VolumeMount {
	servingSidecarMounts := []v1.VolumeMount{}

	fineTunedWeightMountPath := filepath.Join(constants.ModelDefaultMountPathPrefix, fineTunedWeightFTStrategy)
	fineTunedWeightVolumeMount := v1.VolumeMount{
		Name:      constants.ModelEmptyDirVolumeName,
		MountPath: fineTunedWeightMountPath,
		ReadOnly:  false,
		SubPath:   constants.FineTunedWeightVolumeMountSubPath,
	}
	fineTunedWeightDownloadMount := v1.VolumeMount{
		Name:      constants.ModelEmptyDirVolumeName,
		MountPath: constants.FineTunedWeightDownloadMountPath,
		ReadOnly:  false,
		SubPath:   constants.FineTunedWeightDownloadVolumeMountSubPath,
	}

	servingSidecarMounts = append(servingSidecarMounts, fineTunedWeightDownloadMount)
	servingSidecarMounts = append(servingSidecarMounts, fineTunedWeightVolumeMount)
	return servingSidecarMounts
}

func (ss *ServingSidecarInjector) getServingSidecarEnvs(fineTunedWeightFTStrategy string) []v1.EnvVar {
	envVars := []v1.EnvVar{
		{Name: constants.AgentAuthTypeEnvVarKey, Value: ss.AuthType},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: ss.CompartmentId},
		{Name: constants.AgentRegionEnvVarKey, Value: ss.Region},
		{Name: constants.AgentFineTunedWeightInfoFilePath, Value: constants.AgentFineTunedWeightInfoFilePath},
		{Name: constants.AgentUnzippedFineTunedWeightDirectory, Value: filepath.Join(constants.ModelDefaultMountPathPrefix, fineTunedWeightFTStrategy)},
		{Name: constants.AgentZippedFineTunedWeightDirectory, Value: constants.FineTunedWeightDownloadMountPath},
	}

	return envVars
}

// createServingSidecarContainer constructs the serving sidecar configuration.
func (ss *ServingSidecarInjector) createServingSidecarContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	return &v1.Container{
		Name:                     constants.ServingSidecarContainerName,
		Image:                    ss.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"serving-agent", "--config", "/ome-agent.yaml", "--debug"},
		Resources: v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(ss.CpuLimit),
				v1.ResourceMemory: resource.MustParse(ss.MemoryLimit),
			},
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(ss.CpuRequest),
				v1.ResourceMemory: resource.MustParse(ss.MemoryRequest),
			},
		},
		SecurityContext: securityContext,
	}
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (ss *ServingSidecarInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.MainContainerName)
}
