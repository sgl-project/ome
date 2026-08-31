package workload_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

// Dispatcher-level contracts for the migration expiry pass: it runs
// BEFORE the drive pass (an expired record is consumed, never driven),
// it runs regardless of MigrationMode (a mode flip to Never cannot
// strand a non-terminal record), and the post-expiry unpinned surge is
// an ordinary scale-down extra. The full expiry transition (pair
// unpinning, source restore from observation, surge teardown, retry
// acceptance) is covered by the workload/ops expiry tests.

// expiryDispatchFixture wires a Reconcile input holding one Manual
// record and observation stubs that record what the dispatcher did.
func expiryDispatchFixture(t *testing.T, rec workload.MigrationRecord) (workload.ReconcileInput, workload.Deps, *[]workload.MigrationRecord, *bool) {
	t.Helper()
	scheme := makeScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	clk := clocktesting.NewFakeClock(time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC))

	records := []workload.MigrationRecord{rec}
	migrateStamped := false

	in := minimalInput(t)
	in.Clock = clk
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady, RunningRevision: "rev-1"},
	}
	in.ObservedState.Migrations = append([]workload.MigrationRecord(nil), records...)
	in.MutateMigration = func(_ context.Context, uuid string, mutate func(*workload.MigrationRecord) bool) error {
		for i := range records {
			if records[i].RequestUUID == uuid {
				r := records[i]
				if mutate(&r) {
					records[i] = r
				}
				return nil
			}
		}
		return nil
	}
	in.MutateInstance = func(_ context.Context, _ int32, mutate func(*workload.InstanceStatus) bool) error {
		s := workload.InstanceStatus{Index: 0, Phase: workload.InstancePhaseReady}
		if mutate(&s) && s.Operation != nil && s.Operation.Type == workload.InstanceOperationMigrate {
			migrateStamped = true
		}
		return nil
	}
	return in, workload.Deps{Client: c, Clock: clk}, &records, &migrateStamped
}

// pastDeadlineRecord builds a non-terminal Manual record whose Deadline
// already elapsed relative to the fixture clock.
func pastDeadlineRecord(uuid string) workload.MigrationRecord {
	base := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	return workload.MigrationRecord{
		RequestUUID:    uuid,
		Trigger:        workload.MigrationTriggerManual,
		Phase:          workload.MigrationPhaseAccepted,
		SourceInstance: 0,
		FromNode:       "node-a",
		StartedAt:      metav1.NewTime(base.Add(-time.Hour)),
		Deadline:       metav1.NewTime(base.Add(-30 * time.Minute)),
	}
}

// TestReconcile_ExpiryBeforeDrive pins the precedence rule: a record
// past its Deadline is consumed by the expiry pass BEFORE the drive
// pass can pick it — the record closes Failed, no Migrate op is ever
// stamped, and the dispatcher requeues immediately so the next pass
// rebuilds plan + ObservedState from the post-expiry status.
func TestReconcile_ExpiryBeforeDrive(t *testing.T) {
	in, deps, records, migrateStamped := expiryDispatchFixture(t, pastDeadlineRecord("u-expired"))
	in.ObservedState.RetryBlocks = []workload.RetryBlock{{TargetRevision: "superseded"}}
	prunes := 0
	in.MutateRetryBlock = func(context.Context, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		prunes++
		return nil
	}
	plan := minimalPlan()
	plan.MigrationMode = workload.MigrationModeAuto

	res, err := workload.Reconcile(context.Background(), deps, in, plan, nil)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !res.Requeue {
		t.Errorf("dispatcher must requeue immediately after an expiry; got %+v", res)
	}
	got := (*records)[0]
	if got.Phase != workload.MigrationPhaseFailed || got.CompletedAt == nil {
		t.Fatalf("expired record must close Failed; got %+v", got)
	}
	if *migrateStamped {
		t.Errorf("the drive pass must never stamp a Migrate op for an expired record")
	}
	if prunes != 1 {
		t.Errorf("non-scale-down immediate requeue pruned %d RetryBlocks, want 1", prunes)
	}
}

// TestReconcile_ExpiryRunsUnderModeNever pins that the expiry pass is
// NOT gated on MigrationMode: a mode flip to Never after a record was
// accepted must not strand the non-terminal record forever.
func TestReconcile_ExpiryRunsUnderModeNever(t *testing.T) {
	in, deps, records, migrateStamped := expiryDispatchFixture(t, pastDeadlineRecord("u-never-mode"))
	plan := minimalPlan()
	plan.MigrationMode = workload.MigrationModeNever

	res, err := workload.Reconcile(context.Background(), deps, in, plan, nil)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !res.Requeue {
		t.Errorf("dispatcher must requeue after the expiry; got %+v", res)
	}
	if got := (*records)[0]; got.Phase != workload.MigrationPhaseFailed {
		t.Fatalf("record must expire under MigrationMode=Never too; got %+v", got)
	}
	if *migrateStamped {
		t.Errorf("no Migrate op may be stamped under MigrationMode=Never")
	}
}

// TestReconcile_NotYetExpiredRecordNotConsumed is the negative
// dispatcher control: a non-terminal record whose Deadline is still in
// the future is left alone by the expiry pass. MigrationMode=Never
// isolates the expiry pass (the drive pass — which owns a live record
// — is skipped, and would in this podless stub environment terminally
// reject it for unrelated reasons).
func TestReconcile_NotYetExpiredRecordNotConsumed(t *testing.T) {
	rec := pastDeadlineRecord("u-live")
	rec.Deadline = metav1.NewTime(time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)) // 1h ahead of the fixture clock
	in, deps, records, _ := expiryDispatchFixture(t, rec)
	plan := minimalPlan()
	plan.MigrationMode = workload.MigrationModeNever

	if _, err := workload.Reconcile(context.Background(), deps, in, plan, nil); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if got := (*records)[0]; got.Phase.Terminal() {
		t.Fatalf("record before its Deadline must not be expired; got %+v", got)
	}
}

// TestExtraInstanceIndices_UnpinnedSurgeIsExtra pins the teardown
// hand-off: once expiry clears the surge's Migrate op, the surviving
// Creating-phase status is an ordinary scale-down extra (contrast
// TestReconcile_MigrationOperationOwned_NotScaleDown, where the pinned
// op excludes it) — the standard Delete pipeline owns the teardown. The
// restored source stays covered by the plan and can never become an
// extra.
func TestExtraInstanceIndices_UnpinnedSurgeIsExtra(t *testing.T) {
	observed := []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady},    // restored source, in plan
		{Index: 1, Phase: workload.InstancePhaseCreating}, // unpinned surge, op cleared
	}
	plan := minimalPlan() // covers only index 0

	extras := workload.ExtraInstanceIndices(observed, plan, false)
	if len(extras) != 1 || extras[0] != 1 {
		t.Fatalf("unpinned surge must be the sole scale-down extra; got %v", extras)
	}
}
