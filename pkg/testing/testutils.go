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
