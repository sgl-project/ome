package pod

import (
	"encoding/json"
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	ServingInitConfigMapKeyName = "servingInit"
)

type ServingInitInjector struct {
	Image                 string `json:"image"`
	MemoryRequest         string `json:"memoryRequest"`
	MemoryLimit           string `json:"memoryLimit"`
	CpuRequest            string `json:"cpuRequest"`
	CpuLimit              string `json:"cpuLimit"`
	CompartmentId         string `json:"compartmentId"`
	AuthType              string `json:"authType"`
	VaultId               string `json:"vaultId"`
	Region                string `json:"region"`
	KmsCryptoEndpoint     string `json:"kmsCryptoEndpoint"`
	KmsManagementEndpoint string `json:"kmsManagementEndpoint"`
}

func newServingInitInjector(configMap *v1.ConfigMap) *ServingInitInjector {
	ci := &ServingInitInjector{}

	if ciConfigVal, ok := configMap.Data[ServingInitConfigMapKeyName]; ok {
		err := json.Unmarshal([]byte(ciConfigVal), &ci)
		if err != nil {
			panic(fmt.Errorf("Unable to unmarshall %v json string due to %w ", ServingInitConfigMapKeyName, err))
		}
	}

	return ci
}

func (si *ServingInitInjector) InjectServingInit(pod *v1.Pod) error {
	enableServingInit, ok := pod.ObjectMeta.Annotations[constants.SevingInitInjectionKey]
	if !ok {
		return nil
	}

	if enableServingInit == "true" {
		err := si.injectServingInit(pod)
		if err != nil {
			return err
		}
	}
	return nil
}

func (si *ServingInitInjector) injectServingInit(pod *v1.Pod) error {
	for _, container := range pod.Spec.InitContainers {
		if strings.Compare(container.Name, constants.ServingInitContainerName) == 0 {
			return nil
		}
	}

	if si.AuthType == constants.AuthtypeOKEWorkloadIdentity {
		if len(pod.Spec.ServiceAccountName) == 0 {
			return fmt.Errorf("a service account should be specified when serving-init is authenticated with OKEWorkloadIdentity")
		}
		automountServiceAccountToken := true
		pod.Spec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	servingInitMounts := []v1.VolumeMount{}

	// Empty dir Volume mount for init container
	initContainerVolumeMount := v1.VolumeMount{
		Name:      constants.EmptyDirVolumeSourceName,
		MountPath: constants.InitContainerModelFinalDefaultPath,
		ReadOnly:  false,
	}

	servingInitMounts = append(servingInitMounts, initContainerVolumeMount)

	baseModelName := pod.ObjectMeta.Annotations[constants.BaseModelName]
	modelMountPath := constants.InitContainerModelSourceDefaultPath 
	modelPvcVolumeMount := v1.VolumeMount{
		Name:      baseModelName,
		MountPath: modelMountPath,
		ReadOnly:  false,
	}

	servingInitMounts = append(servingInitMounts, modelPvcVolumeMount)

	initEnvs, err := si.getServingInitEnvs(pod)
	if err != nil {
		return err
	}

	var omeServingContainer *v1.Container
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.InferenceServiceContainerName {
			omeServingContainer = &container
		}
	}

	if omeServingContainer == nil {
		return fmt.Errorf("no ome main serving container specified")
	}

	securityContext := omeServingContainer.SecurityContext.DeepCopy()
	initContainer := &v1.Container{
		Name:                     constants.ServingInitContainerName,
		Image:                    si.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      initEnvs,
		VolumeMounts:             servingInitMounts,
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

	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)

	return nil
}

func (si *ServingInitInjector) getServingInitEnvs(pod *v1.Pod) ([]v1.EnvVar, error) {
	servingInitEnvVars := make([]v1.EnvVar, 0)
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_COMPARTMENT_ID",
		Value: si.CompartmentId,
	})
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_AUTH_TYPE",
		Value: si.AuthType,
	})
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_REGION",
		Value: si.Region,
	})

	servingRuntime := pod.ObjectMeta.Annotations[constants.ServingRuntimeKeyName]
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_SERVING_RUNTIME",
		Value: servingRuntime,
	})

	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_DISABLE_MODEL_DECRYPTION",
		Value: "false",
	})

	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_MODEL_STORE_PATH",
		Value: constants.InitContainerModelSourceDefaultPath,
	})
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_LOCAL_STORE_PATH",
		Value: constants.InitContainerModelFinalDefaultPath,
	})

	decryptionKeyName := pod.ObjectMeta.Annotations[constants.BaseModelDecryptionKeyName]
	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_KEY_NAME",
		Value: decryptionKeyName,
	})

	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_VAULT_ID",
		Value: si.VaultId,
	})

	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_KMS_CRYPTO_ENDPOINT",
		Value: si.KmsCryptoEndpoint,
	})

	servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
		Name:  "OME_AGENT_KMS_MANAGEMENT_ENDPOINT",
		Value: si.KmsManagementEndpoint,
	})

	modelFormat := pod.ObjectMeta.Annotations[constants.BaseModelFormat]
	if modelFormat == "tensorrt_llm" {
		servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
			Name:  "OME_AGENT_IS_TENSORRT_MODEL",
			Value: "true",
		})
		modelFormatVersion := pod.ObjectMeta.Annotations[constants.BaseModelFormatVersion]

		servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
			Name:  "OME_AGENT_TENSORRT_LLM_VERSION",
			Value: modelFormatVersion,
		})

		var omeServingContainer *v1.Container
		for _, container := range pod.Spec.Containers {
			if container.Name == constants.InferenceServiceContainerName {
				omeServingContainer = &container
			}
		}

		if omeServingContainer == nil {
			return nil, fmt.Errorf("no ome main serving container specified")
		}

		if omeServingContainer.Resources.Limits == nil {
			return nil, fmt.Errorf("nvidia gpu resource never set for serving container")
		}

		if gpus, exists := omeServingContainer.Resources.Limits[constants.NvidiaGPUResourceType]; exists {
			servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
				Name:  "OME_AGENT_TENSORRT_NUM_OF_GPU",
				Value: gpus.String(),
			})
		} else {
			return nil, fmt.Errorf("nvidia gpu resource never set for serving container")
		}

		servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
			Name: "OME_AGENT_NODE_INSTANCE_TYPE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.labels['node.kubernetes.io/instance-type']",
				},
			},
		})
	} else {
		servingInitEnvVars = append(servingInitEnvVars, v1.EnvVar{
			Name:  "OME_AGENT_IS_TENSORRT_MODEL",
			Value: "false",
		})
	}

	return servingInitEnvVars, nil
}
