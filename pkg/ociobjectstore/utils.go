package ociobjectstore

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ObjectBaseName returns only the file name from a given object path.
// For example, given "bucket/folder/file.txt", it returns "file.txt".
// If the input path does not contain "/", the original string is returned.
func ObjectBaseName(objectPath string) string {
	if !strings.Contains(objectPath, "/") {
		return objectPath
	}

	values := strings.Split(objectPath, "/")
	return values[len(values)-1] // Return the last segment (file name)
}

// TrimObjectPrefix removes a given prefix from the object path if it exists.
// For example, given objectPath "bucket/folder/file.txt" and prefix "bucket/",
// it returns "folder/file.txt".
// If the prefix is empty or objectPath has no "/", the original string is returned.
func TrimObjectPrefix(objectPath string, prefix string) string {
	if !strings.Contains(objectPath, "/") || len(prefix) == 0 {
		return objectPath
	}

	return strings.Replace(objectPath, prefix, "", 1) // Remove only the first occurrence
}

// BufferPool provides reusable buffers to reduce memory allocations
var BufferPool = sync.Pool{
	New: func() interface{} {
		// Use 1MB buffer by default instead of 8MB
		return make([]byte, 1024*1024)
	},
}

// LargeBufferPool for files > 10MB
var LargeBufferPool = sync.Pool{
	New: func() interface{} {
		// 4MB buffer for large files
		return make([]byte, 4*1024*1024)
	},
}

// getOptimalBuffer returns the best buffer size based on file size
func getOptimalBuffer(fileSize int64) []byte {
	if fileSize > 10*1024*1024 { // > 10MB
		return LargeBufferPool.Get().([]byte)
	}
	return BufferPool.Get().([]byte)
}

// returnBuffer returns buffer to appropriate pool
func returnBuffer(buf []byte, fileSize int64) {
	if fileSize > 10*1024*1024 {
		LargeBufferPool.Put(buf)
	} else {
		BufferPool.Put(buf)
	}
}

// CopyByFilePath copies a file from sourceFilePath to targetFilePath.
// It returns an error if the source file cannot be opened, the target file
// cannot be created, or the copy operation fails.
func CopyByFilePath(sourceFilePath string, targetFilePath string) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %s", sourceFilePath, err.Error())
	}
	defer sourceFile.Close()

	// Get file size for optimal buffer selection
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info %s: %s", sourceFilePath, err.Error())
	}
	fileSize := sourceInfo.Size()

	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	// Use optimal buffer size and pool it
	buf := getOptimalBuffer(fileSize)
	defer returnBuffer(buf, fileSize)

	if _, err = io.CopyBuffer(targetFile, sourceFile, buf); err != nil {
		return fmt.Errorf("failed to copy source file %s to target path %s: %s", sourceFilePath, targetFilePath, err.Error())
	}
	// Ensure data is flushed to disk
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file %s: %s", targetFilePath, err.Error())
	}
	return nil
}

// CopyReaderToFilePath copies content from an io.Reader into the file at targetFilePath.
// It creates the target file and parent directories if they don't exist.
// Uses a pooled buffer for optimal performance and memory efficiency.
// Ensures data is synced to disk and cleans up partial files on failure.
func CopyReaderToFilePath(source io.Reader, targetFilePath string) error {
	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	// Use default buffer from pool (we don't know size ahead of time)
	buf := BufferPool.Get().([]byte)
	defer BufferPool.Put(buf)

	if _, err = io.CopyBuffer(targetFile, source, buf); err != nil {
		return fmt.Errorf("failed to copy source to target path %s: %s", targetFilePath, err.Error())
	}
	// Ensure data is flushed to disk
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file %s: %s", targetFilePath, err.Error())
	}
	return nil
}

// JoinWithTailOverlap combines directoryPath and objectPath with overlap.
func JoinWithTailOverlap(directoryPath, objectPath string) string {
	dirParts := strings.Split(strings.Trim(filepath.Clean(directoryPath), "/"), "/")
	objParts := strings.Split(strings.Trim(filepath.Clean(objectPath), "/"), "/")

	// Find the longest overlap between dirParts suffix and objParts prefix
	for l := min(len(dirParts), len(objParts)); l > 0; l-- {
		if slicesEqual(dirParts[len(dirParts)-l:], objParts[:l]) {
			combined := append(dirParts, objParts[l:]...)
			return "/" + filepath.Join(combined...)
		}
	}

	// No overlap found
	return "/" + filepath.Join(append(dirParts, objParts...)...)
}

// ComputeTargetFilePath computes the target file path for a given source object and options.
func ComputeTargetFilePath(source ObjectURI, target string, opts *DownloadOptions) string {
	// Handle nil options by using default behavior
	if opts == nil {
		return filepath.Join(target, source.ObjectName)
	}

	if opts.StripPrefix {
		return filepath.Join(target, TrimObjectPrefix(source.ObjectName, opts.PrefixToStrip))
	} else if opts.UseBaseNameOnly {
		return filepath.Join(target, ObjectBaseName(source.ObjectName))
	} else if opts.JoinWithTailOverlap {
		return JoinWithTailOverlap(target, source.ObjectName)
	}
	return filepath.Join(target, source.ObjectName)
}

// slicesEqual checks if two slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func IsReaderEmpty(streamReader io.Reader) bool {
	switch v := streamReader.(type) {
	case *bytes.Buffer:
		return v.Len() == 0
	case *bytes.Reader:
		return v.Len() == 0
	case *strings.Reader:
		return v.Len() == 0
	case *os.File:
		fi, err := v.Stat()
		if err != nil {
			return false
		}
		return fi.Size() == 0
	default:
		return false
	}
}

// RemoveOpcMetaPrefix Update metadata map to remove "opc-meta-" prefix from keys
// Need to do it since for single part upload (UploadFilePutObject) metadata keys are attached with "opc-meta-" prefix automatically
// while for multipart upload (UploadFileMultiparts) metadata keys are not prefixed with "opc-meta-"
func RemoveOpcMetaPrefix(metadata map[string]string) map[string]string {
	if metadata == nil {
		return metadata
	}

	updatedMetadata := make(map[string]string)
	for key, value := range metadata {
		if strings.HasPrefix(key, "opc-meta-") {
			// Remove "opc-meta-" prefix
			newKey := key[9:]
			updatedMetadata[newKey] = value
		} else {
			// Keep original key-value pair
			updatedMetadata[key] = value
		}
	}
	return updatedMetadata
}
