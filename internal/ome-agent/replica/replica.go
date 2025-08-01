package replica

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"

	"golang.org/x/net/context"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	GB = 1073741824

	SourceStorageConfigKeyName = "source"
	TargetStorageConfigKeyName = "target"
)

type ReplicaAgent struct {
	Logger           logging.Interface
	Config           Config
	ReplicationInput common.ReplicationInput
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
		if !strings.HasSuffix(sourceObjectURI.Prefix, "/") && sourceObjectURI.Prefix != "" {
			sourceObjectURI.Prefix += "/"
		}
	}
	if targetStorageType == storage.StorageTypeOCI {
		targetObjectURI.Region = config.Target.OCIOSDataStore.Config.Region
		if !strings.HasSuffix(targetObjectURI.Prefix, "/") && targetObjectURI.Prefix != "" {
			targetObjectURI.Prefix += "/"
		}
	}

	return &ReplicaAgent{
		Logger: config.AnotherLogger,
		Config: *config,
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: sourceStorageType,
			TargetStorageType: targetStorageType,
			Source:            *sourceObjectURI,
			Target:            *targetObjectURI,
		},
	}, nil
}

// Start initiates the replication process.
func (r *ReplicaAgent) Start() error {
	r.Logger.Infof("Start replication from %s %v to %s %v", r.ReplicationInput.SourceStorageType, r.ReplicationInput.Source, r.ReplicationInput.TargetStorageType, r.ReplicationInput.Target)

	sourceObjs, err := r.listSourceObjects()
	if err != nil {
		return err
	}

	r.validateModelSize(sourceObjs)

	replicatorImp, err := NewReplicator(r)
	if err != nil {
		return err
	}

	return replicatorImp.Replicate(sourceObjs)
}

func (r *ReplicaAgent) listSourceObjects() ([]common.ReplicationObject, error) {
	switch r.ReplicationInput.SourceStorageType {
	case storage.StorageTypeOCI:
		listOfObjectSummary, err := r.Config.Source.OCIOSDataStore.ListObjects(r.ReplicationInput.Source)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight objects under prefix %s", len(listOfObjectSummary), r.ReplicationInput.Source.Prefix)
		return common.ConvertToReplicationObjectsFromObjectSummary(listOfObjectSummary), nil
	case storage.StorageTypeHuggingFace:
		repoFiles, err := r.Config.Source.HubClient.ListFiles(context.Background(), r.ReplicationInput.Source.BucketName, hub.WithRepoType(hub.RepoTypeModel))
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under model %s with %s branch", len(repoFiles), r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
		return common.ConvertToReplicationObjectsFromRepoFile(repoFiles), nil
	case storage.StorageTypePVC:
		sourceDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Source.Prefix)
		files, err := r.Config.Source.PVCFileSystem.ListFiles(sourceDirPath)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under path %s", len(files), sourceDirPath)
		return common.ConvertToReplicationObjectsFromFileInfo(files), nil
	default:
		return nil, fmt.Errorf("unsupported source storage type: %s", string(r.ReplicationInput.SourceStorageType))
	}
}

func (r *ReplicaAgent) validateModelSize(objects []common.ReplicationObject) {
	r.Logger.Info("Calculating model size from source")

	sizeLimit := int64(r.Config.DownloadSizeLimitGB) * GB
	var totalSize int64

	for _, object := range objects {
		if object.GetName() == "" || object.GetSize() == 0 {
			r.Logger.Errorf("Invalid object with missing name or size: %+v", object)
			continue
		}

		totalSize += object.GetSize()
		if r.Config.EnableSizeLimitCheck && totalSize > sizeLimit {
			r.Logger.Fatalf("Model weights exceed size limit of %d bytes", sizeLimit)
		}
	}

	if totalSize == 0 {
		r.Logger.Fatal("No model weights exist in the model folder")
	}
	r.Logger.Infof("Total model size: %d bytes", totalSize)
}
