package replicator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/ome/pkg/xet"

	"sigs.k8s.io/ome/internal/ome-agent/replica/artifactcache"
	"sigs.k8s.io/ome/internal/ome-agent/replica/common"

	"sigs.k8s.io/ome/pkg/hfutil/hub"
	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

type HFToOCIReplicator struct {
	Logger           logging.Interface
	Config           HFToOCIReplicatorConfig
	ReplicationInput common.ReplicationInput

	preparedCacheEntry    string
	preparedCacheManifest artifactcache.Manifest
}

type HFToOCIReplicatorConfig struct {
	LocalPath                      string
	NumConnections                 int
	ChecksumConfig                 *common.ChecksumConfig
	HubClient                      *xet.Client
	OCIOSDataStore                 *ociobjectstore.OCIOSDataStore
	HFDownloadTimeout              time.Duration
	HFDownloadStaleProgressTimeout time.Duration
	ModelArtifactCache             artifactcache.Config
}

type artifactCacheReplicationObject struct {
	name string
	size int64
}

func (o artifactCacheReplicationObject) GetName() string { return o.name }
func (o artifactCacheReplicationObject) GetPath() string { return o.name }
func (o artifactCacheReplicationObject) GetSize() int64  { return o.size }

func (r *HFToOCIReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")
	tempDirPath := filepath.Join(r.Config.LocalPath, ReplicaWorkspacePath)
	cleanup := func() error { return os.RemoveAll(tempDirPath) }
	if r.Config.ModelArtifactCache.Enabled {
		if err := r.PrepareArtifactCache(); err != nil {
			return err
		}
		uploadView, cleanupUploadView, err := r.Config.ModelArtifactCache.NewUploadView(
			r.preparedCacheEntry,
			r.preparedCacheManifest,
			false,
		)
		if err != nil {
			return fmt.Errorf("create model artifact cache upload view: %w", err)
		}
		tempDirPath = uploadView
		cleanup = cleanupUploadView
	} else {
		downloadPath, err := downloadFromHFFunc(r.ReplicationInput, r.Config.HubClient, tempDirPath, hfDownloadOptions{
			DownloadTimeout:      r.Config.HFDownloadTimeout,
			StaleProgressTimeout: r.Config.HFDownloadStaleProgressTimeout,
		}, r.Logger)
		if err != nil {
			r.Logger.Errorf("Failed to download model %s from HuggingFace: %v", r.ReplicationInput.Source.BucketName, err)
			return err
		}
		r.Logger.Infof("Successfully downloaded model %s from HF to %s", r.ReplicationInput.Source.BucketName, downloadPath)
	}
	defer func() {
		if err := cleanup(); err != nil {
			r.Logger.Warnf("Failed to clean up the temp local directory %s: %v", tempDirPath, err)
		}
	}()

	// Upload
	if err := uploadDirectoryToOCIOSDataStoreFunc(
		r.Config.OCIOSDataStore,
		r.ReplicationInput.Target,
		tempDirPath,
		r.Config.ChecksumConfig,
		len(objects),
		r.Config.NumConnections,
	); err != nil {
		r.Logger.Errorf("Failed to upload files under %s to OCI Object Storage %v: %v", tempDirPath, r.ReplicationInput.Target, err)
		return err
	}
	r.Logger.Infof("All files under %s uploaded successfully", tempDirPath)
	r.Logger.Infof("Replication completed from HuggingFace to OCI Object Storage for model %s", r.ReplicationInput.Source.BucketName)
	return nil
}

func (r *HFToOCIReplicator) PrepareArtifactCache() error {
	if !r.Config.ModelArtifactCache.Enabled {
		return nil
	}
	if r.preparedCacheEntry != "" {
		return nil
	}
	return r.Config.ModelArtifactCache.WithEntryLock(r.prepareArtifactCacheLocked)
}

func (r *HFToOCIReplicator) InspectArtifactCache() ([]common.ReplicationObject, bool, error) {
	if !r.Config.ModelArtifactCache.Enabled {
		return nil, false, nil
	}
	manifest, hit, err := r.inspectArtifactCacheForMode()
	if err != nil {
		r.Logger.Warnf("Model artifact cache entry is invalid and will be rebuilt: %v", err)
		return nil, false, nil
	}
	if !hit {
		return nil, false, nil
	}
	entry, err := r.Config.ModelArtifactCache.EntryPath(r.Config.ModelArtifactCache.Root)
	if err != nil {
		return nil, false, err
	}
	if err := r.Config.ModelArtifactCache.EnsureEntryReadable(r.Config.ModelArtifactCache.Root); err != nil {
		return nil, false, err
	}
	r.preparedCacheEntry = entry
	r.preparedCacheManifest = manifest
	objects := make([]common.ReplicationObject, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		objects = append(objects, artifactCacheReplicationObject{name: file.Name, size: file.Size})
	}
	r.Logger.Infof("Reusing model artifact cache entry %s", entry)
	return objects, true, nil
}

func (r *HFToOCIReplicator) prepareArtifactCacheLocked() error {
	if r.preparedCacheEntry != "" {
		return nil
	}
	manifest, hit, err := r.inspectArtifactCacheForMode()
	if err == nil && hit {
		entry, err := r.Config.ModelArtifactCache.EntryPath(r.Config.ModelArtifactCache.Root)
		if err != nil {
			return err
		}
		if err := r.Config.ModelArtifactCache.EnsureEntryReadable(r.Config.ModelArtifactCache.Root); err != nil {
			return err
		}
		r.preparedCacheEntry = entry
		r.preparedCacheManifest = manifest
		r.Logger.Infof("Reusing model artifact cache entry %s", entry)
		return nil
	}
	removed, repairErr := r.Config.ModelArtifactCache.RemoveInvalidEntry()
	if repairErr != nil {
		if err != nil {
			return fmt.Errorf("repair model artifact cache entry after inspection failed (%v): %w", err, repairErr)
		}
		return fmt.Errorf("repair incomplete model artifact cache entry: %w", repairErr)
	}
	if removed {
		r.Logger.Infof("Removed invalid model artifact cache entry before rebuilding")
	}

	staging, err := r.Config.ModelArtifactCache.NewStagingDir()
	if err != nil {
		return fmt.Errorf("create model artifact cache staging directory: %w", err)
	}
	defer func() {
		if staging != "" {
			_ = r.Config.ModelArtifactCache.CleanupScratch(staging)
		}
	}()
	downloadPath, err := downloadFromHFFunc(r.ReplicationInput, r.Config.HubClient, staging, hfDownloadOptions{
		DownloadTimeout:      r.Config.HFDownloadTimeout,
		StaleProgressTimeout: r.Config.HFDownloadStaleProgressTimeout,
	}, r.Logger)
	if err != nil {
		r.Logger.Errorf("Failed to download model %s from HuggingFace: %v", r.ReplicationInput.Source.BucketName, err)
		return err
	}
	r.Logger.Infof("Successfully downloaded model %s from HF to cache staging %s", r.ReplicationInput.Source.BucketName, downloadPath)

	entry, manifest, reused, err := r.Config.ModelArtifactCache.PublishStaging(staging)
	if err != nil {
		return fmt.Errorf("publish model artifact cache entry: %w", err)
	}
	staging = ""
	r.preparedCacheEntry = entry
	r.preparedCacheManifest = manifest
	r.Logger.Infof("Model artifact cache entry ready at %s; reused=%t", entry, reused)
	return nil
}

func (r *HFToOCIReplicator) inspectArtifactCacheForMode() (artifactcache.Manifest, bool, error) {
	if r.isArtifactCacheRepair() {
		return r.Config.ModelArtifactCache.Verify(r.Config.ModelArtifactCache.Root)
	}
	return r.Config.ModelArtifactCache.Inspect(r.Config.ModelArtifactCache.Root)
}

func (r *HFToOCIReplicator) isArtifactCacheRepair() bool {
	return strings.EqualFold(strings.TrimSpace(r.Config.ModelArtifactCache.Mode), artifactcache.ModeRepair)
}

func downloadFromHF(input common.ReplicationInput, hubClient *xet.Client, downloadDir string, opts hfDownloadOptions, logger logging.Interface) (string, error) {
	req := &xet.SnapshotRequest{
		RepoID:   input.Source.BucketName,
		RepoType: hub.RepoTypeModel,
		Revision: input.Source.Prefix,
		LocalDir: downloadDir,
	}

	path, err := downloadSnapshotWithTimeouts(hubClient, req, opts, logger)
	if err != nil {
		logger.Errorf("Failed to download snapshot: %v", err)
	}
	return path, err
}

type UploadTask struct {
	targetObj ociobjectstore.ObjectURI
	filePath  string
}

func uploadDirectoryToOCIOSDataStore(
	ociOSDataStore *ociobjectstore.OCIOSDataStore,
	object ociobjectstore.ObjectURI,
	localDirectoryPath string,
	checksumConfig *common.ChecksumConfig,
	numberOfObjects int,
	numberOfConnections int) error {
	if ociOSDataStore == nil {
		return fmt.Errorf("target ociOSDataStore is nil")
	}

	// Early return if no objects to upload
	if numberOfObjects <= 0 {
		ociOSDataStore.Config.AnotherLogger.Infof("No objects to upload (numberOfObjects: %d), skipping upload", numberOfObjects)
		return nil
	}

	tasks := make(chan UploadTask, numberOfObjects)
	errCh := make(chan error, numberOfObjects)

	var wg sync.WaitGroup
	// Start worker goroutines
	for i := 0; i < numberOfConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := UploadObjectToOCIOSDataStore(ociOSDataStore, task.targetObj, task.filePath); err != nil {
					errCh <- fmt.Errorf("upload failed for %s: %w", task.filePath, err)
				}
			}
		}()
	}

	err := filepath.WalkDir(localDirectoryPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get object name relative to the root dir
		relPath, err := filepath.Rel(localDirectoryPath, path)
		if err != nil {
			return err
		}

		// Normalize path to use "/" for OCI Object Storage
		relPath = filepath.ToSlash(relPath)
		filePath := filepath.Join(localDirectoryPath, relPath)

		// Create the OCI ObjectURI with target prefix
		objectName := strings.TrimSuffix(object.Prefix, "/") + "/" + relPath
		// Handle the case when directly uploading to root directory in OCI bucket
		if object.Prefix == "" {
			objectName = relPath
		}

		metadata := GetObjectMetadatWithFileChecksum(checksumConfig, filePath, ociOSDataStore.Config.AnotherLogger)
		targetObj := ociobjectstore.ObjectURI{
			BucketName: object.BucketName,
			Namespace:  object.Namespace,
			ObjectName: objectName,
			Metadata:   metadata,
		}

		tasks <- UploadTask{targetObj: targetObj, filePath: filePath}
		return nil
	})

	close(tasks) // signal no more work
	wg.Wait()    // wait for workers to finish
	close(errCh) // no more errors to collect

	if err != nil {
		ociOSDataStore.Config.AnotherLogger.Errorf("Failed to upload files: %+v", err)
		return err
	}

	if len(errCh) > 0 {
		for err := range errCh {
			ociOSDataStore.Config.AnotherLogger.Errorf("error when uploading a file: %+v", err)
		}
		return fmt.Errorf("some files failed to upload")
	}
	return nil
}
