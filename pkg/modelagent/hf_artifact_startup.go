package modelagent

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (m *HfArtifactManager) InitializeAtStartup(ctx context.Context) bool {
	m.startupMutex.Lock()
	defer m.startupMutex.Unlock()
	return m.initializeAtStartupLocked(ctx)
}

func (m *HfArtifactManager) initializeAtStartupLocked(ctx context.Context) bool {
	configMap, err := m.repository.configMaps.getConfigMap(ctx)
	if err != nil {
		m.startupValidationPending = map[string]struct{}{}
		m.startupValidationTried = map[string]struct{}{}
		m.startupSnapshotMissing = !apierrors.IsNotFound(err)
		m.startupRecoveryPending = !apierrors.IsNotFound(err)
		return apierrors.IsNotFound(err)
	}

	recovered := true
	validationPending := make(map[string]struct{})
	for key, raw := range configMap.Data {
		if !isHfArtifactConfigMapKey(key) {
			continue
		}
		parent, decodeErr := decodeArtifactParent(key, raw)
		if decodeErr != nil || !parent.Identity.isValid() || validateDesiredArtifactParent(parent) != nil {
			m.logger.Warnf("Cannot recover invalid Hugging Face artifact parent %s", key)
			if err := m.deleteInvalidParent(ctx, key); err != nil {
				recovered = false
			}
			continue
		}
		switch parent.Status {
		case ModelStatusUpdating:
			markerMatchesReservation := parent.ReservationToken == "" && m.files.HasReadyMarker(parent.Path) ||
				m.files.ReadyMarkerMatches(parent.Path, parent.ReservationToken)
			ownedParent, err := m.repository.TakeOverUpdating(ctx, parent)
			if err != nil {
				recovered = false
				continue
			}
			if markerMatchesReservation {
				if err := m.repository.MarkReady(ctx, ownedParent); err != nil {
					recovered = false
				} else {
					validationPending[key] = struct{}{}
				}
			} else if err := m.repository.MarkFailed(ctx, ownedParent); err != nil {
				recovered = false
			}
		case ModelStatusReady:
			validationPending[key] = struct{}{}
		}
	}
	m.startupValidationPending = validationPending
	m.startupValidationTried = map[string]struct{}{}
	m.startupSnapshotMissing = false
	m.startupRecoveryPending = !recovered
	return recovered
}

func (m *HfArtifactManager) deleteInvalidParent(ctx context.Context, key string) error {
	return m.repository.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		if configMap.Data == nil {
			return false, nil
		}
		if _, exists := configMap.Data[key]; !exists {
			return false, nil
		}
		delete(configMap.Data, key)
		return true, nil
	})
}

func (m *HfArtifactManager) ensureStartupRecovery(
	ctx context.Context,
	plan ArtifactPlan,
) (ArtifactLifecycleResult, bool, error) {
	m.startupMutex.Lock()
	defer m.startupMutex.Unlock()
	if !m.startupRecoveryPending {
		return ArtifactLifecycleResult{}, false, nil
	}
	if m.initializeAtStartupLocked(ctx) {
		return ArtifactLifecycleResult{}, false, nil
	}
	return deferredArtifact(plan.Parent.Key), true, nil
}

func (m *HfArtifactManager) validateAtStartupIfNeeded(
	ctx context.Context,
	plan ArtifactPlan,
	download artifactDownload,
) (ArtifactLifecycleResult, bool, error) {
	m.startupMutex.Lock()
	shouldValidate := false
	if _, ok := m.startupValidationPending[plan.Parent.Key]; ok {
		delete(m.startupValidationPending, plan.Parent.Key)
		shouldValidate = true
	} else if m.startupSnapshotMissing {
		if _, tried := m.startupValidationTried[plan.Parent.Key]; !tried {
			m.startupValidationTried[plan.Parent.Key] = struct{}{}
			shouldValidate = true
		}
	}
	if !shouldValidate {
		m.startupMutex.Unlock()
		return ArtifactLifecycleResult{}, false, nil
	}

	parent, found, err := m.repository.Get(ctx, plan.Parent.Identity)
	if err != nil {
		m.startupValidationPending[plan.Parent.Key] = struct{}{}
		m.startupMutex.Unlock()
		return deferredArtifactAfterError(plan.Parent.Key, err), true, nil
	}
	if !found || parent.Status != ModelStatusReady {
		m.startupMutex.Unlock()
		return ArtifactLifecycleResult{}, false, nil
	}
	plan.Parent = parent
	reservation, err := m.repository.AcquireRepair(ctx, plan.Parent)
	if err != nil {
		m.startupValidationPending[plan.Parent.Key] = struct{}{}
		m.startupMutex.Unlock()
		return deferredArtifactAfterError(plan.Parent.Key, err), true, nil
	}
	m.startupMutex.Unlock()

	if reservation.Outcome == ParentBusy {
		return deferredArtifact(plan.Parent.Key), true, nil
	}
	if reservation.Outcome != ParentAcquired {
		return ArtifactLifecycleResult{}, false, nil
	}
	result, err := m.downloadAndLink(ctx, plan, reservation.Parent, download)
	return result, true, err
}
