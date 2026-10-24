package modelagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestCollectReferencedPathsExcludesModelsAndDeduplicatesPaths(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "model")
	kept := filepath.Join(target, "revision")
	missing := filepath.Join(target, "future-revision")
	deleted := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "model", Namespace: "deleted"},
		Spec:       v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(target)}},
	}
	deletedCluster := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "deleted-model"},
		Spec:       deleted.Spec,
	}
	g := &Gopher{
		baseModelLister: &mockBaseModelLister{models: []*v1beta1.BaseModel{
			deleted,
			{ObjectMeta: metav1.ObjectMeta{Name: deleted.Name, Namespace: "other"}, Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(kept)}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "sibling"}, Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(target + "-bf16")}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "without-storage"}},
		}},
		clusterBaseModelLister: &mockClusterBaseModelLister{models: []*v1beta1.ClusterBaseModel{
			deletedCluster,
			{ObjectMeta: metav1.ObjectMeta{Name: "duplicate"}, Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(kept + "/")}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "future"}, Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(missing)}}},
		}},
	}

	paths, err := g.collectReferencedPaths(target, deleted, deletedCluster)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{kept, missing}, paths)
}

func TestCollectReferencedPathsPreservesSymlinkReferences(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(root, "model")
	first := filepath.Join(target, "rev-001")
	second := filepath.Join(target, "rev-002")
	require.NoError(t, os.MkdirAll(first, 0o755))
	require.NoError(t, os.MkdirAll(second, 0o755))
	alias := filepath.Join(root, "alias")
	require.NoError(t, os.Symlink(root, alias))
	aliasedTarget := filepath.Join(alias, "model")
	consumer := filepath.Join(root, "consumer")
	require.NoError(t, os.Symlink(second, consumer))
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "revision"), 0o755))
	container := filepath.Join(target, "mounted")
	require.NoError(t, os.Symlink(outside, container))
	throughContainer := filepath.Join(aliasedTarget, "mounted", "revision")
	g := &Gopher{
		baseModelLister: &mockBaseModelLister{models: []*v1beta1.BaseModel{
			{Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(filepath.Join(aliasedTarget, "rev-001"))}}},
			{Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(consumer)}}},
			{Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(throughContainer)}}},
		}},
		clusterBaseModelLister: &mockClusterBaseModelLister{},
	}

	paths, err := g.collectReferencedPaths(aliasedTarget, nil, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{first, second, filepath.Join(container, "revision")}, paths)
}

func TestCollectReferencedPathsDistinguishesLeafSymlinkReferences(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	revision := filepath.Join(root, "shared", "revision")
	require.NoError(t, os.MkdirAll(revision, 0o755))
	link := filepath.Join(root, "model")
	require.NoError(t, os.Symlink(filepath.Dir(revision), link))
	consumer := &v1beta1.BaseModel{Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(revision)}}}
	g := &Gopher{
		baseModelLister:        &mockBaseModelLister{models: []*v1beta1.BaseModel{consumer}},
		clusterBaseModelLister: &mockClusterBaseModelLister{},
	}

	paths, err := g.collectReferencedPaths(link, nil, nil)
	require.NoError(t, err)
	require.Empty(t, paths, "using the real directory does not require this model symlink")
	throughLink := filepath.Join(link, "revision")
	consumer.Spec.Storage.Path = stringPtr(throughLink)
	paths, err = g.collectReferencedPaths(link, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{throughLink}, paths, "a consumer using this symlink must retain it")
}

func TestCollectReferencedPathsUnresolvableReference(t *testing.T) {
	root := t.TempDir()
	loop := filepath.Join(root, "loop")
	require.NoError(t, os.Symlink(loop, loop))
	g := &Gopher{
		baseModelLister: &mockBaseModelLister{models: []*v1beta1.BaseModel{{
			ObjectMeta: metav1.ObjectMeta{Name: "consumer"},
			Spec:       v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{Path: stringPtr(loop)}},
		}}},
		clusterBaseModelLister: &mockClusterBaseModelLister{},
	}
	paths, err := g.collectReferencedPaths(filepath.Join(root, "model"), nil, nil)
	require.ErrorContains(t, err, "check BaseModel")
	require.Nil(t, paths)
}

func TestArtifactCleanupStaysWithinOpenedDirectory(t *testing.T) {
	root := t.TempDir()
	parentPath := filepath.Join(root, "parent")
	modelPath := filepath.Join(parentPath, "model")
	writeSelectiveDeletionFile(t, filepath.Join(modelPath, "weights"))
	outside := filepath.Join(root, "outside", "model", "weights")
	writeSelectiveDeletionFile(t, outside)
	parent, err := os.Open(parentPath)
	require.NoError(t, err)
	defer parent.Close()
	moved := filepath.Join(root, "moved")
	require.NoError(t, os.Rename(parentPath, moved))
	require.NoError(t, os.Symlink(filepath.Dir(filepath.Dir(outside)), parentPath))

	retained, err := removeUnreferencedFilesAt(parent, "model", modelPath, nil)
	require.NoError(t, err)
	require.False(t, retained)
	contents, err := os.ReadFile(outside)
	require.NoError(t, err, "cleanup must not follow a replacement ancestor symlink")
	require.Equal(t, outside, string(contents))
	require.NoDirExists(t, filepath.Join(moved, "model"))
}
