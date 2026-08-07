package s3

import (
	utilstorage "sigs.k8s.io/ome/pkg/utils/storage"
)

// parseS3URI parses an S3 URI and returns bucket and key
// Leverages the existing parsing in pkg/utils/storage
func parseS3URI(uri string) (bucket string, key string, err error) {
	// Use the existing S3 URI parser
	components, err := utilstorage.ParseS3StorageURI(uri)
	if err != nil {
		return "", "", err
	}

	return components.Bucket, components.Prefix, nil
}
