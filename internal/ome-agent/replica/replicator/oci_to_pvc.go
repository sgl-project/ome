package replicator

import (
	"fmt"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"path/filepath"
	"sync"
	"time"
)

type OCIToPVCReplicator struct {
	Logger           logging.Interface
	Config           OCIToPVCReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type OCIToPVCReplicatorConfig struct {
	LocalPath      string
	NumConnections int
	OCIOSDataStore *ociobjectstore.OCIOSDataStore
}

func (r *OCIToPVCReplicator) Replicate(objects []common.ReplicationObject) error {
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
			r.Logger.Errorf("Replication failed for %s to PVC %s under path %s: %v", result.source, r.ReplicationInput.Target.BucketName, r.ReplicationInput.Target.Prefix, result.error)
		} else {
			successCount++
			r.Logger.Infof("Replication succeeded for %s to PVC %s under path %s", result.source, r.ReplicationInput.Target.Prefix, result.target)
		}
		LogProgress(successCount, errorCount, len(objects), startTime, r.Logger)
	}

	r.Logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	if errorCount > 0 {
		return fmt.Errorf("%d/%d replications failed", errorCount, len(objects))
	}
	return nil
}

func (r *OCIToPVCReplicator) processObjectReplication(objects <-chan common.ReplicationObject, results chan<- *ReplicationResult) {
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
		err := DownloadObject(r.Config.OCIOSDataStore, srcObj, filepath.Join(r.Config.LocalPath, r.ReplicationInput.Target.BucketName))
		downloadDuration := time.Since(downloadStart)
		if err != nil {
			result.error = err
			results <- &result
			continue
		}
		r.Logger.Infof("Downloaded object %s in %v", srcObj.ObjectName, downloadDuration)
		results <- &result
	}
}
