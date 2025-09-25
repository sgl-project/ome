package common

import (
	"fmt"
	"reflect"

	"github.com/sgl-project/ome/pkg/xet"

	"github.com/sgl-project/ome/pkg/afero"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

type ChecksumConfig struct {
	UploadEnabled     bool   `mapstructure:"upload_enabled"`
	ChecksumAlgorithm string `mapstructure:"algorithm"`
}

func ConvertToReplicationObjectsFromObjectSummary(summaries []objectstorage.ObjectSummary) []ReplicationObject {
	result := make([]ReplicationObject, len(summaries))
	for i, summary := range summaries {
		result[i] = ObjectSummaryReplicationObject{ObjectSummary: summary}
	}
	return result
}

func ConvertToReplicationObjectsFromHFRepoFileInfo(repoFiles []xet.FileInfo) []ReplicationObject {
	result := make([]ReplicationObject, len(repoFiles))
	for i, file := range repoFiles {
		result[i] = HFRepoFileInfoReplicationObject{FileInfo: file}
	}
	return result
}

func ConvertToReplicationObjectsFromPVCFileEntry(files []afero.FileEntry) []ReplicationObject {
	result := make([]ReplicationObject, len(files))
	for i, file := range files {
		result[i] = PVCFileReplicationObject{FileEntry: file}
	}
	return result
}

func RequireNonNil(name string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("required %s is nil", name)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		if v.IsNil() {
			return fmt.Errorf("required %s is nil", name)
		}
	}
	return nil
}
