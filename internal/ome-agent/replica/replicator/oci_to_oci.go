package replicator

import (
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type OCIToOCIReplicator struct {
	Logger           logging.Interface
	Config           OCIToOCIReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type OCIToOCIReplicatorConfig struct {
	LocalPath            string
	NumConnections       int
	SourceOCIOSDataStore *ociobjectstore.OCIOSDataStore
	TargetOCIOSDataStore *ociobjectstore.OCIOSDataStore
}

type ReplicationResult struct {
	source ociobjectstore.ObjectURI
	target ociobjectstore.ObjectURI
	error  error
}

func (r *OCIToOCIReplicator) Replicate(objects []common.ReplicationObject) error {
	r.Logger.Info("Starting replication to target")

	startTime := time.Now()
	objChan := PrepareObjectChannel(objects)
	resultChan := make(chan *ReplicationResult, len(objects))

	var wg sync.WaitGroup
	for i := 0; i < r.Config.NumConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.processObjectReplication(objChan, resultChan)
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
			r.Logger.Errorf("Replication failed for %s to %s: %v", result.source, result.target, result.error)
		} else {
			successCount++
			r.Logger.Infof("Replication succeeded for %s to %s", result.source, result.target)
		}
		LogProgress(successCount, errorCount, len(objects), startTime, r.Logger)
	}

	r.Logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	if errorCount > 0 {
		return fmt.Errorf("%d/%d replications failed", errorCount, len(objects))
	}
	return nil
}

func (r *OCIToOCIReplicator) processObjectReplication(objects <-chan common.ReplicationObject, results chan<- *ReplicationResult) {
	for obj := range objects {
		if obj.GetName() == r.ReplicationInput.Source.Prefix {
			continue
		}

		srcObj := ociobjectstore.ObjectURI{
			Namespace:  r.ReplicationInput.Source.Namespace,
			BucketName: r.ReplicationInput.Source.BucketName,
			ObjectName: obj.GetName(),
		}
		result := ReplicationResult{source: srcObj}

		downloadStart := time.Now()
		err := DownloadObject(r.Config.SourceOCIOSDataStore, srcObj, r.Config.LocalPath)
		downloadDuration := time.Since(downloadStart)
		if err != nil {
			result.error = err
			results <- &result
			continue
		}
		r.Logger.Infof("Downloaded object %s in %v", srcObj.ObjectName, downloadDuration)

		targetObj := r.getTargetObjectURI(obj.GetName())
		result.target = targetObj

		uploadStart := time.Now()
		err = UploadObjectToOCIOSDataStore(r.Config.TargetOCIOSDataStore, targetObj, filepath.Join(r.Config.LocalPath, obj.GetName()))
		uploadDuration := time.Since(uploadStart)
		if err != nil {
			result.error = err
		} else {
			r.Logger.Infof("Uploaded object to %s in %v", targetObj.ObjectName, uploadDuration)
		}
		results <- &result
	}
}

func (r *OCIToOCIReplicator) getTargetObjectURI(objName string) ociobjectstore.ObjectURI {
	targetObjName := strings.Replace(objName, r.ReplicationInput.Source.Prefix, r.ReplicationInput.Target.Prefix, 1)
	return ociobjectstore.ObjectURI{
		Namespace:  r.ReplicationInput.Target.Namespace,
		BucketName: r.ReplicationInput.Target.BucketName,
		ObjectName: targetObjName,
	}
}
