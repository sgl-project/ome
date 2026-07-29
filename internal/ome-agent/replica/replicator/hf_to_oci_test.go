package replicator

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/ome/pkg/xet"

	"sigs.k8s.io/ome/internal/ome-agent/replica/artifactcache"
	"sigs.k8s.io/ome/internal/ome-agent/replica/common"
	"sigs.k8s.io/ome/pkg/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestHFToOCIReplicator_ReplicateSeedsCacheAndUploadsOnlyArtifactFiles(t *testing.T) {
	origDownloadFromHF := downloadFromHFFunc
	origUploadDirectoryToOCIOSDataStore := uploadDirectoryToOCIOSDataStoreFunc
	t.Cleanup(func() {
		downloadFromHFFunc = origDownloadFromHF
		uploadDirectoryToOCIOSDataStoreFunc = origUploadDirectoryToOCIOSDataStore
	})

	root := t.TempDir()
	cacheConfig := artifactcache.Config{
		Enabled:   true,
		Mode:      "seed",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: "cdbee75f17c01a7cc42f958dc650907174af0554",
	}
	downloadCalls := 0
	downloadFromHFFunc = func(_ common.ReplicationInput, _ *xet.Client, downloadDir string, _ hfDownloadOptions, _ logging.Interface) (string, error) {
		downloadCalls++
		require.NoError(t, os.WriteFile(filepath.Join(downloadDir, "model.safetensors"), []byte("weights"), 0o644))
		return downloadDir, nil
	}
	uploadDirectoryToOCIOSDataStoreFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI, localPath string, _ *common.ChecksumConfig, _ int, _ int) error {
		require.FileExists(t, filepath.Join(localPath, "model.safetensors"))
		require.NoFileExists(t, filepath.Join(localPath, artifactcache.ManifestFileName))
		require.NoFileExists(t, filepath.Join(localPath, artifactcache.ReadyFileName))
		return nil
	}
	replicator := newCacheTestHFToOCIReplicator(cacheConfig)

	err := replicator.Replicate(CreateCommonMockReplicationObjects(1))

	require.NoError(t, err)
	require.Equal(t, 1, downloadCalls)
	_, hit, err := cacheConfig.Inspect(root)
	require.NoError(t, err)
	require.True(t, hit)
}

func TestHFToOCIReplicator_ReplicateReusesCacheHitWithoutHuggingFaceDownload(t *testing.T) {
	origDownloadFromHF := downloadFromHFFunc
	origUploadDirectoryToOCIOSDataStore := uploadDirectoryToOCIOSDataStoreFunc
	t.Cleanup(func() {
		downloadFromHFFunc = origDownloadFromHF
		uploadDirectoryToOCIOSDataStoreFunc = origUploadDirectoryToOCIOSDataStore
	})

	root := t.TempDir()
	cacheConfig := artifactcache.Config{
		Enabled:   true,
		Mode:      "seed",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: "cdbee75f17c01a7cc42f958dc650907174af0554",
	}
	staging, err := cacheConfig.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o644))
	_, _, _, err = cacheConfig.PublishStaging(staging)
	require.NoError(t, err)

	downloadFromHFFunc = func(_ common.ReplicationInput, _ *xet.Client, _ string, _ hfDownloadOptions, _ logging.Interface) (string, error) {
		t.Fatal("Hugging Face download must not run on a valid cache hit")
		return "", nil
	}
	uploadCalled := false
	uploadDirectoryToOCIOSDataStoreFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI, localPath string, _ *common.ChecksumConfig, _ int, _ int) error {
		uploadCalled = true
		require.FileExists(t, filepath.Join(localPath, "model.safetensors"))
		return nil
	}
	replicator := newCacheTestHFToOCIReplicator(cacheConfig)

	err = replicator.Replicate(CreateCommonMockReplicationObjects(1))

	require.NoError(t, err)
	require.True(t, uploadCalled)
}

func TestHFToOCIReplicator_PrepareArtifactCacheSerializesConcurrentCacheMisses(t *testing.T) {
	origDownloadFromHF := downloadFromHFFunc
	t.Cleanup(func() { downloadFromHFFunc = origDownloadFromHF })

	root := t.TempDir()
	cacheConfig := artifactcache.Config{
		Enabled:   true,
		Mode:      "seed",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: "cdbee75f17c01a7cc42f958dc650907174af0554",
	}
	var downloadCalls atomic.Int32
	downloadFromHFFunc = func(_ common.ReplicationInput, _ *xet.Client, downloadDir string, _ hfDownloadOptions, _ logging.Interface) (string, error) {
		downloadCalls.Add(1)
		time.Sleep(100 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(downloadDir, "model.safetensors"), []byte("weights"), 0o644); err != nil {
			return "", err
		}
		return downloadDir, nil
	}
	replicators := []*HFToOCIReplicator{
		newCacheTestHFToOCIReplicator(cacheConfig),
		newCacheTestHFToOCIReplicator(cacheConfig),
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(replicators))
	for _, implementation := range replicators {
		wg.Add(1)
		go func(rep *HFToOCIReplicator) {
			defer wg.Done()
			errs <- rep.PrepareArtifactCache()
		}(implementation)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), downloadCalls.Load())
}

func TestHFToOCIReplicator_PrepareArtifactCacheRepairsCorruptEntry(t *testing.T) {
	origDownloadFromHF := downloadFromHFFunc
	t.Cleanup(func() { downloadFromHFFunc = origDownloadFromHF })

	root := t.TempDir()
	cacheConfig := artifactcache.Config{
		Enabled:   true,
		Mode:      "seed",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: "cdbee75f17c01a7cc42f958dc650907174af0554",
	}
	entry, err := cacheConfig.EntryPath(root)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(entry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(entry, artifactcache.ManifestFileName), []byte(`{"files":[{"name":"model.safetensors","size":7}]}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(entry, artifactcache.ReadyFileName), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(entry, "model.safetensors"), []byte("bad"), 0o644))

	downloadFromHFFunc = func(_ common.ReplicationInput, _ *xet.Client, downloadDir string, _ hfDownloadOptions, _ logging.Interface) (string, error) {
		require.NoError(t, os.WriteFile(filepath.Join(downloadDir, "model.safetensors"), []byte("weights"), 0o644))
		return downloadDir, nil
	}
	replicator := newCacheTestHFToOCIReplicator(cacheConfig)

	require.NoError(t, replicator.PrepareArtifactCache())
	manifest, hit, err := cacheConfig.Inspect(root)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, int64(7), manifest.Files[0].Size)
}

func TestHFToOCIReplicator_InspectArtifactCacheMakesReusedEntryReadable(t *testing.T) {
	root := t.TempDir()
	cacheConfig := artifactcache.Config{
		Enabled:   true,
		Mode:      "repair",
		Root:      root,
		KeyRoot:   "_artifacts",
		HFModelID: "Qwen/Qwen3-4B-Instruct-2507",
		CommitSHA: "cdbee75f17c01a7cc42f958dc650907174af0554",
	}
	staging, err := cacheConfig.NewStagingDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, "model.safetensors"), []byte("weights"), 0o600))
	entry, _, _, err := cacheConfig.PublishStaging(staging)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(entry, 0o700))
	require.NoError(t, os.Chmod(filepath.Join(entry, "model.safetensors"), 0o600))

	replicator := newCacheTestHFToOCIReplicator(cacheConfig)
	_, hit, err := replicator.InspectArtifactCache()

	require.NoError(t, err)
	require.True(t, hit)
	entryInfo, err := os.Stat(entry)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), entryInfo.Mode().Perm())
	fileInfo, err := os.Stat(filepath.Join(entry, "model.safetensors"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), fileInfo.Mode().Perm())
}

func newCacheTestHFToOCIReplicator(cacheConfig artifactcache.Config) *HFToOCIReplicator {
	return &HFToOCIReplicator{
		Logger: testingPkg.SetupMockLogger(),
		Config: HFToOCIReplicatorConfig{
			LocalPath:          cacheConfig.Root,
			NumConnections:     1,
			ModelArtifactCache: cacheConfig,
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypeHuggingFace,
			TargetStorageType: storage.StorageTypeOCI,
			Source:            ociobjectstore.ObjectURI{BucketName: cacheConfig.HFModelID, Prefix: cacheConfig.CommitSHA},
			Target:            ociobjectstore.ObjectURI{BucketName: "target-bucket", Namespace: "target-ns", Prefix: "target-prefix/"},
		},
	}
}
