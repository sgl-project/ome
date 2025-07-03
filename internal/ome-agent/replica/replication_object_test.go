package replica

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/stretchr/testify/assert"
)

func TestObjectSummaryReplicationObject(t *testing.T) {
	name := "test-object"
	size := int64(1234)
	obj := objectstorage.ObjectSummary{
		Name: &name,
		Size: &size,
	}
	ro := ObjectSummaryReplicationObject{ObjectSummary: obj}

	assert.Equal(t, name, ro.GetName(), "GetName should return the object's name")
	assert.Equal(t, size, ro.GetSize(), "GetSize should return the object's size")
	assert.Equal(t, name, ro.GetPath(), "GetPath should return the object's name")
}

func TestObjectSummaryReplicationObject_NilFields(t *testing.T) {
	obj := objectstorage.ObjectSummary{}
	ro := ObjectSummaryReplicationObject{ObjectSummary: obj}

	assert.Equal(t, "", ro.GetName(), "GetName should return empty string if Name is nil")
	assert.Equal(t, int64(0), ro.GetSize(), "GetSize should return 0 if Size is nil")
	assert.Equal(t, "", ro.GetPath(), "GetPath should return empty string if Name is nil")
}

func TestRepoFileReplicationObject(t *testing.T) {
	path := "models/model.bin"
	size := int64(2048)
	typeStr := "model"
	repoFile := hub.RepoFile{
		Path: path,
		Size: size,
		Type: typeStr,
	}
	ro := RepoFileReplicationObject{RepoFile: repoFile}

	assert.Equal(t, path, ro.GetName(), "GetName should return the file's path")
	assert.Equal(t, size, ro.GetSize(), "GetSize should return the file's size")
	assert.Equal(t, path, ro.GetPath(), "GetPath should return the file's path")
}

func TestRepoFileReplicationObject_EmptyFields(t *testing.T) {
	repoFile := hub.RepoFile{}
	ro := RepoFileReplicationObject{RepoFile: repoFile}

	assert.Equal(t, "", ro.GetName(), "GetName should return empty string if Path is empty")
	assert.Equal(t, int64(0), ro.GetSize(), "GetSize should return 0 if Size is zero")
	assert.Equal(t, "", ro.GetPath(), "GetPath should return empty string if Path is empty")
}
