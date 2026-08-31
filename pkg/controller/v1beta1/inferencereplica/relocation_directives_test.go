package inferencereplica

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/audit"
)

// directiveEntry builds one terminal relocation-directive ledger row.
func directiveEntry(uid, component string, idx int32, node string) audit.Entry {
	return audit.Entry{
		RequestUUID:    uid,
		Component:      component,
		SourceInstance: idx,
		Phase:          audit.PhaseCompleted,
		Reason:         audit.ReasonAutoRecover,
		Outcome:        audit.OutcomeRelocateRecreate,
		FromNode:       node,
		StartedAt:      time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
}

// reconcileRelocationDirectives projects the exclusion map from the
// ledger's AutoRecover directives, bounded to the most recent
// autoMigrateBudget DISTINCT nodes per instance, scoped to the IR's
// component, and ignoring non-AutoRecover rows.
func TestReconcileRelocationDirectives_BuildsBoundedExclusionMap(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate}},
	}
	r, c := newReconciler(t, ir)

	ledger := &audit.Ledger{}
	// Five directives for instance 0 — dedup happens BEFORE the budget
	// window, so the repeated n4 doesn't shrink the memory: the last
	// three DISTINCT nodes {n2,n3,n4} survive.
	for i, node := range []string{"n1", "n2", "n3", "n4", "n4"} {
		ledger.UpsertEntry(directiveEntry(fmt.Sprintf("u%d", i), "engine", 0, node))
	}
	// Noise: other component + non-AutoRecover operator migration.
	ledger.UpsertEntry(directiveEntry("dec", "decoder", 0, "n9"))
	ledger.UpsertEntry(audit.Entry{RequestUUID: "op1", Component: "engine", SourceInstance: 0,
		Phase: audit.PhaseStarted, Reason: "fragmentation", FromNode: "n8",
		StartedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)})
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, ledger); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 {
		t.Fatalf("map: got %v want exactly instance 0", got)
	}
	nodes := got[0]
	if len(nodes) != 3 || nodes[0] != "n2" || nodes[1] != "n3" || nodes[2] != "n4" {
		t.Errorf("instance 0 exclusions: got %v want [n2 n3 n4] (last 3 distinct nodes)", nodes)
	}

	// Budget 0 (unconfigured) → no exclusions at all.
	if got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 0); got != nil {
		t.Errorf("budget 0: got %v want nil", got)
	}
}

// An instance observed Phase=Ready with no in-flight Operation has
// proven its placement: its AutoRecover directives are pruned from the
// persisted ledger (success-prune mirror of the RetryBlock prune) and
// drop out of the exclusion map. Foreign entries survive.
func TestReconcileRelocationDirectives_PrunesOnReady(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate}},
	}
	r, c := newReconciler(t, ir)

	ledger := &audit.Ledger{}
	ledger.UpsertEntry(directiveEntry("u0", "engine", 0, "n1"))
	ledger.UpsertEntry(directiveEntry("u1", "engine", 1, "n2"))
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, ledger); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 || len(got[1]) != 1 || got[1][0] != "n2" {
		t.Fatalf("map: got %v want only instance 1 → [n2] (instance 0 pruned on Ready)", got)
	}

	// The prune persisted: instance 0's directive is gone from the CM,
	// instance 1's survives.
	after, err := audit.LoadLedgerForOwner(context.Background(), c, ir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 0) != 0 {
		t.Errorf("instance 0 directives not pruned: %+v", after.Entries)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 1) != 1 {
		t.Errorf("instance 1 directives must survive: %+v", after.Entries)
	}
}

// The prune-persist branch re-loads the ledger LIVE and re-prunes
// before writing: the persist writes the snapshot wholesale, so basing
// it on a lagged cache would drop rows written concurrently by sibling
// IRs. The sibling decoder row here exists ONLY behind APIReader and
// must survive the persisted prune.
func TestReconcileRelocationDirectives_PrunePersistsFromLiveSnapshot(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate}},
	}
	r, c := newReconciler(t, ir)

	cached := &audit.Ledger{}
	cached.UpsertEntry(directiveEntry("u0", "engine", 0, "n1"))
	cached.UpsertEntry(directiveEntry("u1", "engine", 1, "n2"))
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, cached); err != nil {
		t.Fatalf("seed cached ledger: %v", err)
	}
	readerClient := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	live := &audit.Ledger{}
	live.UpsertEntry(directiveEntry("u0", "engine", 0, "n1"))
	live.UpsertEntry(directiveEntry("u1", "engine", 1, "n2"))
	live.UpsertEntry(directiveEntry("dec", "decoder", 0, "n9"))
	if err := audit.PersistLedgerForOwner(context.Background(), readerClient, ir, irGVK, live); err != nil {
		t.Fatalf("seed live ledger: %v", err)
	}
	r.APIReader = readerClient

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 || len(got[1]) != 1 || got[1][0] != "n2" {
		t.Fatalf("map: got %v want only instance 1 → [n2]", got)
	}

	after, err := audit.LoadLedgerForOwner(context.Background(), c, ir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 0) != 0 {
		t.Errorf("instance 0 directives not pruned: %+v", after.Entries)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 1) != 1 {
		t.Errorf("instance 1 directives must survive: %+v", after.Entries)
	}
	if audit.CountAutoRecoverAttempts(after, "decoder", 0) != 1 {
		t.Errorf("sibling decoder row must survive the wholesale persist: %+v", after.Entries)
	}
}

// failingGetReader errors every Get — stands in for a live re-load
// failure inside the prune-persist branch.
type failingGetReader struct{ client.Reader }

func (failingGetReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return fmt.Errorf("live read down")
}

// A live re-load failure in the persist branch fails open: the
// projection still uses the cache-pruned in-memory view, and the
// persisted ledger stays untouched so the prune retries next pass.
func TestReconcileRelocationDirectives_LiveReloadFailureSkipsPersist(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate}},
	}
	r, c := newReconciler(t, ir)

	ledger := &audit.Ledger{}
	ledger.UpsertEntry(directiveEntry("u0", "engine", 0, "n1"))
	ledger.UpsertEntry(directiveEntry("u1", "engine", 1, "n2"))
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, ledger); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	r.APIReader = failingGetReader{}

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 || len(got[1]) != 1 || got[1][0] != "n2" {
		t.Fatalf("map: got %v want only instance 1 → [n2] (cache-pruned view)", got)
	}

	after, err := audit.LoadLedgerForOwner(context.Background(), c, ir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 0) != 1 {
		t.Errorf("persist must be skipped on live re-load failure (prune retries next pass): %+v", after.Entries)
	}
}

// autoRecord builds one born-terminal Auto migration status record.
func autoRecord(uuid string, idx int32, node string, startedAt time.Time) v1beta1.MigrationStatus {
	started := metav1.NewTime(startedAt)
	completed := started
	return v1beta1.MigrationStatus{
		RequestUUID:    uuid,
		Trigger:        v1beta1.MigrationTriggerAuto,
		SourceInstance: idx,
		FromNode:       node,
		Phase:          v1beta1.MigrationPhaseRelocated,
		Attempt:        1,
		Reason:         audit.ReasonAutoRecover,
		StartedAt:      started,
		Deadline:       started,
		CompletedAt:    &completed,
	}
}

// The success touch: an instance observed Ready with no in-flight
// Operation stamps Succeeded=true + CompletedAt on its NEWEST
// un-Succeeded Auto record only — older un-Succeeded records and other
// instances' records stay untouched — while the same pass prunes the
// instance's ledger rows. The asymmetry is deliberate: ledger =
// working memory for exclusions (pruned on Ready), record = visible
// history (persists until the trim window).
func TestReconcileRelocationDirectives_SuccessTouchStampsNewestAutoRecord(t *testing.T) {
	t0 := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	ir := baselineIR("llama-engine", "prod", 2)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate}},
	}
	ir.Status.Migrations = []v1beta1.MigrationStatus{
		autoRecord("u-old", 0, "n1", t0.Add(-10*time.Minute)),
		autoRecord("u-new", 0, "n2", t0.Add(-5*time.Minute)),
		autoRecord("u-other", 1, "n3", t0.Add(-5*time.Minute)),
	}
	r, c := newReconciler(t, ir)

	ledger := &audit.Ledger{}
	ledger.UpsertEntry(directiveEntry("u-old", "engine", 0, "n1"))
	ledger.UpsertEntry(directiveEntry("u-new", "engine", 0, "n2"))
	ledger.UpsertEntry(directiveEntry("u-other", "engine", 1, "n3"))
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, ledger); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 || len(got[1]) != 1 || got[1][0] != "n3" {
		t.Fatalf("exclusion map: got %v want only instance 1 → [n3]", got)
	}

	fresh := &v1beta1.InferenceReplica{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(ir), fresh); err != nil {
		t.Fatalf("re-read IR: %v", err)
	}
	byUUID := map[string]v1beta1.MigrationStatus{}
	for _, e := range fresh.Status.Migrations {
		byUUID[e.RequestUUID] = e
	}
	newest := byUUID["u-new"]
	if newest.Succeeded == nil || !*newest.Succeeded {
		t.Errorf("u-new Succeeded: got %v want true (newest record for the Ready instance)", newest.Succeeded)
	}
	if newest.CompletedAt == nil || !newest.CompletedAt.Time.After(t0.Add(-5*time.Minute)) {
		t.Errorf("u-new CompletedAt: got %v want restamped at success time", newest.CompletedAt)
	}
	if older := byUUID["u-old"]; older.Succeeded != nil {
		t.Errorf("u-old Succeeded: got %v want nil (only the newest record is stamped)", *older.Succeeded)
	}
	if other := byUUID["u-other"]; other.Succeeded != nil {
		t.Errorf("u-other Succeeded: got %v want nil (instance 1 is not Ready)", *other.Succeeded)
	}
	if len(ir.Status.Migrations) != 3 || ir.Status.Migrations[1].Succeeded == nil {
		t.Errorf("in-memory mirror: got %+v want the committed stamp mirrored back", ir.Status.Migrations)
	}

	// The ledger rows for instance 0 pruned in the same pass; the
	// status records persist — the asymmetry under test.
	after, err := audit.LoadLedgerForOwner(context.Background(), c, ir)
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 0) != 0 {
		t.Errorf("instance 0 ledger rows must prune on Ready: %+v", after.Entries)
	}
	if audit.CountAutoRecoverAttempts(after, "engine", 1) != 1 {
		t.Errorf("instance 1 ledger rows must survive: %+v", after.Entries)
	}
}

// Crash-window heal: the ledger rows were already pruned (prior pass
// crashed between the prune persist and the stamp) — the success touch
// still fires because it is keyed on the status records, not the
// ledger.
func TestReconcileRelocationDirectives_SuccessTouchIndependentOfLedger(t *testing.T) {
	t0 := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
	}
	ir.Status.Migrations = []v1beta1.MigrationStatus{autoRecord("u-orphan", 0, "n1", t0)}
	r, c := newReconciler(t, ir)

	// No ledger seeded at all.
	if got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3); got != nil {
		t.Fatalf("exclusion map: got %v want nil (empty ledger)", got)
	}

	fresh := &v1beta1.InferenceReplica{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(ir), fresh); err != nil {
		t.Fatalf("re-read IR: %v", err)
	}
	if len(fresh.Status.Migrations) != 1 || fresh.Status.Migrations[0].Succeeded == nil || !*fresh.Status.Migrations[0].Succeeded {
		t.Errorf("record: got %+v want u-orphan stamped Succeeded=true without any ledger rows", fresh.Status.Migrations)
	}
}

func TestStampAutoRelocationSuccess_SameNameReplacementUntouched(t *testing.T) {
	t0 := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	stale := baselineIR("llama-engine", "prod", 1)
	stale.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
	}
	stale.Status.Migrations = []v1beta1.MigrationStatus{autoRecord("u-stale", 0, "n1", t0)}

	replacement := stale.DeepCopy()
	replacement.UID = "replacement-uid"
	r, c := newReconciler(t, replacement)

	err := r.stampAutoRelocationSuccess(context.Background(), stale)
	if !errors.Is(err, workload.ErrStatusOwnerGone) {
		t.Fatalf("stamp error: got %v want ErrStatusOwnerGone", err)
	}

	fresh := &v1beta1.InferenceReplica{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(replacement), fresh); err != nil {
		t.Fatalf("re-read replacement: %v", err)
	}
	if got := fresh.Status.Migrations[0].Succeeded; got != nil {
		t.Fatalf("replacement Succeeded: got %v want nil", *got)
	}
	if got := stale.Status.Migrations[0].Succeeded; got != nil {
		t.Fatalf("stale in-memory Succeeded: got %v want nil", *got)
	}
}

// A Ready instance with an in-flight Operation (e.g. a fresh update
// just stamped) is NOT pruned — only settled Ready-no-op instances
// reset their relocation memory.
func TestReconcileRelocationDirectives_ReadyWithOpNotPruned(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Phase: v1beta1.OMENativeInstanceReady,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationUpdate,
				StartedAt: metav1.NewTime(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))}},
	}
	r, c := newReconciler(t, ir)
	ledger := &audit.Ledger{}
	ledger.UpsertEntry(directiveEntry("u0", "engine", 0, "n1"))
	if err := audit.PersistLedgerForOwner(context.Background(), c, ir, irGVK, ledger); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	got := r.reconcileRelocationDirectives(context.Background(), logf.Log.WithName("test"), ir, nil, 3)
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != "n1" {
		t.Fatalf("map: got %v want instance 0 → [n1] (no prune while op in flight)", got)
	}
}
