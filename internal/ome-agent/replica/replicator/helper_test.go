package replicator

import (
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPrepareObjectChannel(t *testing.T) {
	objName1 := "test1.bin"
	objName2 := "test2.bin"

	objects := []common.ReplicationObject{
		common.ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName1,
			},
		},
		common.ObjectSummaryReplicationObject{
			ObjectSummary: objectstorage.ObjectSummary{
				Name: &objName2,
			},
		},
	}

	objChan := PrepareObjectChannel(objects)

	// Collect objects from channel
	var receivedObjects []common.ReplicationObject
	for obj := range objChan {
		receivedObjects = append(receivedObjects, obj)
	}

	assert.Equal(t, len(objects), len(receivedObjects))
	assert.Equal(t, objects[0].GetName(), receivedObjects[0].GetName())
	assert.Equal(t, objects[1].GetName(), receivedObjects[1].GetName())
}

func TestLogProgress(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()

	startTime := time.Now().Add(-10 * time.Second)
	LogProgress(5, 1, 10, startTime, mockLogger)

	// Verify the logger was called with the expected info
	mockLogger.AssertCalled(t, "Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
