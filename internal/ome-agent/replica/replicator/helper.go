package replicator

import (
	"fmt"
	"time"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

const (
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 20

	ReplicaWorkspacePath = "replica"
)

// Indirection for testability
var downloadFromHFFunc = downloadFromHF
var uploadDirectoryToOCIOSDataStoreFunc = uploadDirectoryToOCIOSDataStore
var downloadObjectsFromOCIOSDataStoreFunc = downloadObjectsFromOCIOSDataStore

func UploadObjectToOCIOSDataStore(ociOSDataStore *ociobjectstore.OCIOSDataStore, object ociobjectstore.ObjectURI, filePath string) error {
	if ociOSDataStore == nil {
		return fmt.Errorf("target ociOSDataStore is nil")
	}

	err := ociOSDataStore.MultipartFileUpload(filePath, object, DefaultUploadChunkSizeInMB, DefaultUploadThreads)
	if err != nil {
		ociOSDataStore.Config.AnotherLogger.Errorf("Failed to upload %s: %+v", object.ObjectName, err)
		return err
	}
	return nil
}

func DownloadObject(ociOSDataStore *ociobjectstore.OCIOSDataStore, srcObj ociobjectstore.ObjectURI, downloadPath string) error {
	if ociOSDataStore == nil {
		return fmt.Errorf("source ociOSDataStore is nil")
	}

	err := ociOSDataStore.MultipartDownload(srcObj, downloadPath,
		ociobjectstore.WithChunkSize(DefaultDownloadChunkSizeInMB),
		ociobjectstore.WithThreads(DefaultDownloadThreads))
	if err != nil {
		ociOSDataStore.Config.AnotherLogger.Errorf("Failed to download object %s: %+v", srcObj.ObjectName, err)
		return err
	}
	return nil
}

func PrepareObjectChannel(objects []common.ReplicationObject) chan common.ReplicationObject {
	objChan := make(chan common.ReplicationObject, len(objects))
	go func() {
		defer close(objChan)
		for _, object := range objects {
			objChan <- object
		}
	}()
	return objChan
}

func LogProgress(successCount, errorCount, totalObjects int, startTime time.Time, logger logging.Interface) {
	progress := float64(successCount+errorCount) / float64(totalObjects) * 100
	elapsedTime := time.Since(startTime)
	logger.Infof("Progress: %.2f%%, Success: %d, Errors: %d, Total: %d, Elapsed Time: %v", progress, successCount, errorCount, totalObjects, elapsedTime)
}
