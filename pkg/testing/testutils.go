package testing

import (
	"os"
)

// TempDir will return a temporary directory and a closer func for deleting
// the directory tree.
func TempDir() (string, func(), error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", nil, err
	}
	return tmp, func() { _ = os.RemoveAll(tmp) }, nil
}

// TempFile will return a temporary file and a closer func for the file.
func TempFile() (*os.File, func(), error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return nil, nil, err
	}
	return tmp, func() { _ = os.Remove(tmp.Name()) }, nil
}
