package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	goerrors "github.com/pkg/errors"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterBaseModel Get the cluster base model from the given model name.
func GetClusterBaseModel(cl client.Client, name string) (*v1beta1.ClusterBaseModel, error) {
	clusterBaseModel := &v1beta1.ClusterBaseModel{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterBaseModel)
	if err == nil {
		return clusterBaseModel, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No BaseModel or ClusterBaseModel with the name: " + name)
}

// GetTrainingRuntime Get a TrainingRuntime by name.
// First, TrainingRuntime in the given namespace will be checked.
// If a resource of the specified name is not found, then ClusterTrainingRuntime will be checked.
func GetTrainingRuntime(cl client.Client, name string, namespace string) (*v1beta1.TrainingRuntimeSpec, error) {
	runtime := &v1beta1.TrainingRuntime{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, runtime)
	if err == nil {
		return &runtime.Spec, nil
	} else if !k8serrors.IsNotFound(err) {
		return nil, err
	}

	clusterRuntime := &v1beta1.ClusterTrainingRuntime{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterRuntime)
	if err == nil {
		return &clusterRuntime.Spec, nil
	} else if !k8serrors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No available TrainingRuntime or ClusterTrainingRuntime with the name: " + name)
}

// GetDedicatedAIClusterResource Get a DedicatedAICluster by a reference.
func GetDedicatedAIClusterResource(cl client.Client, dedicatedAIClusterRef *v1.ObjectReference) (*v1beta1.DedicatedAICluster, error) {
	if dedicatedAIClusterRef == nil {
		return nil, nil
	}

	dedicatedAiCluster := &v1beta1.DedicatedAICluster{}
	err := cl.Get(context.TODO(), types.NamespacedName{
		Name: dedicatedAIClusterRef.Name,
	}, dedicatedAiCluster)
	if err != nil {
		return nil, err
	}

	if dedicatedAiCluster.Status.DacLifecycleState != v1beta1.ACTIVE {
		return nil, fmt.Errorf("dedicatedAiCluster %s is not in a Active life cycle state", dedicatedAIClusterRef.Name)
	}

	return dedicatedAiCluster, nil
}

// GetHyperparameterValueByKey Get the value of hyperparameter raw extension by key.
func GetHyperparameterValueByKey(key string, parameters runtime.RawExtension) (interface{}, error) {
	var parametersMap map[string]interface{}
	err := json.Unmarshal(parameters.Raw, &parametersMap)
	if err != nil {
		return "", err
	}
	return parametersMap[key], nil
}

// GetTensorParallelSize Get the TensorParallelSize from base model.
func GetTensorParallelSize(baseModel *v1beta1.ClusterBaseModel) (string, error) {
	category, ok := baseModel.ObjectMeta.Annotations[constants.ModelCategoryAnnotation]
	if !ok {
		return "0", fmt.Errorf("no model category annotation for model: %s", baseModel.Name)
	}

	if category == "SMALL" {
		return "1", nil
	} else if category == "LARGE" {
		return "4", nil
	}
	return "0", nil
}

// ExtractModelNameFromObjectStorageUri Get the model name from object storage uri.
func ExtractModelNameFromObjectStorageUri(uri string) string {
	if !strings.Contains(uri, "/") {
		return uri
	}

	modelName := strings.Split(uri, "/")
	return modelName[len(modelName)-1]
}

// ExtractObjectFileNameFromObjectStorageUri Get the object file name from object storage uri.
func ExtractObjectFileNameFromObjectStorageUri(uri string) string {
	if !strings.Contains(uri, "/") {
		return uri
	}

	values := strings.Split(uri, "/o/")
	return values[len(values)-1]
}

// ExtractBucketNameFromObjectStorageUri Get the object bucket name from object storage uri.
func ExtractBucketNameFromObjectStorageUri(uri string) string {
	if !strings.Contains(uri, "/") {
		return uri
	}

	valuesWithoutObjectName := strings.Split(uri, "/o/")[0]
	bucketNames := strings.Split(valuesWithoutObjectName, "/b/")
	return bucketNames[len(bucketNames)-1]
}

// ExtractNamespaceFromObjectStorageUri Get the object namespace name from object storage uri.
func ExtractNamespaceFromObjectStorageUri(uri string) string {
	if !strings.Contains(uri, "/") {
		return uri
	}

	valuesWithoutObjectName := strings.Split(uri, "/o/")[0]
	valuesWithoutBucketName := strings.Split(valuesWithoutObjectName, "/b/")[0]
	namespaceNames := strings.Split(valuesWithoutBucketName, "/n/")
	return namespaceNames[len(namespaceNames)-1]
}

// GetFineTunedModelName Get the ft model name from training job name.
func GetFineTunedModelName(trainingJobName string) string {
	return trainingJobName[len(constants.TrainingJobNamePrefix):]
}

// GetShortTrainJobName Get the first 20 characters of training job name.
func GetShortTrainJobName(name string) string {
	// The train job name will be reused for jobset/pod name, which cannot exceed 63 characters. Use first 20 characters for pod name and trainer node name prefix/suffix buffer
	if len(name) >= 20 {
		return name[:20]
	}
	return name
}

// IsCommandRFTWeightMerged check if it is command-r finetune weight merged.
func IsCommandRFTWeightMerged(trainingStrategy string, tensorParallel string) bool {
	return trainingStrategy == "tfew" || (trainingStrategy == "lora" && tensorParallel == "1")
}
