package casper

import (
	"fmt"
	"os"
	"path/filepath"
)

/*
 * LocalDataStore used to perform data store operations with local file system
 */

type LocalDataStore struct {
	WorkingDirectory string
}

func (lds *LocalDataStore) createWorkingDirectory() error {
	return os.MkdirAll(lds.WorkingDirectory, os.ModePerm)
}

func (lds *LocalDataStore) Download(source ObjectURI, target string) error {
	err := os.MkdirAll(target, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create target directory %s: %s", target, err.Error())
	}

	dataSourcePath := filepath.Join(lds.WorkingDirectory, source.ObjectName)
	dataTargetPath := filepath.Join(target, source.ObjectName)
	return CopyByFilePath(dataSourcePath, dataTargetPath)
}

func (lds *LocalDataStore) Upload(source string, target ObjectURI) error {
	err := lds.createWorkingDirectory()
	if err != nil {
		return fmt.Errorf("failed to create working directory %s: %s", lds.WorkingDirectory, err.Error())
	}

	dataTargetPath := filepath.Join(lds.WorkingDirectory, target.ObjectName)
	return CopyByFilePath(source, dataTargetPath)
}
