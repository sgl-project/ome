package benchmarkutils

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

// GetInferenceService fetches the InferenceService based on the provided InferenceServiceReference.
func GetInferenceService(ctx context.Context, c client.Client, ref *v1beta1.InferenceServiceReference) (*v1beta1.InferenceService, error) {
	if ref == nil {
		return nil, fmt.Errorf("inferenceservice reference is nil")
	}

	inferenceService := &v1beta1.InferenceService{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, inferenceService); err != nil {
		return nil, fmt.Errorf("failed to get InferenceService %s/%s: %w",
			ref.Namespace, ref.Name, err)
	}

	return inferenceService, nil
}

// GetBaseModelName extracts the base model name from an InferenceService
func GetBaseModelName(isvc *v1beta1.InferenceService) string {
	if isvc.Spec.Predictor.Model != nil && isvc.Spec.Predictor.Model.BaseModel != nil {
		return *isvc.Spec.Predictor.Model.BaseModel
	}
	if isvc.Spec.Model != nil {
		return isvc.Spec.Model.Name
	}
	return ""
}

// BuildInferenceServiceArgs constructs a map of arguments for the benchmark command
// based on either a direct Endpoint or an InferenceService reference in the EndpointSpec.
func BuildInferenceServiceArgs(ctx context.Context, c client.Client, endpointSpec v1beta1.EndpointSpec, namespace string) (map[string]string, error) {
	if endpointSpec.Endpoint != nil {
		return buildArgsFromEndpoint(endpointSpec.Endpoint), nil
	}

	if endpointSpec.InferenceService != nil {
		ref := endpointSpec.InferenceService
		inferenceService, err := GetInferenceService(ctx, c, ref)
		if err != nil {
			return nil, err
		}

		baseModelName := GetBaseModelName(inferenceService)
		if baseModelName == "" {
			return nil, fmt.Errorf("InferenceService %s/%s has no Model defined", ref.Namespace, ref.Name)
		}

		baseModel, _, err := isvcutils.GetBaseModel(c, baseModelName, inferenceService.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get BaseModel %s: %w", baseModelName, err)
		}
		if baseModel.Storage == nil || baseModel.Storage.Path == nil {
			return nil, fmt.Errorf("BaseModel %s has missing Storage or Path information", baseModelName)
		}

		args := map[string]string{
			"--api-key":         "sample-key", // TODO: Use actual service account key later
			"--api-model-name":  "vllm-model",
			"--model-tokenizer": *baseModel.Storage.Path,
		}

		// Use protocol version if available
		var protocolVersion string
		if inferenceService.Spec.Predictor.Model != nil &&
			inferenceService.Spec.Predictor.Model.ProtocolVersion != nil &&
			*inferenceService.Spec.Predictor.Model.ProtocolVersion != "" {
			protocolVersion = string(*inferenceService.Spec.Predictor.Model.ProtocolVersion)
		} else if inferenceService.Spec.Engine != nil && inferenceService.Spec.Engine.Runner != nil {
			for _, env := range inferenceService.Spec.Engine.Runner.Env {
				if env.Name == "PROTOCOL_VERSION" {
					protocolVersion = env.Value
					break
				}
			}
		}
		if protocolVersion != "" {
			args["--api-backend"] = string(protocolVersion)
		} else {
			// Default to openai for inference service
			args["--api-backend"] = "openai"
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
	return map[string]string{
		"--api-backend":    endpoint.APIFormat,
		"--api-model-name": endpoint.ModelName,
		"--api-base":       endpoint.URL,
	}
}

// UpdateVolumeMounts updates the volume mounts for the benchmark container if a base model is defined.
func UpdateVolumeMounts(container *v1.Container, baseModelName string, baseModel *v1beta1.BaseModelSpec) {
	if baseModelName == "" || baseModel == nil {
		return
	}

	// Define the volume mount
	volumeMount := v1.VolumeMount{
		Name:      baseModelName,
		MountPath: *baseModel.Storage.Path,
		ReadOnly:  true,
	}

	isvcutils.AppendVolumeMount(container, &volumeMount)
	isvcutils.AppendEnvVarsIfNotExist(container, &[]v1.EnvVar{
		{Name: "MODEL_PATH", Value: *baseModel.Storage.Path},
	})
}

// storageArgsBuilder is a function type for building storage-specific arguments
type storageArgsBuilder func(uri string, params map[string]string) ([]string, error)

// storageBuilders maps storage types to their argument builders
var storageBuilders = map[storage.StorageType]storageArgsBuilder{
	storage.StorageTypeOCI:    buildOCIArgs,
	storage.StorageTypePVC:    buildPVCArgs,
	storage.StorageTypeS3:     buildS3Args,
	storage.StorageTypeAzure:  buildAzureArgs,
	storage.StorageTypeGCS:    buildGCSArgs,
	storage.StorageTypeGitHub: buildGitHubArgs,
}

// addParam appends a flag and value to args if the key exists in params
func addParam(args []string, params map[string]string, key, flag string) []string {
	if params == nil {
		return args
	}
	if v, ok := params[key]; ok {
		return append(args, flag, v)
	}
	return args
}

// BuildStorageArgs builds command line arguments for storage configuration
func BuildStorageArgs(storageSpec *v1beta1.StorageSpec) ([]string, error) {
	if storageSpec == nil {
		return nil, fmt.Errorf("storageSpec cannot be nil")
	}
	if storageSpec.StorageUri == nil {
		return nil, fmt.Errorf("storageUri cannot be nil")
	}

	storageType, err := storage.GetStorageType(*storageSpec.StorageUri)
	if err != nil {
		return nil, fmt.Errorf("invalid storage URI: %v", err)
	}

	builder, ok := storageBuilders[storageType]
	if !ok {
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	var params map[string]string
	if storageSpec.Parameters != nil {
		params = *storageSpec.Parameters
	}

	return builder(*storageSpec.StorageUri, params)
}

func buildOCIArgs(uri string, params map[string]string) ([]string, error) {
	components, err := storage.ParseOCIStorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid OCI storage URI: %v", err)
	}

	args := []string{
		"--upload-results",
		"--namespace", components.Namespace,
		"--storage-bucket", components.Bucket,
		"--storage-prefix", components.Prefix,
	}

	args = addParam(args, params, "auth", "--auth")
	args = addParam(args, params, "config_file", "--config-file")
	args = addParam(args, params, "profile", "--profile")
	args = addParam(args, params, "security_token", "--security-token")
	args = addParam(args, params, "region", "--region")

	return args, nil
}

func buildPVCArgs(uri string, _ map[string]string) ([]string, error) {
	components, err := storage.ParsePVCStorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid PVC storage URI: %v", err)
	}
	return []string{"--experiment-base-dir", "/" + components.SubPath}, nil
}

func buildS3Args(uri string, params map[string]string) ([]string, error) {
	components, err := storage.ParseS3StorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 storage URI: %v", err)
	}

	args := []string{
		"--upload-results",
		"--storage-provider", "aws",
		"--storage-bucket", components.Bucket,
	}

	if components.Prefix != "" {
		args = append(args, "--storage-prefix", components.Prefix)
	}

	args = addParam(args, params, "aws_access_key_id", "--storage-aws-access-key-id")
	args = addParam(args, params, "aws_secret_access_key", "--storage-aws-secret-access-key")
	args = addParam(args, params, "aws_profile", "--storage-aws-profile")

	// Region from params takes precedence over URI-derived region
	if region, ok := params["aws_region"]; ok {
		args = append(args, "--storage-aws-region", region)
	} else if components.Region != "" {
		args = append(args, "--storage-aws-region", components.Region)
	}

	return args, nil
}

func buildAzureArgs(uri string, params map[string]string) ([]string, error) {
	components, err := storage.ParseAzureStorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid Azure storage URI: %v", err)
	}

	args := []string{
		"--upload-results",
		"--storage-provider", "azure",
		"--storage-bucket", components.ContainerName,
	}

	if components.BlobPath != "" {
		args = append(args, "--storage-prefix", components.BlobPath)
	}

	// Account name from params takes precedence
	if accountName, ok := params["azure_account_name"]; ok {
		args = append(args, "--storage-azure-account-name", accountName)
	} else {
		args = append(args, "--storage-azure-account-name", components.AccountName)
	}

	args = addParam(args, params, "azure_account_key", "--storage-azure-account-key")
	args = addParam(args, params, "azure_connection_string", "--storage-azure-connection-string")
	args = addParam(args, params, "azure_sas_token", "--storage-azure-sas-token")

	return args, nil
}

func buildGCSArgs(uri string, params map[string]string) ([]string, error) {
	components, err := storage.ParseGCSStorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid GCS storage URI: %v", err)
	}

	args := []string{
		"--upload-results",
		"--storage-provider", "gcp",
		"--storage-bucket", components.Bucket,
	}

	if components.Object != "" {
		args = append(args, "--storage-prefix", components.Object)
	}

	args = addParam(args, params, "gcp_project_id", "--storage-gcp-project-id")
	args = addParam(args, params, "gcp_credentials_path", "--storage-gcp-credentials-path")

	return args, nil
}

func buildGitHubArgs(uri string, params map[string]string) ([]string, error) {
	components, err := storage.ParseGitHubStorageURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub storage URI: %v", err)
	}

	args := []string{
		"--upload-results",
		"--storage-provider", "github",
		"--github-owner", components.Owner,
		"--github-repo", components.Repository,
	}

	if components.Tag != "latest" {
		args = append(args, "--github-tag", components.Tag)
	}

	args = addParam(args, params, "github_token", "--github-token")

	return args, nil
}
