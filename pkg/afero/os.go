package afero

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/afero"
)

type OsFs struct {
	*afero.OsFs
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

var _ Fs = (*OsFs)(nil)
var _ afero.Fs = (*OsFs)(nil)

func NewOsFs() Fs {
	return &OsFs{
		OsFs: afero.NewOsFs().(*afero.OsFs),
	}
}
