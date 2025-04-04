package pod

import (
	"encoding/json"
	"fmt"

	casper "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casperagent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	isvcutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/utils"
	casperutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/model-agent"
	"github.com/go-playground/validator/v10"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fineTunedAdapterConfigMapKeyName = "fineTunedAdapter"
)

// FineTunedAdapterInjector represents configuration parameters for the Fine-Tuned Adapter.
type FineTunedAdapterInjector struct {
	Image               string `json:"image" validate:"required"`
	MemoryRequest       string `json:"memoryRequest"`
	MemoryLimit         string `json:"memoryLimit"`
	CpuRequest          string `json:"cpuRequest"`
	CpuLimit            string `json:"cpuLimit"`
	CompartmentId       string `json:"compartmentId" validate:"required"`
	AuthType            string `json:"authType" validate:"required"`
	Region              string `json:"region"`
	fineTunedWeightName string
	client              client.Client
}

// newFineTunedAdapterInjector initializes a FineTunedAdapterInjector from a ConfigMap.
func newFineTunedAdapterInjector(configMap *v1.ConfigMap, client client.Client) *FineTunedAdapterInjector {
	fineTunedAdapterInjector := &FineTunedAdapterInjector{}
	if fineTunedAdapterConfigVal, ok := configMap.Data[fineTunedAdapterConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(fineTunedAdapterConfigVal), fineTunedAdapterInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", fineTunedAdapterConfigMapKeyName, err))
		}
	}
	fineTunedAdapterInjector.client = client
	return fineTunedAdapterInjector
}

// InjectFineTunedAdapter injects the fine-tuned weight initialization container into the pod if necessary.
func (fa *FineTunedAdapterInjector) InjectFineTunedAdapter(pod *v1.Pod) error {
	if fineTunedWeightName, ok := pod.ObjectMeta.Annotations[constants.FineTunedAdapterInjectionKey]; ok && len(fineTunedWeightName) > 0 {
		// set the fine-tuned weight name
		fa.fineTunedWeightName = fineTunedWeightName
		return fa.injectFineTunedAdapter(pod)
	}
	return nil
}

// injectFineTunedAdapter adds a special Model Init container and its configurations for downloading and setting up the fine-tuned weight.
func (fa *FineTunedAdapterInjector) injectFineTunedAdapter(pod *v1.Pod) error {
	if fa.containerExists(pod) {
		return nil
	}

	// general validation
	if err := fa.validate(); err != nil {
		return err
	}

	// validate specially for auth type
	if err := fa.validateAuth(pod); err != nil {
		return err
	}

	modelInitMounts := fa.getVolumeMounts(pod)

	fineTunedWeightUri, _ := fa.getFineTunedWeightUri()

	initEnvs, err := fa.getModelInitEnvs(pod, fineTunedWeightUri)
	if err != nil {
		return err
	}

	securityContext, err := fa.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	initContainer := fa.createInitContainer(initEnvs, modelInitMounts, securityContext)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)
	return nil
}

// getVolumeMounts defines and returns volume mounts for the Model Init container.
func (fa *FineTunedAdapterInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	fineTunedWeightVolumeMount := v1.VolumeMount{
		Name:      constants.EmptyDirVolumeSourceName,
		MountPath: constants.InitContainerModelFinalDefaultPath,
		ReadOnly:  false,
	}
	fineTunedWeightDownloadMount := v1.VolumeMount{
		Name:      constants.EmptyDirVolumeSourceName,
		MountPath: constants.FineTunedModelDownloadDefaultMountPath,
		ReadOnly:  false,
		SubPath:   constants.FineTunedModelDownloadDefaultSubPath,
	}

	mounts = append(mounts, fineTunedWeightDownloadMount)
	mounts = append(mounts, fineTunedWeightVolumeMount)
	return mounts
}

// createInitContainer constructs the init container configuration.
func (fa *FineTunedAdapterInjector) createInitContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	return &v1.Container{
		Name:                     constants.ModelInitContainerName,
		Image:                    fa.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"fine-tuned-adapter", "--config", "/ome-agent.yaml", "--debug"},
		Resources: v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(fa.CpuLimit),
				v1.ResourceMemory: resource.MustParse(fa.MemoryLimit),
			},
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(fa.CpuRequest),
				v1.ResourceMemory: resource.MustParse(fa.MemoryRequest),
			},
		},
		SecurityContext: securityContext,
	}
}

// getFineTunedWeightUri retrieves the fine-tuned weight uri from the fine-tuned weight CR
func (fa *FineTunedAdapterInjector) getFineTunedWeightUri() (*casper.ObjectURI, error) {
	fineTunedWeight, err := isvcutils.GetFineTunedWeight(fa.client, fa.fineTunedWeightName)
	if err != nil {
		return nil, err
	}

	storageUri := fineTunedWeight.Spec.Storage.StorageUri
	osUri, err := casperutils.NewObjectStorageUri(*storageUri)
	if err != nil {
		return nil, err
	}

	return osUri, nil
}

// getModelInitEnvs generates environment variables for the Model Init container.
func (fa *FineTunedAdapterInjector) getModelInitEnvs(pod *v1.Pod, fineTunedWeightUri *casper.ObjectURI) ([]v1.EnvVar, error) {
	envVars := []v1.EnvVar{
		{Name: constants.AgentAuthTypeEnvVarKey, Value: fa.AuthType},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: fa.CompartmentId},
		{Name: constants.AgentRegionEnvVarKey, Value: fa.Region},
		{Name: constants.AgentUnzippedFineTunedWeightDirectory, Value: constants.InitContainerModelFinalDefaultPath},
		{Name: constants.AgentZippedFineTunedWeightDirectory, Value: constants.FineTunedModelDownloadDefaultMountPath},
		{Name: constants.AgentModelBucketNameEnvVarKey, Value: fineTunedWeightUri.BucketName},
		{Name: constants.AgentModelNamespaceEnvVarKey, Value: fineTunedWeightUri.Namespace},
		{Name: constants.AgentModelObjectName, Value: fineTunedWeightUri.ObjectName},
	}

	return envVars, nil
}

// containerExists checks if the Model Init container is already in the pod.
func (fa *FineTunedAdapterInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == constants.ModelInitContainerName {
			return true
		}
	}
	return false
}

// validateAuth checks if the correct authentication type is set for the Model Init container.
func (fa *FineTunedAdapterInjector) validateAuth(pod *v1.Pod) error {
	if fa.AuthType == constants.AuthtypeOKEWorkloadIdentity && len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}

	if fa.AuthType == constants.AuthtypeOKEWorkloadIdentity {
		automount := true
		pod.Spec.AutomountServiceAccountToken = &automount
	}
	return nil
}

func (fa *FineTunedAdapterInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(fa); err != nil {
		return fmt.Errorf("failed to validate FineTunedAdapterInjector: %w", err)
	}
	return nil
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (fa *FineTunedAdapterInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.MainContainerName)
}
