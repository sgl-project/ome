package modelagent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// collectReferencedPaths snapshots exact and descendant references before any
// file removal. Symlink consumers also protect their real directory in target.
func (s *Gopher) collectReferencedPaths(target string, excludeBaseModel *v1beta1.BaseModel, excludeClusterBaseModel *v1beta1.ClusterBaseModel) ([]string, error) {
	lexicalTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	target, err = canonicalModelPath(target)
	if err != nil {
		return nil, err
	}
	if s.baseModelLister == nil || s.clusterBaseModelLister == nil {
		return nil, fmt.Errorf("model listers are unavailable for artifact deletion")
	}
	baseModels, err := s.baseModelLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list BaseModels: %w", err)
	}
	clusterBaseModels, err := s.clusterBaseModelLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterBaseModels: %w", err)
	}

	references := make(map[string]struct{})
	add := func(path string) {
		if modelPathWithin(target, path) {
			references[path] = struct{}{}
		}
	}
	collect := func(spec v1beta1.BaseModelSpec) error {
		if spec.Storage == nil {
			return nil
		}
		var path string
		if spec.Storage.Path != nil && *spec.Storage.Path != "" {
			path = *spec.Storage.Path
		} else if spec.Storage.StorageUri != nil {
			path = getDestPath(&spec, s.modelRootDir)
		} else {
			return nil
		}
		lexicalPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		// Preserve a symlink container when a consumer names a path through it.
		if modelPathWithin(lexicalTarget, lexicalPath) {
			relative, err := filepath.Rel(lexicalTarget, lexicalPath)
			if err != nil {
				return err
			}
			add(filepath.Join(target, relative))
		}
		canonical, err := canonicalModelPath(path)
		if err != nil {
			return err
		}
		add(canonical)
		resolved, err := filepath.EvalSymlinks(lexicalPath)
		if err == nil {
			add(resolved)
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	for _, model := range baseModels {
		if excludeBaseModel != nil && model.Namespace == excludeBaseModel.Namespace && model.Name == excludeBaseModel.Name {
			continue
		}
		if err := collect(model.Spec); err != nil {
			return nil, fmt.Errorf("check BaseModel %s/%s path: %w", model.Namespace, model.Name, err)
		}
	}
	for _, model := range clusterBaseModels {
		if excludeClusterBaseModel != nil && model.Name == excludeClusterBaseModel.Name {
			continue
		}
		if err := collect(model.Spec); err != nil {
			return nil, fmt.Errorf("check ClusterBaseModel %s path: %w", model.Name, err)
		}
	}
	paths := make([]string, 0, len(references))
	for path := range references {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// removeUnreferencedFiles returns whether path was retained. Directory handles
// keep cleanup inside the opened tree even if an ancestor path is replaced.
func removeUnreferencedFiles(path string, protectedPaths []string) (bool, error) {
	parent, err := os.Open(filepath.Dir(path))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer parent.Close()
	return removeUnreferencedFilesAt(parent, filepath.Base(path), path, protectedPaths)
}

func removeUnreferencedFilesAt(parent *os.File, name, path string, protectedPaths []string) (bool, error) {
	if name == hfArtifactLockDirectory || name == constants.ModelArtifactsDirectory {
		return true, nil
	}
	containsReference := false
	for _, protected := range protectedPaths {
		if modelPathWithin(protected, path) {
			return true, nil
		}
		containsReference = containsReference || modelPathWithin(path, protected)
	}
	parentFD := int(parent.Fd())
	var stat unix.Stat_t
	err := unix.Fstatat(parentFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if os.IsNotExist(err) {
		return containsReference, nil
	}
	if err != nil {
		return false, &os.PathError{Op: "fstatat", Path: path, Err: err}
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		if containsReference {
			return true, nil
		}
		if err := unix.Unlinkat(parentFD, name, 0); err != nil && !os.IsNotExist(err) {
			return false, &os.PathError{Op: "unlinkat", Path: path, Err: err}
		}
		return false, nil
	}
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if os.IsNotExist(err) {
		return containsReference, nil
	}
	if err != nil {
		return false, &os.PathError{Op: "openat", Path: path, Err: err}
	}
	directory := os.NewFile(uintptr(fd), path)
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return false, err
	}
	retained := containsReference
	for _, child := range names {
		kept, err := removeUnreferencedFilesAt(directory, child, filepath.Join(path, child), protectedPaths)
		if err != nil {
			return false, err
		}
		retained = retained || kept
	}
	if retained {
		return true, nil
	}
	if err := unix.Unlinkat(parentFD, name, unix.AT_REMOVEDIR); err != nil && !os.IsNotExist(err) {
		return false, &os.PathError{Op: "unlinkat", Path: path, Err: err}
	}
	return false, nil
}
