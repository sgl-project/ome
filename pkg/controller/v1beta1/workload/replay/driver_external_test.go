package replay_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferencereplica"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/replay"
	types "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// coldCreate is the smallest scenario that exercises every layer of the
// driver: a plan with no rows, one reconcile that materializes the pod,
// and one that promotes it once the kubelet reports it Ready.
const coldCreate = `
scenario: cold-create
arrows: [T-empty-create]
initial:
  spec:
    replicas: 1
    strategy: SurgeThenDrain
    image: registry.example.com/runtime:v1
    instanceReadyTimeout: 30m
  config:
    requeueOperation: 5s
    requeueGate: 3s
timeline:
  - tick: 1
  - tick: 2
    events:
      - pod.ready: {pod: {index: 0}}
`

func TestRunRecordsCreateAndPromote(t *testing.T) {
	scenario, err := replay.Parse([]byte(coldCreate), replay.Vocabulary{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	trace, err := replay.Run(context.Background(), scenario, replay.Options{})
	if err != nil {
		t.Fatalf("run: %v\ntrace:\n%s", err, trace)
	}
	for _, want := range []string{"pod create", "row=0 write", "pass result="} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q:\n%s", want, trace)
		}
	}
	t.Logf("trace:\n%s", trace)
}

// TestRunIsDeterministic is the package-local form of the repository's
// determinism gate: the same scenario replayed twice must produce the
// same bytes.
func TestRunIsDeterministic(t *testing.T) {
	scenario, err := replay.Parse([]byte(coldCreate), replay.Vocabulary{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	first, err := replay.Run(context.Background(), scenario, replay.Options{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for i := 0; i < 5; i++ {
		next, err := replay.Run(context.Background(), scenario, replay.Options{})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if next != first {
			t.Fatalf("run %d diverged:\nfirst:\n%s\ngot:\n%s", i, first, next)
		}
	}
}

// TestParseRejectsUndeclaredEvent pins the loader contract: the table's
// event vocabulary is the scenario's vocabulary.
func TestParseRejectsUndeclaredEvent(t *testing.T) {
	vocab := replay.Vocabulary{Events: map[string][]string{"pod.ready": {"source", "target"}}}
	_, err := replay.Parse([]byte(coldCreate), vocab)
	if err != nil {
		t.Fatalf("declared event rejected: %v", err)
	}
	bad := strings.Replace(coldCreate, "pod.ready", "pod.invented", 1)
	if _, err := replay.Parse([]byte(bad), vocab); err == nil {
		t.Fatalf("an undeclared event must fail to load")
	}
	badVariant := strings.Replace(coldCreate, "pod.ready:", "pod.ready[sideways]:", 1)
	if _, err := replay.Parse([]byte(badVariant), vocab); err == nil {
		t.Fatalf("an undeclared variant must fail to load")
	}
}

// resumedSurge starts with the surge already in flight, as a controller
// restarted mid-surge finds it: the row carries the operation and the
// replacement exists beside the serving source.
const resumedSurge = `
scenario: resumed-surge
arrows: [T-surge-target-ready, T-surgedrain-promote]
initial:
  spec:
    replicas: 1
    strategy: SurgeThenDrain
    image: registry.example.com/runtime:v2
    instanceReadyTimeout: 30m
  config:
    requeueOperation: 5s
    requeueGate: 3s
  currentRevision: registry.example.com/runtime:v1
  rows:
    - index: 0
      phase: Updating
      runningRevision: registry.example.com/runtime:v1
      targetRevision: current
      readySince: 0s
      operation: {type: Update, step: Surge, strategy: SurgeThenDrain}
  pods:
    - {index: 0, ordinal: 0, previousImage: registry.example.com/runtime:v1, ready: true, serving: true, routed: true}
    - {index: 0, ordinal: 1, phase: Pending}
timeline:
  - tick: 1
  - tick: 2
    events:
      - pod.ready[target]: {pod: {index: 0, ordinal: 1}}
      - endpoint.rotationIn[target]: {pod: {index: 0, ordinal: 1}}
  - tick: 3
    events:
      - endpoint.rotationOut[source]: {pod: {index: 0, ordinal: 0}}
  - tick: 4
    events:
      - pod.deleted[source]: {pod: {index: 0, ordinal: 0}}
`

// TestRunResumesASeededOperation pins that an operation seeded in status is
// resumed rather than restarted: the engine creates no replacement of its
// own, advances the seeded attempt and promotes it.
func TestRunResumesASeededOperation(t *testing.T) {
	scenario, err := replay.Parse([]byte(resumedSurge), replay.Vocabulary{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	trace, err := replay.Run(context.Background(), scenario, replay.Options{})
	if err != nil {
		t.Fatalf("run: %v\ntrace:\n%s", err, trace)
	}
	if strings.Contains(trace, "pod create") {
		t.Fatalf("a resumed surge must not create a second replacement:\n%s", trace)
	}
	for _, want := range []string{"step=SurgeDrain", "phase=Ready", "activeOrdinal=1", "RecreateUpdateCompleted"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q:\n%s", want, trace)
		}
	}
}

// deletedPastDeadline deletes a settled owner and lets the configured
// teardown deadline elapse with its pod still standing.
const deletedPastDeadline = `
scenario: deleted-past-deadline
arrows: [T-ready-teardown]
initial:
  spec:
    replicas: 1
    strategy: SurgeThenDrain
    image: registry.example.com/runtime:v1
    instanceReadyTimeout: 30m
  config:
    requeueOperation: 5s
    scaleDownRequeueInterval: 30s
    teardownDeadline: 10m
  rows: [{index: 0, phase: Ready, runningRevision: current, readySince: 0s}]
  pods: [{index: 0, ready: true, serving: true, routed: true}]
timeline:
  - tick: 1
    events:
      - spec.teardown[deleted]: {}
  - tick: 2
    events:
      - timer.teardownDeadline: {slack: 1s}
  - tick: 3
`

// TestRunReleasesTheOwnerPastItsTeardownDeadline pins the driver's
// stand-in for the adapter's deadline release: the Warning carries the
// adapter's reason, every row the owner carried is recorded leaving from
// the state it was in (the pass that observed the deletion had already
// stamped the row Deleting), the owner goes with them, and a later pass
// finds nothing left to act on. Both ticks reach the observer: the
// release with the removals as its commits and no row left, the later one
// with no row and no commit.
func TestRunReleasesTheOwnerPastItsTeardownDeadline(t *testing.T) {
	scenario, err := replay.Parse([]byte(deletedPastDeadline), replay.Vocabulary{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	views := map[int]replay.TickView{}
	observe := func(view replay.TickView) { views[view.Tick] = view }
	trace, err := replay.Run(context.Background(), scenario, replay.Options{Observer: observe})
	if err != nil {
		t.Fatalf("run: %v\ntrace:\n%s", err, trace)
	}
	for _, want := range []string{
		"tick=2 event Warning " + inferencereplica.ReasonTeardownDeadlineExceeded + " teardown deadline 10m exceeded (1 owned pod(s) observed",
		"tick=2 row=0 remove incarnation=1 phase=Deleting",
		"tick=2 owner deleted rows=1",
		"tick=2 pass result=none",
		"tick=3 owner gone; nothing reconciles",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q:\n%s", want, trace)
		}
	}
	if strings.Index(trace, "tick=2 row=0 remove") > strings.Index(trace, "tick=2 owner deleted") {
		t.Fatalf("the row's removal is part of the owner's deletion and reads before it:\n%s", trace)
	}
	if strings.Contains(trace, "tick=3 row=") || strings.Contains(trace, "tick=3 pod ") {
		t.Fatalf("a pass after the release must act on nothing:\n%s", trace)
	}

	release, ok := views[2]
	if !ok {
		t.Fatalf("the release tick was not handed to the observer; ticks seen: %v", tickNumbers(views))
	}
	if len(release.Commits) != 1 {
		t.Fatalf("release commits = %+v, want the one row's removal", release.Commits)
	}
	removal := release.Commits[0]
	switch {
	case !removal.Remove || removal.Outcome != replay.PreconditionOK || removal.After != nil:
		t.Fatalf("release commit = %+v, want a committed removal", removal)
	case removal.Before == nil || removal.Index != 0 || removal.Before.Phase != types.InstancePhaseDeleting:
		t.Fatalf("release commit before = %+v, want the Deleting row at index 0", removal.Before)
	}
	if len(release.Rows) != 0 || !release.Spec.Teardown {
		t.Fatalf("release view rows = %+v teardown = %v, want no row left of a deleted owner", release.Rows, release.Spec.Teardown)
	}
	after, ok := views[3]
	if !ok {
		t.Fatalf("the tick after the release was not handed to the observer; ticks seen: %v", tickNumbers(views))
	}
	if len(after.Rows) != 0 || len(after.Commits) != 0 || !after.Spec.Teardown {
		t.Fatalf("view after the release: rows = %+v commits = %+v teardown = %v, want nothing to act on", after.Rows, after.Commits, after.Spec.Teardown)
	}
}

func tickNumbers(views map[int]replay.TickView) []int {
	ticks := make([]int, 0, len(views))
	for tick := range views {
		ticks = append(ticks, tick)
	}
	sort.Ints(ticks)
	return ticks
}
