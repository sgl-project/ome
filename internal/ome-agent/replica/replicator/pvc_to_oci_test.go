package replicator

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"github.com/stretchr/testify/assert"
)

func TestPVCToOCIReplicator_Replicate_Success(t *testing.T) {
	// Save the original function so we can restore it later
	originalUploadFunc := uploadDirectoryToOCIOSDataStoreFunc

	// Defer restoring original function
	defer func() {
		uploadDirectoryToOCIOSDataStoreFunc = originalUploadFunc
	}()

	// Replace uploadDirectoryToOCIOSDataStoreFunc with a mock version
	uploadCalled := false
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
		uploadCalled = true
		expectedPath := "/mnt/data/models"
		if localPath != expectedPath {
			t.Errorf("unexpected localPath: got %s, want %s", localPath, expectedPath)
		}
		if numObjects != 2 {
			t.Errorf("unexpected numObjects: got %d, want 2", numObjects)
		}
		if numConnections != 5 {
			t.Errorf("unexpected numConnections: got %d, want 5", numConnections)
		}
		return nil
	}

	replicator := &PVCToOCIReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: PVCToOCIReplicatorConfig{
			LocalPath:      "/mnt/data",
			NumConnections: 5,
			OCIOSDataStore: &ociobjectstore.OCIOSDataStore{},
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypePVC,
			TargetStorageType: storage.StorageTypeOCI,
			Source: ociobjectstore.ObjectURI{
				Namespace:  "default",
				BucketName: "model-pvc",
				Prefix:     "models",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "target-namespace",
				BucketName: "model-storage",
				Prefix:     "models",
			},
		},
	}

	err := replicator.Replicate(CreateCommonMockReplicationObjects(2))
	assert.NoError(t, err)
	assert.True(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should be called")
}

func TestPVCToOCIReplicator_Replicate_Failure(t *testing.T) {
	originalUploadFunc := uploadDirectoryToOCIOSDataStoreFunc
	defer func() {
		uploadDirectoryToOCIOSDataStoreFunc = originalUploadFunc
	}()

	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
		return fmt.Errorf("mock upload error")
	}

	replicator := &PVCToOCIReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: PVCToOCIReplicatorConfig{
			LocalPath:      "/tmp",
			NumConnections: 3,
			OCIOSDataStore: &ociobjectstore.OCIOSDataStore{},
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypePVC,
			TargetStorageType: storage.StorageTypeOCI,
			Source: ociobjectstore.ObjectURI{
				Namespace:  "default",
				BucketName: "model-pvc",
				Prefix:     "models",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "target-namespace",
				BucketName: "model-storage",
				Prefix:     "models",
			},
		},
	}

	objects := []common.ReplicationObject{NewMockReplicationObject()}
	err := replicator.Replicate(objects)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock upload error")
}

func TestPVCToOCIReplicator_Replicate_WithNilOCIOSDataStore(t *testing.T) {
	originalUploadFunc := uploadDirectoryToOCIOSDataStoreFunc
	defer func() {
		uploadDirectoryToOCIOSDataStoreFunc = originalUploadFunc
	}()

	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
		if ds == nil {
			return errors.New("OCIOSDataStore is nil")
		}
		return nil
	}

	replicator := &PVCToOCIReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: PVCToOCIReplicatorConfig{
			LocalPath:      "/mnt/data",
			NumConnections: 5,
			OCIOSDataStore: nil, // Explicitly set to nil
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypePVC,
			TargetStorageType: storage.StorageTypeOCI,
			Source: ociobjectstore.ObjectURI{
				Namespace:  "default",
				BucketName: "model-pvc",
				Prefix:     "models",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "target-namespace",
				BucketName: "model-storage",
				Prefix:     "models",
			},
		},
	}

	objects := []common.ReplicationObject{NewMockReplicationObject()}
	err := replicator.Replicate(objects)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OCIOSDataStore is nil")
}
