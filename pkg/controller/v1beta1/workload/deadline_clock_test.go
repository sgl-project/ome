package workload_test

// Verifies the escalation pass consults the injected clock for deadline
// expiry:
// one tick before the deadline nothing expires; one tick after, the
// instance fails with DeadlineExceeded. Only possible with a fake clock.

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"

	wl "sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestExpireOperations_ExactBoundary(t *testing.T) {
	t0 := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	deadline := t0.Add(30 * time.Minute)
	fc := clocktesting.NewFakeClock(deadline.Add(-time.Second)) // 1s BEFORE

	instances := []workload.InstanceStatus{{
		Index: 0, Phase: workload.InstancePhaseCreating,
		Operation: &workload.InstanceOperation{
			Type:      workload.InstanceOperationCreate,
			StartedAt: metav1.NewTime(t0), Deadline: metav1.NewTime(deadline),
		},
	}}

	var mutated []int32
	var warned []int32
	input := workload.ReconcileInput{
		Clock: fc,
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			if mutate(&instances[idx]) {
				mutated = append(mutated, idx)
			}
			return nil
		},
		WarnInstanceFailed: func(idx int32, podName, reason string) {
			warned = append(warned, idx)
		},
	}
	// Same backing slice: the second and third pass observe the prior
	// pass's mutations, like consecutive reconciles would.
	input.ObservedState.InstanceStatuses = instances

	if err := wl.EscalateFromEvidenceForTest(context.Background(), wl.Deps{}, input, wl.ComponentPlan{}, nil, wl.SnapshotWithPodsForTest(input, nil)); err != nil {
		t.Fatalf("expire (before deadline): %v", err)
	}
	if len(mutated) != 0 {
		t.Fatalf("1s before deadline must not expire; mutated %v", mutated)
	}

	// AT the deadline: now.After(deadline) is false — still no expiry.
	fc.SetTime(deadline)
	if err := wl.EscalateFromEvidenceForTest(context.Background(), wl.Deps{}, input, wl.ComponentPlan{}, nil, wl.SnapshotWithPodsForTest(input, nil)); err != nil {
		t.Fatalf("expire (at deadline): %v", err)
	}
	if len(mutated) != 0 {
		t.Fatalf("now == deadline must not expire (After, not !Before); mutated %v", mutated)
	}

	fc.SetTime(deadline.Add(time.Second)) // 1s AFTER
	if err := wl.EscalateFromEvidenceForTest(context.Background(), wl.Deps{}, input, wl.ComponentPlan{}, nil, wl.SnapshotWithPodsForTest(input, nil)); err != nil {
		t.Fatalf("expire (after deadline): %v", err)
	}
	if len(mutated) != 1 || mutated[0] != 0 {
		t.Fatalf("1s after deadline must expire instance 0; mutated %v", mutated)
	}
	if len(warned) != 1 || warned[0] != 0 {
		t.Fatalf("expiry must emit exactly one operator-facing warning for instance 0; warned %v", warned)
	}
}
