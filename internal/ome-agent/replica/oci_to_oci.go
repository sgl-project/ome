package replica

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type OCIToOCIReplicator struct {
	logger           logging.Interface
	Config           Config
	ReplicationInput ReplicationInput
}

type ReplicationResult struct {
	source ociobjectstore.ObjectURI
	target ociobjectstore.ObjectURI
	error  error
}

func (r *OCIToOCIReplicator) Replicate(objects []ReplicationObject) error {
	r.logger.Info("Starting replication to target")

	startTime := time.Now()
	objChan := r.prepareObjectChannel(objects)
	resultChan := make(chan *ReplicationResult, len(objects))

	var wg sync.WaitGroup
	for i := 0; i < r.Config.NumConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.processObjectReplication(objChan, resultChan, len(objects))
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	successCount, errorCount := 0, 0
	for result := range resultChan {
		if result.error != nil {
			errorCount++
			r.logger.Errorf("Replication failed for %s to %s: %v", result.source, result.target, result.error)
		} else {
			successCount++
			r.logger.Infof("Replication succeeded for %s to %s", result.source, result.target)
		}
		r.logProgress(successCount, errorCount, len(objects), startTime)
	}

	r.logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	if errorCount > 0 {
		return fmt.Errorf("%d/%d replications failed", errorCount, len(objects))
	}
	return nil
}

func (r *OCIToOCIReplicator) prepareObjectChannel(objects []ReplicationObject) chan ReplicationObject {
	objChan := make(chan ReplicationObject, len(objects))
	go func() {
		defer close(objChan)
		for _, object := range objects {
			objChan <- object
		}
	}()
	return objChan
}

func (r *OCIToOCIReplicator) processObjectReplication(objects <-chan ReplicationObject, results chan<- *ReplicationResult, totalObjects int) {
	for obj := range objects {
		if obj.GetName() == r.ReplicationInput.source.Prefix {
			continue
		}

		srcObj := ociobjectstore.ObjectURI{
			Namespace:  r.ReplicationInput.source.Namespace,
			BucketName: r.ReplicationInput.source.BucketName,
			ObjectName: obj.GetName(),
		}
		result := ReplicationResult{source: srcObj}

		downloadStart := time.Now()
		err := r.downloadObject(srcObj)
		downloadDuration := time.Since(downloadStart)
		if err != nil {
			result.error = err
			results <- &result
			continue
		}
		r.logger.Infof("Downloaded object %s in %v", srcObj.ObjectName, downloadDuration)

		targetObj := r.getTargetObjectURI(obj.GetName())
		result.target = targetObj

		uploadStart := time.Now()
		err = uploadObjectToOCIOSDataStore(r.Config.Target.OCIOSDataStore, targetObj, filepath.Join(r.Config.LocalPath, obj.GetName()))
		uploadDuration := time.Since(uploadStart)
		if err != nil {
			result.error = err
		} else {
			r.logger.Infof("Uploaded object to %s in %v", targetObj.ObjectName, uploadDuration)
		}
		results <- &result
	}
}

func (r *OCIToOCIReplicator) downloadObject(srcObj ociobjectstore.ObjectURI) error {
	err := r.Config.Source.OCIOSDataStore.MultipartDownload(srcObj, r.Config.LocalPath,
		ociobjectstore.WithChunkSize(DefaultDownloadChunkSizeInMB),
		ociobjectstore.WithThreads(DefaultDownloadThreads))
	if err != nil {
		r.logger.Errorf("Failed to download object %s: %+v", srcObj.ObjectName, err)
		return err
	}
	return nil
}

func (r *OCIToOCIReplicator) getTargetObjectURI(objName string) ociobjectstore.ObjectURI {
	targetObjName := strings.Replace(objName, r.ReplicationInput.source.Prefix, r.ReplicationInput.target.Prefix, 1)
	return ociobjectstore.ObjectURI{
		Namespace:  r.ReplicationInput.target.Namespace,
		BucketName: r.ReplicationInput.target.BucketName,
		ObjectName: targetObjName,
	}
}

func (r *OCIToOCIReplicator) logProgress(successCount, errorCount, totalObjects int, startTime time.Time) {
	progress := float64(successCount+errorCount) / float64(totalObjects) * 100
	elapsedTime := time.Since(startTime)
	r.logger.Infof("Progress: %.2f%%, Success: %d, Errors: %d, Total: %d, Elapsed Time: %v", progress, successCount, errorCount, totalObjects, elapsedTime)
}
