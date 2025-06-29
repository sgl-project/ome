package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
