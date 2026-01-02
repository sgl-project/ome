package modelagent

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/sgl-project/ome/pkg/constants"
)

// P2PLeaseManager handles Kubernetes Lease-based coordination for P2P model downloads.
// It ensures only one node downloads from HuggingFace at a time while others wait
// for P2P availability.
type P2PLeaseManager struct {
	k8s                  kubernetes.Interface
	namespace            string
	holderIdentity       string
	leaseDurationSeconds int32
	logger               *zap.SugaredLogger
}

// NewP2PLeaseManager creates a new P2PLeaseManager.
func NewP2PLeaseManager(k8s kubernetes.Interface, namespace, holderIdentity string, logger *zap.SugaredLogger) *P2PLeaseManager {
	return &P2PLeaseManager{
		k8s:                  k8s,
		namespace:            namespace,
		holderIdentity:       holderIdentity,
		leaseDurationSeconds: int32(constants.P2PDefaultLeaseDurationSeconds),
		logger:               logger,
	}
}

// WithLeaseDuration sets a custom lease duration.
func (m *P2PLeaseManager) WithLeaseDuration(seconds int32) *P2PLeaseManager {
	m.leaseDurationSeconds = seconds
	return m
}

// GetLeaseName returns the lease name for a model hash.
func (m *P2PLeaseManager) GetLeaseName(modelHash string) string {
	// Truncate hash to fit Kubernetes name constraints
	if len(modelHash) > 16 {
		modelHash = modelHash[:16]
	}
	return constants.P2PLeasePrefix + modelHash
}

// TryAcquire attempts to acquire a lease for the given name.
// Returns true if the lease was acquired, false if another holder has it.
func (m *P2PLeaseManager) TryAcquire(ctx context.Context, name string) (bool, error) {
	now := metav1.NowMicro()

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.P2PLeaseTypeLabel: constants.P2PLeaseTypeValue,
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &m.holderIdentity,
			AcquireTime:          &now,
			RenewTime:            &now,
			LeaseDurationSeconds: ptr.To(m.leaseDurationSeconds),
		},
	}

	// Try to create the lease
	_, err := m.k8s.CoordinationV1().Leases(m.namespace).Create(ctx, lease, metav1.CreateOptions{})
	if err == nil {
		m.logger.Debugf("Acquired P2P lease %s", name)
		return true, nil
	}

	if !errors.IsAlreadyExists(err) {
		return false, fmt.Errorf("failed to create lease: %w", err)
	}

	// Lease exists, check if we can take it over
	existing, getErr := m.k8s.CoordinationV1().Leases(m.namespace).Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return false, fmt.Errorf("failed to get existing lease: %w", getErr)
	}

	// If completed, no need to acquire
	if m.IsComplete(existing) {
		m.logger.Debugf("Lease %s is already complete", name)
		return false, nil
	}

	// If expired, try to take over
	if m.IsExpired(existing) {
		m.logger.Debugf("Lease %s expired, attempting takeover", name)
		existing.Spec.HolderIdentity = &m.holderIdentity
		existing.Spec.AcquireTime = &now
		existing.Spec.RenewTime = &now
		existing.Spec.LeaseDurationSeconds = ptr.To(m.leaseDurationSeconds)

		_, updateErr := m.k8s.CoordinationV1().Leases(m.namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if updateErr == nil {
			m.logger.Debugf("Took over expired lease %s", name)
			return true, nil
		}

		if errors.IsConflict(updateErr) {
			m.logger.Debugf("Lease %s taken by another node", name)
			return false, nil
		}

		return false, fmt.Errorf("failed to update lease: %w", updateErr)
	}

	m.logger.Debugf("Lease %s held by %s", name, *existing.Spec.HolderIdentity)
	return false, nil
}

// Renew renews the lease to extend its duration.
func (m *P2PLeaseManager) Renew(ctx context.Context, name string) error {
	lease, err := m.k8s.CoordinationV1().Leases(m.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get lease: %w", err)
	}

	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != m.holderIdentity {
		return fmt.Errorf("not the lease holder")
	}

	now := metav1.NowMicro()
	lease.Spec.RenewTime = &now

	_, err = m.k8s.CoordinationV1().Leases(m.namespace).Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to renew lease: %w", err)
	}

	m.logger.Debugf("Renewed lease %s", name)
	return nil
}

// MarkComplete marks the lease as complete.
func (m *P2PLeaseManager) MarkComplete(ctx context.Context, name string) error {
	lease, err := m.k8s.CoordinationV1().Leases(m.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get lease: %w", err)
	}

	if lease.Labels == nil {
		lease.Labels = make(map[string]string)
	}
	lease.Labels[constants.P2PLeaseStatusLabel] = constants.P2PLeaseStatusComplete

	_, err = m.k8s.CoordinationV1().Leases(m.namespace).Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to mark complete: %w", err)
	}

	m.logger.Debugf("Marked lease %s complete", name)
	return nil
}

// Release deletes the lease.
func (m *P2PLeaseManager) Release(ctx context.Context, name string) error {
	err := m.k8s.CoordinationV1().Leases(m.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete lease: %w", err)
	}
	m.logger.Debugf("Released lease %s", name)
	return nil
}

// Get retrieves the current lease state.
func (m *P2PLeaseManager) Get(ctx context.Context, name string) (*coordinationv1.Lease, error) {
	return m.k8s.CoordinationV1().Leases(m.namespace).Get(ctx, name, metav1.GetOptions{})
}

// IsExpired checks if a lease has expired.
func (m *P2PLeaseManager) IsExpired(lease *coordinationv1.Lease) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return true
	}
	expiry := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
	return time.Now().After(expiry)
}

// IsComplete checks if a lease is marked as complete.
func (m *P2PLeaseManager) IsComplete(lease *coordinationv1.Lease) bool {
	if lease.Labels == nil {
		return false
	}
	return lease.Labels[constants.P2PLeaseStatusLabel] == constants.P2PLeaseStatusComplete
}

// StartRenewal starts a background goroutine that renews the lease periodically.
// Returns a cancel function to stop renewal.
func (m *P2PLeaseManager) StartRenewal(ctx context.Context, name string) context.CancelFunc {
	renewCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(time.Duration(constants.P2PDefaultLeaseRenewSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := m.Renew(renewCtx, name); err != nil {
					m.logger.Warnf("Failed to renew lease %s: %v", name, err)
				}
			}
		}
	}()

	return cancel
}
