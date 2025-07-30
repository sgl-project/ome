package common

import (
	"fmt"
	"reflect"

	"github.com/sgl-project/ome/pkg/afero"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	hf "github.com/sgl-project/ome/pkg/hfutil/hub"
)

func ConvertToReplicationObjectsFromObjectSummary(summaries []objectstorage.ObjectSummary) []ReplicationObject {
	result := make([]ReplicationObject, len(summaries))
	for i, summary := range summaries {
		result[i] = ObjectSummaryReplicationObject{ObjectSummary: summary}
	}
	return result
}

func ConvertToReplicationObjectsFromRepoFile(repo []hf.RepoFile) []ReplicationObject {
	result := make([]ReplicationObject, len(repo))
	for i, file := range repo {
		result[i] = RepoFileReplicationObject{RepoFile: file}
	}
	return result
}

func ConvertToReplicationObjectsFromFileInfo(files []afero.FileEntry) []ReplicationObject {
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
