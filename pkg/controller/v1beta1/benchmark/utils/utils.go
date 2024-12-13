package benchmarkutils

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	isvcutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetInferenceService fetches the InferenceService based on the provided InferenceServiceReference.
func GetInferenceService(c client.Client, ref *v1beta1.InferenceServiceReference) (*v1beta1.InferenceService, error) {
	if ref == nil {
		return nil, fmt.Errorf("inferenceservice reference is nil")
	}

	namespacedName := types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}

	inferenceService := &v1beta1.InferenceService{}
	if err := c.Get(context.TODO(), namespacedName, inferenceService); err != nil {
		return nil, fmt.Errorf("failed to get InferenceService %s/%s: %w",
			ref.Namespace, ref.Name, err)
	}

	return inferenceService, nil
}

// BuildInferenceServiceArgs constructs a map of arguments for the benchmark command
// based on either a direct Endpoint or an InferenceService reference in the EndpointSpec.
func BuildInferenceServiceArgs(c client.Client, endpointSpec v1beta1.EndpointSpec, namespace string) (map[string]string, error) {
	if endpointSpec.Endpoint != nil {
		return buildArgsFromEndpoint(endpointSpec.Endpoint), nil
	}

	if endpointSpec.InferenceService != nil {
		return buildArgsFromInferenceService(c, endpointSpec.InferenceService)
	}

	return nil, fmt.Errorf("invalid EndpointSpec: both Endpoint and InferenceService are nil")
}

// UpdateVolumeMounts updates the volume mounts for the benchmark container if a base model is defined.
func UpdateVolumeMounts(isvc *v1beta1.InferenceService, container *v1.Container) {
	if isvc.Spec.Predictor.Model == nil || isvc.Spec.Predictor.Model.BaseModel == nil {
		return
	}

	baseModelName := *isvc.Spec.Predictor.Model.BaseModel
	modelMountPath := fmt.Sprintf("/model/%s", baseModelName)

	// Define the volume mount
	volumeMount := v1.VolumeMount{
		Name:      baseModelName,
		MountPath: modelMountPath,
		ReadOnly:  true,
	}

	isvcutils.UpdateVolumeMounts(container, &volumeMount)
	isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
		{Name: "MODEL_PATH", Value: modelMountPath},
	})
}

// buildArgsFromEndpoint constructs the arguments map when an Endpoint is directly provided.
func buildArgsFromEndpoint(endpoint *v1beta1.Endpoint) map[string]string {
	args := make(map[string]string)
	args["--api-backend"] = endpoint.APIFormat
	args["--api-model-name"] = endpoint.ModelName
	args["--api-base"] = endpoint.URL

	// TODO: add --model-tokenizer once available
	return args
}

// buildArgsFromInferenceService constructs the arguments map by querying the InferenceService.
func buildArgsFromInferenceService(c client.Client, ref *v1beta1.InferenceServiceReference) (map[string]string, error) {
	inferenceService, err := GetInferenceService(c, ref)
	if err != nil {
		return nil, err
	}

	args := make(map[string]string)
	args["--api-key"] = "sample-key"

	if inferenceService.Spec.Predictor.Model != nil {
		model := inferenceService.Spec.Predictor.Model
		// Use protocol version if available
		if model.ProtocolVersion != nil {
			args["--api-backend"] = string(*model.ProtocolVersion)
		}

		// Use a generic model name and set the model-tokenizer if BaseModel is defined
		if model.BaseModel != nil {
			args["--api-model-name"] = "vllm-model"
			args["--model-tokenizer"] = fmt.Sprintf("/model/%s", *model.BaseModel)
		}
	}

	// Extract the URL from the InferenceService's status if available
	if inferenceService.Status.URL == nil {
		return nil, fmt.Errorf("InferenceService %s/%s has no URL in status",
			ref.Namespace, ref.Name)
	}
	args["--api-base"] = fmt.Sprintf("%s:%d", inferenceService.Status.URL, 8080)

	return args, nil
}
