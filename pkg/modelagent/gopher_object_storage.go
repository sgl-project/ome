package modelagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
	"sigs.k8s.io/ome/pkg/utils"
)

type modelTaskOutcome string

const (
	modelTaskCompleted modelTaskOutcome = "Completed"
	modelTaskDeferred  modelTaskOutcome = "Deferred"
)

func (s *Gopher) processObjectStorageModel(
	ctx context.Context,
	task *GopherTask,
	spec v1beta1.BaseModelSpec,
	modelInfo string,
	modelType string,
	namespace string,
	name string,
	allowFallbackDownload bool,
) (modelTaskOutcome, error) {
	uri, err := getTargetDirPath(&spec)
	if err != nil {
		return modelTaskCompleted, err
	}
	childPath := getDestPath(&spec, s.modelRootDir)
	download := func(path string) error {
		return s.downloadObjectStorageArtifact(ctx, task, uri, path, modelInfo, modelType, namespace, name)
	}

	if plan, ok := planHfOriginOCIArtifact(task, spec, s.modelRootDir, childPath); ok {
		result, outcome, err := s.processSharedArtifactPlan(ctx, task, plan, allowFallbackDownload, download)
		if err != nil || outcome == modelTaskDeferred {
			return outcome, err
		}
		s.parseModelConfigForTask(childPath, task, result.Artifact)
		return modelTaskCompleted, nil
	}

	outcome, err := s.processLegacyObjectStorageModel(ctx, task, spec, childPath, allowFallbackDownload, download)
	if err != nil || outcome == modelTaskDeferred {
		return outcome, err
	}
	s.parseModelConfigForTask(childPath, task, nil)
	return modelTaskCompleted, nil
}

func (s *Gopher) processSharedArtifactPlan(
	ctx context.Context,
	task *GopherTask,
	plan ArtifactPlan,
	allowFallbackDownload bool,
	download artifactDownload,
) (ArtifactLifecycleResult, modelTaskOutcome, error) {
	manager := s.getHfArtifactManager()
	var result ArtifactLifecycleResult
	var err error
	if task.ArtifactCleanupPending {
		var ready bool
		result, ready, err = manager.ReadyChild(ctx, plan)
		if err != nil {
			return ArtifactLifecycleResult{}, modelTaskCompleted, err
		}
		if !ready {
			task.ArtifactCleanupPending = false
		}
	}
	if !task.ArtifactCleanupPending {
		result, err = s.executeSharedArtifactPlan(ctx, plan, allowFallbackDownload, download)
		if err != nil {
			return ArtifactLifecycleResult{}, modelTaskCompleted, err
		}
	}
	outcome, err := s.consumeArtifactLifecycleResult(task, result)
	if err != nil || outcome == modelTaskDeferred {
		return result, outcome, err
	}
	task.ArtifactCleanupPending = true
	outcome, err = s.releaseSupersededHfParents(ctx, task, plan.ChildPath, plan.Parent.Identity)
	if err != nil || outcome == modelTaskDeferred {
		return result, outcome, err
	}
	task.ArtifactCleanupPending = false
	return result, modelTaskCompleted, nil
}

func (s *Gopher) releaseSupersededHfParents(
	ctx context.Context,
	task *GopherTask,
	childPath string,
	currentIdentity ArtifactIdentity,
) (modelTaskOutcome, error) {
	manager := s.getHfArtifactManager()
	current, found, err := manager.repository.Get(ctx, currentIdentity)
	if err != nil {
		return s.consumeArtifactLifecycleResult(task, deferredArtifactAfterError(hfArtifactConfigMapKey(currentIdentity), err))
	}
	if !found {
		return modelTaskCompleted, fmt.Errorf("shared artifact parent %s is missing after child registration", hfArtifactConfigMapKey(currentIdentity))
	}
	return s.releaseHfParentReferences(ctx, task, childPath, &current)
}

func (s *Gopher) releaseHfParentReferences(
	ctx context.Context,
	task *GopherTask,
	childPath string,
	excludedParent *ArtifactParent,
) (modelTaskOutcome, error) {
	manager := s.getHfArtifactManager()
	parents, err := manager.repository.ListByChildPath(ctx, childPath)
	if err != nil {
		waitKey := getModelInfoForLogging(task)
		if excludedParent != nil {
			waitKey = excludedParent.Key
		}
		return s.consumeArtifactLifecycleResult(task, deferredArtifactAfterError(waitKey, err))
	}
	for _, parent := range parents {
		if excludedParent != nil && parent.Key == excludedParent.Key &&
			filepath.Clean(parent.Path) == filepath.Clean(excludedParent.Path) {
			continue
		}
		plan := ArtifactPlan{
			Operation:  ArtifactOperationRelease,
			Parent:     parent,
			ChildPath:  childPath,
			SearchRoot: artifactSearchRoot(s.modelRootDir, childPath),
		}
		result, err := manager.ReleaseReference(ctx, plan)
		if err != nil {
			return s.consumeArtifactLifecycleResult(task, deferredArtifactAfterError(parent.Key, err))
		}
		outcome, err := s.consumeArtifactLifecycleResult(task, result)
		if err != nil || outcome == modelTaskDeferred {
			return outcome, err
		}
	}
	return modelTaskCompleted, nil
}

func (s *Gopher) executeSharedArtifactPlan(
	ctx context.Context,
	plan ArtifactPlan,
	allowFallbackDownload bool,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	manager := s.getHfArtifactManager()
	switch plan.Operation {
	case ArtifactOperationEnsure:
		return manager.Ensure(ctx, plan, allowFallbackDownload, download)
	case ArtifactOperationRepair:
		return manager.Repair(ctx, plan, download)
	default:
		return ArtifactLifecycleResult{}, fmt.Errorf("unsupported shared artifact operation %s", plan.Operation)
	}
}

func (s *Gopher) consumeArtifactLifecycleResult(task *GopherTask, result ArtifactLifecycleResult) (modelTaskOutcome, error) {
	switch result.Outcome {
	case ArtifactLifecycleCompleted:
		return modelTaskCompleted, nil
	case ArtifactLifecycleDemote:
		s.demoteToNormalPriority(task)
		return modelTaskDeferred, nil
	case ArtifactLifecycleDeferred:
		if s.requeueSamePathInFlightReuseWait(task, result.WaitKey) {
			if result.Cause != nil {
				s.logger.Warnf("Deferring shared artifact operation for %s after state-store error: %v", result.WaitKey, result.Cause)
			}
			return modelTaskDeferred, nil
		}
		if result.Cause != nil {
			return modelTaskCompleted, fmt.Errorf("timed out retrying shared artifact parent %s: %w", result.WaitKey, result.Cause)
		}
		return modelTaskCompleted, fmt.Errorf("timed out waiting for shared artifact parent %s", result.WaitKey)
	default:
		return modelTaskCompleted, fmt.Errorf("unknown shared artifact lifecycle outcome %q", result.Outcome)
	}
}

func (s *Gopher) processLegacyObjectStorageModel(
	ctx context.Context,
	task *GopherTask,
	spec v1beta1.BaseModelSpec,
	destPath string,
	allowFallbackDownload bool,
	download artifactDownload,
) (modelTaskOutcome, error) {
	if !shouldUseSamePathObjectStorageReuse(task) {
		return s.downloadLegacyObjectStorageArtifact(ctx, task, destPath, download)
	}
	if _, reused := s.findReadyObjectStorageModelWithSamePath(ctx, task, spec, destPath); reused {
		return modelTaskCompleted, nil
	}
	if key, wait := s.findUpdatingObjectStorageModelWithSamePath(ctx, task, spec, destPath); wait &&
		s.requeueSamePathInFlightReuseWait(task, key) {
		return modelTaskDeferred, nil
	}
	if !allowFallbackDownload {
		s.demoteToNormalPriority(task)
		return modelTaskDeferred, nil
	}
	return s.downloadLegacyObjectStorageArtifact(ctx, task, destPath, download)
}

func (s *Gopher) downloadLegacyObjectStorageArtifact(
	ctx context.Context,
	task *GopherTask,
	destPath string,
	download artifactDownload,
) (modelTaskOutcome, error) {
	localInfo, statErr := os.Lstat(destPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return modelTaskCompleted, fmt.Errorf("inspect legacy OCI destination %s: %w", destPath, statErr)
	}
	if statErr == nil && localInfo.Mode()&os.ModeSymlink != 0 {
		outcome, _, err := s.releasePersistedSharedArtifactForLegacyDownload(ctx, task, destPath)
		if err != nil || outcome == modelTaskDeferred {
			return outcome, err
		}
	}
	return modelTaskCompleted, download(destPath)
}

// releasePersistedSharedArtifactForLegacyDownload detaches a model-local path
// from shared lifecycle management before a legacy downloader writes to it.
// Without this transition, a downloader can follow the existing child symlink
// and mutate the shared parent after the CR no longer qualifies for reuse.
func (s *Gopher) releasePersistedSharedArtifactForLegacyDownload(
	ctx context.Context,
	task *GopherTask,
	childPath string,
) (modelTaskOutcome, bool, error) {
	modelKey := s.configMapReconciler.getModelConfigMapKey(task.BaseModel, task.ClusterBaseModel)
	plan, shared, err := loadHfArtifactReleasePlan(ctx, s.configMapReconciler, modelKey, childPath, s.modelRootDir)
	if err != nil {
		outcome, waitErr := s.consumeArtifactLifecycleResult(task, deferredArtifactAfterError(modelKey, err))
		return outcome, true, waitErr
	}
	if !shared {
		return modelTaskCompleted, false, nil
	}
	result, err := s.getHfArtifactManager().Release(ctx, plan)
	if err != nil {
		return modelTaskCompleted, true, err
	}
	outcome, err := s.consumeArtifactLifecycleResult(task, result)
	return outcome, true, err
}

func (s *Gopher) downloadObjectStorageArtifact(
	ctx context.Context,
	task *GopherTask,
	uri *ociobjectstore.ObjectURI,
	destPath string,
	modelInfo string,
	modelType string,
	namespace string,
	name string,
) error {
	err := utils.Retry(s.downloadRetry, 100*time.Millisecond, func() error {
		downloadErr := s.downloadModel(ctx, uri, destPath, task)
		if downloadErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Errorf("Failed to download model %s: %v", modelInfo, downloadErr)
		}
		return downloadErr
	})
	if err == nil {
		return nil
	}
	errorType := "download_error"
	if strings.Contains(err.Error(), "MD5") {
		errorType = "md5_verification_error"
	}
	s.metrics.RecordFailedDownload(modelType, namespace, name, errorType)
	s.markModelOnNodeFailed(task)
	return err
}

func (s *Gopher) parseModelConfigForTask(path string, task *GopherTask, artifact *Artifact) {
	var err error
	if task.BaseModel != nil {
		err = s.safeParseAndUpdateModelConfig(path, task.BaseModel, nil, artifact)
	} else {
		err = s.safeParseAndUpdateModelConfig(path, nil, task.ClusterBaseModel, artifact)
	}
	if err != nil {
		s.logger.Errorf("Failed to parse and update model config: %v", err)
	}
}

func (s *Gopher) processObjectStorageDelete(ctx context.Context, task *GopherTask, spec v1beta1.BaseModelSpec) (modelTaskOutcome, error) {
	childPath := getDestPath(&spec, s.modelRootDir)
	outcome, shared, err := s.releaseSharedArtifact(ctx, task, childPath)
	if err != nil || shared {
		return outcome, err
	}
	return modelTaskCompleted, s.deleteLegacyObjectStorageModel(ctx, task, childPath)
}

func (s *Gopher) releaseSharedArtifact(ctx context.Context, task *GopherTask, childPath string) (modelTaskOutcome, bool, error) {
	modelKey := s.configMapReconciler.getModelConfigMapKey(task.BaseModel, task.ClusterBaseModel)
	plan, shared, err := loadHfArtifactReleasePlan(ctx, s.configMapReconciler, modelKey, childPath, s.modelRootDir)
	if err != nil {
		outcome, waitErr := s.deferSharedArtifactDelete(task, err)
		return outcome, true, waitErr
	}
	if !shared {
		return modelTaskCompleted, false, nil
	}
	if skip, err := s.shouldPreserveArtifactPath(task, childPath); err != nil || skip {
		return modelTaskCompleted, true, err
	}
	result, err := s.getHfArtifactManager().Release(ctx, plan)
	if err != nil {
		return modelTaskCompleted, true, err
	}
	if result.Outcome == ArtifactLifecycleDeferred {
		cause := result.Cause
		if cause == nil {
			cause = fmt.Errorf("shared artifact parent %s is updating", result.WaitKey)
		}
		outcome, waitErr := s.deferSharedArtifactDelete(task, cause)
		return outcome, true, waitErr
	}
	outcome, err := s.releaseHfParentReferences(ctx, task, childPath, nil)
	return outcome, true, err
}

func (s *Gopher) deleteLegacyObjectStorageModel(ctx context.Context, task *GopherTask, destPath string) error {
	skip, _, _, _ := s.isSkippingArtifactDeletion(ctx, task, destPath, false)
	if skip {
		return nil
	}
	return s.deleteModel(destPath, task)
}

func (s *Gopher) shouldPreserveArtifactPath(task *GopherTask, path string) (bool, error) {
	referenced, err := s.isPathReferencedByOtherModels(path, task.BaseModel, task.ClusterBaseModel)
	if err != nil {
		return true, err
	}
	return referenced || s.isReservingModelArtifact(task), nil
}

func (s *Gopher) deferSharedArtifactDelete(task *GopherTask, cause error) (modelTaskOutcome, error) {
	if s.requeueSamePathInFlightReuseWait(task, getModelInfoForLogging(task)) {
		s.logger.Warnf("Deferring shared artifact deletion for %s: %v", getModelInfoForLogging(task), cause)
		return modelTaskDeferred, nil
	}
	return modelTaskCompleted, cause
}
