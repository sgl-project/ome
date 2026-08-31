package workload_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

// TestPodAdmissionGated pins the gated-pod detector: a pod is "gated"
// (queued for admission, e.g. by Kueue) iff it still carries a scheduling
// gate. An un-gated or nil pod is not.
func TestPodAdmissionGated(t *testing.T) {
	gated := &corev1.Pod{Spec: corev1.PodSpec{
		SchedulingGates: []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}},
	}}
	if !workload.PodAdmissionGated(gated) {
		t.Errorf("pod with a scheduling gate: got false want true")
	}
	if workload.PodAdmissionGated(&corev1.Pod{}) {
		t.Errorf("pod with no scheduling gate: got true want false")
	}
	if workload.PodAdmissionGated(nil) {
		t.Errorf("nil pod: got true want false")
	}
}

// TestReconcileGatedDeadlines_ParksGatedInstance: while an Instance's pods
// are admission-gated, its Operation.Deadline is parked (zeroed → "never
// expires") so the gated wait cannot count against InstanceReadyTimeout.
// The Instance is NOT failed — it is queued, not stuck.
func TestReconcileGatedDeadlines_ParksGatedInstance(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index: 0,
		Phase: workload.InstancePhaseCreating,
		Operation: &workload.InstanceOperation{
			Type:     workload.InstanceOperationCreate,
			Deadline: metav1.NewTime(now.Add(-1 * time.Minute)), // already past
		},
	}}
	input, store, _ := expireFixture(insts)

	if err := workload.ReconcileGatedDeadlines(context.Background(), input, insts, map[int32]bool{0: true}, 30*time.Minute); err != nil {
		t.Fatalf("ReconcileGatedDeadlines: %v", err)
	}
	if !(*store)[0].Operation.Deadline.IsZero() {
		t.Errorf("gated Deadline: got %v want zero (parked)", (*store)[0].Operation.Deadline)
	}
	if (*store)[0].Phase != workload.InstancePhaseCreating {
		t.Errorf("Phase: got %q want Creating (paused, not failed)", (*store)[0].Phase)
	}
}

// TestReconcileGatedDeadlines_RestartsOnUngate: once the pods clear the
// gate, a parked (zero) Deadline is (re)started as now+timeout — i.e. the
// timeout is measured from admission, not from operation start.
func TestReconcileGatedDeadlines_RestartsOnUngate(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index: 0,
		Phase: workload.InstancePhaseCreating,
		Operation: &workload.InstanceOperation{
			Type: workload.InstanceOperationCreate,
			// Deadline zero: parked while previously gated.
		},
	}}
	input, store, _ := expireFixture(insts)

	if err := workload.ReconcileGatedDeadlines(context.Background(), input, insts, map[int32]bool{}, 30*time.Minute); err != nil {
		t.Fatalf("ReconcileGatedDeadlines: %v", err)
	}
	d := (*store)[0].Operation.Deadline
	if d.IsZero() {
		t.Fatalf("ungated Deadline: got zero want ~now+timeout")
	}
	if !d.Time.After(now.Add(25 * time.Minute)) {
		t.Errorf("restarted Deadline %v should be ~now+30m (measured from admission)", d.Time)
	}
}

// TestReconcileGatedDeadlines_UngatedNormalUntouched: the no-Kueue path is
// unchanged — an un-gated Instance with a normal (non-zero) Deadline is
// left exactly as the per-op writer stamped it.
func TestReconcileGatedDeadlines_UngatedNormalUntouched(t *testing.T) {
	now := time.Now()
	orig := metav1.NewTime(now.Add(20 * time.Minute))
	insts := []workload.InstanceStatus{{
		Index:     0,
		Phase:     workload.InstancePhaseCreating,
		Operation: &workload.InstanceOperation{Type: workload.InstanceOperationCreate, Deadline: orig},
	}}
	input, store, _ := expireFixture(insts)

	if err := workload.ReconcileGatedDeadlines(context.Background(), input, insts, map[int32]bool{}, 30*time.Minute); err != nil {
		t.Fatalf("ReconcileGatedDeadlines: %v", err)
	}
	if !(*store)[0].Operation.Deadline.Equal(&orig) {
		t.Errorf("un-gated normal Deadline changed: got %v want %v (untouched)", (*store)[0].Operation.Deadline, orig)
	}
}

// TestReconcileGatedDeadlines_NonTransientUntouched: only transient ops
// (Creating/Updating/Restarting/Migrating) are subject to the clock; a
// Ready Instance is never touched even if its pods report gated.
func TestReconcileGatedDeadlines_NonTransientUntouched(t *testing.T) {
	now := time.Now()
	orig := metav1.NewTime(now.Add(5 * time.Minute))
	insts := []workload.InstanceStatus{{
		Index:     0,
		Phase:     workload.InstancePhaseReady,
		Operation: &workload.InstanceOperation{Type: workload.InstanceOperationCreate, Deadline: orig},
	}}
	input, store, _ := expireFixture(insts)

	if err := workload.ReconcileGatedDeadlines(context.Background(), input, insts, map[int32]bool{0: true}, 30*time.Minute); err != nil {
		t.Fatalf("ReconcileGatedDeadlines: %v", err)
	}
	if !(*store)[0].Operation.Deadline.Equal(&orig) {
		t.Errorf("non-transient Deadline changed: got %v want untouched", (*store)[0].Operation.Deadline)
	}
}

// TestGatedInstance_NotFailedByDeadlineBackstop is the regression guard for
// the fix: a gated Instance long past its ORIGINAL deadline must survive
// the deadline backstop — pause (parks the clock) then expire (sees the
// parked zero, skips). Without the pause step the backstop would fail it.
func TestGatedInstance_NotFailedByDeadlineBackstop(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index: 0,
		Phase: workload.InstancePhaseCreating,
		Operation: &workload.InstanceOperation{
			Type:     workload.InstanceOperationCreate,
			Deadline: metav1.NewTime(now.Add(-1 * time.Hour)), // long past
		},
	}}
	input, store, event := expireFixture(insts)

	if err := workload.ReconcileGatedDeadlines(context.Background(), input, insts, map[int32]bool{0: true}, 30*time.Minute); err != nil {
		t.Fatalf("ReconcileGatedDeadlines: %v", err)
	}
	// Rebuild the observation from the parked store, mirroring the fresh
	// status read the next reconcile's escalation pass consumes.
	input.ObservedState.InstanceStatuses = append([]workload.InstanceStatus(nil), (*store)...)
	if err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{}, nil); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if (*store)[0].Phase == workload.InstancePhaseFailed {
		t.Errorf("gated Instance was failed by the deadline backstop; want alive (clock paused while gated)")
	}
	if event.count != 0 {
		t.Errorf("event count: got %d want 0 (gated Instance must not be failed)", event.count)
	}
}
