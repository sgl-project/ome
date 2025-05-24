package afero

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestMemMapFs_Lchown(t *testing.T) {
	fs := NewMemMapFs()
	srcFilePath := "/target"
	err := afero.WriteFile(fs, srcFilePath, []byte("hello"), 0777)
	assert.NoError(t, err)

	t.Run("checks ownership", func(t *testing.T) {
		uid, gid, err := fs.LOwnership(srcFilePath)
		assert.NoError(t, err)
		assert.Equal(t, -1, uid)
		assert.Equal(t, -1, gid)
	})

	t.Run("checks ownership of non-existent file", func(t *testing.T) {
		uid, gid, err := fs.LOwnership(srcFilePath + "foobar")
		assert.Error(t, err)
		assert.Equal(t, -1, uid)
		assert.Equal(t, -1, gid)
	})

	t.Run("sets ownership properly", func(t *testing.T) {
		// Lchown doesn't work unless tests are run by root
		// A proper test should run in a docker container
		err = fs.Lchown(srcFilePath, 1234, 12345)

		uid, gid, err := fs.LOwnership(srcFilePath)
		assert.NoError(t, err)
		assert.Equal(t, 1234, uid)
		assert.Equal(t, 12345, gid)
	})
}
