package benchmarkutils

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils/storage"
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

		var baseModelName *string
		var protocolVersion string
		if inferenceService.Spec.Predictor.Model != nil {
			baseModelName = inferenceService.Spec.Predictor.Model.BaseModel
			protocolVersion = string(*inferenceService.Spec.Predictor.Model.ProtocolVersion)
		} else if inferenceService.Spec.Model != nil &&
			inferenceService.Spec.Engine != nil &&
			inferenceService.Spec.Engine.Runner != nil {
			baseModelName = &inferenceService.Spec.Model.Name
			for _, env := range inferenceService.Spec.Engine.Runner.Env {
				if env.Name == "PROTOCOL_VERSION" {
					protocolVersion = env.Value
					break
				}
			}
		} else {
			return nil, fmt.Errorf("InferenceService %s/%s has no Model defined in Predictor spec", ref.Namespace, ref.Name)
		}

		// Use protocol version if available
		if protocolVersion != "" {
			args["--api-backend"] = string(protocolVersion)
		} else {
			// Default or error if protocol is mandatory?
			// For now, let's assume a default or leave it empty if not critical
			args["--api-backend"] = "vllm" // Assuming default if nil
		}

		// Use a generic model name and set the model-tokenizer if BaseModel is defined
		if baseModelName != nil {
			baseModel, _, err := isvcutils.GetBaseModel(c, *baseModelName, inferenceService.Namespace)
			if err != nil {
				return nil, fmt.Errorf("failed to get BaseModel %s: %w", *baseModelName, err)
			}
			if baseModel.Storage == nil || baseModel.Storage.Path == nil {
				return nil, fmt.Errorf("BaseModel %s has missing Storage or Path information", *baseModelName)
			}
			args["--api-model-name"] = "vllm-model" // Or derive from somewhere?
			args["--model-tokenizer"] = *baseModel.Storage.Path
		} else {
			// Handle case where BaseModel is not specified but needed?
			// Or maybe model name comes from somewhere else?
			args["--api-model-name"] = "some-default-model" // Placeholder
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

	case storage.StorageTypeS3:
		// Parse and add S3 storage URI components
		components, err := storage.ParseS3StorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 storage URI: %v", err)
		}
		args = append(args, "--upload-results")
		args = append(args, "--storage-provider", "aws")
		args = append(args, "--storage-bucket", components.Bucket)
		if components.Prefix != "" {
			args = append(args, "--storage-prefix", components.Prefix)
		}

		// Handle storage parameters
		if storageSpec.Parameters != nil {
			params := *storageSpec.Parameters
			// AWS credentials
			if accessKey, ok := params["aws_access_key_id"]; ok {
				args = append(args, "--storage-aws-access-key-id", accessKey)
			}
			if secretKey, ok := params["aws_secret_access_key"]; ok {
				args = append(args, "--storage-aws-secret-access-key", secretKey)
			}
			if profile, ok := params["aws_profile"]; ok {
				args = append(args, "--storage-aws-profile", profile)
			}
			if region, ok := params["aws_region"]; ok {
				args = append(args, "--storage-aws-region", region)
			} else if components.Region != "" {
				args = append(args, "--storage-aws-region", components.Region)
			}
		}

	case storage.StorageTypeAzure:
		// Parse and add Azure storage URI components
		components, err := storage.ParseAzureStorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid Azure storage URI: %v", err)
		}
		args = append(args, "--upload-results")
		args = append(args, "--storage-provider", "azure")
		args = append(args, "--storage-bucket", components.ContainerName)
		if components.BlobPath != "" {
			args = append(args, "--storage-prefix", components.BlobPath)
		}

		// Always add the account name
		if storageSpec.Parameters != nil {
			params := *storageSpec.Parameters
			// Check if account name is provided in parameters
			if accountName, ok := params["azure_account_name"]; ok {
				args = append(args, "--storage-azure-account-name", accountName)
			} else {
				args = append(args, "--storage-azure-account-name", components.AccountName)
			}
			// Azure credentials
			if accountKey, ok := params["azure_account_key"]; ok {
				args = append(args, "--storage-azure-account-key", accountKey)
			}
			if connString, ok := params["azure_connection_string"]; ok {
				args = append(args, "--storage-azure-connection-string", connString)
			}
			if sasToken, ok := params["azure_sas_token"]; ok {
				args = append(args, "--storage-azure-sas-token", sasToken)
			}
		} else {
			// Even without parameters, we need to add the account name
			args = append(args, "--storage-azure-account-name", components.AccountName)
		}

	case storage.StorageTypeGCS:
		// Parse and add GCS storage URI components
		components, err := storage.ParseGCSStorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid GCS storage URI: %v", err)
		}
		args = append(args, "--upload-results")
		args = append(args, "--storage-provider", "gcp")
		args = append(args, "--storage-bucket", components.Bucket)
		if components.Object != "" {
			args = append(args, "--storage-prefix", components.Object)
		}

		// Handle storage parameters
		if storageSpec.Parameters != nil {
			params := *storageSpec.Parameters
			// GCP credentials
			if projectID, ok := params["gcp_project_id"]; ok {
				args = append(args, "--storage-gcp-project-id", projectID)
			}
			if credsPath, ok := params["gcp_credentials_path"]; ok {
				args = append(args, "--storage-gcp-credentials-path", credsPath)
			}
		}

	case storage.StorageTypeGitHub:
		// Parse and add GitHub storage URI components
		components, err := storage.ParseGitHubStorageURI(*storageSpec.StorageUri)
		if err != nil {
			return nil, fmt.Errorf("invalid GitHub storage URI: %v", err)
		}
		args = append(args, "--upload-results")
		args = append(args, "--storage-provider", "github")
		// GitHub doesn't use bucket/prefix model, but owner/repo
		args = append(args, "--github-owner", components.Owner)
		args = append(args, "--github-repo", components.Repository)
		if components.Tag != "latest" {
			args = append(args, "--github-tag", components.Tag)
		}

		// Handle storage parameters
		if storageSpec.Parameters != nil {
			params := *storageSpec.Parameters
			// GitHub token
			if token, ok := params["github_token"]; ok {
				args = append(args, "--github-token", token)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return args, nil
}
