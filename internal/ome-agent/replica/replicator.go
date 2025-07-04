package replica

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/utils/storage"
)

type Replicator interface {
	Replicate(objects []ReplicationObject) error
}

func NewReplicator(r *ReplicaAgent) (Replicator, error) {
	sourceStorageType := r.ReplicationInput.sourceStorageType
	targetStorageType := r.ReplicationInput.targetStorageType
	switch {
	case sourceStorageType == storage.StorageTypeHuggingFace && targetStorageType == storage.StorageTypeOCI:
		return &HFToOCIReplicator{
			logger:           r.logger,
			Config:           r.Config,
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypeOCI && targetStorageType == storage.StorageTypeOCI:
		return &OCIToOCIReplicator{
			logger:           r.logger,
			Config:           r.Config,
			ReplicationInput: r.ReplicationInput,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported replication: %s → %s", sourceStorageType, targetStorageType)
	}
}
