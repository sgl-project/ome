package modelagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Recovery-image only: opt in with node/CR-UID/generation after restoring the
// original directory. Freeze changes to these CRs and retain the parent keeper
// throughout recovery. Keep the annotation until returning to the normal image
// so retries after reference removal cannot fall through to a download.
const localArtifactRestoreAnnotation = "ome.io/restore-local-artifact"

func localArtifactRestoreToken(node string, model *v1beta1.BaseModel) string {
	return fmt.Sprintf("%s/%s/%d", node, model.UID, model.Generation)
}

func (s *Gopher) processLocalArtifactRestore(ctx context.Context, task *GopherTask, allowValidation bool, validate hfArtifactValidateFunc) (bool, error) {
	model := task.BaseModel
	if model == nil {
		return false, nil
	}
	if s.baseModelLister == nil {
		if model.Annotations[localArtifactRestoreAnnotation] != "" {
			return true, fmt.Errorf("local restore requires the BaseModel lister")
		}
		return false, nil
	}
	live, lookupErr := s.baseModelLister.BaseModels(model.Namespace).Get(model.Name)
	if model.Annotations[localArtifactRestoreAnnotation] == "" && (lookupErr != nil || live.Annotations[localArtifactRestoreAnnotation] == "") {
		return false, nil
	}
	if lookupErr != nil {
		return true, lookupErr
	}
	if s.configMapReconciler == nil || (task.TaskType != Download && task.TaskType != DownloadOverride) || model.UID == "" || live.UID != model.UID || live.Generation != model.Generation ||
		model.Annotations[localArtifactRestoreAnnotation] != localArtifactRestoreToken(s.configMapReconciler.nodeName, model) {
		return true, fmt.Errorf("local restore requires an explicitly selected current download task")
	}
	spec := model.Spec.Storage
	if spec == nil || spec.Path == nil || spec.StorageUri == nil || !strings.HasPrefix(*spec.StorageUri, "oci://") ||
		spec.DownloadPolicy == nil || *spec.DownloadPolicy != v1beta1.AlwaysDownload || isHfArtifactShapeFiltered(task) {
		return true, fmt.Errorf("local restore requires a complete OCI model with AlwaysDownload")
	}
	if !allowValidation {
		s.demoteToNormalPriority(task)
		return true, nil
	}
	root, err := canonicalHfArtifactStoreRoot(s.modelRootDir)
	if err != nil {
		return true, err
	}
	path, err := hfArtifactPathInRoot(*spec.Path, s.modelRootDir, root)
	if err != nil || path == root || strings.Contains(path+"/", "/_artifacts/") || strings.Contains(path+"/", "/"+hfArtifactLockDirectory+"/") {
		return true, fmt.Errorf("invalid local restore path %q: %v", *spec.Path, err)
	}
	h := s.sharedHfArtifactHandler()
	key := getModelID(model, nil)
	parent, referenced, err := h.repository.GetParentForChild(ctx, key)
	if err != nil {
		return true, err
	}
	if referenced {
		unlock, acquired, err := h.tryParentFileOperation(parent, s.modelRootDir)
		if err != nil {
			return true, err
		}
		if !acquired {
			return true, s.requeueTaskOnWait(task, gopherTaskWait)
		}
		defer unlock()
	}
	childLock, acquired, err := tryHfArtifactChildFileLock(hfArtifactTaskInput{ModelStoreRoot: s.modelRootDir, ChildModelPath: path})
	if err != nil {
		return true, err
	}
	if !acquired {
		return true, s.requeueTaskOnWait(task, gopherTaskWait)
	}
	defer func() { _ = childLock.Close() }()
	if err := s.checkLocalRestoreGroup(task, root, path); err != nil {
		return true, s.requeueHfArtifactTask(task, newHfArtifactRetryResult(key, err))
	}
	if err := validateHfArtifactPathAncestors(root, path, true); err != nil {
		return true, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() {
		return true, fmt.Errorf("local restore requires a real directory at %s: %v", path, err)
	}
	valid, err := validate(path)
	if err != nil || !valid {
		return true, fmt.Errorf("local restore validation failed for %s: %v", path, err)
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return true, fmt.Errorf("local restore directory changed during validation")
	}
	// One ConfigMap write removes both reference directions and records Ready.
	// Keep the parent Ready and pinned; this path never deletes model bytes.
	err = s.configMapReconciler.mutateConfigMapWithModelUID(ctx, key, model.UID, func(cm *corev1.ConfigMap) (bool, error) {
		if err := s.checkLocalRestoreGroup(task, root, path); err != nil {
			return false, err
		}
		if h.repository.isChildMutationBlocked(key, model.UID) {
			return false, fmt.Errorf("local restore task was superseded")
		}
		child, err := existingModelEntry(cm.Data, key)
		if err != nil || child.HfArtifactPendingDeletion != nil || child.HfArtifactPendingDownload != nil {
			return false, fmt.Errorf("local restore requires a recorded child without pending work: %v", err)
		}
		current, found, err := parentForChildEntry(cm.Data, key, child)
		if err != nil {
			return false, err
		}
		if found {
			if !referenced || current.Key != parent.Key || current.Status != HfArtifactStatusReady || current.blocksNewOperation() || filepath.Clean(current.Children[key]) != filepath.Clean(*spec.Path) {
				return false, fmt.Errorf("local restore shared reference changed or is busy")
			}
			pinned := false
			for other, otherPath := range current.Children {
				keeper, err := existingModelEntry(cm.Data, other)
				pinned = pinned || (other != key && filepath.Clean(otherPath) != filepath.Clean(*spec.Path) && err == nil && keeper.HfArtifactKey == current.Key)
			}
			if !pinned {
				return false, fmt.Errorf("local restore requires a separate parent keeper reference")
			}
			delete(current.Children, key)
			delete(current.ChildStatusesBeforeRepair, key)
			if _, err := writeHfArtifactEntry(cm.Data, current); err != nil {
				return false, err
			}
		}
		child.HfArtifactKey, child.Status, child.Progress = "", ModelStatusReady, nil
		return writeModelEntry(cm.Data, key, child)
	})
	if err != nil {
		return true, err
	}
	return true, s.safeNodeLabelReconciliation(ctx, &NodeLabelOp{BaseModel: model, ModelStateOnNode: Ready})
}

func (s *Gopher) checkLocalRestoreGroup(task *GopherTask, root, path string) error {
	models, err := s.baseModelLister.List(labels.Everything())
	if err != nil || s.clusterBaseModelLister == nil {
		return fmt.Errorf("cannot inspect local restore consumers: %v", err)
	}
	seen := false
	for _, model := range models {
		storage := model.Spec.Storage
		if storage == nil || storage.Path == nil {
			continue
		}
		candidate, err := hfArtifactPathInRoot(filepath.Clean(*storage.Path), s.modelRootDir, root)
		if err != nil || candidate != path {
			continue
		}
		if model.DeletionTimestamp != nil || model.Annotations[localArtifactRestoreAnnotation] != localArtifactRestoreToken(s.configMapReconciler.nodeName, model) ||
			storage.DownloadPolicy == nil || *storage.DownloadPolicy != v1beta1.AlwaysDownload || !sameModelStoragePath(task.BaseModel.Spec.Storage, storage, s.modelRootDir, *task.BaseModel.Spec.Storage.Path) {
			return fmt.Errorf("incompatible local restore consumer %s/%s", model.Namespace, model.Name)
		}
		if model.Namespace == task.BaseModel.Namespace && model.Name == task.BaseModel.Name {
			seen = model.UID == task.BaseModel.UID && model.Generation == task.BaseModel.Generation
		}
	}
	clusterModels, err := s.clusterBaseModelLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, model := range clusterModels {
		if storage := model.Spec.Storage; storage != nil && storage.Path != nil {
			if candidate, err := hfArtifactPathInRoot(filepath.Clean(*storage.Path), s.modelRootDir, root); err == nil && candidate == path {
				return fmt.Errorf("local restore does not support ClusterBaseModel consumers")
			}
		}
	}
	if !seen {
		return fmt.Errorf("local restore CR changed or disappeared")
	}
	return nil
}
