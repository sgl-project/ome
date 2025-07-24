package replica

import (
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/replicator"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

func NewReplicator(r *ReplicaAgent) (replicator.Replicator, error) {
	sourceStorageType := r.ReplicationInput.SourceStorageType
	targetStorageType := r.ReplicationInput.TargetStorageType
	switch {
	case sourceStorageType == storage.StorageTypeHuggingFace && targetStorageType == storage.StorageTypeOCI:
		return &replicator.HFToOCIReplicator{
			Logger: r.Logger,
			Config: replicator.HFToOCIReplicatorConfig{
				Logger:         r.Logger,
				LocalPath:      r.Config.LocalPath,
				NumConnections: r.Config.NumConnections,
				HubClient:      r.Config.Source.HubClient,
				OCIOSDataStore: r.Config.Target.OCIOSDataStore,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypeOCI && targetStorageType == storage.StorageTypeOCI:
		return &replicator.OCIToOCIReplicator{
			Logger: r.Logger,
			Config: replicator.OCIToOCIReplicatorConfig{
				Logger:               r.Logger,
				LocalPath:            r.Config.LocalPath,
				NumConnections:       r.Config.NumConnections,
				SourceOCIOSDataStore: r.Config.Source.OCIOSDataStore,
				TargetOCIOSDataStore: r.Config.Target.OCIOSDataStore,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported replication: %s → %s", sourceStorageType, targetStorageType)
	}
}
