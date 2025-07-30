package replicator

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"

	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"github.com/stretchr/testify/assert"
)

type TestReplicator struct {
	*OCIToOCIReplicator
	mockPrepareObjectChannel     func(objects []common.ReplicationObject) chan common.ReplicationObject
	mockProcessObjectReplication func(objChan chan common.ReplicationObject, resultChan chan *ReplicationResult, total int)
	mockLogProgress              func(successCount, errorCount, total int, start time.Time)
	prepChan                     chan common.ReplicationObject
	resultSet                    []*ReplicationResult
	logs                         []string
	logMu                        sync.Mutex
}

func (t *TestReplicator) Replicate(objects []common.ReplicationObject) error {
	t.Logger.Info("Starting replication to target")

	startTime := time.Now()
	objChan := t.mockPrepareObjectChannel(objects)
	resultChan := make(chan *ReplicationResult, len(objects))

	var wg sync.WaitGroup
	for i := 0; i < t.Config.NumConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.mockProcessObjectReplication(objChan, resultChan, len(objects))
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
			t.Logger.Errorf("Replication failed for %s to %s: %v", result.source, result.target, result.error)
		} else {
			successCount++
			t.Logger.Infof("Replication succeeded for %s to %s", result.source, result.target)
		}
		t.mockLogProgress(successCount, errorCount, len(objects), startTime)
	}

	t.Logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	return nil
}

func TestReplicate(t *testing.T) {
	logger := testingPkg.SetupMockLogger()

	objects := []common.ReplicationObject{
		NewCustomMockReplicationObject("file1", "file1", 123),
		NewCustomMockReplicationObject("file2", "file2", 456),
	}
	objectChan := make(chan common.ReplicationObject, len(objects))
	for _, obj := range objects {
		objectChan <- obj
	}
	close(objectChan)

	results := []*ReplicationResult{
		{source: ociobjectstore.ObjectURI{ObjectName: "file1"}, target: ociobjectstore.ObjectURI{ObjectName: "target1"}, error: nil},
		{source: ociobjectstore.ObjectURI{ObjectName: "file2"}, target: ociobjectstore.ObjectURI{ObjectName: "target2"}, error: errors.New("failed")},
	}

	logs := []string{}
	logMu := &sync.Mutex{}

	testRep := &TestReplicator{
		OCIToOCIReplicator: &OCIToOCIReplicator{
			Logger: logger,
			Config: OCIToOCIReplicatorConfig{
				LocalPath: "/tmp/model",
			},
			ReplicationInput: common.ReplicationInput{
				SourceStorageType: storage.StorageTypeOCI,
				TargetStorageType: storage.StorageTypeOCI,
				Source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "src-prefix/",
				},
				Target: ociobjectstore.ObjectURI{
					Namespace:  "tgt-ns",
					BucketName: "tgt-bucket",
					Prefix:     "tgt-prefix/",
				},
			},
		},
		mockPrepareObjectChannel: func(objects []common.ReplicationObject) chan common.ReplicationObject { return objectChan },
		mockProcessObjectReplication: func(objChan chan common.ReplicationObject, resultChan chan *ReplicationResult, total int) {
			for _, result := range results {
				resultChan <- result
			}
		},
		mockLogProgress: func(successCount, errorCount, total int, start time.Time) {
			logMu.Lock()
			defer logMu.Unlock()
			logs = append(logs, "progress called")
		},
		prepChan:  objectChan,
		resultSet: results,
		logs:      logs,
		logMu:     sync.Mutex{},
	}

	err := testRep.Replicate(objects)
	assert.NoError(t, err)
}

func TestGetTargetObjectURI(t *testing.T) {
	tests := []struct {
		name             string
		replicationInput common.ReplicationInput
		objName          string
		expectedURI      ociobjectstore.ObjectURI
	}{
		{
			name: "replace source prefix with target prefix",
			replicationInput: common.ReplicationInput{
				SourceStorageType: storage.StorageTypeOCI,
				TargetStorageType: storage.StorageTypeOCI,
				Source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "src-prefix/",
				},
				Target: ociobjectstore.ObjectURI{
					Namespace:  "tgt-ns",
					BucketName: "tgt-bucket",
					Prefix:     "tgt-prefix/",
				},
			},
			objName: "src-prefix/model.bin",
			expectedURI: ociobjectstore.ObjectURI{
				Namespace:  "tgt-ns",
				BucketName: "tgt-bucket",
				ObjectName: "tgt-prefix/model.bin",
			},
		},
		{
			name: "source and target with same prefix",
			replicationInput: common.ReplicationInput{
				Source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "models/",
				},
				Target: ociobjectstore.ObjectURI{
					Namespace:  "tgt-ns",
					BucketName: "tgt-bucket",
					Prefix:     "models/",
				},
			},
			objName: "models/model.bin",
			expectedURI: ociobjectstore.ObjectURI{
				Namespace:  "tgt-ns",
				BucketName: "tgt-bucket",
				ObjectName: "models/model.bin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicator := &OCIToOCIReplicator{
				ReplicationInput: tt.replicationInput,
			}

			result := replicator.getTargetObjectURI(tt.objName)

			assert.Equal(t, tt.expectedURI.Namespace, result.Namespace)
			assert.Equal(t, tt.expectedURI.BucketName, result.BucketName)
			assert.Equal(t, tt.expectedURI.ObjectName, result.ObjectName)
		})
	}
}
