package peft

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	podconfig "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/pod_configs"
	trainingJobUtils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type PeftLauncherPodConfig struct {
	launcherPodConfig *podconfig.LauncherPodConfig
	client            client.Client
	podSpec           *v1.PodSpec
	trainingContainer *v1.Container
	BaseModelResource *v1beta1.BaseModel
	Hyperparameters   *runtime.RawExtension
}

func NewPeftLauncherPodConfig(launcherPodConfig *podconfig.LauncherPodConfig, client client.Client, podSpec *v1.PodSpec, baseModel *v1beta1.BaseModel, hyperparameters *runtime.RawExtension) error {
	peftPodConfig := &PeftLauncherPodConfig{}
	peftPodConfig.launcherPodConfig = launcherPodConfig
	peftPodConfig.client = client
	peftPodConfig.podSpec = podSpec
	peftPodConfig.BaseModelResource = baseModel
	peftPodConfig.Hyperparameters = hyperparameters

	err := peftPodConfig.configure()

	return err
}

func (plpc *PeftLauncherPodConfig) configure() error {
	genaiContainerIdx := trainingJobUtils.GetContainerIndex(constants.TrainingJobContainerName, plpc.podSpec.Containers)
	if genaiContainerIdx == -1 {
		return fmt.Errorf("invalid configuration: cannot find container: %s", constants.TrainingJobContainerName)
	}
	plpc.trainingContainer = &plpc.podSpec.Containers[genaiContainerIdx]

	if plpc.BaseModelResource == nil {
		return fmt.Errorf("a Base Model must be specified for peft training")
	}

	err := plpc.podConfig()
	if err != nil {
		return err
	}

	// TODO: Switch to use mutating admission webhook for training-init after moving data downloading part to sidecar
	err = plpc.trainingInitConfig()
	if err != nil {
		return err
	}

	fineTunedModelName := trainingJobUtils.GetFineTunedModelName(plpc.launcherPodConfig.TrainingJobName)
	err = plpc.trainingSidecarConfig(fineTunedModelName)
	if err != nil {
		return err
	}

	err = plpc.trainingContainerConfig()
	if err != nil {
		return err
	}
	plpc.podSpec.Containers[genaiContainerIdx] = *plpc.trainingContainer
	return nil
}

func (plpc *PeftLauncherPodConfig) podConfig() error {
	var podVolumes []v1.Volume
	// Add the PVC volume on the pod
	pvcSourceVolume := v1.Volume{
		Name: constants.ModelStorePVCSourceName,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: constants.GetPvcName(plpc.launcherPodConfig.Namespace), // DAC operator will create this PVC under the DAC namespace
			},
		},
	}
	podVolumes = append(podVolumes, pvcSourceVolume)

	// Create EmptyDir volume for data
	emptyDirDataVolume := v1.Volume{
		Name: constants.DataEmptyDirName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
	podVolumes = append(podVolumes, emptyDirDataVolume)

	// Add volumes to the PodSpec
	plpc.podSpec.Volumes = append(plpc.podSpec.Volumes, podVolumes...)

	// Set node affinity from DAC if necessary
	dedicatedAiClusterResource, err := trainingJobUtils.GetDedicatedAIClusterResource(plpc.client, &v1.ObjectReference{
		Name: plpc.launcherPodConfig.Namespace,
	})
	if err == nil && dedicatedAiClusterResource != nil {
		setNodeAffinityIfNecessary(plpc.podSpec, dedicatedAiClusterResource)
	}

	return nil
}

func (plpc *PeftLauncherPodConfig) trainingInitConfig() error {
	// Don't add if training-init InitContainer already added
	for _, container := range plpc.podSpec.InitContainers {
		if strings.Compare(container.Name, constants.TrainingInitContainerName) == 0 {
			return nil
		}
	}

	// Get training-init config
	trainingInitConfig, err := podconfig.GetTrainingInitConfig(plpc.client)
	if err != nil {
		return err
	}

	// Auth token setup if using OkeWorkloadIdentity
	if trainingInitConfig.AuthType == constants.TrainingInitAuthtypeOKEWorkloadIdentity {
		if len(plpc.podSpec.ServiceAccountName) == 0 {
			return fmt.Errorf("a service account should be specified when training-init is authenticated with OKEWorkloadIdentity")
		}
		automountServiceAccountToken := true
		plpc.podSpec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	// Get volume mounts and envs for training-init
	trainingInitMounts := plpc.getPeftTrainingInitVolumeMounts()
	trainingInitEnvs, err := plpc.getPeftTrainingInitEnvs(*trainingInitConfig)
	if err != nil {
		return err
	}

	// Create init container
	initContainer := &v1.Container{
		Name:                     constants.TrainingInitContainerName,
		Image:                    trainingInitConfig.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      *trainingInitEnvs,
		VolumeMounts:             trainingInitMounts,
		Resources: v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(trainingInitConfig.CpuLimit),
				v1.ResourceMemory: resource.MustParse(trainingInitConfig.MemoryLimit),
			},
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(trainingInitConfig.CpuRequest),
				v1.ResourceMemory: resource.MustParse(trainingInitConfig.MemoryRequest),
			},
		},
		SecurityContext: plpc.trainingContainer.SecurityContext.DeepCopy(),
	}

	// Add init container to the spec
	plpc.podSpec.InitContainers = append(plpc.podSpec.InitContainers, *initContainer)
	return nil
}

func (plpc *PeftLauncherPodConfig) trainingContainerConfig() error {
	// Set volume mounts for training container
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: constants.PeftTrainingDataEmptyDirMountPath,
		ReadOnly:  false,
	}
	modelPVCSourceVolumeMount := v1.VolumeMount{
		Name:      constants.ModelStorePVCSourceName,
		MountPath: constants.PeftTrainingModelStorePVCMountPath,
		ReadOnly:  false,
	}
	plpc.trainingContainer.VolumeMounts = append(plpc.trainingContainer.VolumeMounts, dataEmptyDirVolumeMount)
	plpc.trainingContainer.VolumeMounts = append(plpc.trainingContainer.VolumeMounts, modelPVCSourceVolumeMount)

	// Set environment variables for training container
	pathPrefixEnv := v1.EnvVar{
		Name:  constants.PeftTrainingPathPrefixEnvVarKey,
		Value: constants.PeftTrainingDataEmptyDirMountPath,
	}
	baselineModelEnv := v1.EnvVar{
		Name:  constants.PeftTrainingBaselineModelEnvVarKey,
		Value: filepath.Join(constants.PeftTrainingModelStorePVCMountPath, *plpc.BaseModelResource.Spec.Vendor, *plpc.BaseModelResource.Spec.DisplayName),
	}
	plpc.trainingContainer.Env = append(plpc.trainingContainer.Env, pathPrefixEnv)
	plpc.trainingContainer.Env = append(plpc.trainingContainer.Env, baselineModelEnv)

	// Set resources from DAC if possible
	// otherwise resource requirements already in pod spec which we resolved from its runtime spec
	dedicatedAiClusterResource, err := trainingJobUtils.GetDedicatedAIClusterResource(plpc.client, &v1.ObjectReference{
		Name: plpc.launcherPodConfig.Namespace,
	})

	if err == nil && dedicatedAiClusterResource != nil {
		// DAC resource found, set GPU resource limits
		setGpuResourceLimitsIfNecessary(plpc.trainingContainer, dedicatedAiClusterResource)
	}
	return nil
}

func (plpc *PeftLauncherPodConfig) trainingSidecarConfig(fineTunedModelName string) error {
	// Don't add if training-sidecar container already added
	for _, container := range plpc.podSpec.Containers {
		if strings.Compare(container.Name, constants.TrainingSidecarContainerName) == 0 {
			return nil
		}
	}

	// Get training-sidecar config
	trainingSidecarConfig, err := podconfig.GetTrainingSidecarConfig(plpc.client)
	if err != nil {
		return err
	}

	// Get volume mounts and envs for training-sidecar
	trainingSidecarMounts := plpc.getPeftTrainingSidecarVolumeMounts()
	trainingSidecarEnvs, err := plpc.getPeftTrainingSidecarEnvs(*trainingSidecarConfig, fineTunedModelName)
	if err != nil {
		return err
	}

	// Create sidecar container
	sidecarContainer := &v1.Container{
		Name:                     constants.TrainingSidecarContainerName,
		Image:                    trainingSidecarConfig.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      *trainingSidecarEnvs,
		VolumeMounts:             trainingSidecarMounts,
		SecurityContext:          plpc.trainingContainer.SecurityContext.DeepCopy(),
	}

	// Add sidecar container to the spec
	plpc.podSpec.Containers = append(plpc.podSpec.Containers, *sidecarContainer)
	return nil
}

func (plpc *PeftLauncherPodConfig) getPeftTrainingInitVolumeMounts() []v1.VolumeMount {
	var trainingInitMounts []v1.VolumeMount
	// model-storage PVC volume mount for init container
	modelPVCSourceVolumeMount := v1.VolumeMount{
		Name:      constants.ModelStorePVCSourceName,
		MountPath: constants.PeftTrainingModelStorePVCMountPath,
		ReadOnly:  false,
	}
	trainingInitMounts = append(trainingInitMounts, modelPVCSourceVolumeMount)

	// data empty dir volume mount for init container
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: constants.PeftTrainingDataEmptyDirMountPath,
		ReadOnly:  false,
	}

	trainingInitMounts = append(trainingInitMounts, dataEmptyDirVolumeMount)
	return trainingInitMounts
}

func (plpc *PeftLauncherPodConfig) getPeftTrainingInitEnvs(trainingInitConfig podconfig.TrainingInitConfig) (*[]v1.EnvVar, error) {
	trainingInitEnvVars := make([]v1.EnvVar, 0)
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.ModelNameEnvVarKey,
		Value: *plpc.BaseModelResource.Spec.DisplayName,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.ModelStorePathEnvVarKey,
		Value: filepath.Join(constants.PeftTrainingModelStorePVCMountPath, *plpc.BaseModelResource.Spec.Vendor),
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataPathEnvVarKey,
		Value: constants.PeftTrainingDataEmptyDirMountPath,
	}) // Note: need to pass validation data path when it is supported.
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.RegionEnvVarKey,
		Value: trainingInitConfig.Region,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.DisableModelDecryptionEnvVarKey,
		Value: "true",
	})

	trainingInitEnvVars = podconfig.SetDatasetsEnvs(plpc.launcherPodConfig.Datasets, trainingInitConfig, trainingInitEnvVars)

	return &trainingInitEnvVars, nil
}

func (plpc *PeftLauncherPodConfig) getPeftTrainingSidecarEnvs(trainingSidecarConfig podconfig.TrainingSidecarConfig, fineTunedModelName string) (*[]v1.EnvVar, error) {
	trainingSidecarEnvVars := make([]v1.EnvVar, 0)

	// Set env vars from values set in trainingSidecar config map
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.RegionEnvVarKey,
		Value: trainingSidecarConfig.Region,
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingMetricsBucketEnvVarKey,
		Value: trainingSidecarConfig.TrainingMetricsBucket,
	})
	trainingSidecarEnvVars = podconfig.SetModelStorageEnvs(plpc.launcherPodConfig.ModelStorage, trainingSidecarConfig, trainingSidecarEnvVars)

	// Set env var from values given in hyperparameters
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.BatchSizeEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.BatchSizeConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EarlyStoppingPatienceEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.EarlyStoppingPatienceConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EarlyStoppingThresholdEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.EarlyStoppingThresholdConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EpochsEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.EpochsConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LearningRateEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.LearningRateConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LogMetricsIntervalInStepsEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.LogTrainStatusEveryStepConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LoraREnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.LoraRConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LoraAlphaEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.LoraAlphaConfigKey),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LoraDropoutEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(plpc.Hyperparameters, constants.LoraDropoutConfigKey),
	})

	// Other env variables
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.RuntimeEnvVarKey,
		Value: string(constants.PeftTrainingSidecar),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainNameEnvVarKey,
		Value: fineTunedModelName,
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ModelDirectoryEnvVarKey,
		Value: filepath.Join(constants.PeftTrainingDataEmptyDirMountPath, constants.PeftTrainingOutputModelDirectoryName),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ZippedModelPathEnvVarKey,
		Value: filepath.Join(constants.PeftTrainingDataEmptyDirMountPath, constants.PeftTrainingOutputModelDirectoryName, fineTunedModelName),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ZippedMergedModelPathEnvVarKey,
		Value: filepath.Join(constants.PeftTrainingDataEmptyDirMountPath, constants.PeftTrainingOutputModelDirectoryName, fineTunedModelName+constants.PeftTrainingMergedModelWeightSuffix),
	})

	objectName := trainingJobUtils.ExtractPureObjectName(plpc.launcherPodConfig.Datasets[constants.Training].OSStorageSpec.ObjectName)
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataFileNameEnvVarKey,
		Value: objectName,
	}) // Note: Pass validation data file name to sidecar as well when it is supported.

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ModelNameEnvVarKey,
		Value: *plpc.BaseModelResource.Spec.DisplayName,
	})

	return &trainingSidecarEnvVars, nil
}

func (plpc *PeftLauncherPodConfig) getPeftTrainingSidecarVolumeMounts() []v1.VolumeMount {
	var trainingSidecarMounts []v1.VolumeMount
	// data empty dir volume mount for sidecar container
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: constants.PeftTrainingDataEmptyDirMountPath,
		ReadOnly:  false,
	}
	trainingSidecarMounts = append(trainingSidecarMounts, dataEmptyDirVolumeMount)
	return trainingSidecarMounts
}

func setNodeAffinityIfNecessary(podSpec *v1.PodSpec, dedicatedAiClusterResource *v1beta1.DedicatedAICluster) {
	if dedicatedAiClusterResource != nil {
		if dedicatedAiClusterResource.Spec.Affinity != nil {
			podSpec.Affinity = dedicatedAiClusterResource.Spec.Affinity.DeepCopy()
		}
	}
}

func setGpuResourceLimitsIfNecessary(trainingContainer *v1.Container, dedicatedAiClusterResource *v1beta1.DedicatedAICluster) {
	if dedicatedAiClusterResource == nil {
		return
	}

	if dedicatedAiClusterResource.Spec.Resources.Limits == nil {
		return
	}

	if gpus, exists := dedicatedAiClusterResource.Spec.Resources.Limits[constants.NvidiaGPUResourceType]; exists {
		trainingContainer.Resources.Limits[constants.NvidiaGPUResourceType] = gpus.DeepCopy()
		trainingContainer.Resources.Requests[constants.NvidiaGPUResourceType] = gpus.DeepCopy()
	}
}
