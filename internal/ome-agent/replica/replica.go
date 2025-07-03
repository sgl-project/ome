package replica

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"golang.org/x/net/context"
)

const (
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 20
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	GB                           = 1073741824
)

type ReplicaAgent struct {
	logger logging.Interface
	Config Config
}

type ReplicationResult struct {
	source ociobjectstore.ObjectURI
	target ociobjectstore.ObjectURI
	error  error
}

// NewReplicaAgent constructs a new replica agent from the given configuration.
func NewReplicaAgent(config *Config) (*ReplicaAgent, error) {
	return &ReplicaAgent{
		logger: config.AnotherLogger,
		Config: *config,
	}, nil
}

// Start initiates the replication process.
func (r *ReplicaAgent) Start() error {
	r.logger.Infof("Start replication from %s to %s", r.Config.SourceObjectStoreURI, r.Config.TargetObjectStoreURI)

	sourceObjs, err := r.listSourceObjects()
	if err != nil {
		return err
	}
	r.validateModelSize(sourceObjs)

	replicator, err := NewReplicator(r)
	if err != nil {
		return err
	}

	return replicator.Replicate(sourceObjs)
}

func (r *ReplicaAgent) listSourceObjects() ([]ReplicationObject, error) {
	switch r.Config.SourceStorageType {
	case storage.StorageTypeOCI:
		r.Config.ObjectStorageDataStore.SetRegion(r.Config.SourceObjectStoreURI.Region)
		listOfObjectSummary, err := r.Config.ObjectStorageDataStore.ListObjects(r.Config.SourceObjectStoreURI)
		if err != nil {
			return nil, err
		}
		r.logger.Infof("Listed %d model weight objects under prefix %s", len(listOfObjectSummary), r.Config.SourceObjectStoreURI.Prefix)
		return convertToReplicationObjectsFromObjectSummary(listOfObjectSummary), nil
	case storage.StorageTypeHuggingFace:
		repoFiles, err := r.Config.HubClient.ListFiles(context.Background(), r.Config.SourceObjectStoreURI.BucketName, hub.WithRepoType(hub.RepoTypeModel))
		if err != nil {
			return nil, err
		}
		r.logger.Infof("Listed %d model weight files under model %s with %s branch", len(repoFiles), r.Config.SourceObjectStoreURI.BucketName, r.Config.SourceObjectStoreURI.Prefix)
		return convertToReplicationObjectsFromRepoFile(repoFiles), nil
	default:
		return nil, fmt.Errorf("unsupported source storage type: %s", r.Config.SourceStorageType)
	}
}

func (r *ReplicaAgent) validateModelSize(objects []ReplicationObject) {
	r.logger.Info("Calculating model size from source")

	sizeLimit := int64(r.Config.DownloadSizeLimitGB) * GB
	var totalSize int64

	for _, object := range objects {
		if object.GetName() == "" || object.GetSize() == 0 {
			r.logger.Errorf("Invalid object with missing name or size: %+v", object)
			continue
		}

		totalSize += object.GetSize()
		if r.Config.EnableSizeLimitCheck && totalSize > sizeLimit {
			r.logger.Fatalf("Model weights exceed size limit of %d bytes", sizeLimit)
		}
	}

	if totalSize == 0 {
		r.logger.Fatal("No model weights exist in the model folder")
	}
	r.logger.Infof("Total model size: %d bytes", totalSize)
}
