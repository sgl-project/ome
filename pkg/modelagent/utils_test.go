package modelagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalModelPath(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	real := filepath.Join(root, "real")
	model := filepath.Join(real, "model")
	require.NoError(t, os.MkdirAll(model, 0o755))
	alias := filepath.Join(root, "alias")
	require.NoError(t, os.Symlink(real, alias))
	link := filepath.Join(real, "model-link")
	require.NoError(t, os.Symlink(model, link))
	broken := filepath.Join(real, "broken")
	require.NoError(t, os.Symlink(filepath.Join(root, "absent"), broken))
	loop := filepath.Join(real, "loop")
	require.NoError(t, os.Symlink(loop, loop))

	for _, tc := range []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "directory alias", path: filepath.Join(alias, "model"), want: model},
		{name: "missing directories below alias", path: filepath.Join(alias, "missing", "revision"), want: filepath.Join(real, "missing", "revision")},
		{name: "leaf symlink through alias", path: filepath.Join(alias, "model-link"), want: link},
		{name: "broken leaf symlink", path: filepath.Join(alias, "broken"), want: broken},
		{name: "looping ancestor", path: filepath.Join(loop, "revision"), wantErr: true},
		{name: "empty path", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalModelPath(tc.path)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestModelPathWithin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		parent string
		path   string
		want   bool
	}{
		{name: "same path", parent: "/models/model", path: "/models/model", want: true},
		{name: "descendant", parent: "/models/model", path: "/models/model/releases/rev-002", want: true},
		{name: "sibling prefix", parent: "/models/model", path: "/models/model-bf16"},
		{name: "ancestor", parent: "/models/model", path: "/models"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, modelPathWithin(tc.parent, tc.path))
		})
	}
}

func TestSameStringPtr(t *testing.T) {
	left := "value"
	same := "value"
	different := "different"

	tests := []struct {
		name  string
		left  *string
		right *string
		want  bool
	}{
		{name: "both nil", want: true},
		{name: "left nil", right: &same},
		{name: "right nil", left: &left},
		{name: "same value", left: &left, right: &same, want: true},
		{name: "different value", left: &left, right: &different},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameStringPtr(tt.left, tt.right))
		})
	}
}

func TestSameStringMapPtr(t *testing.T) {
	left := map[string]string{"region": "us-phoenix-1", "auth": "InstancePrincipal"}
	same := map[string]string{"auth": "InstancePrincipal", "region": "us-phoenix-1"}
	differentValue := map[string]string{"region": "us-ashburn-1", "auth": "InstancePrincipal"}
	missingKey := map[string]string{"region": "us-phoenix-1"}
	emptyLeft := map[string]string{}
	emptyRight := map[string]string{}

	tests := []struct {
		name  string
		left  *map[string]string
		right *map[string]string
		want  bool
	}{
		{name: "both nil", want: true},
		{name: "left nil", right: &same},
		{name: "right nil", left: &left},
		{name: "same entries", left: &left, right: &same, want: true},
		{name: "different value", left: &left, right: &differentValue},
		{name: "missing key", left: &left, right: &missingKey},
		{name: "both empty maps", left: &emptyLeft, right: &emptyRight, want: true},
		{name: "nil and empty map", right: &emptyRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sameStringMapPtr(tt.left, tt.right))
		})
	}
}
