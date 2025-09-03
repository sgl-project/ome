package s3

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// validateETag validates a file against an S3 ETag
func validateETag(filePath string, etag string) error {
	// Remove quotes from ETag if present
	etag = strings.Trim(etag, "\"")

	// Check if this is a multipart upload ETag
	if isMultipartETag(etag) {
		// For multipart uploads, we can't easily validate without knowing part sizes
		// S3 multipart ETags are in format: "<md5>-<parts>"
		// The MD5 is computed from the MD5s of each part, not the whole file
		return nil // Skip validation for multipart
	}

	// For simple uploads, ETag is the MD5 of the file
	fileMD5, err := calculateFileMD5(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file MD5: %w", err)
	}

	if fileMD5 != etag {
		return fmt.Errorf("ETag validation failed: expected %s, got %s", etag, fileMD5)
	}

	return nil
}

// calculateFileMD5 calculates the MD5 hash of a file
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// calculateMD5 calculates the MD5 hash of data
func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// calculateBase64MD5 calculates the base64-encoded MD5 hash
func calculateBase64MD5(data []byte) string {
	hash := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// parseMultipartETag parses a multipart ETag and returns the MD5 and part count
func parseMultipartETag(etag string) (md5Hash string, partCount int, err error) {
	// Remove quotes if present
	etag = strings.Trim(etag, "\"")

	// Split by hyphen
	parts := strings.Split(etag, "-")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid multipart ETag format: %s", etag)
	}

	md5Hash = parts[0]
	partCount, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid part count in ETag: %s", parts[1])
	}

	return md5Hash, partCount, nil
}

// isServerSideEncryptedETag checks if an ETag indicates server-side encryption
// SSE-S3 and SSE-KMS encrypted objects have ETags that are not MD5 hashes
func isServerSideEncryptedETag(etag string, metadata map[string]string) bool {
	// Check for server-side encryption headers
	if _, ok := metadata["x-amz-server-side-encryption"]; ok {
		return true
	}

	// SSE-C (customer-provided keys) also affects ETag
	if _, ok := metadata["x-amz-server-side-encryption-customer-algorithm"]; ok {
		return true
	}

	// Check if ETag doesn't look like an MD5 (32 hex characters)
	etag = strings.Trim(etag, "\"")
	if !isMultipartETag(etag) && len(etag) != 32 {
		return true
	}

	return false
}

// validateIntegrity validates the integrity of downloaded data
func (p *S3Provider) validateIntegrity(filePath string, expectedETag string, metadata map[string]string) error {
	if expectedETag == "" {
		// No ETag to validate against
		return nil
	}

	// Skip validation for server-side encrypted objects
	if isServerSideEncryptedETag(expectedETag, metadata) {
		p.logger.Debug("Skipping ETag validation for SSE object")
		return nil
	}

	// Skip validation for multipart uploads
	if isMultipartETag(expectedETag) {
		p.logger.Debug("Skipping ETag validation for multipart upload")
		return nil
	}

	// Validate ETag
	return validateETag(filePath, expectedETag)
}

// validateUploadIntegrity validates the integrity of uploaded data
func (p *S3Provider) validateUploadIntegrity(data []byte, responseETag string) error {
	if responseETag == "" {
		// No ETag returned
		return nil
	}

	// Remove quotes from ETag
	responseETag = strings.Trim(responseETag, "\"")

	// Calculate MD5 of uploaded data
	calculatedMD5 := calculateMD5(data)

	// For simple uploads, ETag should match MD5
	if !isMultipartETag(responseETag) && calculatedMD5 != responseETag {
		return fmt.Errorf("upload integrity check failed: expected ETag %s, got %s", calculatedMD5, responseETag)
	}

	return nil
}
