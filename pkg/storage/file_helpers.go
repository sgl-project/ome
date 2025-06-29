package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GetFileInfo returns file information
func GetFileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// MkdirAll creates a directory and all necessary parents
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// CreateFile creates or truncates the named file
func CreateFile(name string) (*os.File, error) {
	return os.Create(name)
}

// OpenFile opens a file for reading
func OpenFile(name string) (*os.File, error) {
	return os.Open(name)
}

// CopyData copies from reader to writer
func CopyData(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// FileExists checks if a file exists
func FileExists(filepath string) (bool, error) {
	_, err := os.Stat(filepath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsLocalFileValid checks if a local file matches the expected metadata
func IsLocalFileValid(filepath string, metadata Metadata) (bool, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return false, err
	}

	// Check size first
	if fileInfo.Size() != metadata.Size {
		return false, nil
	}

	// Check MD5 if available
	if metadata.ContentMD5 != "" {
		valid, err := ValidateFileMD5(filepath, metadata.ContentMD5)
		if err != nil {
			return false, err
		}
		return valid, nil
	}

	// If no MD5, consider valid if size matches
	return true, nil
}

// WriteReaderToFile writes reader content to a file
func WriteReaderToFile(reader io.Reader, filePath string) error {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ComputeLocalPath computes the local file path based on download options
func ComputeLocalPath(targetDir, objectName string, opts DownloadOptions) string {
	// Handle UseBaseNameOnly option - use only the filename
	if opts.UseBaseNameOnly {
		return filepath.Join(targetDir, filepath.Base(objectName))
	}

	// Handle StripPrefix option - remove prefix from object path
	if opts.StripPrefix && opts.PrefixToStrip != "" {
		trimmed := strings.TrimPrefix(objectName, opts.PrefixToStrip)
		// If prefix was removed, use the trimmed path
		if trimmed != objectName {
			objectName = trimmed
		}
		return filepath.Join(targetDir, objectName)
	}

	// Handle JoinWithTailOverlap option - smart path joining
	if opts.JoinWithTailOverlap {
		return joinWithTailOverlap(targetDir, objectName)
	}

	// Default behavior - simple join
	return filepath.Join(targetDir, objectName)
}

// joinWithTailOverlap joins paths intelligently by detecting overlap between
// the tail of the target directory and the head of the object name
func joinWithTailOverlap(targetDir, objectName string) string {
	// Handle special cases
	if targetDir == "" {
		return objectName
	}
	if objectName == "" {
		return targetDir
	}

	// Preserve absolute path indicator
	isAbsolute := strings.HasPrefix(targetDir, "/") || filepath.IsAbs(targetDir)

	// Split both paths into components
	targetParts := strings.Split(filepath.ToSlash(targetDir), "/")
	objectParts := strings.Split(filepath.ToSlash(objectName), "/")

	// Remove empty parts
	targetParts = removeEmptyParts(targetParts)
	objectParts = removeEmptyParts(objectParts)

	if len(targetParts) == 0 || len(objectParts) == 0 {
		return filepath.Join(targetDir, objectName)
	}

	// Find overlap - check if the tail of target matches head of object
	maxOverlap := min(len(targetParts), len(objectParts))
	overlap := 0

	for i := 1; i <= maxOverlap; i++ {
		match := true
		for j := 0; j < i; j++ {
			if targetParts[len(targetParts)-i+j] != objectParts[j] {
				match = false
				break
			}
		}
		if match {
			overlap = i
		}
	}

	// Join paths with overlap removed
	var resultPath string
	if overlap > 0 {
		result := make([]string, 0, len(targetParts)+len(objectParts)-overlap)
		result = append(result, targetParts...)
		result = append(result, objectParts[overlap:]...)
		resultPath = filepath.Join(result...)
	} else {
		// No overlap found, use simple join
		resultPath = filepath.Join(targetDir, objectName)
	}

	// Restore absolute path if needed
	if isAbsolute && !filepath.IsAbs(resultPath) {
		resultPath = "/" + resultPath
	}

	return resultPath
}

// removeEmptyParts removes empty strings from a slice
func removeEmptyParts(parts []string) []string {
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
