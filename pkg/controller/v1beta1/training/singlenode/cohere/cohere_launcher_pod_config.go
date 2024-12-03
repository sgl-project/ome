package cohere

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	podconfig "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/singlenode"
	trainingJobUtils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

type CohereLauncherPodConfig struct {
	launcherPodConfig *podconfig.LauncherPodConfig
	client            client.Client
	podSpec           *v1.PodSpec
	trainingContainer *v1.Container
	BaseModelResource *v1beta1.BaseModel
	Hyperparameters   *runtime.RawExtension
}

func NewCohereLauncherPodConfig(podConfig *podconfig.LauncherPodConfig, client client.Client,
	podSpec *v1.PodSpec, baseModel *v1beta1.BaseModel, hyperparameters *runtime.RawExtension) error {
	coherePodConfig := &CohereLauncherPodConfig{}
	coherePodConfig.launcherPodConfig = podConfig
	coherePodConfig.client = client
	coherePodConfig.podSpec = podSpec
	coherePodConfig.BaseModelResource = baseModel
	coherePodConfig.Hyperparameters = hyperparameters

	err := coherePodConfig.configure()

	return err
}

func (clpc *CohereLauncherPodConfig) configure() error {
	genaiContainerIdx := trainingJobUtils.GetContainerIndex(constants.TrainingJobContainerName, clpc.podSpec.Containers)
	if genaiContainerIdx == -1 {
		return fmt.Errorf("invalid configuration: cannot find container: %s", constants.TrainingJobContainerName)
	}
	clpc.trainingContainer = &clpc.podSpec.Containers[genaiContainerIdx]

	if clpc.BaseModelResource == nil {
		return fmt.Errorf("a Base Model must be specified for peft training")
	}

	err := clpc.podConfig()
	if err != nil {
		return err
	}

	// TODO: Switch to use mutating admission webhook for training-init after moving data downloading part to sidecar
	err = clpc.trainingInitConfig()
	if err != nil {
		return err
	}

	fineTunedModelName := trainingJobUtils.GetFineTunedModelName(clpc.launcherPodConfig.TrainingJobName)
	err = clpc.trainingSidecarConfig(fineTunedModelName)
	if err != nil {
		return err
	}

	err = clpc.trainingContainerConfig(fineTunedModelName)
	if err != nil {
		return err
	}
	clpc.podSpec.Containers[genaiContainerIdx] = *clpc.trainingContainer
	return nil
}

func (clpc *CohereLauncherPodConfig) podConfig() error {
	var podVolumes []v1.Volume
	// Add the PVC volume on the pod
	pvcSourceVolume := v1.Volume{
		Name: constants.ModelStorePVCSourceName,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: constants.GetPvcName(clpc.launcherPodConfig.Namespace), // DAC operator will create this PVC under the DAC namespace
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
	clpc.podSpec.Volumes = append(clpc.podSpec.Volumes, podVolumes...)

	// Set node affinity from DAC if necessary
	dedicatedAiClusterResource, err := trainingJobUtils.GetDedicatedAIClusterResource(clpc.client, &v1.ObjectReference{
		Name: clpc.launcherPodConfig.Namespace,
	})
	if err == nil && dedicatedAiClusterResource != nil {
		podconfig.SetNodeAffinityIfNecessary(clpc.podSpec, dedicatedAiClusterResource)
	}

	return nil
}

func (clpc *CohereLauncherPodConfig) trainingInitConfig() error {
	// Don't add if training-init InitContainer already added
	for _, container := range clpc.podSpec.InitContainers {
		if strings.Compare(container.Name, constants.TrainingInitContainerName) == 0 {
			return nil
		}
	}
	trainingInitConfig, err := podconfig.GetTrainingInitConfig(clpc.client)
	if err != nil {
		return err
	}

	if trainingInitConfig.AuthType == constants.TrainingInitAuthtypeOKEWorkloadIdentity {
		if len(clpc.podSpec.ServiceAccountName) == 0 {
			return fmt.Errorf("a service account should be specified when training-init is authenticated with OKEWorkloadIdentity")
		}
		automountServiceAccountToken := true
		clpc.podSpec.AutomountServiceAccountToken = &automountServiceAccountToken
	}

	trainingInitMounts := clpc.getCohereTrainingInitVolumeMounts()
	trainingInitEnvs, err := clpc.getCohereTrainingInitEnvs(*trainingInitConfig)
	if err != nil {
		return err
	}

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
		SecurityContext: clpc.trainingContainer.SecurityContext.DeepCopy(),
	}

	// Add init container to the spec
	clpc.podSpec.InitContainers = append(clpc.podSpec.InitContainers, *initContainer)
	return nil
}

func (clpc *CohereLauncherPodConfig) trainingContainerConfig(fineTunedModelName string) error {
	// Set volume mounts for training container
	modelEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.ModelEmptyDirName,
		MountPath: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName),
		ReadOnly:  false,
	}
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, "input"),
		ReadOnly:  false,
	}
	clpc.trainingContainer.VolumeMounts = append(clpc.trainingContainer.VolumeMounts, modelEmptyDirVolumeMount)
	clpc.trainingContainer.VolumeMounts = append(clpc.trainingContainer.VolumeMounts, dataEmptyDirVolumeMount)

	// Set environment variables for training container
	pathPrefixEnv := v1.EnvVar{
		Name:  constants.CohereTrainingPathPrefixEnvVarKey,
		Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName),
	}
	baselineModelEnv := v1.EnvVar{
		Name:  constants.CohereTrainingBaselineModelEnvVarKey,
		Value: clpc.getBaselineModelEnvValue(fineTunedModelName),
	}
	clpc.trainingContainer.Env = append(clpc.trainingContainer.Env, pathPrefixEnv)
	clpc.trainingContainer.Env = append(clpc.trainingContainer.Env, baselineModelEnv)

	// Set resources from DAC if possible, otherwise hardcoded
	dedicatedAiClusterResource, err := trainingJobUtils.GetDedicatedAIClusterResource(clpc.client, &v1.ObjectReference{
		Name: clpc.launcherPodConfig.Namespace,
	})

	if err == nil && dedicatedAiClusterResource != nil {
		// DAC resource found, set GPU resource limits
		podconfig.SetGpuResourceLimitsIfNecessary(clpc.trainingContainer, dedicatedAiClusterResource)
	} else {
		// Fallback to existing implementation for setting GPU resources
		// Additional GPU resources setup required for large(52b) Cohere FT, since it is sharing the same runtime as small(6b) Cohere FT
		if v1beta1.IsCohereFTRuntime(clpc.launcherPodConfig.LauncherRuntimeName) && *clpc.BaseModelResource.Spec.ModelParameterSize == "52b" {
			clpc.trainingContainer.Resources.Requests[constants.NvidiaGPUResourceType] = resource.MustParse(constants.CohereTrainingLargeGpuRequest)
			clpc.trainingContainer.Resources.Limits[constants.NvidiaGPUResourceType] = resource.MustParse(constants.CohereTrainingLargeGpuRequest)
		}
	}
	return nil
}

func (clpc *CohereLauncherPodConfig) getCohereTrainingInitEnvs(trainingInitConfig podconfig.TrainingInitConfig) (*[]v1.EnvVar, error) {
	trainingInitEnvVars := make([]v1.EnvVar, 0)
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.ModelNameEnvVarKey,
		Value: *clpc.BaseModelResource.Spec.DisplayName,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.KeyNameEnvVarKey,
		Value: *clpc.BaseModelResource.Spec.Storage.StorageKey,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.ModelStorePathEnvVarKey,
		Value: filepath.Join(constants.CohereTrainingInitModelStorePVCMountPath, *clpc.BaseModelResource.Spec.Vendor),
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.CompartmentIdEnvVarKey,
		Value: trainingInitConfig.CompartmentId,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.VaultIdEnvVarKey,
		Value: trainingInitConfig.VaultId,
	})
	trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
		Name:  constants.RegionEnvVarKey,
		Value: trainingInitConfig.Region,
	})

	if v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName) {
		trainingInitEnvVars = append(trainingInitEnvVars, v1.EnvVar{
			Name:  constants.ModelServingPathEnvVarKey,
			Value: constants.CohereTrainingInitModelEmptyDirMountPathTensorRT,
		})
	}
	trainingInitEnvVars = podconfig.SetDatasetsEnvs(clpc.launcherPodConfig.Datasets, trainingInitConfig, trainingInitEnvVars)

	return &trainingInitEnvVars, nil
}

func (clpc *CohereLauncherPodConfig) getCohereTrainingInitVolumeMounts() []v1.VolumeMount {
	var trainingInitMounts []v1.VolumeMount
	// model-storage PVC source volume mount for init container
	modelPVCSourceVolumeMount := v1.VolumeMount{
		Name:      constants.ModelStorePVCSourceName,
		MountPath: constants.CohereTrainingInitModelStorePVCMountPath,
		ReadOnly:  false,
	}
	trainingInitMounts = append(trainingInitMounts, modelPVCSourceVolumeMount)

	// model empty dir volume mount for init container
	modelEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.ModelEmptyDirName,
		MountPath: clpc.getModelEmptyDirMountPath(),
		ReadOnly:  false,
	}
	trainingInitMounts = append(trainingInitMounts, modelEmptyDirVolumeMount)

	// data empty dir volume mount for init container
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: constants.CohereTrainingInitDataEmptyDirMountPath,
		ReadOnly:  false,
	}
	trainingInitMounts = append(trainingInitMounts, dataEmptyDirVolumeMount)

	return trainingInitMounts
}

func (clpc *CohereLauncherPodConfig) trainingSidecarConfig(fineTunedModelName string) error {
	// Don't add if training-sidecar container already added
	for _, container := range clpc.podSpec.Containers {
		if strings.Compare(container.Name, constants.TrainingSidecarContainerName) == 0 {
			return nil
		}
	}
	trainingSidecarConfig, err := podconfig.GetTrainingSidecarConfig(clpc.client)
	if err != nil {
		return err
	}

	trainingSidecarMounts := clpc.getCohereTrainingSidecarVolumeMounts(fineTunedModelName)
	trainingSidecarEnvs, err := clpc.getCohereTrainingSidecarEnvs(*trainingSidecarConfig, fineTunedModelName)
	if err != nil {
		return err
	}

	sidecarContainer := &v1.Container{
		Name:                     constants.TrainingSidecarContainerName,
		Image:                    trainingSidecarConfig.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      *trainingSidecarEnvs,
		VolumeMounts:             trainingSidecarMounts,
		SecurityContext:          clpc.trainingContainer.SecurityContext.DeepCopy(),
	}

	// Add sidecar container to the spec
	clpc.podSpec.Containers = append(clpc.podSpec.Containers, *sidecarContainer)
	return nil
}

func (clpc *CohereLauncherPodConfig) getCohereTrainingSidecarVolumeMounts(fineTunedModelName string) []v1.VolumeMount {
	var trainingSidecarMounts []v1.VolumeMount
	// model empty dir volume mount for sidecar container
	modelEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.ModelEmptyDirName,
		MountPath: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName),
		ReadOnly:  false,
	}
	trainingSidecarMounts = append(trainingSidecarMounts, modelEmptyDirVolumeMount)
	return trainingSidecarMounts
}

func (clpc *CohereLauncherPodConfig) getCohereTrainingSidecarEnvs(trainingSidecarConfig podconfig.TrainingSidecarConfig, fineTunedModelName string) (*[]v1.EnvVar, error) {
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

	trainingSidecarEnvVars = podconfig.SetModelStorageEnvs(clpc.launcherPodConfig.ModelStorage, trainingSidecarConfig, trainingSidecarEnvVars)

	// Set env var from values given in hyperparameters
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.BatchSizeEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.BatchSizeConfigKey),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EpochsEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.EpochsConfigKey),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.LearningRateEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.LearningRateConfigKey),
	})
	strategy := v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.TrainingConfigTypeConfigKey)
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.StrategyEnvVarKey,
		Value: strategy,
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EarlyStoppingPatienceEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.EarlyStoppingPatienceConfigKey),
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.EarlyStoppingThresholdEnvVarKey,
		Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.EarlyStoppingThresholdConfigKey),
	})

	trainingStrategy, err := getTrainingStrategy(strategy)
	if err != nil {
		return nil, err
	}
	if v1beta1.IsCohereFTRuntime(clpc.launcherPodConfig.LauncherRuntimeName) {
		trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
			Name:  constants.LogTrainStatusEveryStepEnvVarKey,
			Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.LogTrainStatusEveryStepConfigKey),
		})

		// Set N_LAST_LAYERS value
		if trainingStrategy == constants.VanillaTrainingStrategy {
			nLastLayersStr := v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.NLastLayersConfigKey)
			nLastLayers, err := getAndValidateNLastLayers(nLastLayersStr, clpc.BaseModelResource.Spec.ModelParameterSize)
			if err != nil {
				return nil, err
			}
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.NLastLayersEnvVarKey,
				Value: strconv.Itoa(nLastLayers),
			})
		} else {
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.NLastLayersEnvVarKey,
				Value: strconv.Itoa(0), // Always set to 0 for tfew
			})
		}
	}

	// set training sidecar env vars dedicated for cohere command R FT
	if v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName) {
		// set base model version
		baseModelVersion, err := clpc.getCommandRBaseModelVersion()
		if err != nil {
			return nil, err
		}
		trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
			Name:  constants.BaseModelEnvVarKey,
			Value: string(baseModelVersion),
		})

		if clpc.isCommandRFTWeightMerged(trainingStrategy) {
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.ServingStrategyEnvVarKey,
				Value: string(constants.VanillaServingStrategy),
			})

			// set tensor parallel size
			tensorParallelSize, err := clpc.getTensorParallelSize()
			if err != nil {
				return nil, err
			}
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.TensorParallelEnvVarKey,
				Value: string(tensorParallelSize),
			})
			// set zipped merged model path
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.ZippedMergedModelPathEnvVarKey,
				Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, fineTunedModelName+constants.CohereCommandRFTMergedModelWeightSuffix),
			})
		} else {
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.ServingStrategyEnvVarKey,
				Value: string(constants.LoraServingStrategy),
			})
		}

		if trainingStrategy == constants.LoraTrainingStrategy {
			// set lora related hyperparameters
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.LoraREnvVarKey,
				Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.LoraRConfigKey),
			})
			trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
				Name:  constants.LoraAlphaEnvVarKey,
				Value: v1beta1.GetHyperparameterValueByKey(clpc.Hyperparameters, constants.LoraAlphaConfigKey),
			})
		}
	}

	// Other env variables
	trainingSidecarRuntime, err := clpc.getTrainingSidecarRuntime()
	if err != nil {
		return nil, err
	}
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.RuntimeEnvVarKey,
		Value: string(trainingSidecarRuntime),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ModelSizeEnvVarKey,
		Value: *clpc.BaseModelResource.Spec.ModelParameterSize,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainNameEnvVarKey,
		Value: fineTunedModelName,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ModelDirectoryEnvVarKey,
		Value: clpc.getModelDirectoryEnvValue(fineTunedModelName, trainingStrategy),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.ZippedModelPathEnvVarKey,
		Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, fineTunedModelName),
	})

	return &trainingSidecarEnvVars, nil
}

func getTrainingStrategy(trainingStrategyStr string) (constants.TrainingStrategy, error) {
	switch trainingStrategyStr {
	case "tfew":
		return constants.TFewTrainingStrategy, nil
	case "vanilla":
		return constants.VanillaTrainingStrategy, nil
	case "lora":
		return constants.LoraTrainingStrategy, nil
	}
	return constants.UnknownTraningStrategy, fmt.Errorf("failed to determine training strategy for TrainingStrategy: %s", trainingStrategyStr)
}

func getAndValidateNLastLayers(nLastLayersStr string, modelParaSize *string) (int, error) {
	layerNumber, err := strconv.Atoi(nLastLayersStr)
	if err != nil {
		return 0, err
	}

	// Below errors should be blocked from upstream
	if layerNumber <= 0 {
		return 0, fmt.Errorf("invalid number set for n last layer: %d", layerNumber)
	}

	if *modelParaSize == "6b" && layerNumber > constants.Max6BVanillaFineTunedLayers {
		return 0, fmt.Errorf("for 6b finetuning, n last layers should not exceed %d layer, now is set as %d", constants.Max6BVanillaFineTunedLayers, layerNumber)
	}

	if *modelParaSize == "52b" && layerNumber > constants.Max52BVanillaFineTunedLayers {
		return 0, fmt.Errorf("for 52b finetuning, n last layers should not exceed %d layer, now is set as %d", constants.Max52BVanillaFineTunedLayers, layerNumber)
	}

	return layerNumber, nil
}

func (clpc *CohereLauncherPodConfig) getBaselineModelEnvValue(fineTunedModelName string) string {
	baselineModelEnvValue := filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, "ckpt-0")
	if v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName) {
		baselineModelEnvValue = filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, *clpc.BaseModelResource.Spec.DisplayName)
	}
	return baselineModelEnvValue
}

func (clpc *CohereLauncherPodConfig) getModelEmptyDirMountPath() string {
	modelEmptyDirMountPath := filepath.Join(constants.CohereTrainingInitModelEmptyDirMountPathTensorRT)
	if v1beta1.IsCohereFTRuntime(clpc.launcherPodConfig.LauncherRuntimeName) {
		modelEmptyDirMountPath = filepath.Join(constants.CohereTrainingInitModelEmptyDirMountPathFastTransformer, *clpc.BaseModelResource.Spec.DisplayName)
	}
	return modelEmptyDirMountPath
}

func (clpc *CohereLauncherPodConfig) getModelDirectoryEnvValue(fineTunedModelName string, strategy constants.TrainingStrategy) string {
	switch {
	case v1beta1.IsCohereFTRuntime(clpc.launcherPodConfig.LauncherRuntimeName):
		return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, constants.CohereTrainingInitModelEmptyDirMountPathFastTransformer)
	case v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName):
		if clpc.isCommandRFTWeightMerged(strategy) {
			return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName)
		} else {
			return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, constants.CohereCommandRLoraTrainingModelDirectory)
		}
	}
	// Set <path_prefix> as default model directory
	return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName)
}

func (clpc *CohereLauncherPodConfig) getTensorParallelSize() (constants.CommandRTensorParallelSize, error) {
	if *clpc.BaseModelResource.Spec.ModelParameterSize == "6b" {
		return constants.CommandR16KFTTensorParallelSize, nil
	} else if *clpc.BaseModelResource.Spec.ModelParameterSize == "52b" {
		return constants.CommandR128KFTTensorParallelSize, nil
	}
	return "", fmt.Errorf("failed to determine command r tensor parallel size for model: %s", clpc.BaseModelResource.Name)
}

func (clpc *CohereLauncherPodConfig) getTrainingSidecarRuntime() (constants.TrainingSidecarRuntime, error) {
	switch {
	case v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName):
		return constants.CohereCommandRTrainingSidecar, nil
	case v1beta1.IsCohereFTRuntime(clpc.launcherPodConfig.LauncherRuntimeName):
		return constants.CohereCommand1TrainingSidecar, nil
	default:
		return "", fmt.Errorf("failed to determine training sidecar runtime for model: %s", clpc.BaseModelResource.Name)
	}
}

func (clpc *CohereLauncherPodConfig) getCommandRBaseModelVersion() (constants.CommandRBaseModelVersion, error) {
	switch {
	case strings.Contains(*clpc.BaseModelResource.Spec.DisplayName, constants.CohereCommandRV1Version):
		return constants.CommandRBaseModelV1, nil
	case strings.Contains(*clpc.BaseModelResource.Spec.DisplayName, constants.CohereCommandRV2Version):
		return constants.CommandRBaseModelV2, nil
	default:
		return "", fmt.Errorf("failed to determine command r base model version for model: %s", clpc.BaseModelResource.Name)
	}
}

func (clpc *CohereLauncherPodConfig) isCommandRFTWeightMerged(trainingStrategy constants.TrainingStrategy) bool {
	return (v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName) && trainingStrategy == constants.TFewTrainingStrategy) ||
		(v1beta1.IsCohereFTCommandRRuntime(clpc.launcherPodConfig.LauncherRuntimeName) && trainingStrategy == constants.LoraTrainingStrategy && *clpc.BaseModelResource.Spec.ModelParameterSize == "6b")
}
