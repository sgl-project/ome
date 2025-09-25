package common

import (
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
	"github.com/sgl-project/ome/pkg/xet"
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

type HFRepoFileInfoReplicationObject struct {
	xet.FileInfo
}

func (a HFRepoFileInfoReplicationObject) GetName() string {
	return a.GetPath()
}

func (a HFRepoFileInfoReplicationObject) GetPath() string {
	return a.Path
}

func (a HFRepoFileInfoReplicationObject) GetSize() int64 {
	return int64(a.Size)
}

type PVCFileReplicationObject struct {
	afero.FileEntry
}

func (a PVCFileReplicationObject) GetName() string {
	if a.FileInfo == nil {
		return ""
	}
	return a.FileInfo.Name()
}

func (a PVCFileReplicationObject) GetPath() string {
	return a.FilePath
}

func (a PVCFileReplicationObject) GetSize() int64 {
	if a.FileInfo == nil {
		return 0
	}
	return a.FileInfo.Size()
}
