// This package wraps spf13's afero and adds a couple of methods that we need
// for testing against in-mem fs

package afero

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/sgl-project/sgl-ome/pkg/logging"
)

type File interface {
	afero.File
}

type Fs interface {
	afero.Fs

	// LOwnership returns the numeric uid and gid of the named file.
	LOwnership(name string) (uid, gid int, err error)

	// Lchown changes the numeric uid and gid of the named file.
	// If the file is a symbolic link, it changes the uid and gid of the link itself.
	// If there is an error, it will be of type *PathError.
	//
	// On Windows, it always returns the syscall.EWINDOWS error, wrapped
	// in *PathError.
	Lchown(name string, uid, gid int) error
}

func TempDir(fs Fs, dir, prefix string) (name string, err error) {
	return afero.TempDir(fs, dir, prefix)
}

func TempFile(fs Fs, dir, prefix string) (f File, err error) {
	return afero.TempFile(fs, dir, prefix)
}

func Walk(fs Fs, root string, walkFn filepath.WalkFunc) error {
	return afero.Walk(fs, root, walkFn)
}

func WriteFile(fs Fs, filename string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(fs, filename, data, perm)
}

func ReadFile(fs Fs, filename string) ([]byte, error) {
	return afero.ReadFile(fs, filename)
}

func ReadDir(fs Fs, dirname string) ([]os.FileInfo, error) {
	return afero.ReadDir(fs, dirname)
}

// AtomicFileUpdate automatically updates a file if file content hasn't changed.
func AtomicFileUpdate(
	fs afero.Fs,
	destDir string,
	destFile string,
	data []byte,
	fileMode os.FileMode,
	log logging.Interface,
) error {
	destPath := filepath.Join(destDir, destFile)
	oldContents, err := afero.ReadFile(fs, destPath)
	if err == nil && bytes.Equal(oldContents, data) {
		return fs.Chmod(destPath, fileMode)
	}

	log.WithField("destPath", destPath).
		Info("Writing file...")

	if isRenameBugged(fs) {
		log.WithField("fsType", fmt.Sprintf("%T", fs)).
			WithField("destPath", destPath).
			Debug("Renaming files in this fs implementation is bugged. " +
				"Skipping atomic rename and just writing into file directly")

		if err := afero.WriteFile(fs, destPath, data, fileMode); err != nil {
			return fmt.Errorf("error writing into a temp file: %v", err)
		}

		return nil
	}

	// there might have been an error (i.e. os.IsNotExist etc.) or contents are different.
	// we'll try to write new contents anyways, as a best effort
	tmp, err := afero.TempFile(fs, destDir, "."+destFile+"~")
	if err != nil {
		return fmt.Errorf("creating tmp file for atomic write: %v", err)
	}
	defer func() { _ = tmp.Close() }()
	defer func() { _ = fs.Remove(tmp.Name()) }()

	if err := afero.WriteFile(fs, tmp.Name(), data, fileMode); err != nil {
		return fmt.Errorf("error writing into a temp file: %v", err)
	}

	return fs.Rename(tmp.Name(), destPath)
}

// HACK(achebatu): MemMapFs has a bug when renaming files.
// Since we're using it only for tests, it's ok not to do atomic rename.
func isRenameBugged(fs afero.Fs) bool {
	switch fs.(type) {
	case *MemMapFs, *afero.MemMapFs:
		return true
	default:
		return false
	}
}

// Exists returns true and nil error if the given path for a file or directory
// exists.
func Exists(fs afero.Fs, path string) (bool, error) {
	return afero.Exists(fs, path)
}
