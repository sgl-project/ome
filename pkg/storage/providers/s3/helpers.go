package s3

import (
	"io"
	"strings"
)

// IsReaderEmpty checks if a reader is empty without consuming it
func IsReaderEmpty(streamReader io.Reader) bool {
	switch v := streamReader.(type) {
	case *strings.Reader:
		return v.Len() == 0
	case interface{ Len() int }:
		return v.Len() == 0
	default:
		return false
	}
}

// ConvertMetadataToS3 converts storage metadata to S3 format
// S3 metadata keys must be prefixed with "x-amz-meta-" when set via API
// but the SDK handles this automatically
func ConvertMetadataToS3(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}

	s3Meta := make(map[string]string)
	for k, v := range metadata {
		// S3 SDK automatically adds x-amz-meta- prefix
		// So we just pass the keys as-is
		s3Meta[k] = v
	}
	return s3Meta
}

// ConvertMetadataFromS3 converts S3 metadata to storage format
// S3 returns metadata without the x-amz-meta- prefix
func ConvertMetadataFromS3(s3Meta map[string]string) map[string]string {
	if s3Meta == nil {
		return nil
	}

	metadata := make(map[string]string)
	for k, v := range s3Meta {
		metadata[k] = v
	}
	return metadata
}

// GetS3ErrorCode extracts the error code from an S3 error
func GetS3ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	// The actual error code extraction is handled in provider.go
	// using the smithy.APIError interface
	return ""
}
