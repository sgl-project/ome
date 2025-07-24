package afero

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/afero"
)

type OsFs struct {
	*afero.OsFs
}

type FileEntry struct {
	FileInfo os.FileInfo
	FilePath string
}

// Lchown changes the numeric uid and gid of the named file.
// If the file is a symbolic link, it changes the uid and gid of the link itself.
// If there is an error, it will be of type *PathError.
//
// On Windows, it always returns the syscall.EWINDOWS error, wrapped
// in *PathError.
func (m *OsFs) Lchown(name string, uid, gid int) error {
	return os.Lchown(name, uid, gid)
}

// LOwnership returns the numeric uid and gid of the named file.
func (m *OsFs) LOwnership(name string) (uid, gid int, err error) {
	info, err := os.Lstat(name)
	if err != nil {
		return -1, -1, err
	}

	sys := info.Sys()
	switch stat := sys.(type) {
	case *syscall.Stat_t: // MacOS
		return int(stat.Uid), int(stat.Gid), nil
	case syscall.Stat_t: // linux
		return int(stat.Uid), int(stat.Gid), nil
	}

	return -1, -1, fmt.Errorf("unable to get ownership info of %s", name)
}

func (m *OsFs) ListFiles(dir string) ([]FileEntry, error) {
	var files []FileEntry
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err // Stop walking on error
		}

		if d.IsDir() {
			return nil // Skip directories
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		files = append(files, FileEntry{
			FileInfo: info,
			FilePath: path,
		})
		return nil
	})
	return files, err
}

var _ Fs = (*OsFs)(nil)
var _ afero.Fs = (*OsFs)(nil)

func NewOsFs() Fs {
	return &OsFs{
		OsFs: afero.NewOsFs().(*afero.OsFs),
	}
}

func CopyFileBetweenFS(srcFs, dstFs afero.Fs, srcPath, dstPath string, mode os.FileMode) error {
	in, err := srcFs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		cerr := in.Close()
		if cerr != nil && err == nil {
			err = fmt.Errorf("error closing source file: %w", cerr)
		}
	}()

	out, err := dstFs.Create(dstPath)
	if err != nil {
		_ = in.Close() // Best effort
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		cerr := out.Close()
		if cerr != nil && err == nil {
			err = fmt.Errorf("error closing destination file: %w", cerr)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	if err = dstFs.Chmod(dstPath, mode); err != nil {
		return fmt.Errorf("failed to chmod destination file: %w", err)
	}
	return nil
}
