package singlenode

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LauncherPodConfig struct {
	Namespace           string
	LauncherRuntimeName string
	TrainingJobName     string
	Datasets            map[constants.DatasetType]*v1beta1.Storage
	ModelStorage        *v1beta1.Storage
}

type TrainingInitConfig struct {
	Image          string `json:"image"`
	Region         string `json:"region"`
	CpuRequest     string `json:"cpuRequest"`
	CpuLimit       string `json:"cpuLimit"`
	MemoryRequest  string `json:"memoryRequest"`
	MemoryLimit    string `json:"memoryLimit"`
	CompartmentId  string `json:"compartmentId"`
	VaultId        string `json:"vaultId"`
	EnableOboToken string `json:"enableOboToken"`
	AuthType       string `json:"authType"`
}

type TrainingSidecarConfig struct {
	Region                string `json:"region"`
	Image                 string `json:"image"`
	Namespace             string `json:"namespace"`
	FineTunedModelBucket  string `json:"fineTunedModelBucket"`
	TrainingMetricsBucket string `json:"trainingMetricsBucket"`
}

func GetTrainingJobConfig(client client.Client) (*v1.ConfigMap, error) {
	tjobConfigMap := &v1.ConfigMap{}

	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: constants.OMEName,
		Name:      constants.TrainingJobConfigMapName,
	}, tjobConfigMap)

	return tjobConfigMap, err
}
func GetTrainingInitConfig(client client.Client) (*TrainingInitConfig, error) {
	trainingJobConfig, err := GetTrainingJobConfig(client)
	if err != nil {
		return nil, err
	}
	unmarshalledTrainingInitConfig := &TrainingInitConfig{}
	if trainingInitConfig, ok := trainingJobConfig.Data[constants.TrainingInitConfigMapKeyName]; ok {
		err := json.Unmarshal([]byte(trainingInitConfig), &unmarshalledTrainingInitConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to parse trainingInit config json: %+v", err)
		}
	}
	return unmarshalledTrainingInitConfig, nil
}

func GetTrainingSidecarConfig(client client.Client) (*TrainingSidecarConfig, error) {
	trainingJobConfig, err := GetTrainingJobConfig(client)
	if err != nil {
		return nil, err
	}
	unmarshalledTrainingSidecarConfig := &TrainingSidecarConfig{}
	if trainingSidecarConfig, ok := trainingJobConfig.Data[constants.TrainingSidecarConfigMapKeyName]; ok {
		err := json.Unmarshal([]byte(trainingSidecarConfig), &unmarshalledTrainingSidecarConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to parse trainingSidecar config json: %+v", err)
		}
	}
	return unmarshalledTrainingSidecarConfig, nil
}

func SetDatasetsEnvs(datasets map[constants.DatasetType]*v1beta1.Storage, trainingInitConfig TrainingInitConfig, envVar []v1.EnvVar) []v1.EnvVar {
	envVar = append(envVar, v1.EnvVar{
		Name:  constants.EnableOboTokenEnvVarKey,
		Value: trainingInitConfig.EnableOboToken,
	})

	trainingDataStore := datasets[constants.Training]

	switch trainingDataStore.StorageType {
	case v1beta1.ObjectStorage:
		envVar = append(envVar, v1.EnvVar{
			Name:  constants.OboTokenEnvVarKey,
			Value: trainingDataStore.OSStorageSpec.OboToken,
		})
		envVar = append(envVar, v1.EnvVar{
			Name:  constants.TrainingDataBucketNameEnvVarKey,
			Value: trainingDataStore.OSStorageSpec.BucketName,
		})
		envVar = append(envVar, v1.EnvVar{
			Name:  constants.TrainingDataObjectNameEnvVarKey,
			Value: trainingDataStore.OSStorageSpec.ObjectName,
		})
		envVar = append(envVar, v1.EnvVar{
			Name:  constants.TrainingDataObjectNamespaceEnvVarKey,
			Value: trainingDataStore.OSStorageSpec.Namespace,
		})
	}

	evaluationBucketName, evaluationObjectName, evaluationObjectNamespace := "", "", ""
	evaluationDataStore := datasets[constants.Evaluation]
	if evaluationDataStore != nil {
		switch evaluationDataStore.StorageType {
		case v1beta1.ObjectStorage:
			if evaluationDataStore.OSStorageSpec != nil {
				evaluationBucketName = evaluationDataStore.OSStorageSpec.BucketName
				evaluationObjectName = evaluationDataStore.OSStorageSpec.ObjectName
				evaluationObjectNamespace = evaluationDataStore.OSStorageSpec.Namespace
			}
			envVar = append(envVar, v1.EnvVar{
				Name:  constants.EvaluationDataBucketNameEnvVarKey,
				Value: evaluationBucketName,
			})
			envVar = append(envVar, v1.EnvVar{
				Name:  constants.EvaluationDataObjectNameEnvVarKey,
				Value: evaluationObjectName,
			})
			envVar = append(envVar, v1.EnvVar{
				Name:  constants.EvaluationDataObjectNamespaceEnvVarKey,
				Value: evaluationObjectNamespace,
			})
		}
	}
	return envVar
}

// SetModelStorageEnvs sets fine-tuned model bucket and namespace
// If modelStorage from training job is nil, use the config values from trainingSidecar's configmap.
func SetModelStorageEnvs(modelStorage *v1beta1.Storage, trainingSidecarConfig TrainingSidecarConfig, envVars []v1.EnvVar) []v1.EnvVar {
	namespace := trainingSidecarConfig.Namespace
	if modelStorage != nil && modelStorage.OSStorageSpec != nil && len(modelStorage.OSStorageSpec.Namespace) > 0 {
		namespace = modelStorage.OSStorageSpec.Namespace
	}
	envVars = append(envVars, v1.EnvVar{
		Name:  constants.NamespaceEnvVarKey,
		Value: namespace,
	})

	modelBucket := trainingSidecarConfig.FineTunedModelBucket
	if modelStorage != nil && modelStorage.OSStorageSpec != nil && len(modelStorage.OSStorageSpec.BucketName) > 0 {
		modelBucket = modelStorage.OSStorageSpec.BucketName
	}

	envVars = append(envVars, v1.EnvVar{
		Name:  constants.BucketNameEnvVarKey,
		Value: modelBucket,
	})
	return envVars
}

func SetNodeAffinityIfNecessary(podSpec *v1.PodSpec, dedicatedAiClusterResource *v1beta1.DedicatedAICluster) {
	if dedicatedAiClusterResource != nil {
		if dedicatedAiClusterResource.Spec.Affinity != nil {
			podSpec.Affinity = dedicatedAiClusterResource.Spec.Affinity.DeepCopy()
		}
	}
}

func SetGpuResourceLimitsIfNecessary(trainingContainer *v1.Container, dedicatedAiClusterResource *v1beta1.DedicatedAICluster) {
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
