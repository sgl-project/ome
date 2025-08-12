package replicator

import (
	"path/filepath"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type PVCToOCIReplicator struct {
	Logger           logging.Interface
	Config           PVCToOCIReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type PVCToOCIReplicatorConfig struct {
	LocalPath      string
	NumConnections int
	ChecksumConfig *common.ChecksumConfig
	OCIOSDataStore *ociobjectstore.OCIOSDataStore
}

func (r *PVCToOCIReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")

	sourceDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Source.Prefix)
	if err := uploadDirectoryToOCIOSDataStoreFunc(
		r.Config.OCIOSDataStore,
		r.ReplicationInput.Target,
		sourceDirPath,
		r.Config.ChecksumConfig,
		len(objects),
		r.Config.NumConnections,
	); err != nil {
		r.Logger.Errorf("Failed to upload files under %s to OCI Object Storage %v: %v", sourceDirPath, r.ReplicationInput.Target, err)
		return err
	}
	r.Logger.Infof("All files under %s uploaded successfully", sourceDirPath)
	r.Logger.Infof("Replication completed from PVC %s to OCI Object Storage", r.ReplicationInput.Source.BucketName)
	return nil
}
