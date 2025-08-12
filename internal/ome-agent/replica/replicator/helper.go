package replicator

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"os"
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

	MD5ChecksumAlgorithm    = "MD5"
	SHA256ChecksumAlgorithm = "SHA256"

	OCIObjectMD5MetadataKey    = "opc-meta-md5"
	OCIObjectSHA256MetadataKey = "opc-meta-sha256"
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

func GetFileChecksum(filePath string, algorithm string) (string, error) {
	var h hash.Hash

	switch algorithm {
	case MD5ChecksumAlgorithm:
		h = md5.New()
	case SHA256ChecksumAlgorithm:
		h = sha256.New()
	default:
		return "", fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func GetObjectMetadatWithFileChecksum(config *common.ChecksumConfig, filePath string, logger logging.Interface) map[string]string {
	var metadata map[string]string = nil
	if config != nil && config.UploadEnabled {
		checksum, err := GetFileChecksum(filePath, config.ChecksumAlgorithm)
		if err != nil {
			logger.Warnf("Failed to compute checksum for %s: %+v", filePath, err)
		}

		if config.ChecksumAlgorithm == MD5ChecksumAlgorithm {
			metadata = map[string]string{
				OCIObjectMD5MetadataKey: checksum,
			}
		} else if config.ChecksumAlgorithm == SHA256ChecksumAlgorithm {
			metadata = map[string]string{
				OCIObjectSHA256MetadataKey: checksum,
			}
		}
	}
	return metadata
}
