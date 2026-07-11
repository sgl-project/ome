package modelagent

import (
	"strings"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

func filterInternalArtifactObjectSummaries(objects []objectstorage.ObjectSummary) []objectstorage.ObjectSummary {
	filtered := make([]objectstorage.ObjectSummary, 0, len(objects))
	for _, object := range objects {
		if object.Name != nil && isInternalArtifactObjectName(*object.Name) {
			continue
		}
		filtered = append(filtered, object)
	}
	return filtered
}

func isInternalArtifactObjectName(objectName string) bool {
	return isArtifactCompleteMarkerObjectName(objectName) || isArtifactUploadLockObjectName(objectName)
}

func isArtifactCompleteMarkerObjectName(objectName string) bool {
	return objectName == artifactCompleteMarkerFileName || strings.HasSuffix(objectName, "/"+artifactCompleteMarkerFileName)
}

func isArtifactUploadLockObjectName(objectName string) bool {
	return objectName == artifactUploadLockFileName || strings.HasSuffix(objectName, "/"+artifactUploadLockFileName)
}
