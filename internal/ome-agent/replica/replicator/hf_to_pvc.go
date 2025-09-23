package replicator

import (
	"path/filepath"

	"github.com/sgl-project/ome/pkg/xet"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
)

type HFToPVCReplicator struct {
	Logger           logging.Interface
	Config           HFToPVCReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type HFToPVCReplicatorConfig struct {
	LocalPath string
	HubClient *xet.Client
}

func (r *HFToPVCReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")

	targetDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Target.Prefix)
	downloadPath, err := downloadFromHFFunc(r.ReplicationInput, r.Config.HubClient, targetDirPath, r.Logger)
	if err != nil {
		r.Logger.Errorf("Failed to download model %s from HuggingFace: %v", r.ReplicationInput.Source.BucketName, err)
		return err
	}
	r.Logger.Infof("Successfully downloaded model %s from HF to %s ", r.ReplicationInput.Source.BucketName, downloadPath)

	r.Logger.Infof("Replication completed from HuggingFace to PVC %s for model %s", r.ReplicationInput.Target.BucketName, r.ReplicationInput.Source.BucketName)
	return nil
}
