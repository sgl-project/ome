package placement

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

const testUID types.UID = "uid-disp"

func TestDispatcherFor_DefaultsToAllAtOnce(t *testing.T) {
	// Empty and unrecognized modes both degrade to all-at-once (graceful
	// fallback: absent/bad config preserves the historical fleet-wide fan-out).
	for _, mode := range []DispatcherMode{"", "bogus", DispatcherModeAllAtOnce} {
		d := dispatcherFor(mode, 0, 0)
		_, isAllAtOnce := d.(allAtOnceDispatcher)
		assert.True(t, isAllAtOnce, "mode %q should resolve to all-at-once", mode)
	}
}

func TestDispatcherFor_IncrementalSelected(t *testing.T) {
	d := dispatcherFor(DispatcherModeIncremental, 2, time.Minute)
	_, isIncremental := d.(*incrementalDispatcher)
	assert.True(t, isIncremental, "Incremental mode should resolve to the incremental dispatcher")
}

func TestAllAtOnce_NominatesEverythingNoHold(t *testing.T) {
	d := allAtOnceDispatcher{}
	cands := []string{"a", "b", "c"}
	got, hold := d.Nominate(testUID, cands, time.Now())
	assert.Equal(t, cands, got, "all-at-once nominates the whole candidate set")
	assert.Zero(t, hold, "all-at-once never asks the caller to wait")
}

func TestAllAtOnce_EmptyCandidates(t *testing.T) {
	d := allAtOnceDispatcher{}
	got, hold := d.Nominate(testUID, nil, time.Now())
	assert.Empty(t, got)
	assert.Zero(t, hold)
}

func TestIncremental_FirstBatchAndHold(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	now := time.Now()
	cands := []string{"a", "b", "c", "d", "e"}

	got, hold := d.Nominate(testUID, cands, now)
	assert.Equal(t, []string{"a", "b"}, got, "first round nominates the leading step-size batch")
	assert.Equal(t, time.Minute, hold, "first round holds for the full round timeout")
}

func TestIncremental_HoldsWithinRoundThenWidens(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	start := time.Now()
	cands := []string{"a", "b", "c", "d", "e"}

	// Round 1: first batch.
	got, _ := d.Nominate(testUID, cands, start)
	assert.Equal(t, []string{"a", "b"}, got)

	// Still within the round: same set, shrinking hold, no widening.
	got, hold := d.Nominate(testUID, cands, start.Add(30*time.Second))
	assert.Equal(t, []string{"a", "b"}, got, "within the round the nominated set does not grow")
	assert.Equal(t, 30*time.Second, hold, "hold reports the remaining round time")

	// Round elapsed: next batch is appended and the clock restarts.
	got, hold = d.Nominate(testUID, cands, start.Add(time.Minute+time.Second))
	assert.Equal(t, []string{"a", "b", "c", "d"}, got, "after the round elapses the next batch is added")
	assert.Equal(t, time.Minute, hold, "a fresh round holds for the full timeout again")
}

func TestIncremental_WalksToExhaustionThenNoHold(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	tm := time.Now()
	cands := []string{"a", "b", "c", "d", "e"}
	advance := func() ([]string, time.Duration) {
		tm = tm.Add(time.Minute + time.Second)
		return d.Nominate(testUID, cands, tm)
	}

	got, _ := d.Nominate(testUID, cands, tm) // a,b
	assert.Equal(t, []string{"a", "b"}, got)
	got, _ = advance() // a,b,c,d
	assert.Equal(t, []string{"a", "b", "c", "d"}, got)

	// Final partial batch (only "e" left): set becomes full, so no further hold.
	got, hold := advance()
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got, "the trailing partial batch completes the set")
	assert.Zero(t, hold, "a full nominated set imposes no further wait")

	// Subsequent passes keep returning the full set with no hold (nothing to widen).
	got, hold = advance()
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got)
	assert.Zero(t, hold)
}

func TestIncremental_StepSizeCoveringAllInOneRound(t *testing.T) {
	// A step size >= the candidate count nominates everything in the first round
	// and imposes no hold (already full).
	d := newIncrementalDispatcher(10, time.Minute)
	cands := []string{"a", "b", "c"}
	got, hold := d.Nominate(testUID, cands, time.Now())
	assert.Equal(t, cands, got)
	assert.Zero(t, hold, "a first batch that already covers every candidate imposes no hold")
}

func TestIncremental_NonPositiveStepAdvancesByOne(t *testing.T) {
	// A misconfigured non-positive step must still make forward progress: one
	// candidate per round, not a wedged empty walk.
	d := newIncrementalDispatcher(0, time.Minute)
	start := time.Now()
	cands := []string{"a", "b", "c"}

	got, _ := d.Nominate(testUID, cands, start)
	assert.Equal(t, []string{"a"}, got, "non-positive step nominates one candidate")

	got, _ = d.Nominate(testUID, cands, start.Add(time.Minute+time.Second))
	assert.Equal(t, []string{"a", "b"}, got, "and advances by one each round")
}

func TestIncremental_NonPositiveRoundTimeoutAdvancesEveryPass(t *testing.T) {
	// With no enforced dwell, each pass may add the next batch immediately.
	d := newIncrementalDispatcher(1, 0)
	now := time.Now()
	cands := []string{"a", "b", "c"}

	got, hold := d.Nominate(testUID, cands, now)
	assert.Equal(t, []string{"a"}, got)
	assert.Zero(t, hold, "a non-positive round timeout imposes no hold")

	got, _ = d.Nominate(testUID, cands, now)
	assert.Equal(t, []string{"a", "b"}, got, "the next pass advances without waiting")
}

func TestIncremental_DroppedCandidateLeavesNominatedSet(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	start := time.Now()

	// Round 1 nominates a,b.
	got, _ := d.Nominate(testUID, []string{"a", "b", "c", "d"}, start)
	assert.Equal(t, []string{"a", "b"}, got)

	// "a" leaves the fleet. It must not stay nominated; the retained set keeps
	// only surviving candidates (here "b"), in order.
	got, hold := d.Nominate(testUID, []string{"b", "c", "d"}, start.Add(30*time.Second))
	assert.Equal(t, []string{"b"}, got, "a departed cluster is dropped from the retained set")
	assert.Equal(t, 30*time.Second, hold, "the round clock is unaffected by the membership change")
}

func TestIncremental_AllNomineesLeaveRestartsWalk(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	start := time.Now()

	got, _ := d.Nominate(testUID, []string{"a", "b", "c", "d"}, start)
	assert.Equal(t, []string{"a", "b"}, got)

	// Both nominees leave; the retained set empties, so the walk restarts from the
	// leading batch of the new candidate set and the round clock resets.
	got, hold := d.Nominate(testUID, []string{"c", "d", "e"}, start.Add(30*time.Second))
	assert.Equal(t, []string{"c", "d"}, got, "an emptied retained set restarts the walk from the new leading batch")
	assert.Equal(t, time.Minute, hold, "the restarted walk starts a fresh round")
}

func TestIncremental_EmptyCandidatesClearsRound(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	start := time.Now()

	got, _ := d.Nominate(testUID, []string{"a", "b", "c"}, start)
	assert.Equal(t, []string{"a", "b"}, got)

	// Candidate set drains to empty (no Ready cluster matches now).
	got, hold := d.Nominate(testUID, nil, start.Add(time.Second))
	assert.Empty(t, got)
	assert.Zero(t, hold)

	// When candidates reappear the walk starts fresh (the stale round was cleared),
	// not mid-stream past an elapsed clock.
	got, hold = d.Nominate(testUID, []string{"a", "b", "c"}, start.Add(2*time.Second))
	assert.Equal(t, []string{"a", "b"}, got, "a re-matched ISVC restarts from the first batch")
	assert.Equal(t, time.Minute, hold)
}

func TestIncremental_ForgetResetsWalk(t *testing.T) {
	d := newIncrementalDispatcher(2, time.Minute)
	start := time.Now()
	cands := []string{"a", "b", "c", "d"}

	got, _ := d.Nominate(testUID, cands, start)
	assert.Equal(t, []string{"a", "b"}, got)

	d.forget(testUID)

	// After forgetting, the next pass starts a fresh walk from the first batch.
	got, hold := d.Nominate(testUID, cands, start.Add(30*time.Second))
	assert.Equal(t, []string{"a", "b"}, got, "forget drops round state so the walk restarts")
	assert.Equal(t, time.Minute, hold)
}

func TestIncremental_PerISVCIsolation(t *testing.T) {
	// Two ISVCs walk independently; advancing one must not move the other.
	d := newIncrementalDispatcher(1, time.Minute)
	start := time.Now()
	const uidA types.UID = "uid-a"
	const uidB types.UID = "uid-b"
	cands := []string{"x", "y", "z"}

	gotA, _ := d.Nominate(uidA, cands, start)
	assert.Equal(t, []string{"x"}, gotA)

	// Advance A only.
	gotA, _ = d.Nominate(uidA, cands, start.Add(time.Minute+time.Second))
	assert.Equal(t, []string{"x", "y"}, gotA)

	// B is still on its first batch — A's progress did not leak across.
	gotB, _ := d.Nominate(uidB, cands, start.Add(time.Minute+time.Second))
	assert.Equal(t, []string{"x"}, gotB, "each ISVC's walk is isolated by UID")
}

func TestIncremental_ReturnedSlicesAreCopies(t *testing.T) {
	// Mutating a returned slice must not corrupt the dispatcher's retained set.
	d := newIncrementalDispatcher(3, time.Minute)
	cands := []string{"a", "b", "c", "d"}
	got, _ := d.Nominate(testUID, cands, time.Now())
	wantFirst := []string{"a", "b", "c"}
	assert.Equal(t, wantFirst, got)

	got[0] = "MUTATED"
	got2, _ := d.Nominate(testUID, cands, time.Now())
	assert.Equal(t, wantFirst, got2, "a caller mutating the returned slice must not affect retained state")
}
