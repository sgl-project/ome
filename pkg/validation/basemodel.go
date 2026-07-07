// Package validation holds pure validation helpers (no Kubernetes client
// dependencies) shared between CRD validators and admission webhooks.
package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

// ValidatePVCStorage enforces the pvc:// URI rules for a BaseModel /
// ClusterBaseModel spec:
//
//   - namespaced BaseModel: pvc:// URIs must NOT carry a namespace prefix
//     (pvc://{pvc-name}/{sub-path}); the model's own namespace is used.
//   - ClusterBaseModel: pvc:// URIs MUST carry a namespace prefix
//     (pvc://{namespace}:{pvc-name}/{sub-path}).
//   - the URI itself must parse (a sub-path is required).
//
// Non-PVC URIs are accepted unchanged.
func ValidatePVCStorage(spec *v1beta1.BaseModelSpec, isClusterScoped bool) error {
	if spec == nil || spec.Storage == nil || spec.Storage.StorageUri == nil {
		return nil
	}
	uri := *spec.Storage.StorageUri
	storageType, err := storage.GetStorageType(uri)
	if err != nil || storageType != storage.StorageTypePVC {
		return nil
	}

	components, err := storage.ParsePVCStorageURI(uri)
	if err != nil {
		return fmt.Errorf("invalid PVC storage URI %q: %w", uri, err)
	}
	if isClusterScoped && components.Namespace == "" {
		return fmt.Errorf("ClusterBaseModel PVC URI must specify a namespace (format: pvc://{namespace}:{pvc-name}/{sub-path}), got %q", uri)
	}
	if !isClusterScoped && components.Namespace != "" {
		return fmt.Errorf("namespaced BaseModel PVC URI must not specify a namespace; the BaseModel's own namespace is used, got %q", uri)
	}
	return nil
}
