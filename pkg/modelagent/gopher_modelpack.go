package modelagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/llmman"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

// processModelPackModel acquires a CNCF ModelPack OCI artifact through a
// running `llmman serve` daemon.
//
// The daemon owns the registry work -- ModelPack media types, registry auth,
// resumable blob download and a content-addressed store -- so it is not
// reimplemented here. It does the pull (POST /api/pull, streamed as NDJSON so
// a multi-gigabyte fetch is not silent) but deliberately exposes no local
// path, so `llmman resolve --no-pull` reports where the bytes landed.
func (s *Gopher) processModelPackModel(ctx context.Context, task *GopherTask,
	baseModelSpec v1beta1.BaseModelSpec, modelInfo, modelType, namespace, name string) error {

	components, err := storage.ParseOCIStorageURI(*baseModelSpec.Storage.StorageUri)
	if err != nil {
		s.logger.Errorf("Failed to parse OCI URI for model %s: %v", modelInfo, err)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "invalid_oci_uri")
		s.markModelOnNodeFailed(task)
		return err
	}

	destPath := getDestPath(&baseModelSpec, s.modelRootDir)

	progress := func(status string, completed, total int64) {
		if total > 0 {
			s.logger.Infof("llmman: %s (%d/%d bytes) for model %s", status, completed, total, modelInfo)
		} else {
			s.logger.Infof("llmman: %s for model %s", status, modelInfo)
		}
	}

	s.logger.Infof("Pulling ModelPack artifact %s for model %s", components.Reference, modelInfo)
	resolved, err := llmman.PullAndResolve(ctx, components.Reference, progress)
	if err != nil {
		s.logger.Errorf("Failed to pull ModelPack artifact for model %s: %v", modelInfo, err)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "llmman_pull_failed")
		s.markModelOnNodeFailed(task)
		return err
	}

	if err := materializeModelPack(resolved, destPath); err != nil {
		s.logger.Errorf("Failed to place ModelPack artifact for model %s: %v", modelInfo, err)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "llmman_place_failed")
		s.markModelOnNodeFailed(task)
		return err
	}

	s.logger.Infof("Placed ModelPack artifact %s at %s", components.Reference, destPath)
	return nil
}

// materializeModelPack places llmman's extracted artifact at dest.
//
// Files are hard-linked where possible so a model shared with llmman's store
// costs its bytes once, falling back to a copy across filesystems.
func materializeModelPack(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("resolved path %q: %w", src, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("creating %q: %w", dest, err)
	}

	if !info.IsDir() {
		// A single-file payload, e.g. GGUF.
		return linkOrCopyFile(src, filepath.Join(dest, filepath.Base(src)))
	}

	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !fi.Mode().IsRegular() {
			// Symlinks and devices carry no model payload.
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return linkOrCopyFile(path, target)
	})
}

func linkOrCopyFile(src, dest string) error {
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Link(src, dest); err == nil {
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
