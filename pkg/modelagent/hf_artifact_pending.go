package modelagent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// finishParentWithError releases this owner's lock. Cancellation leaves work
// Updating and retryable; other errors publish Failed.
// If the state update fails, return Retry with both errors rather than hiding it.
func (h *hfArtifactTaskHandler) finishParentWithError(
	ctx context.Context,
	parent HfArtifactEntry,
	cause error,
) (hfArtifactTaskResult, error) {
	status := HfArtifactStatusFailed
	if errors.Is(cause, context.Canceled) {
		status = HfArtifactStatusUpdating
	}
	if err := h.releaseParentLock(ctx, parent, status); err != nil {
		return newHfArtifactRetryResult(parent.Key, errors.Join(cause, err)), nil
	}
	return hfArtifactTaskResult{}, cause
}

// releaseParentLock persists the result even when the task was canceled.
// A failed or lost response retains the owner and desired status for retry.
func (h *hfArtifactTaskHandler) releaseParentLock(ctx context.Context, parent HfArtifactEntry, status HfArtifactStatus) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := h.repository.setStatus(cleanupCtx, parent, status)
	if err != nil {
		parent.Status = status
		h.pendingLockReleases.Store(parent.Key, &parent)
	}
	return err
}

// retryPendingLockRelease completes an uncertain lock release after an API
// outage. It cannot release a different owner's lock. The pending pointer also
// fences an old retry from clearing a newer release for the same identity.
func (h *hfArtifactTaskHandler) retryPendingLockRelease(ctx context.Context, key string) error {
	value, pending := h.pendingLockReleases.Load(key)
	if !pending {
		return nil
	}
	expected := value.(*HfArtifactEntry)
	stored, found, err := h.repository.Get(ctx, expected.Identity)
	if err != nil {
		return err
	}
	if found && stored.Status == HfArtifactStatusUpdating && stored.LockID == expected.LockID {
		if err := h.releaseParentLock(ctx, *expected, expected.Status); err != nil {
			return fmt.Errorf("finish shared artifact lock release: %w", err)
		}
	}
	// Missing, terminal, or superseded state no longer belongs to this cleanup.
	h.pendingLockReleases.CompareAndDelete(key, expected)
	return nil
}
