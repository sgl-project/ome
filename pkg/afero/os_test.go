package afero

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOS_LChown(t *testing.T) {
	fs := NewOsFs()

	dir, err := os.MkdirTemp("", "sml")
	assert.NoError(t, err)

	dstFilePath := filepath.Join(dir, "target")
	err = os.WriteFile(dstFilePath, []byte("hello"), 0777)
	assert.NoError(t, err)

	srcFilePath := filepath.Join(dir, "source")
	err = os.Symlink(dstFilePath, srcFilePath)
	assert.NoError(t, err)

	currentUid, currentGid := mustParseUser(t, user.Current)

	t.Run("checks ownership", func(t *testing.T) {
		uid, gid, err := fs.LOwnership(srcFilePath)
		assert.NoError(t, err)
		assert.Equal(t, currentUid, uid)
		assert.Equal(t, currentGid, gid)
	})

	t.Run("checks ownership of non-existent file", func(t *testing.T) {
		uid, gid, err := fs.LOwnership(srcFilePath + "foobar")
		assert.Error(t, err)
		assert.Equal(t, -1, uid)
		assert.Equal(t, -1, gid)
	})

	t.Run("sets ownership properly", func(t *testing.T) {
		if currentUid != 0 {
			return
		}

		t.Log(srcFilePath)
		// Lchown doesn't work unless tests are run by root
		// A proper test should run in a docker container
		nobodyUid, nobodyGid := mustParseUser(t, func() (*user.User, error) { return user.Lookup("nobody") })
		if nobodyUid == 0 || nobodyGid == 0 {
			panic("weird case")
		}

		err = fs.Lchown(srcFilePath, nobodyUid, nobodyGid)
		assert.NoError(t, err)

		uid, gid, err := fs.LOwnership(srcFilePath)
		assert.NoError(t, err)
		assert.Equal(t, nobodyUid, uid)
		assert.Equal(t, nobodyGid, gid)
	})
}

func mustParseUser(t *testing.T, fn func() (*user.User, error)) (int, int) {
	t.Helper()

	u, err := fn()
	if err != nil {
		t.Errorf("didn't get user: %v", err)
		t.FailNow()
	}

	t.Logf("nobody: %v + %v", u.Uid, u.Gid)

	currentUid, err := strconv.Atoi(u.Uid)
	if err != nil {
		t.Errorf("uid '%s' is bad: %v", u.Uid, err)
		t.FailNow()
	}
	currentGid, err := strconv.Atoi(u.Gid)
	if err != nil {
		t.Errorf("gid '%s' is bad: %v", u.Gid, err)
		t.FailNow()
	}

	return currentUid, currentGid
}
