package benchmarkutils

import (
	"context"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	isvcutils "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/sgl-ome/pkg/utils/storage"
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
		ref := endpointSpec.InferenceService
		inferenceService, err := GetInferenceService(c, ref)
		if err != nil {
			return nil, err
		}

		args := make(map[string]string)
		// TODO: Use actual service account key later
		args["--api-key"] = "sample-key"

		if inferenceService.Spec.Predictor.Model != nil {
			model := inferenceService.Spec.Predictor.Model

			// Use protocol version if available
			if model.ProtocolVersion != nil {
				args["--api-backend"] = string(*model.ProtocolVersion)
			} else {
				// Default or error if protocol is mandatory?
				// For now, let's assume a default or leave it empty if not critical
				args["--api-backend"] = "vllm" // Assuming default if nil
			}

			// Use a generic model name and set the model-tokenizer if BaseModel is defined
			if model.BaseModel != nil {
				baseModel, _, err := isvcutils.GetBaseModel(c, *model.BaseModel, inferenceService.Namespace)
				if err != nil {
					return nil, fmt.Errorf("failed to get BaseModel %s: %w", *model.BaseModel, err)
				}
				if baseModel.Storage == nil || baseModel.Storage.Path == nil {
					return nil, fmt.Errorf("BaseModel %s has missing Storage or Path information", *model.BaseModel)
				}
				args["--api-model-name"] = "vllm-model" // Or derive from somewhere?
				args["--model-tokenizer"] = *baseModel.Storage.Path
			} else {
				// Handle case where BaseModel is not specified but needed?
				// Or maybe model name comes from somewhere else?
				args["--api-model-name"] = "some-default-model" // Placeholder
			}
		} else {
			return nil, fmt.Errorf("InferenceService %s/%s has no Model defined in Predictor spec", ref.Namespace, ref.Name)
		}

		// Extract the URL from the InferenceService's status if available
		if inferenceService.Status.URL == nil || inferenceService.Status.URL.Host == "" {
			return nil, fmt.Errorf("InferenceService %s/%s has no URL.Host in status", ref.Namespace, ref.Name)
		}
		// Assuming http and standard port for now if scheme/port missing
		scheme := "http"
		if inferenceService.Status.URL.Scheme != "" {
			scheme = inferenceService.Status.URL.Scheme
		}
		args["--api-base"] = fmt.Sprintf("%s://%s", scheme, inferenceService.Status.URL.Host)

		return args, nil
	}

	return nil, fmt.Errorf("invalid EndpointSpec: both Endpoint and InferenceService are nil")
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

// UpdateVolumeMounts updates the volume mounts for the benchmark container if a base model is defined.
func UpdateVolumeMounts(isvc *v1beta1.InferenceService, container *v1.Container, baseModel *v1beta1.BaseModelSpec) {
	if isvc.Spec.Predictor.Model == nil || isvc.Spec.Predictor.Model.BaseModel == nil || baseModel == nil {
		return
	}

	baseModelName := *isvc.Spec.Predictor.Model.BaseModel

	// Define the volume mount
	volumeMount := v1.VolumeMount{
		Name:      baseModelName,
		MountPath: *baseModel.Storage.Path,
		ReadOnly:  true,
	}

	isvcutils.AppendVolumeMount(container, &volumeMount)
	isvcutils.AppendEnvVars(container, &[]v1.EnvVar{
		{Name: "MODEL_PATH", Value: *baseModel.Storage.Path},
	})
}

// BuildStorageArgs builds command line arguments for storage configuration
func BuildStorageArgs(storageSpec *v1beta1.StorageSpec) ([]string, error) {
	if storageSpec == nil {
		return nil, fmt.Errorf("storageSpec cannot be nil")
	}
	if storageSpec.StorageUri == nil {
		return nil, fmt.Errorf("storageUri cannot be nil")
	}

	// Try to determine storage type
	storageType, err := storage.GetStorageType(*storageSpec.StorageUri)
	if err != nil {
		return nil, fmt.Errorf("invalid storage URI: %v", err)
	}

	var args []string

	switch storageType {
	case storage.StorageTypeOCI:
		// Parse and add OCI storage URI components
		components, err := storage.ParseOCIStorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid OCI storage URI: %v", err)
		}
		args = append(args, "--upload-results")
		args = append(args,
			"--namespace", components.Namespace,
			"--bucket", components.Bucket,
			"--prefix", components.Prefix,
		)

		// Handle storage parameters
		if storageSpec.Parameters != nil {
			params := *storageSpec.Parameters
			// Add auth type
			if authType, ok := params["auth"]; ok {
				args = append(args, "--auth", authType)
			}
			// Add config file path if specified
			if configFile, ok := params["config_file"]; ok {
				args = append(args, "--config-file", configFile)
			}
			// Add profile if specified
			if profile, ok := params["profile"]; ok {
				args = append(args, "--profile", profile)
			}
			// Add security token if specified
			if securityToken, ok := params["security_token"]; ok {
				args = append(args, "--security-token", securityToken)
			}
			// Add region if specified
			if region, ok := params["region"]; ok {
				args = append(args, "--region", region)
			}
		}

	case storage.StorageTypePVC:
		// For PVC storage, we don't need to add any command line arguments
		// The storage will be handled by mounting the PVC to the pod
		// We'll just validate that the URI is correct
		components, err := storage.ParsePVCStorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid PVC storage URI: %v", err)
		}
		args = append(args, "--experiment-base-dir", "/"+components.SubPath)
	}

	return args, nil
}
