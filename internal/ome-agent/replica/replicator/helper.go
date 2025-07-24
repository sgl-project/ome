package replicator

import (
	"fmt"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

const (
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 20
)

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
