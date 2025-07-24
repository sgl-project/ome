package replicator

import (
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
)

// MockReplicationObject is a common mock implementation of common.ReplicationObject
// that can be used across all test files in this package.
type MockReplicationObject struct {
	Name string
	Path string
	Size int64
}

// NewMockReplicationObject creates a new mock replication object with default values
func NewMockReplicationObject() *MockReplicationObject {
	return &MockReplicationObject{
		Name: "file1",
		Path: "file1",
		Size: 123,
	}
}

// NewCustomMockReplicationObject creates a new mock replication object with a custom name/path/size
func NewCustomMockReplicationObject(name string, path string, size int64) *MockReplicationObject {
	return &MockReplicationObject{
		Name: name,
		Path: path,
		Size: size,
	}
}

// GetName returns the name of the mock object
func (m *MockReplicationObject) GetName() string {
	return m.Name
}

// GetPath returns the path of the mock object
func (m *MockReplicationObject) GetPath() string {
	return m.Path
}

// GetSize returns the size of the mock object
func (m *MockReplicationObject) GetSize() int64 {
	return m.Size
}

// CreateCommonMockReplicationObjects creates a slice of mock replication objects
func CreateCommonMockReplicationObjects(count int) []common.ReplicationObject {
	objects := make([]common.ReplicationObject, count)
	for i := 0; i < count; i++ {
		objects[i] = NewMockReplicationObject()
	}
	return objects
}
