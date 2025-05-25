package afero

import (
	"github.com/spf13/afero"
)

type BasePathFs struct {
	*afero.BasePathFs
	source Fs
}

// LOwnership returns the numeric uid and gid of the named file.
func (m *BasePathFs) LOwnership(name string) (uid, gid int, err error) {
	path, err := m.BasePathFs.RealPath(name)
	if err != nil {
		return -1, -1, err
	}

	return m.source.LOwnership(path)
}

func (m *BasePathFs) Lchown(name string, uid, gid int) error {
	path, err := m.BasePathFs.RealPath(name)
	if err != nil {
		return err
	}

	return m.source.Lchown(path, uid, gid)
}

var _ Fs = (*BasePathFs)(nil)
var _ afero.Fs = (*BasePathFs)(nil)

func NewBasePathFs(source Fs, path string) Fs {
	return &BasePathFs{
		BasePathFs: afero.NewBasePathFs(source, path).(*afero.BasePathFs),
		source:     source,
	}
}
