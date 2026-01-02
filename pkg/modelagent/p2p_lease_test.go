package modelagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/sgl-project/ome/pkg/constants"
)

func TestP2PLeaseManager(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t).Sugar()

	lm := NewP2PLeaseManager(fakeClient, "ome", "test-pod", logger)

	t.Run("acquire lease that does not exist returns false", func(t *testing.T) {
		// Lease not pre-created by controller, should return false (not error)
		acquired, err := lm.TryAcquire(ctx, "nonexistent-lease")
		require.NoError(t, err)
		assert.False(t, acquired)
	})

	t.Run("acquire pre-created lease with no holder", func(t *testing.T) {
		// Controller creates lease with no holder identity
		now := metav1.NowMicro()
		preCreatedLease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-lease-1",
				Labels: map[string]string{
					constants.P2PLeaseTypeLabel: constants.P2PLeaseTypeValue,
				},
			},
			Spec: coordinationv1.LeaseSpec{
				// HolderIdentity is nil - first agent to acquire
				AcquireTime:          &now,
				RenewTime:            &now,
				LeaseDurationSeconds: ptr.To[int32](120),
			},
		}
		_, err := fakeClient.CoordinationV1().Leases("ome").Create(ctx, preCreatedLease, metav1.CreateOptions{})
		require.NoError(t, err)

		acquired, err := lm.TryAcquire(ctx, "test-lease-1")
		require.NoError(t, err)
		assert.True(t, acquired)

		// Verify we acquired it
		lease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "test-lease-1", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "test-pod", *lease.Spec.HolderIdentity)
	})

	t.Run("cannot acquire existing lease", func(t *testing.T) {
		// Create a lease held by another pod
		now := metav1.NowMicro()
		existingLease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-lease-2",
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr.To("other-pod"),
				AcquireTime:          &now,
				RenewTime:            &now,
				LeaseDurationSeconds: ptr.To[int32](120),
			},
		}
		_, err := fakeClient.CoordinationV1().Leases("ome").Create(ctx, existingLease, metav1.CreateOptions{})
		require.NoError(t, err)

		acquired, err := lm.TryAcquire(ctx, "test-lease-2")
		require.NoError(t, err)
		assert.False(t, acquired)
	})

	t.Run("acquire expired lease", func(t *testing.T) {
		// Create an expired lease
		expiredTime := metav1.NewMicroTime(time.Now().Add(-5 * time.Minute))
		expiredLease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-lease-3",
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr.To("dead-pod"),
				AcquireTime:          &expiredTime,
				RenewTime:            &expiredTime,
				LeaseDurationSeconds: ptr.To[int32](120),
			},
		}
		_, err := fakeClient.CoordinationV1().Leases("ome").Create(ctx, expiredLease, metav1.CreateOptions{})
		require.NoError(t, err)

		acquired, err := lm.TryAcquire(ctx, "test-lease-3")
		require.NoError(t, err)
		assert.True(t, acquired)

		// Verify we took over
		lease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "test-lease-3", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, "test-pod", *lease.Spec.HolderIdentity)
	})

	t.Run("mark lease complete", func(t *testing.T) {
		err := lm.MarkComplete(ctx, "test-lease-1")
		require.NoError(t, err)

		lease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "test-lease-1", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, constants.P2PLeaseStatusComplete, lease.Labels[constants.P2PLeaseStatusLabel])
	})

	t.Run("is complete check", func(t *testing.T) {
		lease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "test-lease-1", metav1.GetOptions{})
		require.NoError(t, err)
		assert.True(t, lm.IsComplete(lease))
	})

	t.Run("release lease", func(t *testing.T) {
		err := lm.Release(ctx, "test-lease-1")
		require.NoError(t, err)

		_, err = fakeClient.CoordinationV1().Leases("ome").Get(ctx, "test-lease-1", metav1.GetOptions{})
		assert.Error(t, err) // Should be not found
	})
}

func TestP2PLeaseExpiration(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	lm := NewP2PLeaseManager(nil, "ome", "test-pod", logger)

	now := metav1.NowMicro()
	expired := metav1.NewMicroTime(time.Now().Add(-5 * time.Minute))

	tests := []struct {
		name        string
		lease       *coordinationv1.Lease
		wantExpired bool
	}{
		{
			name: "active lease",
			lease: &coordinationv1.Lease{
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &now,
					LeaseDurationSeconds: ptr.To[int32](120),
				},
			},
			wantExpired: false,
		},
		{
			name: "expired lease",
			lease: &coordinationv1.Lease{
				Spec: coordinationv1.LeaseSpec{
					RenewTime:            &expired,
					LeaseDurationSeconds: ptr.To[int32](120),
				},
			},
			wantExpired: true,
		},
		{
			name: "nil renew time",
			lease: &coordinationv1.Lease{
				Spec: coordinationv1.LeaseSpec{
					LeaseDurationSeconds: ptr.To[int32](120),
				},
			},
			wantExpired: true,
		},
		{
			name: "nil lease duration",
			lease: &coordinationv1.Lease{
				Spec: coordinationv1.LeaseSpec{
					RenewTime: &now,
				},
			},
			wantExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lm.IsExpired(tt.lease)
			assert.Equal(t, tt.wantExpired, result)
		})
	}
}

func TestP2PLeaseManagerGetLeaseName(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	lm := NewP2PLeaseManager(nil, "ome", "test-pod", logger)

	tests := []struct {
		name      string
		modelHash string
		expected  string
	}{
		{
			name:      "short hash",
			modelHash: "abc123",
			expected:  constants.P2PLeasePrefix + "abc123",
		},
		{
			name:      "long hash is truncated",
			modelHash: "abcdefghijklmnopqrstuvwxyz123456",
			expected:  constants.P2PLeasePrefix + "abcdefghijklmnop",
		},
		{
			name:      "exactly 16 chars",
			modelHash: "1234567890123456",
			expected:  constants.P2PLeasePrefix + "1234567890123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lm.GetLeaseName(tt.modelHash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestP2PLeaseManagerRenew(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t).Sugar()

	lm := NewP2PLeaseManager(fakeClient, "ome", "test-pod", logger)

	// Create a pre-existing lease (as controller would) with no holder
	now := metav1.NowMicro()
	preCreatedLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: "renew-test-lease",
			Labels: map[string]string{
				constants.P2PLeaseTypeLabel: constants.P2PLeaseTypeValue,
			},
		},
		Spec: coordinationv1.LeaseSpec{
			LeaseDurationSeconds: ptr.To[int32](120),
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}
	_, err := fakeClient.CoordinationV1().Leases("ome").Create(ctx, preCreatedLease, metav1.CreateOptions{})
	require.NoError(t, err)

	// Acquire the lease
	acquired, err := lm.TryAcquire(ctx, "renew-test-lease")
	require.NoError(t, err)
	require.True(t, acquired)

	// Get original renew time
	originalLease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "renew-test-lease", metav1.GetOptions{})
	require.NoError(t, err)
	originalRenewTime := originalLease.Spec.RenewTime.Time

	// Wait a bit and renew
	time.Sleep(10 * time.Millisecond)
	err = lm.Renew(ctx, "renew-test-lease")
	require.NoError(t, err)

	// Verify renew time was updated
	renewedLease, err := fakeClient.CoordinationV1().Leases("ome").Get(ctx, "renew-test-lease", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, renewedLease.Spec.RenewTime.Time.After(originalRenewTime))
}

func TestP2PLeaseManagerRenewNotHolder(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t).Sugar()

	lm := NewP2PLeaseManager(fakeClient, "ome", "test-pod", logger)

	// Create a lease held by another pod
	now := metav1.NowMicro()
	otherLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-holder-lease",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("other-pod"),
			AcquireTime:          &now,
			RenewTime:            &now,
			LeaseDurationSeconds: ptr.To[int32](120),
		},
	}
	_, err := fakeClient.CoordinationV1().Leases("ome").Create(ctx, otherLease, metav1.CreateOptions{})
	require.NoError(t, err)

	// Try to renew - should fail
	err = lm.Renew(ctx, "other-holder-lease")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not the lease holder")
}
