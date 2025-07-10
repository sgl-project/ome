package replica

import (
	"fmt"
	"reflect"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	hf "github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

func convertToReplicationObjectsFromObjectSummary(summaries []objectstorage.ObjectSummary) []ReplicationObject {
	result := make([]ReplicationObject, len(summaries))
	for i, summary := range summaries {
		result[i] = ObjectSummaryReplicationObject{ObjectSummary: summary}
	}
	return result
}

func convertToReplicationObjectsFromRepoFile(repo []hf.RepoFile) []ReplicationObject {
	result := make([]ReplicationObject, len(repo))
	for i, file := range repo {
		result[i] = RepoFileReplicationObject{RepoFile: file}
	}
	return result
}

func requireNonNil(name string, value interface{}) error {
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

func uploadObjectToOCIOSDataStore(ociOSDataStore *ociobjectstore.OCIOSDataStore, object ociobjectstore.ObjectURI, filePath string) error {
	if ociOSDataStore == nil {
		return fmt.Errorf("target ociOSDataStore is nil")
	}

	err := ociOSDataStore.MultipartFileUpload(filePath, object, DefaultUploadChunkSizeInMB, DefaultUploadThreads)
	if err != nil {
		ociOSDataStore.Config.AnotherLogger.Errorf("Failed to upload %s: %+v", object.ObjectName, err)
		return err
	}
	return nil
}
