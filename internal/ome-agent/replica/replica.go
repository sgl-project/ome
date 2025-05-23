package replica

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
)

const (
	DefaultDownloadChunkSizeInMB = 20
	DefaultDownloadThreads       = 20
	DefaultUploadChunkSizeInMB   = 50
	DefaultUploadThreads         = 10
	GB                           = 1073741824
)

type ReplicaAgent struct {
	logger logging.Interface
	Config Config
}

type ReplicationResult struct {
	source ociobjectstore.ObjectURI
	target ociobjectstore.ObjectURI
	error  error
}

// NewReplicaAgent constructs a new replica agent from the given configuration.
func NewReplicaAgent(config *Config) (*ReplicaAgent, error) {
	return &ReplicaAgent{
		logger: config.AnotherLogger,
		Config: *config,
	}, nil
}

// Start initiates the replication process.
func (r *ReplicaAgent) Start() error {
	r.logger.Infof("Start replication from %s to %s", r.Config.SourceObjectStoreURI, r.Config.TargetObjectStoreURI)
	sourceObjs, err := r.listSourceObjects()
	if err != nil {
		return err
	}

	r.validateModelSize(sourceObjs)

	startTime := time.Now()
	totalObjects := len(sourceObjs)
	results := r.replicateObjects(sourceObjs, totalObjects)

	successCount, errorCount := 0, 0
	for result := range results {
		if result.error != nil {
			errorCount++
			r.logger.Errorf("Replication failed for %s to %s: %v", result.source, result.target, result.error)
		} else {
			successCount++
			r.logger.Infof("Replication succeeded for %s to %s", result.source, result.target)
		}
		r.logProgress(successCount, errorCount, totalObjects, startTime)
	}

	r.logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	return nil
}

func (r *ReplicaAgent) listSourceObjects() ([]objectstorage.ObjectSummary, error) {
	r.Config.ObjectStorageDataStore.SetRegion(r.Config.SourceObjectStoreURI.Region)
	sourceObjs, err := r.Config.ObjectStorageDataStore.ListObjects(r.Config.SourceObjectStoreURI)
	if err != nil {
		return nil, err
	}
	r.logger.Infof("Listed %d model weight objects under prefix %s", len(sourceObjs), r.Config.SourceObjectStoreURI.Prefix)
	return sourceObjs, nil
}

func (r *ReplicaAgent) replicateObjects(objects []objectstorage.ObjectSummary, totalObjects int) chan *ReplicationResult {
	r.logger.Info("Starting replication to target")

	objChan := r.prepareObjectChannel(objects)
	resultChan := make(chan *ReplicationResult, len(objects))

	var wg sync.WaitGroup
	for i := 0; i < r.Config.NumConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.processObjectReplication(objChan, resultChan, totalObjects)
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

func (r *ReplicaAgent) processObjectReplication(objects <-chan objectstorage.ObjectSummary, results chan<- *ReplicationResult, totalObjects int) {
	for obj := range objects {
		if *obj.Name == r.Config.SourceObjectStoreURI.Prefix {
			continue
		}

		srcObj := ociobjectstore.ObjectURI{
			Namespace:  r.Config.SourceObjectStoreURI.Namespace,
			BucketName: r.Config.SourceObjectStoreURI.BucketName,
			ObjectName: *obj.Name,
		}
		result := ReplicationResult{source: srcObj}

		downloadStart := time.Now()
		err := r.downloadObject(srcObj, &obj)
		downloadDuration := time.Since(downloadStart)
		if err != nil {
			result.error = err
			results <- &result
			continue
		}
		r.logger.Infof("Downloaded object %s in %v", srcObj.ObjectName, downloadDuration)

		targetObj := r.getTargetObjectURI(*obj.Name)
		result.target = targetObj

		uploadStart := time.Now()
		err = r.uploadObject(targetObj, *obj.Name)
		uploadDuration := time.Since(uploadStart)
		if err != nil {
			result.error = err
		} else {
			r.logger.Infof("Uploaded object to %s in %v", targetObj.ObjectName, uploadDuration)
		}
		results <- &result
	}
}

func (r *ReplicaAgent) downloadObject(srcObj ociobjectstore.ObjectURI, obj *objectstorage.ObjectSummary) error {
	r.Config.ObjectStorageDataStore.SetRegion(r.Config.SourceObjectStoreURI.Region)
	err := r.Config.ObjectStorageDataStore.MultipartDownload(srcObj, r.Config.LocalPath,
		ociobjectstore.WithChunkSize(DefaultDownloadChunkSizeInMB),
		ociobjectstore.WithThreads(DefaultDownloadThreads))
	if err != nil {
		r.logger.Errorf("Failed to download object %s: %+v", srcObj.ObjectName, err)
		return err
	}
	return nil
}

func (r *ReplicaAgent) uploadObject(targetObj ociobjectstore.ObjectURI, objName string) error {
	r.Config.ObjectStorageDataStore.SetRegion(r.Config.TargetObjectStoreURI.Region)
	curFilePath := filepath.Join(r.Config.LocalPath, objName)

	err := r.Config.ObjectStorageDataStore.MultipartFileUpload(curFilePath, targetObj, DefaultUploadChunkSizeInMB, DefaultUploadThreads)
	if err != nil {
		r.logger.Errorf("Failed to upload object %s: %+v", targetObj.ObjectName, err)
		return err
	}
	return nil
}

func (r *ReplicaAgent) prepareObjectChannel(objects []objectstorage.ObjectSummary) chan objectstorage.ObjectSummary {
	objChan := make(chan objectstorage.ObjectSummary, len(objects))
	go func() {
		defer close(objChan)
		for _, object := range objects {
			objChan <- object
		}
	}()
	return objChan
}

func (r *ReplicaAgent) getTargetObjectURI(objName string) ociobjectstore.ObjectURI {
	targetObjName := strings.Replace(objName, r.Config.SourceObjectStoreURI.Prefix, r.Config.TargetObjectStoreURI.Prefix, 1)
	return ociobjectstore.ObjectURI{
		Namespace:  r.Config.TargetObjectStoreURI.Namespace,
		BucketName: r.Config.TargetObjectStoreURI.BucketName,
		ObjectName: targetObjName,
	}
}

func (r *ReplicaAgent) validateModelSize(objects []objectstorage.ObjectSummary) {
	r.logger.Info("Calculating model size from source")

	sizeLimit := int64(r.Config.DownloadSizeLimitGB) * GB
	var totalSize int64

	for _, object := range objects {
		if object.Name == nil || object.Size == nil {
			r.logger.Errorf("Invalid object with missing name or size: %+v", object)
			continue
		}

		totalSize += *object.Size
		if r.Config.EnableSizeLimitCheck && totalSize > sizeLimit {
			r.logger.Fatalf("Model weights exceed size limit of %d bytes", sizeLimit)
		}
	}

	if totalSize == 0 {
		r.logger.Fatal("No model weights exist in the model folder")
	}
	r.logger.Infof("Total model size: %d bytes", totalSize)
}

func (r *ReplicaAgent) logProgress(successCount, errorCount, totalObjects int, startTime time.Time) {
	progress := float64(successCount+errorCount) / float64(totalObjects) * 100
	elapsedTime := time.Since(startTime)
	r.logger.Infof("Progress: %.2f%%, Success: %d, Errors: %d, Total: %d, Elapsed Time: %v", progress, successCount, errorCount, totalObjects, elapsedTime)
}
