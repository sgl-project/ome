package afero

import (
	"fmt"
	"sync"

	"github.com/spf13/afero"
)

type ownership struct {
	uid, gid int
}

type MemMapFs struct {
	*afero.MemMapFs

	sync.Mutex
	owners map[string]ownership
}

// LOwnership returns the numeric uid and gid of the named file.
func (m *MemMapFs) LOwnership(name string) (uid, gid int, err error) {
	m.Lock()
	defer m.Unlock()

	_, err = m.Stat(name)
	if err != nil {
		return -1, -1, fmt.Errorf("stat for %s: %v", name, err)
	}

	own, ok := m.owners[name]
	if !ok {
		return -1, -1, nil
	}

	return own.uid, own.gid, nil
}

func (m *MemMapFs) Lchown(name string, uid, gid int) error {
	m.Lock()
	defer m.Unlock()

	m.owners[name] = ownership{uid, gid}
	return nil
}

var _ Fs = (*MemMapFs)(nil)
var _ afero.Fs = (*MemMapFs)(nil)

func NewMemMapFs() Fs {
	return &MemMapFs{
		MemMapFs: afero.NewMemMapFs().(*afero.MemMapFs),
		owners:   make(map[string]ownership),
	}
}
