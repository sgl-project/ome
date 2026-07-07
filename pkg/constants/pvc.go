package constants

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	// PVCStorageConfigMapLabel marks a per-PVC model status ConfigMap
	// (written by the metadata-extraction Job) in the OME namespace, as
	// distinct from the per-node model-agent status ConfigMaps.
	PVCStorageConfigMapLabel = "models.ome/pvc-status"
	// PVCMetadataModelNameLabel records the BaseModel/ClusterBaseModel name
	// on the metadata Job and its per-PVC status ConfigMap.
	PVCMetadataModelNameLabel = "models.ome/model-name"
	// PVCMetadataScopeLabel is "namespaced" or "cluster".
	PVCMetadataScopeLabel = "models.ome/model-scope"
	// PVCMetadataLastErrorAnnotation captures the last extraction error the
	// agent reported on the per-PVC status ConfigMap.
	PVCMetadataLastErrorAnnotation = "models.ome/pvc-metadata-last-error"
)

// pvcMetadataConfigMapPrefix and PVCMetadataNameHashLen together define the
// deterministic per-PVC status ConfigMap name.
const (
	pvcMetadataConfigMapPrefix = "pvc-metadata-"
	PVCMetadataNameHashLen     = 8
)

// GetPVCMetadataConfigMapName returns the deterministic name of the per-PVC
// status ConfigMap for a model. Cluster-scoped models key on the model name
// alone; namespaced models key on namespace/name.
func GetPVCMetadataConfigMapName(modelName, modelNamespace string, isClusterScoped bool) string {
	keySource := modelName
	if !isClusterScoped {
		keySource = modelNamespace + "/" + modelName
	}
	sum := sha256.Sum256([]byte(keySource))
	return pvcMetadataConfigMapPrefix + hex.EncodeToString(sum[:])[:PVCMetadataNameHashLen]
}
