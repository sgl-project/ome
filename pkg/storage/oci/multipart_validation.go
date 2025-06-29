package oci

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sgl-project/ome/pkg/storage"
)

// calculateFileMD5 calculates the MD5 checksum of a file
func calculateFileMD5(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Return base64 encoded MD5 (OCI format)
	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}

// isMultipartMD5 checks if an MD5 string represents a multipart upload
func isMultipartMD5(md5String string) bool {
	// OCI multipart MD5s have the format: "<base64md5>-<part count>"
	parts := strings.Split(md5String, "-")
	if len(parts) != 2 {
		return false
	}

	// Try to parse the second part as a number
	var partCount int
	if _, err := fmt.Sscanf(parts[1], "%d", &partCount); err != nil {
		return false
	}

	return partCount > 0
}

// getActualMD5FromMetadata extracts the actual MD5 from object metadata
func getActualMD5FromMetadata(metadata *storage.Metadata) string {
	// Check if this is a multipart upload
	if metadata.IsMultipart && metadata.Headers != nil {
		// Look for MD5 in headers/metadata
		if md5, ok := metadata.Headers["md5"]; ok {
			return md5
		}
		if md5, ok := metadata.Headers["content-md5"]; ok {
			return md5
		}
	}

	// For non-multipart or if no MD5 in metadata, use ContentMD5
	return metadata.ContentMD5
}

// validateMultipartMD5 validates a file against a multipart object
func (s *OCIStorage) validateMultipartMD5(localPath string, metadata *storage.Metadata) (bool, error) {
	// For multipart uploads, we need to check the metadata for the actual MD5
	actualMD5 := getActualMD5FromMetadata(metadata)

	if actualMD5 == "" {
		s.logger.Warnf("No MD5 available for multipart object; skipping integrity check")
		return true, nil // Can't validate without MD5
	}

	// Calculate local file MD5
	localMD5, err := calculateFileMD5(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate local file MD5: %w", err)
	}

	// Compare MD5s
	if actualMD5 == localMD5 {
		return true, nil
	}

	// Try hex format if base64 doesn't match
	if localMD5Hex, err := storage.ValidateFileMD5(localPath, actualMD5); err == nil {
		return localMD5Hex, nil
	}

	s.logger.Warnf("MD5 mismatch for %s: expected %s, got %s", localPath, actualMD5, localMD5)
	return false, nil
}

// enhanceMultipartUploadWithMD5 adds MD5 to metadata for multipart uploads
func enhanceMultipartUploadWithMD5(metadata map[string]string, filePath string) error {
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Calculate file MD5
	md5Sum, err := calculateFileMD5(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5 for multipart upload: %w", err)
	}

	// Store in metadata
	metadata["md5"] = md5Sum

	return nil
}
