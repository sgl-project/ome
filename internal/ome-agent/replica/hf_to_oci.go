package replica

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
)

type HFToOCIReplicator struct {
	logger logging.Interface
	Config Config
}

func (r *HFToOCIReplicator) Replicate(objects []ReplicationObject) error {
	r.logger.Info("Starting replication to target")
	var downloadOptions []hub.DownloadOption
	// Set revision if specified
	if r.Config.SourceObjectStoreURI.Prefix != "" {
		downloadOptions = append(downloadOptions, hub.WithRevision(r.Config.SourceObjectStoreURI.Prefix))
	}
	// Set repository type (always model for HuggingFace)
	downloadOptions = append(downloadOptions, hub.WithRepoType(hub.RepoTypeModel))

	downloadPath, err := r.Config.HubClient.SnapshotDownload(
		context.Background(),
		r.Config.SourceObjectStoreURI.BucketName,
		r.Config.LocalPath,
		downloadOptions...,
	)
	if err != nil {
		r.logger.Errorf("Failed to download model %s from HF: %+v", r.Config.SourceObjectStoreURI.BucketName, err)
		return fmt.Errorf("model download failed: %w", err)
	}

	r.logger.Infof("Successfully downloaded model %s from HF", r.Config.SourceObjectStoreURI.BucketName)
	r.logger.Infof("Downloaded to: %s", downloadPath)
	return nil
}
