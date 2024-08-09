package afero

import "github.com/spf13/afero"

// Glob returns the names of all files matching pattern or nil
// if there is no matching file. The syntax of patterns is the same
// as in Match. The pattern may describe hierarchical names such as
// /usr/*/bin/ed (assuming the Separator is '/').
//
// Glob ignores file system errors such as I/O errors reading directories.
// The only possible returned error is ErrBadPattern, when pattern
// is malformed.
//
// This was adapted from (http://golang.org/pkg/path/filepath) and uses several
// built-ins from that package.
func Glob(fs Fs, pattern string) (matches []string, err error) {
	// TODO(achebatu): cherry-pick infinite recursion bugfix from upstream:
	//  https://golang.org/src/path/filepath/match.go#255

	return afero.Glob(fs, pattern)
}
