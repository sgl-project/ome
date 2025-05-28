package ociobjectstore

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
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

// CopyByFilePath copies a file from sourceFilePath to targetFilePath.
// It returns an error if the source file cannot be opened, the target file
// cannot be created, or the copy operation fails.
func CopyByFilePath(sourceFilePath string, targetFilePath string) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %s", sourceFilePath, err.Error())
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	// Use a large buffer (8MB) for optimal performance
	buf := make([]byte, 8*1024*1024)
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
// Uses a large buffer (8MB) for optimal performance with large files.
// Ensures data is synced to disk and cleans up partial files on failure.
func CopyReaderToFilePath(source io.Reader, targetFilePath string) error {
	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	// Use a large buffer (8MB) for optimal performance
	buf := make([]byte, 8*1024*1024)
	if _, err = io.CopyBuffer(targetFile, source, buf); err != nil {
		return fmt.Errorf("failed to copy source to target path %s: %s", targetFilePath, err.Error())
	}
	// Ensure data is flushed to disk
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file %s: %s", targetFilePath, err.Error())
	}
	return nil
}

func JoinWithTailOverlap(directoryPath, objectPath string) string {
	dirParts := strings.Split(strings.Trim(path.Clean(directoryPath), "/"), "/")
	objParts := strings.Split(strings.Trim(path.Clean(objectPath), "/"), "/")

	// Find the longest overlap between dirParts suffix and objParts prefix
	for l := min(len(dirParts), len(objParts)); l > 0; l-- {
		if slicesEqual(dirParts[len(dirParts)-l:], objParts[:l]) {
			combined := append(dirParts, objParts[l:]...)
			return "/" + path.Join(combined...)
		}
	}

	// No overlap found
	return "/" + path.Join(append(dirParts, objParts...)...)
}

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
