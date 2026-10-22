package status

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// stepStore is one row under the owner the pass read, with the input a
// guarded write needs over it.
func stepStore(row types.InstanceStatus) (*terminalMutationStore, types.ReconcileInput) {
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]types.InstanceStatus{row.Index: cloneTerminalStatus(row)},
	}
	return store, gangSurgeRecoveryInput("owner-a", "guarded-write", "test-ns", store, cloneTerminalStatus(row))
}

// A step transition moves the row the pass read, once: the first commit
// writes the step and dates the progress, a repeat finds the step in
// place and confirms it without a write, and a row that moved on since
// the pass read it is left alone.
func TestStepTransition_MovesTheStepOnceAndConfirmsItsOwnWrite(t *testing.T) {
	ctx := context.Background()
	row := fixture.UpdatingRowAtSurge(3)
	store, input := stepStore(row)
	now := time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	first, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	step := first.stepTransition(Capture(&row), types.UpdateStepSurgeDrain, nil, clock)
	done, err := first.commit(ctx, input, []types.InstanceMutation{step}, "", nil)
	if err != nil || !done || !first.committed || store.writes != 1 {
		t.Fatalf("first commit: done=%v committed=%v writes=%d err=%v", done, first.committed, store.writes, err)
	}
	moved := store.statuses[row.Index]
	if moved.Operation.Step != types.UpdateStepSurgeDrain || !moved.Operation.LastProgressAt.Time.Equal(now) {
		t.Fatalf("operation = %+v, want the step moved and progress dated now", moved.Operation)
	}

	repeat, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	step = repeat.stepTransition(Capture(&row), types.UpdateStepSurgeDrain, nil, clock)
	done, err = repeat.commit(ctx, input, []types.InstanceMutation{step}, "", nil)
	if err != nil || !done || repeat.committed || !repeat.confirmed || store.writes != 1 {
		t.Fatalf("repeat: done=%v committed=%v confirmed=%v writes=%d err=%v, want confirmed without a write",
			done, repeat.committed, repeat.confirmed, store.writes, err)
	}

	elsewhere := store.statuses[row.Index]
	elsewhere.Operation.ID = "another-attempt"
	store.statuses[row.Index] = elsewhere
	third, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	step = third.stepTransition(Capture(&row), types.UpdateStepSurgeDrain, nil, clock)
	done, err = third.commit(ctx, input, []types.InstanceMutation{step}, "", nil)
	if err != nil || done || store.writes != 1 {
		t.Fatalf("row that moved on: done=%v writes=%d err=%v, want nothing written and no error", done, store.writes, err)
	}
}

// The rest of the guard is asked first: when the other row of the pair
// has moved, the step is refused and nothing is written.
func TestStepTransition_RefusesTheStepWhenThePairHasMoved(t *testing.T) {
	row := fixture.UpdatingRowAtSurge(3)
	store, input := stepStore(row)
	write, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	asked := 0
	pairMoved := func(types.InstanceMutationSnapshot) bool {
		asked++
		return false
	}
	step := write.stepTransition(Capture(&row), types.UpdateStepSurgeDrain, pairMoved, nil)
	done, err := write.commit(context.Background(), input, []types.InstanceMutation{step}, "", nil)
	if err != nil || done || asked != 1 || store.writes != 0 {
		t.Fatalf("pair moved: done=%v asked=%d writes=%d err=%v", done, asked, store.writes, err)
	}
}

// Without a clock the step moves and the progress time stays as it was.
func TestStepTransition_DatesProgressOnlyWithAClock(t *testing.T) {
	row := fixture.UpdatingRowAtSurge(3)
	store, input := stepStore(row)
	write, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	step := write.stepTransition(Capture(&row), types.UpdateStepSurgeDrain, nil, nil)
	done, err := write.commit(context.Background(), input, []types.InstanceMutation{step}, "", nil)
	if err != nil || !done || store.writes != 1 {
		t.Fatalf("commit: done=%v writes=%d err=%v", done, store.writes, err)
	}
	moved := store.statuses[row.Index]
	if moved.Operation.Step != types.UpdateStepSurgeDrain || !moved.Operation.LastProgressAt.Equal(&row.Operation.LastProgressAt) {
		t.Fatalf("operation = %+v, want the step moved and the progress time kept", moved.Operation)
	}
}

// A guard starts from the owner: an owner replaced since the pass read
// it refuses the write before the rows are read, and that is not an
// error.
func TestGuardedWrite_ReplacedOwnerWritesNothing(t *testing.T) {
	row := fixture.UpdatingRowAtSurge(3)
	store, input := stepStore(row)
	store.ownerUID = "owner-b"
	write, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	asked := false
	mutation := types.InstanceMutation{
		Index: row.Index,
		Mutate: func(current *types.InstanceStatus) bool {
			current.Phase = types.InstancePhaseFailed
			return true
		},
		BatchPrecondition: write.guard(func(types.InstanceMutationSnapshot) bool {
			asked = true
			return true
		}),
		OnCommit: write.recordCommit,
	}
	done, err := write.commit(context.Background(), input, []types.InstanceMutation{mutation}, "", nil)
	if err != nil || done || asked || store.writes != 0 {
		t.Fatalf("replaced owner: done=%v asked=%v writes=%d err=%v", done, asked, store.writes, err)
	}
}

// commit reports the write it landed; a status write that fails for any
// other reason is returned to the caller as it is.
func TestGuardedWrite_CommitReportsTheLandedWriteAndReturnsOtherErrors(t *testing.T) {
	ctx := context.Background()
	row := fixture.UpdatingRowAtSurge(3)
	failing := func(current *types.InstanceStatus) bool {
		current.Phase = types.InstancePhaseFailed
		return true
	}

	store, input := stepStore(row)
	write, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	mutation := types.InstanceMutation{
		Index:             row.Index,
		Mutate:            failing,
		BatchPrecondition: write.guard(Capture(&row).holds),
		OnCommit:          write.recordCommit,
	}
	done, err := write.commit(ctx, input, []types.InstanceMutation{mutation}, "", nil)
	if err != nil || !done || store.writes != 1 || store.statuses[row.Index].Phase != types.InstancePhaseFailed {
		t.Fatalf("landed write: done=%v writes=%d err=%v", done, store.writes, err)
	}

	broken, input := stepStore(row)
	broken.applyErr = errors.New("status write failed")
	write, err = newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	mutation = types.InstanceMutation{
		Index:             row.Index,
		Mutate:            failing,
		BatchPrecondition: write.guard(Capture(&row).holds),
		OnCommit:          write.recordCommit,
	}
	done, err = write.commit(ctx, input, []types.InstanceMutation{mutation}, "", nil)
	if !errors.Is(err, broken.applyErr) || done || broken.writes != 0 {
		t.Fatalf("failed write: done=%v writes=%d err=%v, want the adapter's error", done, broken.writes, err)
	}
}

// guardHolds proves the guard and writes nothing: a guard that holds
// answers yes with no status write, a guard that no longer holds answers
// no with no error, and a read that fails is the caller's error.
func TestGuardHolds_ProvesTheGuardWithoutAWrite(t *testing.T) {
	ctx := context.Background()
	row := fixture.UpdatingRowAtSurge(3)
	store, input := stepStore(row)
	write, err := newGuardedWrite(input)
	if err != nil {
		t.Fatal(err)
	}
	guard := write.guard(Capture(&row).holds)

	held, err := guardHolds(ctx, input, row.Index, guard)
	if err != nil || !held || store.writes != 0 || store.applyCall != 1 {
		t.Fatalf("guard holds: held=%v writes=%d applyCalls=%d err=%v", held, store.writes, store.applyCall, err)
	}

	moved := store.statuses[row.Index]
	moved.Incarnation++
	store.statuses[row.Index] = moved
	held, err = guardHolds(ctx, input, row.Index, guard)
	if err != nil || held {
		t.Fatalf("guard no longer holds: held=%v err=%v, want no and no error", held, err)
	}

	store.readErr = errors.New("read failed")
	held, err = guardHolds(ctx, input, row.Index, guard)
	if held || !errors.Is(err, store.readErr) {
		t.Fatalf("read failure: held=%v err=%v, want the read error", held, err)
	}
}

// The adapter's two "another pass got there first" answers are not
// errors; any other error is returned; a nil error reports done as given.
func TestWriteOutcome_RefusedGuardsAreNotErrors(t *testing.T) {
	failed := errors.New("status write failed")
	tests := []struct {
		name     string
		err      error
		done     bool
		wantDone bool
		wantErr  error
	}{
		{name: "guard refused", err: types.ErrStatusMutationPrecondition, done: true},
		{name: "owner gone", err: fmt.Errorf("re-read: %w", types.ErrStatusOwnerGone), done: true},
		{name: "other error", err: failed, done: true, wantErr: failed},
		{name: "applied and done", done: true, wantDone: true},
		{name: "applied with nothing to do"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			done, err := writeOutcome(test.err, test.done)
			if done != test.wantDone || !errors.Is(err, test.wantErr) {
				t.Fatalf("writeOutcome(%v, %v) = %v, %v; want %v, %v", test.err, test.done, done, err, test.wantDone, test.wantErr)
			}
		})
	}
}

// A guarded write needs an owner to guard on and the batch adapter to
// carry the guard; without either it fails closed.
func TestNewGuardedWrite_FailsClosedWithoutOwnerOrBatchAdapter(t *testing.T) {
	store := &terminalMutationStore{ownerUID: "owner-a", statuses: map[int32]types.InstanceStatus{}}
	noOwner := types.ReconcileInput{ApplyInstanceMutationsWithRetryBlock: store.apply}
	if _, err := newGuardedWrite(noOwner); err == nil {
		t.Fatal("an input with no owner must be refused")
	}
	noAdapter := types.ReconcileInput{OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}}}
	if _, err := newGuardedWrite(noAdapter); err == nil {
		t.Fatal("an input with no batch adapter must be refused")
	}
}
