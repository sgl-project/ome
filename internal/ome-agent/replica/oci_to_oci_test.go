package replica

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock replication object ---
type MockReplicationObject struct {
	name string
}

func (m *MockReplicationObject) GetName() string { return m.name }
func (m *MockReplicationObject) GetPath() string { return m.name }
func (m *MockReplicationObject) GetSize() int64  { return 42 }

type TestReplicator struct {
	*OCIToOCIReplicator
	mockPrepareObjectChannel     func(objects []ReplicationObject) chan ReplicationObject
	mockProcessObjectReplication func(objChan chan ReplicationObject, resultChan chan *ReplicationResult, total int)
	mockLogProgress              func(successCount, errorCount, total int, start time.Time)
	prepChan                     chan ReplicationObject
	resultSet                    []*ReplicationResult
	logs                         []string
	logMu                        sync.Mutex
}

func (t *TestReplicator) Replicate(objects []ReplicationObject) error {
	t.logger.Info("Starting replication to target")

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
			t.logger.Errorf("Replication failed for %s to %s: %v", result.source, result.target, result.error)
		} else {
			successCount++
			t.logger.Infof("Replication succeeded for %s to %s", result.source, result.target)
		}
		t.mockLogProgress(successCount, errorCount, len(objects), startTime)
	}

	t.logger.Infof("Replication completed with %d successes and %d errors in %v", successCount, errorCount, time.Since(startTime))
	return nil
}

func TestReplicate(t *testing.T) {
	logger := testingPkg.SetupMockLogger()

	objects := []ReplicationObject{
		&MockReplicationObject{name: "file1"},
		&MockReplicationObject{name: "file2"},
	}
	objectChan := make(chan ReplicationObject, len(objects))
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
			logger: logger,
			Config: Config{
				LocalPath: "/tmp/model",
				Source: sourceStruct{
					StorageURIStr: "oci://n/src-ns/b/src-bucket/o/src-prefix",
				},
				Target: targetStruct{
					StorageURIStr: "oci://n/tgt-ns/b/tgt-bucket/o/tgt-prefix",
				},
			},
			ReplicationInput: ReplicationInput{
				sourceStorageType: storage.StorageTypeOCI,
				targetStorageType: storage.StorageTypeOCI,
				source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "src-prefix/",
				},
				target: ociobjectstore.ObjectURI{
					Namespace:  "tgt-ns",
					BucketName: "tgt-bucket",
					Prefix:     "tgt-prefix/",
				},
			},
		},
		mockPrepareObjectChannel: func(objects []ReplicationObject) chan ReplicationObject { return objectChan },
		mockProcessObjectReplication: func(objChan chan ReplicationObject, resultChan chan *ReplicationResult, total int) {
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

func TestPrepareObjectChannel(t *testing.T) {
	objName1 := "test1.bin"
	objName2 := "test2.bin"

	objects := []ReplicationObject{
		ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName1,
			},
		},
		ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName2,
			},
		},
	}

	replicator := &OCIToOCIReplicator{}
	objChan := replicator.prepareObjectChannel(objects)

	// Collect objects from channel
	var receivedObjects []ReplicationObject
	for obj := range objChan {
		receivedObjects = append(receivedObjects, obj)
	}

	assert.Equal(t, len(objects), len(receivedObjects))
	assert.Equal(t, objects[0].GetName(), receivedObjects[0].GetName())
	assert.Equal(t, objects[1].GetName(), receivedObjects[1].GetName())
}

func TestGetTargetObjectURI(t *testing.T) {
	tests := []struct {
		name             string
		replicationInput ReplicationInput
		objName          string
		expectedURI      ociobjectstore.ObjectURI
	}{
		{
			name: "replace source prefix with target prefix",
			replicationInput: ReplicationInput{
				sourceStorageType: storage.StorageTypeOCI,
				targetStorageType: storage.StorageTypeOCI,
				source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "src-prefix/",
				},
				target: ociobjectstore.ObjectURI{
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
			replicationInput: ReplicationInput{
				source: ociobjectstore.ObjectURI{
					Namespace:  "src-ns",
					BucketName: "src-bucket",
					Prefix:     "models/",
				},
				target: ociobjectstore.ObjectURI{
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

func TestLogProgress(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()

	replicator := &OCIToOCIReplicator{
		logger: mockLogger,
	}

	startTime := time.Now().Add(-10 * time.Second)
	replicator.logProgress(5, 1, 10, startTime)

	// Verify the logger was called with the expected info
	mockLogger.AssertCalled(t, "Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
