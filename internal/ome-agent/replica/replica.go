package replica

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"

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
	r.Logger.Infof("Start replication from %s %v to %s %v with checksum config %+v", r.ReplicationInput.SourceStorageType, r.ReplicationInput.Source, r.ReplicationInput.TargetStorageType, r.ReplicationInput.Target, r.Config.Target.ChecksumConfig)

	sourceObjs, err := r.listSourceObjects()
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}

	r.validateModelSize(sourceObjs)

	replicatorImp, err := NewReplicator(r)
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}

	err = replicatorImp.Replicate(sourceObjs)
	if err != nil {
		r.writeTerminationLog(err.Error())
	}
	return err
}

func (r *ReplicaAgent) writeTerminationLog(message string) {
	f, err := os.OpenFile("/dev/termination-log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to open /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
		return
	}

	if _, err = fmt.Fprintln(f, message); err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to write to /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
	}

	if err = f.Close(); err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to close /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
	}
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
		repoFiles, err := r.Config.Source.HubClient.ListFiles(r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under model %s with %s branch", len(repoFiles), r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
		return common.ConvertToReplicationObjectsFromHFRepoFileInfo(repoFiles), nil
	case storage.StorageTypePVC:
		sourceDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Source.Prefix)
		files, err := r.Config.Source.PVCFileSystem.ListFiles(sourceDirPath)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under path %s", len(files), sourceDirPath)
		return common.ConvertToReplicationObjectsFromPVCFileEntry(files), nil
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
