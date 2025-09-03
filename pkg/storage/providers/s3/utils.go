package s3

import (
	"fmt"
	"strings"

	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
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

// buildS3URI constructs an S3 URI from bucket and key
func buildS3URI(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

// isMultipartETag checks if an ETag indicates a multipart upload
// S3 multipart ETags have the format: "<md5>-<parts>"
func isMultipartETag(etag string) bool {
	// Remove quotes if present
	etag = strings.Trim(etag, "\"")
	// Check if it contains a dash followed by a number
	parts := strings.Split(etag, "-")
	return len(parts) == 2
}

// extractMD5FromETag extracts the MD5 portion from an ETag
func extractMD5FromETag(etag string) string {
	// Remove quotes if present
	etag = strings.Trim(etag, "\"")
	// For multipart uploads, return just the MD5 portion
	if isMultipartETag(etag) {
		parts := strings.Split(etag, "-")
		return parts[0]
	}
	// For simple uploads, the ETag is the MD5
	return etag
}

// normalizeKey ensures the key doesn't start with a slash
// S3 keys should not start with /
func normalizeKey(key string) string {
	return strings.TrimPrefix(key, "/")
}

// isValidBucketName checks if a bucket name is valid for S3
func isValidBucketName(bucket string) bool {
	// Basic S3 bucket naming rules
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}

	// Must start and end with lowercase letter or number
	if !isAlphanumeric(bucket[0]) || !isAlphanumeric(bucket[len(bucket)-1]) {
		return false
	}

	// Can only contain lowercase letters, numbers, hyphens, and periods
	for _, ch := range bucket {
		if !isAlphanumeric(byte(ch)) && ch != '-' && ch != '.' {
			return false
		}
	}

	// Cannot contain consecutive periods or hyphens
	if strings.Contains(bucket, "..") || strings.Contains(bucket, "--") {
		return false
	}

	// Cannot be formatted as an IP address
	if isIPAddress(bucket) {
		return false
	}

	return true
}

// isAlphanumeric checks if a byte is a lowercase letter or number
func isAlphanumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
}

// isIPAddress checks if a string looks like an IP address
func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		// Check if each part is a number between 0-255
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}

	return true
}

// getContentTypeFromKey attempts to determine content type from file extension
func getContentTypeFromKey(key string) string {
	// Extract file extension
	lastDot := strings.LastIndex(key, ".")
	if lastDot == -1 {
		return "application/octet-stream"
	}

	ext := strings.ToLower(key[lastDot+1:])

	// Common content types
	switch ext {
	case "json":
		return "application/json"
	case "xml":
		return "application/xml"
	case "txt":
		return "text/plain"
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "js":
		return "application/javascript"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "pdf":
		return "application/pdf"
	case "zip":
		return "application/zip"
	case "tar":
		return "application/x-tar"
	case "gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}
