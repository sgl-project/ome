package replicator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/ome/pkg/xet"

	"sigs.k8s.io/ome/internal/ome-agent/replica/common"
	"sigs.k8s.io/ome/pkg/logging"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/ome/pkg/ociobjectstore"
	testingPkg "sigs.k8s.io/ome/pkg/testing"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

func TestHFToOCIReplicator_Replicate_Success(t *testing.T) {
	// Save original functions
	origDownloadFromHF := downloadFromHFFunc
	origUploadDirectoryToOCIOSDataStore := uploadDirectoryToOCIOSDataStoreFunc

	t.Cleanup(func() {
		downloadFromHFFunc = origDownloadFromHF
		uploadDirectoryToOCIOSDataStoreFunc = origUploadDirectoryToOCIOSDataStore
	})

	downloadCalled := false
	uploadCalled := false
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *xet.Client, downloadDir string, opts hfDownloadOptions, logger logging.Interface) (string, error) {
		downloadCalled = true
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
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
			Target:            ociobjectstore.ObjectURI{BucketName: "target-bucket", Namespace: "target-bucket-ns", Prefix: "target-prefix/"},
		},
	}
	objs := CreateCommonMockReplicationObjects(1)
	err := replicator.Replicate(objs)
	assert.NoError(t, err)
	assert.True(t, downloadCalled, "downloadFromHF should be called")
	assert.True(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should be called")
}

func TestHFToOCIReplicator_Replicate_Failure(t *testing.T) {
	// Save original functions
	origDownloadFromHF := downloadFromHFFunc
	origUploadDirectoryToOCIOSDataStore := uploadDirectoryToOCIOSDataStoreFunc

	t.Cleanup(func() {
		downloadFromHFFunc = origDownloadFromHF
		uploadDirectoryToOCIOSDataStoreFunc = origUploadDirectoryToOCIOSDataStore
	})

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
			Target:            ociobjectstore.ObjectURI{BucketName: "target-bucket", Namespace: "target-bucket-ns", Prefix: "target-prefix/"},
		},
	}
	objs := CreateCommonMockReplicationObjects(1)

	// Test download error
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *xet.Client, downloadDir string, opts hfDownloadOptions, logger logging.Interface) (string, error) {
		return "", errors.New("download error")
	}
	uploadCalled := false
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
		uploadCalled = true
		return nil
	}
	err := replicator.Replicate(objs)
	assert.Error(t, err)
	assert.False(t, uploadCalled, "uploadDirectoryToOCIOSDataStore should not be called if download fails")
	assert.ErrorContains(t, err, "download error")

	// Test upload error
	downloadFromHFFunc = func(input common.ReplicationInput, hubClient *xet.Client, downloadDir string, opts hfDownloadOptions, logger logging.Interface) (string, error) {
		return "/tmp/model", nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(ds *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, localPath string, checksumConfig *common.ChecksumConfig, numObjects int, numConnections int) error {
		return errors.New("upload error")
	}
	err = replicator.Replicate(objs)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "upload error")
}

func TestEffectiveChecksumConcurrency(t *testing.T) {
	assert.Equal(t, 1, effectiveChecksumConcurrency(nil))
	assert.Equal(t, 1, effectiveChecksumConcurrency(&common.ChecksumConfig{}))
	assert.Equal(t, 1, effectiveChecksumConcurrency(&common.ChecksumConfig{Concurrency: -1}))
	assert.Equal(t, 3, effectiveChecksumConcurrency(&common.ChecksumConfig{Concurrency: 3}))
}

func TestUploadDirectoryLimitsChecksumConcurrency(t *testing.T) {
	originalChecksum := getObjectMetadataWithChecksumFunc
	originalUpload := uploadObjectToOCIOSDataStoreFunc
	t.Cleanup(func() {
		getObjectMetadataWithChecksumFunc = originalChecksum
		uploadObjectToOCIOSDataStoreFunc = originalUpload
	})

	directory := t.TempDir()
	for i := 0; i < 4; i++ {
		err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("file-%d", i)), []byte("test"), 0600)
		assert.NoError(t, err)
	}

	var active atomic.Int32
	var maximum atomic.Int32
	var uploaded atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseChecksums := func() {
		releaseOnce.Do(func() { close(release) })
	}
	t.Cleanup(releaseChecksums)

	getObjectMetadataWithChecksumFunc = func(_ *common.ChecksumConfig, _ string, _ logging.Interface) map[string]string {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return map[string]string{OCIObjectMD5MetadataKey: "checksum"}
	}
	uploadObjectToOCIOSDataStoreFunc = func(_ *ociobjectstore.OCIOSDataStore, object ociobjectstore.ObjectURI, _ string) error {
		if object.Metadata[OCIObjectMD5MetadataKey] != "checksum" {
			return errors.New("checksum metadata missing")
		}
		uploaded.Add(1)
		return nil
	}

	logger := testingPkg.SetupMockLogger()
	dataStore := &ociobjectstore.OCIOSDataStore{
		Config: &ociobjectstore.Config{AnotherLogger: logger},
	}
	result := make(chan error, 1)
	go func() {
		result <- uploadDirectoryToOCIOSDataStore(
			dataStore,
			ociobjectstore.ObjectURI{BucketName: "bucket", Namespace: "namespace"},
			directory,
			&common.ChecksumConfig{UploadEnabled: true, ChecksumAlgorithm: MD5ChecksumAlgorithm, Concurrency: 2},
			4,
			4,
		)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for checksum worker")
		}
	}
	select {
	case <-started:
		t.Fatal("checksum concurrency exceeded configured limit")
	case <-time.After(50 * time.Millisecond):
	}

	releaseChecksums()
	assert.NoError(t, <-result)
	assert.Equal(t, int32(2), maximum.Load())
	assert.Equal(t, int32(4), uploaded.Load())
}
