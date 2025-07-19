package replica

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 20
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	GB                           = 1073741824

	SourceStorageConfigKeyName = "source"
	TargetStorageConfigKeyName = "target"
)

type ReplicaAgent struct {
	logger           logging.Interface
	Config           Config
	ReplicationInput ReplicationInput
}

type ReplicationInput struct {
	sourceStorageType storage.StorageType
	targetStorageType storage.StorageType
	source            ociobjectstore.ObjectURI
	target            ociobjectstore.ObjectURI
}

// NewReplicaAgent constructs a new replica agent from the given configuration.
func NewReplicaAgent(config *Config) (*ReplicaAgent, error) {
	sourceStorageType, err := storage.GetStorageType(config.Source.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get source storage type from source storage URI %s - %w",
			config.Source.StorageURIStr, err)
	}
	targetStorageType, err := storage.GetStorageType(config.Target.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get target storage type from target storage URI %s - %w",
			config.Target.StorageURIStr, err)
	}

	if err = config.ValidateRequiredDependencies(sourceStorageType, targetStorageType); err != nil {
		return nil, fmt.Errorf("failed to validate required dependencies - %w", err)
	}

	sourceObjectURI, err := storage.NewObjectURI(config.Source.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source storage URI %s - %w", config.Source.StorageURIStr, err)
	}
	targetObjectURI, err := storage.NewObjectURI(config.Target.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target storage URI %s - %w", config.Target.StorageURIStr, err)
	}

	if sourceStorageType == storage.StorageTypeOCI {
		sourceObjectURI.Region = config.Source.OCIOSDataStore.Config.Region
	}
	if targetStorageType == storage.StorageTypeOCI {
		targetObjectURI.Region = config.Target.OCIOSDataStore.Config.Region
	}

	return &ReplicaAgent{
		logger: config.AnotherLogger,
		Config: *config,
		ReplicationInput: ReplicationInput{
			sourceStorageType: sourceStorageType,
			targetStorageType: targetStorageType,
			source:            *sourceObjectURI,
			target:            *targetObjectURI,
		},
	}, nil
}

// Start initiates the replication process.
func (r *ReplicaAgent) Start() error {
	r.logger.Infof("Start replication from %+v to %+v", r.ReplicationInput.source, r.ReplicationInput.target)

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
	switch r.ReplicationInput.sourceStorageType {
	case storage.StorageTypeOCI:
		listOfObjectSummary, err := r.Config.Source.OCIOSDataStore.ListObjects(r.ReplicationInput.source)
		if err != nil {
			return nil, err
		}
		r.logger.Infof("Listed %d model weight objects under prefix %s", len(listOfObjectSummary), r.ReplicationInput.source.Prefix)
		return convertToReplicationObjectsFromObjectSummary(listOfObjectSummary), nil
	case storage.StorageTypeHuggingFace:
		repoFiles, err := r.Config.Source.HubClient.ListFiles(context.Background(), r.ReplicationInput.source.BucketName, hub.WithRepoType(hub.RepoTypeModel))
		if err != nil {
			return nil, err
		}
		r.logger.Infof("Listed %d model weight files under model %s with %s branch", len(repoFiles), r.ReplicationInput.source.BucketName, r.ReplicationInput.source.Prefix)
		return convertToReplicationObjectsFromRepoFile(repoFiles), nil
	default:
		return nil, fmt.Errorf("unsupported source storage type: %s", string(r.ReplicationInput.sourceStorageType))
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
