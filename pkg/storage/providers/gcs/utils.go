package gcs

import (
	"fmt"

	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// parseGCSURI parses a GCS URI in the format gs://bucket/object/path
// This is a wrapper around the centralized storage parsing utility
func parseGCSURI(uri string) (bucket, objectName string, err error) {
	components, err := utilstorage.ParseGCSStorageURI(uri)
	if err != nil {
		return "", "", err
	}
	return components.Bucket, components.Object, nil
}

// buildGCSURI constructs a GCS URI from bucket and object name
func buildGCSURI(bucket, objectName string) string {
	if objectName == "" {
		return fmt.Sprintf("gs://%s", bucket)
	}
	return fmt.Sprintf("gs://%s/%s", bucket, objectName)
}
