package replicator

import (
	"context"
	"errors"
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

// Indirection for testability
var downloadFromHFFunc = downloadFromHF
var uploadDirectoryToOCIOSDataStoreFunc = uploadDirectoryToOCIOSDataStore

type HFToOCIReplicator struct {
	Logger           logging.Interface
	Config           HFToOCIReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type HFToOCIReplicatorConfig struct {
	Logger logging.Interface

	LocalPath      string
	NumConnections int
	HubClient      *hub.HubClient
	OCIOSDataStore *ociobjectstore.OCIOSDataStore
}

func (r *HFToOCIReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")
	// Download
	downloadPath, err := downloadFromHFFunc(r.ReplicationInput, r.Config)
	if err != nil {
		r.Logger.Errorf("Failed to download model %s from HuggingFace: %v", r.ReplicationInput.Source.BucketName, err)
		return err
	}
	r.Logger.Infof("Successfully downloaded model %s from HF to %s ", r.ReplicationInput.Source.BucketName, downloadPath)

	// Upload
	if err = uploadDirectoryToOCIOSDataStoreFunc(r.Config.OCIOSDataStore, r.ReplicationInput.Target, r.Config.LocalPath, len(objects), r.Config.NumConnections); err != nil {
		r.Logger.Errorf("Failed to upload files under %s to OCI Object Storage %v: %v", r.Config.LocalPath, r.ReplicationInput.Target, err)
		return err
	}
	r.Logger.Infof("All files under %s uploaded successfully", r.Config.LocalPath)
	r.Logger.Infof("Replication completed from HuggingFace to OCI Object Storage for model %s", r.ReplicationInput.Source.BucketName)
	return nil
}

func downloadFromHF(input common.ReplicationInput, config HFToOCIReplicatorConfig) (string, error) {
	var downloadOptions []hub.DownloadOption
	// Set revision if specified
	if input.Source.Prefix != "" {
		downloadOptions = append(downloadOptions, hub.WithRevision(input.Source.Prefix))
	}
	// Set repository type (always model for HuggingFace)
	downloadOptions = append(downloadOptions, hub.WithRepoType(hub.RepoTypeModel))

	downloadPath, err := config.HubClient.SnapshotDownload(
		context.Background(),
		input.Source.BucketName,
		config.LocalPath,
		downloadOptions...,
	)
	if err != nil {
		// Check error type for better handling
		var rateLimitErr *hub.RateLimitError
		var httpErr *hub.HTTPError
		if errors.As(err, &rateLimitErr) ||
			errors.As(err, &httpErr) && httpErr.StatusCode == 429 ||
			strings.Contains(err.Error(), "429") ||
			strings.Contains(err.Error(), "rate limit") {
			config.Logger.Warnf("Rate limited while downloading HuggingFace model %s: %v", input.Source.BucketName, err)
		} else {
			config.Logger.Errorf("Failed to download HuggingFace model %s: %v", input.Source.BucketName, err)
		}
		return downloadPath, err
	}

	return downloadPath, nil
}

type UploadTask struct {
	targetObj ociobjectstore.ObjectURI
	filePath  string
}

func uploadDirectoryToOCIOSDataStore(
	ociOSDataStore *ociobjectstore.OCIOSDataStore,
	object ociobjectstore.ObjectURI,
	localDirectoryPath string,
	numberOfObjects int,
	numberOfConnections int) error {
	if ociOSDataStore == nil {
		return fmt.Errorf("target ociOSDataStore is nil")
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

		// Create the OCI ObjectURI with target prefix
		objectName := strings.TrimSuffix(object.Prefix, "/") + "/" + relPath
		targetObj := ociobjectstore.ObjectURI{
			BucketName: object.BucketName,
			Namespace:  object.Namespace,
			ObjectName: objectName,
		}

		tasks <- UploadTask{targetObj: targetObj, filePath: filepath.Join(localDirectoryPath, relPath)}
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
