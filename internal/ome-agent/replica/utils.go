package replica

import (
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	hf "github.com/sgl-project/ome/pkg/hfutil/hub"
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
