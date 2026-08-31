package pod

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	isvcutils "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

const (
	fineTunedAdapterConfigMapKeyName = "fineTunedAdapter"
)

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

func newFineTunedAdapterInjector(configMap *v1.ConfigMap, client client.Client) (*FineTunedAdapterInjector, error) {
	fineTunedAdapterInjector := &FineTunedAdapterInjector{}
	if fineTunedAdapterConfigVal, ok := configMap.Data[fineTunedAdapterConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(fineTunedAdapterConfigVal), fineTunedAdapterInjector); err != nil {
			return nil, fmt.Errorf("unable to unmarshal %v json string: %w", fineTunedAdapterConfigMapKeyName, err)
		}
	}
	fineTunedAdapterInjector.client = client
	return fineTunedAdapterInjector, nil
}

// InjectFineTunedAdapter injects the fine-tuned weight init container
// into the pod when the ome.io/fine-tuned-adapter-injection annotation
// names a FineTunedWeight to download.
func (fa *FineTunedAdapterInjector) InjectFineTunedAdapter(pod *v1.Pod) error {
	if fineTunedWeightName, ok := pod.ObjectMeta.Annotations[constants.FineTunedAdapterInjectionKey]; ok && len(fineTunedWeightName) > 0 {
		fa.fineTunedWeightName = fineTunedWeightName
		return fa.injectFineTunedAdapter(pod)
	}
	return nil
}

func (fa *FineTunedAdapterInjector) injectFineTunedAdapter(pod *v1.Pod) error {
	if fa.containerExists(pod) {
		return nil
	}
	if err := fa.validate(); err != nil {
		return err
	}
	if err := fa.validateAuth(pod); err != nil {
		return err
	}

	fineTunedWeightUri, err := fa.getFineTunedWeightUri(pod)
	if err != nil {
		return fmt.Errorf("failed to resolve fine-tuned weight URI for %s: %w", fa.fineTunedWeightName, err)
	}
	initEnvs, err := fa.getModelInitEnvs(pod, fineTunedWeightUri)
	if err != nil {
		return err
	}
	securityContext, err := fa.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	mounts := fa.getVolumeMounts(pod)
	initContainer, err := fa.createInitContainer(initEnvs, mounts, securityContext)
	if err != nil {
		return err
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)
	return nil
}

func (fa *FineTunedAdapterInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	return []v1.VolumeMount{
		{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: constants.FineTunedWeightDownloadMountPath,
		},
		{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: fa.getFineTunedWeightVolumeMountPath(pod),
			SubPath:   fa.getFineTunedWeightVolumeMountSubPath(pod),
		},
	}
}

func (fa *FineTunedAdapterInjector) createInitContainer(envs []v1.EnvVar, mounts []v1.VolumeMount, securityContext *v1.SecurityContext) (*v1.Container, error) {
	resources, err := newResourceRequirements(fa.CpuLimit, fa.MemoryLimit, fa.CpuRequest, fa.MemoryRequest)
	if err != nil {
		return nil, fmt.Errorf("fine-tuned adapter injector: %w", err)
	}
	return &v1.Container{
		Name:                     constants.FineTunedAdapterContainerName,
		Image:                    fa.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      envs,
		VolumeMounts:             mounts,
		Args:                     []string{"fine-tuned-adapter", "--config", "/ome-agent.yaml", "--debug"},
		Resources:                resources,
		SecurityContext:          securityContext,
	}, nil
}

func (fa *FineTunedAdapterInjector) getFineTunedWeightUri(pod *v1.Pod) (*storage.OCIStorageComponents, error) {
	fineTunedWeight, err := isvcutils.GetFineTunedWeight(fa.client, fa.fineTunedWeightName)
	if err != nil {
		return nil, err
	}
	if fineTunedWeight.Spec.Storage == nil || fineTunedWeight.Spec.Storage.StorageUri == nil {
		return nil, fmt.Errorf("fine-tuned weight %s has no storage URI", fa.fineTunedWeightName)
	}
	osUri, err := storage.ParseOCIStorageURI(*fineTunedWeight.Spec.Storage.StorageUri)
	if err != nil {
		return nil, err
	}
	// Merged-weight serving downloads a single archive instead of the
	// adapter layout; suffix the object name so the agent fetches it.
	if pod.ObjectMeta.Annotations[constants.FTServingWithMergedWeightsAnnotationKey] == "true" {
		osUri.Prefix = fmt.Sprintf("%s%s", osUri.Prefix, constants.MergedModelWeightZippedFileSuffix)
		osUri.ObjectName = fmt.Sprintf("%s%s", osUri.ObjectName, constants.MergedModelWeightZippedFileSuffix)
	}
	return osUri, nil
}

func (fa *FineTunedAdapterInjector) getModelInitEnvs(pod *v1.Pod, fineTunedWeightUri *storage.OCIStorageComponents) ([]v1.EnvVar, error) {
	return []v1.EnvVar{
		{Name: constants.AgentAuthTypeEnvVarKey, Value: fa.AuthType},
		{Name: constants.AgentCompartmentIDEnvVarKey, Value: fa.CompartmentId},
		{Name: constants.AgentRegionEnvVarKey, Value: fa.Region},
		{Name: constants.AgentUnzippedFineTunedWeightDirectory, Value: fa.getFineTunedWeightVolumeMountPath(pod)},
		{Name: constants.AgentZippedFineTunedWeightDirectory, Value: constants.FineTunedWeightDownloadMountPath},
		{Name: constants.AgentModelBucketNameEnvVarKey, Value: fineTunedWeightUri.Bucket},
		{Name: constants.AgentModelNamespaceEnvVarKey, Value: fineTunedWeightUri.Namespace},
		{Name: constants.AgentModelObjectName, Value: fineTunedWeightUri.Prefix},
	}, nil
}

func (fa *FineTunedAdapterInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == constants.FineTunedAdapterContainerName {
			return true
		}
	}
	return false
}

// validateAuth enforces that OKEWorkloadIdentity carries a service
// account and turns on token automounting (the runtime uses the SA token
// to obtain workload-identity credentials).
func (fa *FineTunedAdapterInjector) validateAuth(pod *v1.Pod) error {
	if fa.AuthType != constants.AuthtypeOKEWorkloadIdentity {
		return nil
	}
	if len(pod.Spec.ServiceAccountName) == 0 {
		return fmt.Errorf("a service account should be specified when using OKEWorkloadIdentity")
	}
	automount := true
	pod.Spec.AutomountServiceAccountToken = &automount
	return nil
}

func (fa *FineTunedAdapterInjector) validate() error {
	if err := validator.New().Struct(fa); err != nil {
		return fmt.Errorf("failed to validate FineTunedAdapterInjector: %w", err)
	}
	return nil
}

func (fa *FineTunedAdapterInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.MainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.MainContainerName)
}

func (fa *FineTunedAdapterInjector) getFineTunedWeightVolumeMountPath(pod *v1.Pod) string {
	if isvcutils.IsCohereCommand1TFewFTServing(&pod.ObjectMeta) {
		return constants.CohereTFewFineTunedWeightVolumeMountPath
	} else {
		return constants.ModelDefaultMountPath
	}
}

func (fa *FineTunedAdapterInjector) getFineTunedWeightVolumeMountSubPath(pod *v1.Pod) string {
	if pod.ObjectMeta.Annotations[constants.BaseModelFormat] == constants.TensorRTLLM {
		return constants.TensorRTModelVolumeMountSubPath
	}
	if isvcutils.IsCohereCommand1TFewFTServing(&pod.ObjectMeta) {
		return constants.FineTunedWeightVolumeMountSubPath
	}
	return ""
}
