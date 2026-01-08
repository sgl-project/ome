package basemodel

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

var log = logf.Log.WithName(constants.BaseModelValidatorWebhookName)
var clusterLog = logf.Log.WithName(constants.ClusterBaseModelValidatorWebhookName)

// BaseModelValidator validates BaseModel objects
// +kubebuilder:webhook:path=/validate-ome-io-v1beta1-basemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=basemodels,versions=v1beta1,name=basemodel.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1
type BaseModelValidator struct {
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

	// Validate storage URI format only - HuggingFace API validation is done in the reconciler
	return validateStorageURIFormat(baseModel.Spec.Storage, log)
}

// ClusterBaseModelValidator validates ClusterBaseModel objects
// +kubebuilder:webhook:path=/validate-ome-io-v1beta1-clusterbasemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=clusterbasemodels,versions=v1beta1,name=clusterbasemodel.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1
type ClusterBaseModelValidator struct {
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

	// Validate storage URI format only - HuggingFace API validation is done in the reconciler
	return validateStorageURIFormat(clusterBaseModel.Spec.Storage, clusterLog)
}

// validateStorageURIFormat validates the storage URI format for HuggingFace models
// This only validates the format (org/model), not whether the model exists on HuggingFace
func validateStorageURIFormat(storage *v1beta1.StorageSpec, logger logr.Logger) admission.Response {
	// Skip validation if storage is nil or storageUri is nil
	if storage == nil || storage.StorageUri == nil {
		return admission.Allowed("no storage URI specified")
	}

	storageURI := *storage.StorageUri

	// Only validate HuggingFace URIs
	if !IsHuggingFaceURI(storageURI) {
		return admission.Allowed("non-HuggingFace storage URI, skipping format validation")
	}

	// Validate the model ID format only
	result := ValidateModelIDFormat(storageURI)

	if !result.Valid {
		logger.Info("HuggingFace model ID format validation failed",
			"storageUri", storageURI,
			"error", result.ErrorMessage)
		return admission.Denied(result.ErrorMessage)
	}

	return admission.Allowed("storage URI format validation passed")
}
