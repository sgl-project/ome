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
	mergedFinetunedAdapterConfigMapKeyName = "mergedFinetunedAdapter"
)

// MergedFinetunedAdapterInjector represents configuration parameters for the Merged Finetuned Adapter.
type MergedFinetunedAdapterInjector struct {
	Image                     string `json:"image" validate:"required"`
	MemoryRequest             string `json:"memoryRequest"`
	MemoryLimit               string `json:"memoryLimit"`
	CpuRequest                string `json:"cpuRequest"`
	CpuLimit                  string `json:"cpuLimit"`
	CompartmentId             string `json:"compartmentId" validate:"required"`
	AuthType                  string `json:"authType" validate:"required"`
	Region                    string `json:"region"`
	namespace                 string
	mergedFinetunedWeightName string
	client                    client.Client
}

// newMergedFinetunedAdapterInjector initializes a MergedFinetunedAdapterInjector from a ConfigMap.
func newMergedFinetunedAdapterInjector(configMap *v1.ConfigMap, client client.Client, namespace string) *MergedFinetunedAdapterInjector {
	mergedFinetunedAdapterInjector := &MergedFinetunedAdapterInjector{}
	if mergedFinetunedAdapterConfigVal, ok := configMap.Data[mergedFinetunedAdapterConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(mergedFinetunedAdapterConfigVal), mergedFinetunedAdapterInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", mergedFinetunedAdapterConfigMapKeyName, err))
		}
	}
	mergedFinetunedAdapterInjector.namespace = namespace
	mergedFinetunedAdapterInjector.client = client
	return mergedFinetunedAdapterInjector
}

// InjectMergedFinetunedAdapter injects the merged finetuned weights initialization container into the pod if necessary.
func (fa *MergedFinetunedAdapterInjector) InjectMergedFinetunedAdapter(pod *v1.Pod) error {
	if mergedFinetunedWeightName, ok := pod.ObjectMeta.Annotations[constants.MergedFinetunedWeightsInjectionKey]; ok && len(mergedFinetunedWeightName) > 0 {
		// set the finetuned weight name
		fa.mergedFinetunedWeightName = mergedFinetunedWeightName
		return fa.injectMergedFinetunedAdapter(pod)
	}
	return nil
}

// injectMergedFinetunedAdapter adds a special Model Init container and its configurations for downloading and setting up the merged finetuned weight.
func (fa *MergedFinetunedAdapterInjector) injectMergedFinetunedAdapter(pod *v1.Pod) error {
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

	finetunedWeightUri, err := fa.getFinetunedWeightUri()

	initEnvs, err := fa.getModelInitEnvs(pod, finetunedWeightUri)
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
func (fa *MergedFinetunedAdapterInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	finetunedModelsVolumeMount := v1.VolumeMount{
		Name:      constants.EmptyDirVolumeSourceName,
		MountPath: constants.InitContainerModelFinalDefaultPath,
		ReadOnly:  false,
	}
	finetunedDownloadMount := v1.VolumeMount{
		Name:      constants.EmptyDirVolumeSourceName,
		MountPath: constants.FineTunedModelDownloadDefaultMountPath,
		ReadOnly:  false,
		SubPath:   constants.FineTunedModelDownloadDefaultSubPath,
	}

	mounts = append(mounts, finetunedDownloadMount)
	mounts = append(mounts, finetunedModelsVolumeMount)
	return mounts
}

// createInitContainer constructs the init container configuration.
func (fa *MergedFinetunedAdapterInjector) createInitContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	return &v1.Container{
		Name:                     constants.ModelInitContainerName,
		Image:                    fa.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"merged-finetuned-adapter", "--config", "/ome-agent.yaml", "--debug"},
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

// getFinetunedWeightUri retrieves the finetuned weight uri from the finetuned weights.
func (fa *MergedFinetunedAdapterInjector) getFinetunedWeightUri() (*casper.ObjectURI, error) {
	finetunedWeight, err := isvcutils.GetFinetunedWeight(fa.client, fa.mergedFinetunedWeightName, fa.namespace)
	if err != nil {
		return nil, err
	}

	storageUri := finetunedWeight.Spec.Storage.StorageUri
	osUri, err := casperutils.NewObjectStorageUri(*storageUri)
	if err != nil {
		return nil, err
	}

	return osUri, nil
}

// getModelInitEnvs generates environment variables for the Model Init container.
func (fa *MergedFinetunedAdapterInjector) getModelInitEnvs(pod *v1.Pod, finetunedWeightUri *casper.ObjectURI) ([]v1.EnvVar, error) {
	envVars := []v1.EnvVar{
		{Name: constants.AgentAuthTypeEnvVarKey, Value: fa.AuthType},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: fa.CompartmentId},
		{Name: constants.AgentRegionEnvVarKey, Value: fa.Region},
		{Name: constants.AgentUnzippedFinetunedModelDirectory, Value: constants.InitContainerModelFinalDefaultPath},
		{Name: constants.AgentZippedFinetunedModelDirectory, Value: constants.FineTunedModelDownloadDefaultMountPath},
		{Name: constants.AgentModelBucketNameEnvVarKey, Value: finetunedWeightUri.BucketName},
		{Name: constants.AgentModelNamespaceEnvVarKey, Value: finetunedWeightUri.Namespace},
		{Name: constants.AgentModelObjectName, Value: finetunedWeightUri.ObjectName},
	}

	return envVars, nil
}

// containerExists checks if the Model Init container is already in the pod.
func (fa *MergedFinetunedAdapterInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == constants.ModelInitContainerName {
			return true
		}
	}
	return false
}

// validateAuth checks if the correct authentication type is set for the Model Init container.
func (fa *MergedFinetunedAdapterInjector) validateAuth(pod *v1.Pod) error {
	if fa.AuthType == constants.AuthtypeOKEWorkloadIdentity && len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}

	if fa.AuthType == constants.AuthtypeOKEWorkloadIdentity {
		automount := true
		pod.Spec.AutomountServiceAccountToken = &automount
	}
	return nil
}

func (fa *MergedFinetunedAdapterInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(fa); err != nil {
		return fmt.Errorf("failed to validate MergedFinetunedAdapterInjector: %w", err)
	}
	return nil
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (fa *MergedFinetunedAdapterInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.MainContainerName)
}
