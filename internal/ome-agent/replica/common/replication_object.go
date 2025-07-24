package common

import (
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

type ReplicationInput struct {
	SourceStorageType storage.StorageType
	TargetStorageType storage.StorageType
	Source            ociobjectstore.ObjectURI
	Target            ociobjectstore.ObjectURI
}

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
		return *a.Name
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
	return a.GetPath()
}

func (a RepoFileReplicationObject) GetPath() string {
	return a.Path
}

func (a RepoFileReplicationObject) GetSize() int64 {
	return a.Size
}

type PVCFileReplicationObject struct {
	afero.FileEntry
}

func (a PVCFileReplicationObject) GetName() string {
	return a.FileInfo.Name()
}

func (a PVCFileReplicationObject) GetPath() string {
	return a.FilePath
}

func (a PVCFileReplicationObject) GetSize() int64 {
	return a.FileInfo.Size()
}
