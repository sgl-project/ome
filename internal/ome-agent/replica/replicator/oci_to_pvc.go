package replicator

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
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
			downloadObjectsFromOCIOSDataStoreFunc(objChan, r.Config.OCIOSDataStore, r.ReplicationInput, r.Config.LocalPath, resultChan)
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
			r.Logger.Errorf("Replication failed for %+v to PVC %s under path '%s': %v", result.source, r.ReplicationInput.Target.BucketName, r.ReplicationInput.Target.Prefix, result.error)
		} else {
			successCount++
			r.Logger.Infof("Replication succeeded for %+v to PVC %s under path %+v", result.source, r.ReplicationInput.Target.Prefix, result.target)
		}
		LogProgress(successCount, errorCount, len(objects), startTime, r.Logger)
	}

	r.Logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	if errorCount > 0 {
		return fmt.Errorf("%d/%d replications failed", errorCount, len(objects))
	}
	return nil
}

func downloadObjectsFromOCIOSDataStore(
	objects <-chan common.ReplicationObject,
	ociOSDataStore *ociobjectstore.OCIOSDataStore,
	replicationInput common.ReplicationInput,
	localDirectoryPath string,
	results chan<- *ReplicationResult) {
	for obj := range objects {
		if strings.HasSuffix(obj.GetName(), "/") {
			continue // Skip directories
		}

		srcObj := ociobjectstore.ObjectURI{
			Namespace:  replicationInput.Source.Namespace,
			BucketName: replicationInput.Source.BucketName,
			ObjectName: obj.GetName(),
		}
		result := ReplicationResult{source: srcObj}

		downloadStart := time.Now()
		err := DownloadObject(ociOSDataStore, srcObj, filepath.Join(localDirectoryPath, replicationInput.Target.Prefix))
		downloadDuration := time.Since(downloadStart)
		if err != nil {
			result.error = err
			results <- &result
			continue
		}

		ociOSDataStore.Config.AnotherLogger.Infof("Downloaded object %s in %v", srcObj.ObjectName, downloadDuration)
		results <- &result
	}
}
