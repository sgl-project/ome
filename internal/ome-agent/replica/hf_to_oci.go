package replica

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
)

type HFToOCIReplicator struct {
	logger           logging.Interface
	Config           Config
	ReplicationInput ReplicationInput
}

func (r *HFToOCIReplicator) Replicate(objects []ReplicationObject) error {
	r.logger.Info("Starting replication to target")
	var downloadOptions []hub.DownloadOption
	// Set revision if specified
	if r.ReplicationInput.source.Prefix != "" {
		downloadOptions = append(downloadOptions, hub.WithRevision(r.ReplicationInput.source.Prefix))
	}
	// Set repository type (always model for HuggingFace)
	downloadOptions = append(downloadOptions, hub.WithRepoType(hub.RepoTypeModel))

	downloadPath, err := r.Config.HubClient.SnapshotDownload(
		context.Background(),
		r.ReplicationInput.source.BucketName,
		r.Config.LocalPath,
		downloadOptions...,
	)
	if err != nil {
		r.logger.Errorf("Failed to download model %s from HF: %+v", r.ReplicationInput.source.BucketName, err)
		return fmt.Errorf("model download failed: %w", err)
	}

	r.logger.Infof("Successfully downloaded model %s from HF", r.ReplicationInput.source.BucketName)
	r.logger.Infof("Downloaded to: %s", downloadPath)
	return nil
}
