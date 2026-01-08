package basemodel

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

const (
	// DefaultSecretTokenKey is the default key name for HuggingFace tokens in secrets
	DefaultSecretTokenKey = "token"
)

var log = logf.Log.WithName(constants.BaseModelValidatorWebhookName)
var clusterLog = logf.Log.WithName(constants.ClusterBaseModelValidatorWebhookName)

// BaseModelValidator validates BaseModel objects
// +kubebuilder:webhook:path=/validate-ome-io-v1beta1-basemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=basemodels,versions=v1beta1,name=basemodel.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1
type BaseModelValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// Handle implements admission.Handler for BaseModel validation
func (v *BaseModelValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	baseModel := &v1beta1.BaseModel{}

	if err := v.Decoder.Decode(req, baseModel); err != nil {
		log.Error(err, "Failed to decode BaseModel", "name", req.Name, "namespace", req.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	log.Info("Validating BaseModel", "name", baseModel.Name, "namespace", baseModel.Namespace)

	// Validate storage URI
	return validateHuggingFaceStorage(ctx, v.Client, baseModel.Spec.Storage, baseModel.Namespace, log)
}

// ClusterBaseModelValidator validates ClusterBaseModel objects
// +kubebuilder:webhook:path=/validate-ome-io-v1beta1-clusterbasemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=clusterbasemodels,versions=v1beta1,name=clusterbasemodel.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1
type ClusterBaseModelValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// Handle implements admission.Handler for ClusterBaseModel validation
func (v *ClusterBaseModelValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	clusterBaseModel := &v1beta1.ClusterBaseModel{}

	if err := v.Decoder.Decode(req, clusterBaseModel); err != nil {
		clusterLog.Error(err, "Failed to decode ClusterBaseModel", "name", req.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	clusterLog.Info("Validating ClusterBaseModel", "name", clusterBaseModel.Name)

	// Validate storage URI - use OME namespace for secrets since ClusterBaseModel is cluster-scoped
	return validateHuggingFaceStorage(ctx, v.Client, clusterBaseModel.Spec.Storage, constants.OMENamespace, clusterLog)
}

// validateHuggingFaceStorage validates the storage URI for HuggingFace models
// This is a shared helper function used by both BaseModelValidator and ClusterBaseModelValidator
func validateHuggingFaceStorage(ctx context.Context, k8sClient client.Client, storage *v1beta1.StorageSpec, namespace string, logger logr.Logger) admission.Response {
	// Skip validation if storage is nil or storageUri is nil
	if storage == nil || storage.StorageUri == nil {
		return admission.Allowed("no storage URI specified")
	}

	storageURI := *storage.StorageUri

	// Only validate HuggingFace URIs
	if !IsHuggingFaceURI(storageURI) {
		return admission.Allowed("non-HuggingFace storage URI, skipping validation")
	}

	// Get the token from the secret if specified
	token := ""
	if storage.StorageKey != nil && *storage.StorageKey != "" {
		var err error
		token, err = getTokenFromSecret(ctx, k8sClient, *storage.StorageKey, namespace, storage.Parameters)
		if err != nil {
			logger.Info("Failed to retrieve token from secret, proceeding without authentication",
				"secret", *storage.StorageKey,
				"namespace", namespace)
		}
	}

	// Validate the HuggingFace model
	result := ValidateHuggingFaceStorageURI(ctx, storageURI, token)

	// Build the response
	if !result.Valid {
		logger.Info("HuggingFace model validation failed",
			"storageUri", storageURI,
			"error", result.ErrorMessage)
		return admission.Denied(result.ErrorMessage)
	}

	// Create response with warnings if any
	response := admission.Allowed("validation passed")
	if result.WarningMessage != "" {
		logger.Info("HuggingFace model validation passed with warning",
			"storageUri", storageURI,
			"warning", result.WarningMessage)
		response = response.WithWarnings(result.WarningMessage)
	}

	return response
}

// getTokenFromSecret retrieves the authentication token from a Kubernetes secret
func getTokenFromSecret(ctx context.Context, k8sClient client.Client, secretName string, namespace string, parameters *map[string]string) (string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Determine the key to use for the token
	secretKey := DefaultSecretTokenKey
	if parameters != nil {
		if customKey, exists := (*parameters)["secretKey"]; exists && customKey != "" {
			secretKey = customKey
		}
	}

	tokenBytes, exists := secret.Data[secretKey]
	if !exists {
		return "", fmt.Errorf("secret %s/%s does not contain key %q", namespace, secretName, secretKey)
	}

	return string(tokenBytes), nil
}
