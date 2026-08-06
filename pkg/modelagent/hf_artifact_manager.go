package modelagent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ArtifactLifecycleOutcome string

const (
	ArtifactLifecycleCompleted ArtifactLifecycleOutcome = "Completed"
	ArtifactLifecycleDeferred  ArtifactLifecycleOutcome = "Deferred"
	ArtifactLifecycleDemote    ArtifactLifecycleOutcome = "Demote"
)

type ArtifactLifecycleResult struct {
	Outcome  ArtifactLifecycleOutcome
	Artifact *Artifact
	WaitKey  string
	Cause    error
}

type artifactDownload func(path string) error

const artifactStateCleanupTimeout = 10 * time.Second

type pendingArtifactTransition struct {
	parent ArtifactParent
	status ModelStatus
}

type HfArtifactManager struct {
	repository *HfArtifactRepository
	files      artifactFileSystem
	logger     *zap.SugaredLogger

	pendingTransitionMutex sync.Mutex
	pendingTransitions     map[string]map[string]pendingArtifactTransition
}

func newHfArtifactManager(
	repository *HfArtifactRepository,
	files artifactFileSystem,
	logger *zap.SugaredLogger,
) *HfArtifactManager {
	return &HfArtifactManager{
		repository:         repository,
		files:              files,
		logger:             logger,
		pendingTransitions: map[string]map[string]pendingArtifactTransition{},
	}
}

func (m *HfArtifactManager) Ensure(
	ctx context.Context,
	plan ArtifactPlan,
	allowDownload bool,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	if plan.Operation != ArtifactOperationEnsure {
		return ArtifactLifecycleResult{}, fmt.Errorf("artifact plan operation %s cannot be ensured", plan.Operation)
	}
	if result, stop, err := m.prepareForUse(ctx, plan, download); stop || err != nil {
		return result, err
	}

	parent, found, err := m.repository.Get(ctx, plan.Parent.Identity)
	if err != nil {
		if !allowDownload {
			return deferredArtifactAfterError(plan.Parent.Key, err), nil
		}
		reservation, reserveErr := m.repository.Reserve(ctx, plan.Parent)
		if reserveErr != nil {
			return deferredArtifactAfterError(plan.Parent.Key, reserveErr), nil
		}
		return m.finishReservation(ctx, plan, reservation, download)
	}
	if found {
		plan.Parent = parent
		return m.ensureExisting(ctx, plan, parent, allowDownload, download)
	}
	if !allowDownload {
		return ArtifactLifecycleResult{Outcome: ArtifactLifecycleDemote}, nil
	}
	reservation, err := m.repository.Reserve(ctx, plan.Parent)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), nil
	}
	return m.finishReservation(ctx, plan, reservation, download)
}

func (m *HfArtifactManager) Repair(
	ctx context.Context,
	plan ArtifactPlan,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	if plan.Operation != ArtifactOperationRepair {
		return ArtifactLifecycleResult{}, fmt.Errorf("artifact plan operation %s cannot be repaired", plan.Operation)
	}
	if result, stop := m.retryPendingTransition(plan); stop {
		return result, nil
	}
	parent, found, err := m.repository.Get(ctx, plan.Parent.Identity)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), nil
	}
	if found {
		plan.Parent = parent
	}
	if found && parent.Status == ModelStatusUpdating && m.files.ReadyMarkerMatches(parent.Path, parent.ReservationToken) {
		if err := m.repository.MarkReady(ctx, parent); err != nil {
			return deferredArtifactAfterError(parent.Key, err), nil
		}
		parent.Status = ModelStatusReady
		parent.ReservationToken = ""
		return m.linkChild(ctx, plan, parent)
	}
	reservation, err := m.repository.AcquireRepair(ctx, plan.Parent)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), nil
	}
	return m.finishReservation(ctx, plan, reservation, download)
}

func (m *HfArtifactManager) ReadyChild(
	ctx context.Context,
	plan ArtifactPlan,
) (ArtifactLifecycleResult, bool, error) {
	parent, found, err := m.repository.Get(ctx, plan.Parent.Identity)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), true, nil
	}
	if !found || parent.Status != ModelStatusReady || !containsPath(parent.Children, plan.ChildPath) ||
		!m.parentUsable(parent) || !m.files.ChildPointsTo(plan.ChildPath, parent.Path) {
		return ArtifactLifecycleResult{}, false, nil
	}
	artifact := artifactForParent(parent)
	return ArtifactLifecycleResult{Outcome: ArtifactLifecycleCompleted, Artifact: &artifact}, true, nil
}

func (m *HfArtifactManager) Release(ctx context.Context, plan ArtifactPlan) (ArtifactLifecycleResult, error) {
	return m.release(ctx, plan, true)
}

// ReleaseReference cleans up a superseded parent after the child symlink has
// already switched to another ready parent.
func (m *HfArtifactManager) ReleaseReference(ctx context.Context, plan ArtifactPlan) (ArtifactLifecycleResult, error) {
	return m.release(ctx, plan, false)
}

func (m *HfArtifactManager) release(
	ctx context.Context,
	plan ArtifactPlan,
	removeChildPath bool,
) (ArtifactLifecycleResult, error) {
	if plan.Operation != ArtifactOperationRelease {
		return ArtifactLifecycleResult{}, fmt.Errorf("artifact plan operation %s cannot be released", plan.Operation)
	}
	if result, stop := m.retryPendingTransition(plan); stop {
		return result, nil
	}
	parent, found, err := m.repository.Get(ctx, plan.Parent.Identity)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), nil
	}
	if found {
		plan.Parent = parent
	}
	if found && parent.Status == ModelStatusUpdating {
		return deferredArtifact(parent.Key), nil
	}
	if removeChildPath {
		if err := m.files.RemoveChild(plan.ChildPath); err != nil {
			return ArtifactLifecycleResult{}, err
		}
	}
	if !found {
		return ArtifactLifecycleResult{Outcome: ArtifactLifecycleCompleted}, nil
	}
	release, err := m.repository.RemoveChild(ctx, plan.Parent, plan.ChildPath)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), nil
	}
	if !release.Found || !release.DeleteParent {
		return ArtifactLifecycleResult{Outcome: ArtifactLifecycleCompleted}, nil
	}

	searchRoot := plan.SearchRoot
	if searchRoot == "" {
		searchRoot = filepath.Dir(plan.ChildPath)
	}
	hasChild, err := m.files.HasOtherChild(plan.Parent.Path, searchRoot)
	if err != nil || hasChild {
		if readyErr := m.repository.MarkReady(ctx, release.Parent); readyErr != nil {
			return m.deferTransition(release.Parent, ModelStatusReady, errors.Join(err, readyErr)), nil
		}
		if err != nil {
			return ArtifactLifecycleResult{}, fmt.Errorf("cannot verify references to shared artifact %s: %w", plan.Parent.Key, err)
		}
		return ArtifactLifecycleResult{Outcome: ArtifactLifecycleCompleted}, nil
	}
	if err := m.files.RemoveParent(release.Parent.Path); err != nil {
		cleanupCtx, cancel := m.newCleanupContext()
		readyErr := m.repository.MarkReady(cleanupCtx, release.Parent)
		cancel()
		if readyErr != nil {
			return m.deferTransition(release.Parent, ModelStatusReady, errors.Join(err, readyErr)), nil
		}
		return ArtifactLifecycleResult{}, err
	}
	deleted, err := m.repository.DeleteIfUnreferenced(ctx, release.Parent)
	if err != nil {
		return m.failReservation(release.Parent, err)
	}
	if !deleted {
		return m.failReservation(release.Parent, fmt.Errorf("shared artifact parent %s changed during deletion", release.Parent.Key))
	}
	return ArtifactLifecycleResult{Outcome: ArtifactLifecycleCompleted}, nil
}

func (m *HfArtifactManager) prepareForUse(
	ctx context.Context,
	plan ArtifactPlan,
	download artifactDownload,
) (ArtifactLifecycleResult, bool, error) {
	if result, stop := m.retryPendingTransition(plan); stop {
		return result, true, nil
	}
	return ArtifactLifecycleResult{}, false, nil
}

func (m *HfArtifactManager) ensureExisting(
	ctx context.Context,
	plan ArtifactPlan,
	parent ArtifactParent,
	allowDownload bool,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	switch parent.Status {
	case ModelStatusReady:
		if m.parentUsable(parent) {
			return m.linkChild(ctx, plan, parent)
		}
		if !allowDownload {
			return ArtifactLifecycleResult{Outcome: ArtifactLifecycleDemote}, nil
		}
		reservation, err := m.repository.AcquireRepair(ctx, plan.Parent)
		if err != nil {
			return deferredArtifactAfterError(plan.Parent.Key, err), nil
		}
		return m.finishReservation(ctx, plan, reservation, download)
	case ModelStatusUpdating:
		if m.files.ReadyMarkerMatches(parent.Path, parent.ReservationToken) {
			if err := m.repository.MarkReady(ctx, parent); err != nil {
				return deferredArtifactAfterError(parent.Key, err), nil
			}
			parent.Status = ModelStatusReady
			parent.ReservationToken = ""
			return m.linkChild(ctx, plan, parent)
		}
		return deferredArtifact(parent.Key), nil
	default:
		if !allowDownload {
			return ArtifactLifecycleResult{Outcome: ArtifactLifecycleDemote}, nil
		}
		reservation, err := m.repository.AcquireRepair(ctx, plan.Parent)
		if err != nil {
			return deferredArtifactAfterError(plan.Parent.Key, err), nil
		}
		return m.finishReservation(ctx, plan, reservation, download)
	}
}

func (m *HfArtifactManager) finishReservation(
	ctx context.Context,
	plan ArtifactPlan,
	reservation ParentReservation,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	switch reservation.Outcome {
	case ParentReady:
		return m.linkChild(ctx, plan, reservation.Parent)
	case ParentBusy:
		return deferredArtifact(reservation.Parent.Key), nil
	case ParentAcquired:
		return m.downloadAndLink(ctx, plan, reservation.Parent, download)
	default:
		return ArtifactLifecycleResult{}, fmt.Errorf("unknown artifact reservation outcome %q", reservation.Outcome)
	}
}

func (m *HfArtifactManager) downloadAndLink(
	ctx context.Context,
	plan ArtifactPlan,
	parent ArtifactParent,
	download artifactDownload,
) (ArtifactLifecycleResult, error) {
	if err := m.files.RemoveReadyMarker(parent.Path); err != nil {
		return m.failReservation(parent, fmt.Errorf("remove ready marker for %s: %w", parent.Key, err))
	}
	if err := download(parent.Path); err != nil {
		return m.failReservation(parent, err)
	}
	if err := m.files.WriteReadyMarker(parent.Path, parent.ReservationToken); err != nil {
		return m.failReservation(parent, fmt.Errorf("write ready marker for %s: %w", parent.Key, err))
	}
	if err := m.repository.MarkReady(ctx, parent); err != nil {
		return deferredArtifactAfterError(parent.Key, err), nil
	}
	parent.Status = ModelStatusReady
	parent.ReservationToken = ""
	return m.linkChild(ctx, plan, parent)
}

func (m *HfArtifactManager) linkChild(
	ctx context.Context,
	plan ArtifactPlan,
	parent ArtifactParent,
) (ArtifactLifecycleResult, error) {
	if !m.parentUsable(parent) {
		return ArtifactLifecycleResult{}, fmt.Errorf("shared artifact parent %s is not ready at %s", parent.Key, parent.Path)
	}
	if err := m.files.LinkChild(plan.ChildPath, parent.Path); err != nil {
		return ArtifactLifecycleResult{}, err
	}
	artifact := artifactForParent(parent)
	registrationCtx, cancel := m.newCleanupContext()
	err := m.repository.AddChildWithArtifact(registrationCtx, plan.Parent, plan.Child, artifact)
	cancel()
	if err != nil {
		_ = m.files.RemoveChild(plan.ChildPath)
		return m.cleanupUnregisteredChild(plan, err)
	}
	return ArtifactLifecycleResult{
		Outcome:  ArtifactLifecycleCompleted,
		Artifact: &artifact,
	}, nil
}

func artifactForParent(parent ArtifactParent) Artifact {
	return Artifact{
		Sha:           parent.Identity.HFCommitSHA,
		Origin:        parent.Identity.toOrigin(),
		ParentPath:    map[string]string{parent.Key: parent.Path},
		ChildrenPaths: []string{},
	}
}

func (m *HfArtifactManager) parentUsable(parent ArtifactParent) bool {
	return m.files.ParentExists(parent.Path) && m.files.HasReadyMarker(parent.Path)
}

func deferredArtifact(key string) ArtifactLifecycleResult {
	return ArtifactLifecycleResult{Outcome: ArtifactLifecycleDeferred, WaitKey: key}
}

func deferredArtifactAfterError(key string, cause error) ArtifactLifecycleResult {
	return ArtifactLifecycleResult{Outcome: ArtifactLifecycleDeferred, WaitKey: key, Cause: cause}
}

func (m *HfArtifactManager) failReservation(parent ArtifactParent, cause error) (ArtifactLifecycleResult, error) {
	cleanupCtx, cancel := m.newCleanupContext()
	defer cancel()
	if err := m.repository.MarkFailed(cleanupCtx, parent); err != nil {
		return m.deferTransition(parent, ModelStatusFailed, errors.Join(cause, fmt.Errorf("mark shared artifact parent %s Failed: %w", parent.Key, err))), nil
	}
	return ArtifactLifecycleResult{}, cause
}

func (m *HfArtifactManager) deferTransition(parent ArtifactParent, status ModelStatus, cause error) ArtifactLifecycleResult {
	m.pendingTransitionMutex.Lock()
	if m.pendingTransitions[parent.Key] == nil {
		m.pendingTransitions[parent.Key] = make(map[string]pendingArtifactTransition)
	}
	m.pendingTransitions[parent.Key][parent.ReservationToken] = pendingArtifactTransition{parent: parent, status: status}
	m.pendingTransitionMutex.Unlock()
	return deferredArtifactAfterError(parent.Key, cause)
}

func (m *HfArtifactManager) retryPendingTransition(plan ArtifactPlan) (ArtifactLifecycleResult, bool) {
	m.pendingTransitionMutex.Lock()
	pendingSnapshot := make(map[string]pendingArtifactTransition, len(m.pendingTransitions[plan.Parent.Key]))
	for token, pending := range m.pendingTransitions[plan.Parent.Key] {
		pendingSnapshot[token] = pending
	}
	m.pendingTransitionMutex.Unlock()
	if len(pendingSnapshot) == 0 {
		return ArtifactLifecycleResult{}, false
	}

	cleanupCtx, cancel := m.newCleanupContext()
	defer cancel()
	current, found, err := m.repository.Get(cleanupCtx, plan.Parent.Identity)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), true
	}
	if !found || current.Status != ModelStatusUpdating {
		for _, pending := range pendingSnapshot {
			m.clearPendingTransition(plan.Parent.Key, pending)
		}
		if m.hasPendingTransition(plan.Parent.Key) {
			return deferredArtifact(plan.Parent.Key), true
		}
		return ArtifactLifecycleResult{}, false
	}
	pending, exists := pendingSnapshot[current.ReservationToken]
	if !exists {
		return ArtifactLifecycleResult{}, false
	}

	if pending.status == ModelStatusReady {
		err = m.repository.MarkReady(cleanupCtx, pending.parent)
	} else {
		err = m.repository.MarkFailed(cleanupCtx, pending.parent)
	}
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, err), true
	}
	if m.clearPendingTransition(plan.Parent.Key, pending) && !m.hasPendingTransition(plan.Parent.Key) {
		return ArtifactLifecycleResult{}, false
	}
	return deferredArtifact(plan.Parent.Key), true
}

func (m *HfArtifactManager) clearPendingTransition(key string, expected pendingArtifactTransition) bool {
	m.pendingTransitionMutex.Lock()
	defer m.pendingTransitionMutex.Unlock()
	pendingByToken := m.pendingTransitions[key]
	current, exists := pendingByToken[expected.parent.ReservationToken]
	if !exists {
		return true
	}
	if current.status != expected.status {
		return false
	}
	delete(pendingByToken, expected.parent.ReservationToken)
	if len(pendingByToken) == 0 {
		delete(m.pendingTransitions, key)
	}
	return true
}

func (m *HfArtifactManager) hasPendingTransition(key string) bool {
	m.pendingTransitionMutex.Lock()
	defer m.pendingTransitionMutex.Unlock()
	return len(m.pendingTransitions[key]) > 0
}

func (m *HfArtifactManager) newCleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), artifactStateCleanupTimeout)
}

func (m *HfArtifactManager) cleanupUnregisteredChild(plan ArtifactPlan, cause error) (ArtifactLifecycleResult, error) {
	release := plan
	release.Operation = ArtifactOperationRelease
	cleanupCtx, cancel := m.newCleanupContext()
	defer cancel()
	result, err := m.Release(cleanupCtx, release)
	if err != nil {
		return deferredArtifactAfterError(plan.Parent.Key, errors.Join(cause, err)), nil
	}
	if result.Outcome == ArtifactLifecycleDeferred {
		return deferredArtifactAfterError(plan.Parent.Key, errors.Join(cause, result.Cause)), nil
	}
	return deferredArtifactAfterError(plan.Parent.Key, cause), nil
}
