package replica

import (
	"path"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
)

// TODO: add support for MD5
type ReplicationObject interface {
	GetName() string
	GetPath() string
	GetSize() int64
}

type ObjectSummaryReplicationObject struct {
	objectstorage.ObjectSummary
}

func (a ObjectSummaryReplicationObject) GetName() string {
	if a.Name != nil {
		return path.Base(*a.Name)
	}
	return ""
}

func (a ObjectSummaryReplicationObject) GetPath() string {
	return a.GetName()
}

func (a ObjectSummaryReplicationObject) GetSize() int64 {
	if a.Size != nil {
		return *a.Size
	}
	return 0
}

type RepoFileReplicationObject struct {
	hub.RepoFile
}

func (a RepoFileReplicationObject) GetName() string {
	return path.Base(a.GetPath())
}

func (a RepoFileReplicationObject) GetPath() string {
	return a.Path
}

func (a RepoFileReplicationObject) GetSize() int64 {
	return a.Size
}
