package pod

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"
	v1 "k8s.io/api/core/v1"
)

// TrainingSidecarInjector represents configuration parameters for the training sidecar container.
type TrainingSidecarInjector struct {
	Image                 string `json:"image" validate:"required"`
	Region                string `json:"region"`
	Namespace             string `json:"namespace"`
	FineTunedModelBucket  string `json:"fineTunedModelBucket"`
	TrainingMetricsBucket string `json:"trainingMetricsBucket"`
	CompartmentId         string `json:"compartmentId"`
}

// newTrainingSidecarInjector initializes a TrainingSidecarInjector from a ConfigMap.
func newTrainingSidecarInjector(configMap *v1.ConfigMap) *TrainingSidecarInjector {
	trainingSidecarInjector := &TrainingSidecarInjector{}
	if trainingSidecarConfigVal, ok := configMap.Data[constants.TrainingSidecarConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(trainingSidecarConfigVal), trainingSidecarInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", constants.TrainingSidecarConfigMapKeyName, err))
		}
	}
	return trainingSidecarInjector
}

// InjectTrainingSidecar injects the serving sidecar container into the pod if necessary.
func (tsi *TrainingSidecarInjector) InjectTrainingSidecar(pod *v1.Pod) error {
	if enableTrainingSidecar, ok := pod.ObjectMeta.Annotations[constants.TrainingSidecarInjectionKey]; ok && enableTrainingSidecar == "true" {
		return tsi.injectTrainingSidecar(pod)
	}
	return nil
}

func (tsi *TrainingSidecarInjector) injectTrainingSidecar(pod *v1.Pod) error {
	if tsi.containerExists(pod) {
		return nil
	}

	// general validation
	if err := tsi.validate(); err != nil {
		return err
	}

	trainingSidecarMounts := tsi.getVolumeMounts(pod)
	trainingSidecarEnvs := tsi.getTrainingSidecarEnvs(pod)

	securityContext, err := tsi.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	trainingSidecarContainer := tsi.createTrainingSidecarContainer(trainingSidecarEnvs, trainingSidecarMounts, securityContext)

	pod.Spec.Containers = append(pod.Spec.Containers, *trainingSidecarContainer)

	return nil
}

// containerExists checks if the Training Sidecar container is already in the pod.
func (tsi *TrainingSidecarInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.TrainingSidecarContainerName {
			return true
		}
	}
	return false
}

func (tsi *TrainingSidecarInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(tsi); err != nil {
		return fmt.Errorf("failed to validate TrainingSidecarInjector: %w", err)
	}
	return nil
}

func (tsi *TrainingSidecarInjector) getVolumeMounts(pod *v1.Pod) []v1.VolumeMount {
	runtimeType := pod.ObjectMeta.Annotations[constants.TrainingRuntimeTypeAnnotationKey]

	var trainingSidecarMounts []v1.VolumeMount
	if runtimeType == "peft" {
		dataEmptyDirVolumeMount := v1.VolumeMount{
			Name:      constants.DataEmptyDirName,
			MountPath: constants.TrainingDataEmptyDirMountPath,
			ReadOnly:  false,
		}
		trainingSidecarMounts = append(trainingSidecarMounts, dataEmptyDirVolumeMount)
	} else {
		finetunedModelName := utils.GetFineTunedModelName(pod.ObjectMeta.Labels[constants.TrainingJobPodLabelKey])
		modelEmptyDirVolumeMount := v1.VolumeMount{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: filepath.Join(constants.CohereStorePathPrefix, finetunedModelName),
			ReadOnly:  false,
		}
		trainingSidecarMounts = append(trainingSidecarMounts, modelEmptyDirVolumeMount)

		dataEmptyDirVolumeMount := v1.VolumeMount{
			Name:      constants.DataEmptyDirName,
			MountPath: filepath.Join(constants.CohereStorePathPrefix, finetunedModelName, "/input/data/training/"),
			ReadOnly:  false,
		}
		trainingSidecarMounts = append(trainingSidecarMounts, dataEmptyDirVolumeMount)
	}

	// Add region/ad/realm host path volume mounts
	regionADRealmHostPathVolumeMounts := []v1.VolumeMount{
		{
			Name:      constants.RegionFileVolumeName,
			MountPath: constants.RegionFileVolumeMountPath,
		},
		{
			Name:      constants.ADFileVolumeName,
			MountPath: constants.ADFileVolumeMountPath,
		},
		{
			Name:      constants.RealmFileVolumeName,
			MountPath: constants.RealmFileVolumeMountPath,
		},
	}
	trainingSidecarMounts = append(trainingSidecarMounts, regionADRealmHostPathVolumeMounts...)

	return trainingSidecarMounts
}

func (tsi *TrainingSidecarInjector) getTrainingSidecarEnvs(pod *v1.Pod) *[]v1.EnvVar {
	trainingSidecarEnvVars := make([]v1.EnvVar, 0)

	runtimeType := pod.ObjectMeta.Annotations[constants.TrainingRuntimeTypeAnnotationKey]
	trainingName := pod.ObjectMeta.Labels[constants.TrainingJobPodLabelKey]
	// Set env vars from values set in trainingSidecar config map
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.RuntimeEnvVarKey,
		Value: runtimeType,
	})

	if obo_token, ok := pod.ObjectMeta.Annotations[constants.OboTokenConfigKey]; ok {
		trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
			Name:  constants.OboTokenEnvVarKey,
			Value: obo_token,
		})

		trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
			Name:  constants.EnableOboTokenEnvVarKey,
			Value: "true",
		})
	}

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.AgentCompartmentIDEnvVarKey,
		Value: tsi.CompartmentId,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingNameEnvVarKey,
		Value: utils.GetFineTunedModelName(trainingName),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.AgentModelObjectName,
		Value: utils.GetFineTunedModelName(trainingName),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.AgentModelNamespaceEnvVarKey,
		Value: tsi.Namespace,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingMetricsObjectEnvVarKey,
		Value: utils.GetFineTunedModelName(trainingName),
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.AgentAuthTypeEnvVarKey,
		Value: "InstancePrincipal",
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataBucketNameEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.TrainingDataBucketConfigKey],
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataNamespaceEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.TrainingDataNamespaceConfigKey],
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataFileNameEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.TrainingDataFileNameConfigKey],
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.BucketNameEnvVarKey,
		Value: tsi.FineTunedModelBucket,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingMetricsBucketEnvVarKey,
		Value: tsi.TrainingMetricsBucket,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.NamespaceEnvVarKey,
		Value: tsi.Namespace,
	})

	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingMetricsNamespaceEnvVarKey,
		Value: tsi.Namespace,
	})

	if runtimeType == "peft" {
		peftEnvVars := tsi.getPeftEnvVars(pod)
		trainingSidecarEnvVars = append(trainingSidecarEnvVars, peftEnvVars...)
	} else {
		cohereEnvVars := tsi.getCohereEnvVars(trainingName, pod, runtimeType)
		trainingSidecarEnvVars = append(trainingSidecarEnvVars, cohereEnvVars...)
	}

	return &trainingSidecarEnvVars
}

func (tsi *TrainingSidecarInjector) getCohereEnvVars(trainingName string, pod *v1.Pod, runtimeType string) []v1.EnvVar {
	fineTunedModelName := utils.GetFineTunedModelName(trainingName)
	cohereEnvVars := make([]v1.EnvVar, 0)

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereTrainingSidecarNameEnvVarKey,
		Value: trainingName,
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataDirectoryEnvVarKey,
		Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, "/input/data/training/"),
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereEpochsEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EpochsConfigKey],
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereLearningRateEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.LearningRateConfigKey],
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereBatchSizeEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.BatchSizeConfigKey],
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereEarlyStoppingPatienceEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EarlyStoppingPatienceConfigKey],
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.CohereEarlyStoppingThresholdEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EarlyStoppingThresholdConfigKey],
	})

	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.ModelSizeEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.ModelSizeConfigKey],
	})

	strategy := pod.ObjectMeta.Annotations[constants.StrategyConfigKey]
	cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
		Name:  constants.StrategyEnvVarKey,
		Value: strategy,
	})

	if runtimeType == "cohere" {
		cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
			Name:  constants.CohereLogTrainStatusEveryStepEnvVarKey,
			Value: pod.ObjectMeta.Annotations[constants.LogTrainStatusEveryStepConfigKey],
		})

		if strategy == "vanilla" {
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.CohereNLastLayersEnvVarKey,
				Value: pod.ObjectMeta.Annotations[constants.NLastLayersConfigKey],
			})
		}

		cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
			Name:  constants.ModelDirectoryEnvVarKey,
			Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, constants.CohereTrainingInitModelEmptyDirMountPathFastTransformer),
		})

	} else {
		if strings.Contains(pod.ObjectMeta.Annotations[constants.BaseModelConfigKey], constants.CohereCommandRV2Version) {
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.CohereModelNameEnvVarKey,
				Value: constants.CommandRBaseModelV2,
			})
		}

		cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
			Name:  constants.CohereTensorParallelEnvVarKey,
			Value: pod.ObjectMeta.Annotations[constants.TensorParallelConfigKey],
		})

		if utils.IsCommandRFTWeightMerged(strategy, pod.ObjectMeta.Annotations[constants.TensorParallelConfigKey]) {
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.ModelDirectoryEnvVarKey,
				Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName),
			})

			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.ServingStrategyEnvVarKey,
				Value: string(constants.VanillaServingStrategy),
			})

			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.ZippedMergedModelPathEnvVarKey,
				Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, fineTunedModelName+constants.CohereCommandRFTMergedModelWeightSuffix),
			})
		} else {
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.ModelDirectoryEnvVarKey,
				Value: filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, constants.CohereCommandRLoraTrainingModelDirectory),
			})
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.ServingStrategyEnvVarKey,
				Value: string(constants.LoraServingStrategy),
			})
		}

		if strategy == "lora" {
			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.CohereLoraConfigRankEnvVarKey,
				Value: pod.ObjectMeta.Annotations[constants.LoraConfigRankConfigKey],
			})

			cohereEnvVars = append(cohereEnvVars, v1.EnvVar{
				Name:  constants.CohereLoraConfigAlphaEnvVarKey,
				Value: pod.ObjectMeta.Annotations[constants.LoraAlphaConfigKey],
			})
		}
	}

	return cohereEnvVars
}

func (tsi *TrainingSidecarInjector) getPeftEnvVars(pod *v1.Pod) []v1.EnvVar {
	peftEnvVars := make([]v1.EnvVar, 0)

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.ModelDirectoryEnvVarKey,
		Value: filepath.Join(constants.TrainingDataEmptyDirMountPath, constants.PeftTrainingOutputModelDirectoryName),
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.TrainingDataDirectoryEnvVarKey,
		Value: "/mnt/data",
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftModelNameEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.ModelNameConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftTrainingDataSetFileEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.TrainingDataFileNameConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.LogMetricsIntervalInStepsEnvVarKey,
		Value: "10",
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftTypeEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.StrategyConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftLoraREnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.LoraConfigRankConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftLoraConfigAlphaEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.LoraAlphaConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.LoraDropoutEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.LoraDropoutConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftEpochsEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EpochsConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftLearningRateEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.LearningRateConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftBatchSizeEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.BatchSizeConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftEarlyStoppingPatienceEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EarlyStoppingPatienceConfigKey],
	})

	peftEnvVars = append(peftEnvVars, v1.EnvVar{
		Name:  constants.PeftEarlyStoppingThresholdEnvVarKey,
		Value: pod.ObjectMeta.Annotations[constants.EarlyStoppingThresholdConfigKey],
	})

	return peftEnvVars
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (tsi *TrainingSidecarInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.TrainingMainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.TrainingMainContainerName)
}

func (tsi *TrainingSidecarInjector) createTrainingSidecarContainer(trainingSidecarEnvs *[]v1.EnvVar, trainingSidecarMounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	// Create sidecar container
	return &v1.Container{
		Name:                     constants.TrainingSidecarContainerName,
		Image:                    tsi.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      *trainingSidecarEnvs,
		Args:                     []string{"training-agent", "--config", "/ome-agent.yaml", "--debug"},
		VolumeMounts:             trainingSidecarMounts,
		SecurityContext:          securityContext,
	}
}
