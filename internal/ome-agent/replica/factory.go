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
				LocalPath:            r.Config.LocalPath,
				NumConnections:       r.Config.NumConnections,
				SourceOCIOSDataStore: r.Config.Source.OCIOSDataStore,
				TargetOCIOSDataStore: r.Config.Target.OCIOSDataStore,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypePVC && targetStorageType == storage.StorageTypeOCI:
		return &replicator.PVCToOCIReplicator{
			Logger: r.Logger,
			Config: replicator.PVCToOCIReplicatorConfig{
				LocalPath:      r.Config.LocalPath,
				NumConnections: r.Config.NumConnections,
				OCIOSDataStore: r.Config.Target.OCIOSDataStore,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypeHuggingFace && targetStorageType == storage.StorageTypePVC:
		return &replicator.HFToPVCReplicator{
			Logger: r.Logger,
			Config: replicator.HFToPVCReplicatorConfig{
				LocalPath: r.Config.LocalPath,
				HubClient: r.Config.Source.HubClient,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypeOCI && targetStorageType == storage.StorageTypePVC:
		return &replicator.OCIToPVCReplicator{
			Logger: r.Logger,
			Config: replicator.OCIToPVCReplicatorConfig{
				LocalPath:      r.Config.LocalPath,
				NumConnections: r.Config.NumConnections,
				OCIOSDataStore: r.Config.Source.OCIOSDataStore,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	case sourceStorageType == storage.StorageTypePVC && targetStorageType == storage.StorageTypePVC:
		return &replicator.PVCToPVCReplicator{
			Logger: r.Logger,
			Config: replicator.PVCToPVCReplicatorConfig{
				LocalPath:           r.Config.LocalPath,
				SourcePVCFileSystem: r.Config.Source.PVCFileSystem,
				TargetPVCFileSystem: r.Config.Target.PVCFileSystem,
			},
			ReplicationInput: r.ReplicationInput,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported replication: %s → %s", sourceStorageType, targetStorageType)
	}
}
