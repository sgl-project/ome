package replicator

import (
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/logging"
	"os"
	"path/filepath"
)

type PVCToPVCReplicator struct {
	Logger           logging.Interface
	Config           PVCToPVCReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type PVCToPVCReplicatorConfig struct {
	LocalPath           string
	SourcePVCFileSystem *afero.OsFs
	TargetPVCFileSystem *afero.OsFs
}

func (r *PVCToPVCReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")
	if r.ReplicationInput.Source.Namespace == r.ReplicationInput.Target.Namespace &&
		r.ReplicationInput.Source.BucketName == r.ReplicationInput.Target.BucketName &&
		r.ReplicationInput.Source.Prefix == r.ReplicationInput.Target.Prefix {
		r.Logger.Info("Source PVC and target PVC are the same, no replication needed")
		return nil
	}

	sourceDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
	targetDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Target.BucketName, r.ReplicationInput.Target.Prefix)

	err := afero.Walk(r.Config.SourcePVCFileSystem, sourceDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing %q: %w", path, err)
		}

		relPath, err := filepath.Rel(sourceDirPath, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}

		destPath := filepath.Join(targetDirPath, relPath)

		if info.IsDir() {
			return r.Config.TargetPVCFileSystem.MkdirAll(destPath, info.Mode())
		}

		return afero.CopyFileBetweenFS(r.Config.SourcePVCFileSystem, r.Config.TargetPVCFileSystem, path, destPath, info.Mode())
	})

	if err != nil {
		return fmt.Errorf("replication failed: %w", err)
	}

	r.Logger.Infof("Replication completed successfully for PVC %s under path %s to PVC %s under path %s",
		r.ReplicationInput.Source.BucketName,
		r.ReplicationInput.Source.Prefix,
		r.ReplicationInput.Target.BucketName,
		r.ReplicationInput.Target.Prefix)
	return nil
}
