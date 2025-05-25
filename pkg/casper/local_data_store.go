package casper

import (
	"fmt"
	"os"
	"path/filepath"
)

/*
LocalDataStore provides methods to interact with the local file system as a data store.

It supports operations such as downloading a file from the local store to a target directory
and uploading a file into a structured working directory.
*/
type LocalDataStore struct {
	WorkingDirectory string // Base directory where files will be stored or retrieved from
}

// createWorkingDirectory ensures that the working directory exists.
// It creates all necessary parent directories using os.MkdirAll.
func (lds *LocalDataStore) createWorkingDirectory() error {
	return os.MkdirAll(lds.WorkingDirectory, os.ModePerm)
}

// Download copies a file from the local data store to the target directory.
//
// It constructs the source path using lds.WorkingDirectory and the ObjectName in `source`,
// creates the target directory if needed, and then copies the file there.
//
// Parameters:
//   - source: ObjectURI that includes the object (file) name to be copied
//   - target: the directory path where the file should be copied to
//   - opts: functional options (ignored for local data store)
//
// Returns an error if directory creation or file copy fails.
func (lds *LocalDataStore) Download(source ObjectURI, target string, opts ...DownloadOption) error {
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create target directory %s: %s", target, err.Error())
	}

	dataSourcePath := filepath.Join(lds.WorkingDirectory, source.ObjectName)
	dataTargetPath := filepath.Join(target, source.ObjectName)
	return CopyByFilePath(dataSourcePath, dataTargetPath)
}

// Upload copies a file from the given source path into the local data store.
//
// It ensures the working directory exists and then writes the file into
// lds.WorkingDirectory using the ObjectName from `target`.
//
// Parameters:
//   - source: full path to the source file to be uploaded
//   - target: ObjectURI containing the target object (file) name
//
// Returns an error if the working directory cannot be created or if the copy fails.
func (lds *LocalDataStore) Upload(source string, target ObjectURI) error {
	err := lds.createWorkingDirectory()
	if err != nil {
		return fmt.Errorf("failed to create working directory %s: %s", target, err.Error())
	}

	dataTargetPath := filepath.Join(lds.WorkingDirectory, target.ObjectName)
	return CopyByFilePath(source, dataTargetPath)
}
