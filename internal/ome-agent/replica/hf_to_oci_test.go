package replica

import (
	"errors"
	"testing"

	"github.com/sgl-project/ome/pkg/utils/storage"

	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type mockReplicationObject struct{}

func (m mockReplicationObject) GetName() string { return "file1" }
func (m mockReplicationObject) GetPath() string { return "file1" }
func (m mockReplicationObject) GetSize() int64  { return 123 }

func TestHFToOCIReplicator_Replicate(t *testing.T) {
	// Save original functions
	origDownloadFromHF := downloadFromHFFunc
	origUploadDirectoryToOCIOSDataStore := uploadDirectoryToOCIOSDataStoreFunc

	t.Cleanup(func() {
		downloadFromHFFunc = origDownloadFromHF
		uploadDirectoryToOCIOSDataStoreFunc = origUploadDirectoryToOCIOSDataStore
	})

	downloadCalled := false
	uploadCalled := false
	downloadFromHFFunc = func(input ReplicationInput, config Config) (string, error) {
		downloadCalled = true
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, numObjects int, numConnections int) error {
		uploadCalled = true
		return nil
	}

	logger := testingPkg.SetupMockLogger()
	replicator := &HFToOCIReplicator{
		logger: logger,
		Config: Config{
			AnotherLogger:  logger,
			LocalPath:      "/tmp/model",
			NumConnections: 1,
		},
		ReplicationInput: ReplicationInput{
			sourceStorageType: storage.StorageTypeHuggingFace,
			targetStorageType: storage.StorageTypeOCI,
			source:            ociobjectstore.ObjectURI{BucketName: "meta-llama/llama-3-70b-instruct"},
			target:            ociobjectstore.ObjectURI{BucketName: "target-bucket"},
		},
	}
	objs := []ReplicationObject{mockReplicationObject{}}
	err := replicator.Replicate(objs)
	assert.NoError(t, err)
	assert.True(t, downloadCalled, "downloadFromHF should be called")
	assert.True(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should be called")

	// Test download error
	downloadFromHFFunc = func(input ReplicationInput, config Config) (string, error) {
		return "", errors.New("download error")
	}
	uploadCalled = false
	err = replicator.Replicate(objs)
	assert.Error(t, err)
	assert.False(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should not be called if download fails")

	// Test upload error
	downloadFromHFFunc = func(input ReplicationInput, config Config) (string, error) {
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, numObjects int, numConnections int) error {
		return errors.New("upload error")
	}
	err = replicator.Replicate(objs)
	assert.Error(t, err)
}
