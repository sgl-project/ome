package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

// ValidatePVCStorage enforces the PVC URI shape rules:
//
//   - namespaced BaseModel: pvc:// URIs must NOT carry a namespace prefix
//   - ClusterBaseModel: pvc:// URIs MUST carry a namespace prefix
//   - the URI itself must parse
//   - PVC + distribution=Sharded is rejected
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

	if spec.Distribution != nil && *spec.Distribution == v1beta1.DistributionSharded {
		return fmt.Errorf("PVC storage URIs are not compatible with distribution=Sharded; use distribution=PerNode (or omit) for pvc:// models")
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
