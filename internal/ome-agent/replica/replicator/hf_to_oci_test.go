package replicator

import (
	"errors"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

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
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *hub.HubClient, downloadDir string, logger logging.Interface) (string, error) {
		downloadCalled = true
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, numObjects int, numConnections int) error {
		uploadCalled = true
		return nil
	}

	logger := testingPkg.SetupMockLogger()
	replicator := &HFToOCIReplicator{
		Logger: logger,
		Config: HFToOCIReplicatorConfig{
			LocalPath:      "/tmp/model",
			NumConnections: 1,
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypeHuggingFace,
			TargetStorageType: storage.StorageTypeOCI,
			Source:            ociobjectstore.ObjectURI{BucketName: "meta-llama/llama-3-70b-instruct"},
			Target:            ociobjectstore.ObjectURI{BucketName: "target-bucket"},
		},
	}
	objs := CreateCommonMockReplicationObjects(1)
	err := replicator.Replicate(objs)
	assert.NoError(t, err)
	assert.True(t, downloadCalled, "downloadFromHF should be called")
	assert.True(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should be called")

	// Test download error
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *hub.HubClient, downloadDir string, logger logging.Interface) (string, error) {
		return "", errors.New("download error")
	}
	uploadCalled = false
	err = replicator.Replicate(objs)
	assert.Error(t, err)
	assert.False(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should not be called if download fails")

	// Test upload error
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *hub.HubClient, downloadDir string, logger logging.Interface) (string, error) {
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, numObjects int, numConnections int) error {
		return errors.New("upload error")
	}
	err = replicator.Replicate(objs)
	assert.Error(t, err)
}
