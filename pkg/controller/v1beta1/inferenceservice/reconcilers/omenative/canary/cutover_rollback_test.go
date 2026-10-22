package canary

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/rollout"
)

// A manual canary of the engine in a prefill/decode unit of 8 engines and 8
// decoders walks a four-step ladder with a manual gate at every step but
// the cutover. With no scale-down delay the final step shifts 100% on one
// pass and the next pass sets the done sentinel, which releases the last
// stable engine to roll. The operator who asks for a rollback at that
// moment sees the unit Promoting with every engine but one on the new
// version: the cutover has not landed, so the rollback is honored. Every
// engine returns to the stable revision, the rejected revision is recorded
// and the unit holds RolledBack, whether the released engine restarts in
// place or still serves, and whether or not a canary pod was killed once
// and restarted on the way.

// fourStepManual is the ladder of the story: three manual gates, then an
// ungated final step at full traffic, with no scale-down delay.
func fourStepManual() []v1beta1.RolloutGroupStep {
	return []v1beta1.RolloutGroupStep{
		{Capacity: intstr.FromString("10%"), Traffic: 10, Pause: &v1beta1.RolloutPause{}},
		{Capacity: intstr.FromString("30%"), Traffic: 30, Pause: &v1beta1.RolloutPause{}},
		{Capacity: intstr.FromString("60%"), Traffic: 60, Pause: &v1beta1.RolloutPause{}},
		{Capacity: intstr.FromString("100%"), Traffic: 100},
	}
}

// cutoverUnit is the unit of the story: the engine is pushed from "old" to
// "new" and the decoder is a member on its stable revision throughout.
type cutoverUnit struct {
	isvc *v1beta1.InferenceService
	in   ReconcileInputs
	rec  *record.FakeRecorder
}

func newCutoverUnit(t *testing.T) *cutoverUnit {
	t.Helper()
	isvc := canaryISVC(fourStepManual(), nil)
	isvc.Spec.Rollout.Groups[0].Components = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}
	runWithTargets(isvc,
		v1beta1.RolloutRunTarget{Component: v1beta1.EngineComponent, Revision: "new", StableRevision: "old"},
		v1beta1.RolloutRunTarget{Component: v1beta1.DecoderComponent, Revision: "decold", StableRevision: "decold"},
	)
	u := &cutoverUnit{isvc: isvc, in: baseInputs(isvc, nil), rec: record.NewFakeRecorder(64)}
	u.in.Recorder = u.rec
	u.in.TargetID = "t1"
	u.in.DesiredReplicas = 8
	u.in.Secondaries = map[v1beta1.ComponentType]MemberRevisions{
		v1beta1.DecoderComponent: {CanaryRevisionHash: "decold", StableRevisionHash: "decold", ReadyCanaryCapacity: 8, DesiredReplicas: 8},
	}
	u.in.GroupStableRevisionHashes = map[v1beta1.ComponentType]string{v1beta1.EngineComponent: "old", v1beta1.DecoderComponent: "decold"}
	u.engines(map[string]int32{"old": 8}, nil)
	return u
}

// engines sets the engine's ready pods per revision and, in the group
// view, every engine pod per revision, ready or not; nil total means every
// engine pod is ready.
func (u *cutoverUnit) engines(ready, total map[string]int32) {
	if total == nil {
		total = ready
	}
	u.in.PerRevisionPods = ready
	u.in.GroupReadyPerRevisionPods = map[v1beta1.ComponentType]map[string]int32{
		v1beta1.EngineComponent:  ready,
		v1beta1.DecoderComponent: {"decold": 8},
	}
	u.in.GroupTotalPerRevisionPods = map[v1beta1.ComponentType]map[string]int32{
		v1beta1.EngineComponent:  total,
		v1beta1.DecoderComponent: {"decold": 8},
	}
}

func (u *cutoverUnit) pass(t *testing.T) *Result {
	t.Helper()
	return mustReconcile(t, u.isvc, u.in)
}

// promote opens the step's manual gate the way the controller applies the
// verb: the pass takes it and the controller removes it after the flush.
func (u *cutoverUnit) promote(t *testing.T) {
	t.Helper()
	annotate(u.isvc, constants.RolloutPromoteAnnotation, "new")
	u.pass(t)
	delete(u.isvc.Annotations, constants.RolloutPromoteAnnotation)
}

// recordString renders the fields of a record a failure message needs
// without formatting its nil time pointers.
func recordString(cs *v1beta1.CanaryStatus) string {
	if cs == nil {
		return "<nil>"
	}
	failed := ""
	if cs.Failed != nil {
		failed = string(cs.Failed.Reason)
	}
	return fmt.Sprintf("{step=%d canary=%s stable=%s rolledBack=%s weight=%d failed=%q}",
		cs.CurrentStep, cs.CanaryRevisionHash, cs.StableRevisionHash, cs.RolledBackRevisionHash, cs.ObservedTrafficWeight, failed)
}

func (u *cutoverUnit) record() *v1beta1.CanaryStatus {
	return rollout.CanaryStatusFor(&u.isvc.Status, v1beta1.EngineComponent)
}

func (u *cutoverUnit) state() rollout.CanaryState {
	return rollout.StateOf(u.record(), phaseOf(u.isvc), len(fourStepManual()))
}

// projectedPartition is the partition the controller projects onto the
// engine's replica from the record as it stands.
func (u *cutoverUnit) projectedPartition(t *testing.T) int32 {
	t.Helper()
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: intPtr(8)}
	p := StepPartition(u.isvc, rollout.Policies{}, v1beta1.EngineComponent, DesiredReplicas(nil, ext))
	if p == nil {
		t.Fatal("the engine is a member of the canary group, so a partition is projected")
	}
	return *p
}

// expectServing asserts the step's split serves under its manual gate.
func (u *cutoverUnit) expectServing(t *testing.T, step int32, weight int32) {
	t.Helper()
	if u.state() != rollout.CanaryStateServing || u.record().CurrentStep != step || u.record().ObservedTrafficWeight != weight {
		t.Fatalf("step %d must serve %d%% under its gate, got state=%s record=%s", step, weight, u.state(), recordString(u.record()))
	}
}

// walkToTheCutover drives the ladder through its three manual gates and
// the 100% shift onto seven Ready engines, to the pass that sets the done
// sentinel: the unit is Promoting at partition 0 with the eighth engine
// released to roll, and the cutover has not landed.
func walkToTheCutover(t *testing.T) *cutoverUnit {
	t.Helper()
	u := newCutoverUnit(t)
	for i, stage := range []struct {
		newReady int32
		weight   int32
	}{{1, 10}, {3, 30}, {5, 60}} {
		u.engines(map[string]int32{"new": stage.newReady, "old": 8 - stage.newReady}, nil)
		u.pass(t)
		u.expectServing(t, int32(i), stage.weight)
		u.promote(t)
	}
	// The final step stages all but the held engine: seven Ready on the
	// canary revision and the eighth still serving the stable one. The pass
	// that finds them shifts 100% and opens the drain.
	u.engines(map[string]int32{"new": 7, "old": 1}, nil)
	res := u.pass(t)
	if u.state() != rollout.CanaryStateDraining || phaseOf(u.isvc) != v1beta1.RolloutPhasePromoting || res.Partition != 1 {
		t.Fatalf("the final step must shift 100%% and hold the last stable engine, got state=%s phase=%s res=%+v", u.state(), phaseOf(u.isvc), res)
	}
	expectSoleTarget(t, u.isvc, v1beta1.EngineComponent, "new")
	// No scale-down delay: the next pass sets the sentinel and releases the
	// held engine, which starts rolling. Nothing has landed yet.
	res = u.pass(t)
	cs := u.record()
	if int(cs.CurrentStep) != len(fourStepManual()) || phaseOf(u.isvc) != v1beta1.RolloutPhasePromoting || !res.Active || res.Complete {
		t.Fatalf("the sentinel must be set with the unit still Promoting, got record=%s phase=%s res=%+v", recordString(cs), phaseOf(u.isvc), res)
	}
	if u.projectedPartition(t) != 0 {
		t.Fatalf("the sentinel releases the held engine at partition 0, got %d", u.projectedPartition(t))
	}
	if cs.StableRevisionHash != "old" {
		t.Fatalf("the pre-canary identity is kept until the cutover lands, got %q", cs.StableRevisionHash)
	}
	eventsFrom(u.rec)
	return u
}

// requestRollback is the operator's rollback at the cutover: the request
// is read on the next pass.
func (u *cutoverUnit) requestRollback(t *testing.T) *Result {
	t.Helper()
	annotate(u.isvc, constants.RolloutRollbackAnnotation, "true")
	return u.pass(t)
}

// expectRevertStarted asserts the pass that takes the rollback at the
// cutover: the rejected revision is recorded on the record and its alias,
// the unit reads RollingBack, the controller is told to point every
// replica at its stable revision at partition 0, the record keeps the
// sentinel, and the request is left in place behind the recorded rejection.
// It returns the events the pass recorded.
func (u *cutoverUnit) expectRevertStarted(t *testing.T, res *Result) []string {
	t.Helper()
	cs := u.record()
	if cs.RolledBackRevisionHash != "new" || u.isvc.Status.Canary == nil || u.isvc.Status.Canary.RolledBackRevisionHash != "new" {
		t.Fatalf("the rollback at the cutover must record the rejected revision, got record=%s alias=%s", recordString(cs), recordString(u.isvc.Status.Canary))
	}
	if u.state() != rollout.CanaryStateRollingBack || phaseOf(u.isvc) != v1beta1.RolloutPhaseRollingBack {
		t.Fatalf("the unit must read RollingBack, got state=%s phase=%s", u.state(), phaseOf(u.isvc))
	}
	if !res.Active || !res.RolledBack || res.Partition != 0 {
		t.Fatalf("the pass must signal the revert at partition 0, got %+v", res)
	}
	if res.RequeueAfter == 0 && !res.Requeue {
		t.Fatalf("a revert in flight must ask to come back, got %+v", res)
	}
	if int(cs.CurrentStep) != len(fourStepManual()) {
		t.Fatalf("the record keeps the sentinel beside the rejection, got step %d", cs.CurrentStep)
	}
	if len(res.Consume) != 0 || u.isvc.Annotations[constants.RolloutRollbackAnnotation] != "true" {
		t.Fatalf("the recorded rejection keeps the request inert; it is not removed by the pass, got consume=%v annotations=%v", res.Consume, u.isvc.Annotations)
	}
	if u.projectedPartition(t) != 0 {
		t.Fatalf("a revert projects partition 0, got %d", u.projectedPartition(t))
	}
	if cs.Failed != nil {
		t.Fatalf("the operator's rollback is not a park, got %s", cs.Failed.Reason)
	}
	events := eventsFrom(u.rec)
	if hasEvent(events, EventReasonCanaryRollbackIgnored) {
		t.Fatalf("a rollback before the cutover lands is not ignored, got %v", events)
	}
	return events
}

// emptyStableWarnings is the subset of events that warn of a rollback
// toward a stable revision no pod runs.
func emptyStableWarnings(events []string) []string {
	var out []string
	for _, e := range events {
		if strings.Contains(e, EventReasonCanaryStableRevisionEmpty) {
			out = append(out, e)
		}
	}
	return out
}

// revertEngines returns the engines to the stable revision one at a time,
// as an in-place revert under maxUnavailable 1 does, and asserts the unit
// stays RollingBack with traffic on the stable revision until the last one
// is back, then reads RolledBack and records the stable revision as the one
// that owns the engine's traffic.
func (u *cutoverUnit) revertEngines(t *testing.T) {
	t.Helper()
	for back := int32(1); back < 8; back++ {
		u.engines(map[string]int32{"new": 8 - back, "old": back}, nil)
		res := u.pass(t)
		if u.state() != rollout.CanaryStateRollingBack || !res.RolledBack {
			t.Fatalf("%d of 8 engines back: the revert is still in flight, got state=%s res=%+v", back, u.state(), res)
		}
		expectSoleTarget(t, u.isvc, v1beta1.EngineComponent, "old")
	}
	u.engines(map[string]int32{"old": 8}, nil)
	res := u.pass(t)
	if u.state() != rollout.CanaryStateRolledBack || phaseOf(u.isvc) != v1beta1.RolloutPhaseRolledBack || !res.RolledBack || !res.Active {
		t.Fatalf("every engine back on the stable revision must read RolledBack, got state=%s phase=%s res=%+v", u.state(), phaseOf(u.isvc), res)
	}
	if got := query.RevisionFromName(u.isvc.Status.Components[v1beta1.EngineComponent].LatestRolledoutRevision).Hash(); got != "old" {
		t.Fatalf("the completed revert records the stable revision as rolled out, got %q", got)
	}
	expectSoleTarget(t, u.isvc, v1beta1.EngineComponent, "old")
	expectSoleTarget(t, u.isvc, v1beta1.DecoderComponent, "decold")
	if u.record().RolledBackRevisionHash != "new" || u.isvc.Status.Canary.RolledBackRevisionHash != "new" {
		t.Fatalf("the hold keeps the rejected revision on record, got %s", recordString(u.record()))
	}
	// The hold stands on later passes, and once the run that presented
	// the rejected revision has closed.
	u.in.RunActive = false
	if res := u.pass(t); u.state() != rollout.CanaryStateRolledBack || !res.RolledBack {
		t.Fatalf("the settled hold stands without a run, got state=%s res=%+v", u.state(), res)
	}
}

func TestReconcile_ManualRollbackAtTheCutoverReturnsEveryEngine(t *testing.T) {
	for _, tc := range []struct {
		name string
		// ready and total are the engine's pods at the moment of the
		// request: ready per revision, and every pod per revision.
		ready, total map[string]int32
		// restartedOnce: a canary pod was killed once after it was Ready and
		// the kubelet brought it back before the request.
		restartedOnce bool
		// stableServes: a stable engine serves when the request lands, so
		// the traffic moves onto it on the same pass.
		stableServes bool
		// warns: no pod of the stable revision exists, so the rollback warns
		// that it rebuilds the engines onto a revision none of them runs.
		warns bool
	}{
		{name: "the released engine restarts in place, its pod already on the canary revision", ready: map[string]int32{"new": 7}, total: map[string]int32{"new": 8}, warns: true},
		{name: "the released engine restarts in place, its pod still on the stable revision", ready: map[string]int32{"new": 7}, total: map[string]int32{"new": 7, "old": 1}},
		{name: "the held engine still serves at the sentinel", ready: map[string]int32{"new": 7, "old": 1}, stableServes: true},
		{name: "a canary pod was killed once and restarted on the way", ready: map[string]int32{"new": 7}, total: map[string]int32{"new": 8}, restartedOnce: true, warns: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := walkToTheCutover(t)
			if tc.restartedOnce {
				u.in.CanaryRestartedAt = u.in.Now
				u.in.Now = u.in.Now.Add(time.Minute)
			}
			u.engines(tc.ready, tc.total)
			u.pass(t) // the sentinel is re-read on the pods as they stand
			if phaseOf(u.isvc) != v1beta1.RolloutPhasePromoting {
				t.Fatalf("the cutover has not landed, got %s", phaseOf(u.isvc))
			}
			eventsFrom(u.rec)
			res := u.requestRollback(t)
			events := u.expectRevertStarted(t, res)
			if tc.stableServes {
				expectSoleTarget(t, u.isvc, v1beta1.EngineComponent, "old")
			} else {
				// No stable engine serves yet: the traffic stays where it
				// can be carried until the revert brings one back.
				expectSoleTarget(t, u.isvc, v1beta1.EngineComponent, "new")
			}
			if got := emptyStableWarnings(events); (len(got) == 1) != tc.warns {
				t.Fatalf("warn=%v about an empty stable revision, got %v", tc.warns, got)
			}

			// A pause during the revert keeps the rollback signal, so the
			// controller neither arms nor clears the replica's revert.
			annotate(u.isvc, constants.PausedRolloutAnnotation, "true")
			if res := u.pass(t); !res.Active || !res.RolledBack || u.state() != rollout.CanaryStateRollingBack {
				t.Fatalf("a paused revert keeps its signal, got state=%s res=%+v", u.state(), res)
			}
			delete(u.isvc.Annotations, constants.PausedRolloutAnnotation)

			u.revertEngines(t)
		})
	}
}

// A re-shown request after the revert began changes nothing: the recorded
// rejection is the record that the request was applied.
func TestReconcile_RollbackAtTheCutoverIsAppliedOnce(t *testing.T) {
	u := walkToTheCutover(t)
	u.engines(map[string]int32{"new": 7}, map[string]int32{"new": 8})
	u.pass(t)
	eventsFrom(u.rec)
	if got := emptyStableWarnings(u.expectRevertStarted(t, u.requestRollback(t))); len(got) != 1 {
		t.Fatalf("the request toward an empty stable revision warns once, got %v", got)
	}
	for pass := 0; pass < 3; pass++ {
		res := u.pass(t)
		if u.state() != rollout.CanaryStateRollingBack || !res.RolledBack || len(res.Consume) != 0 {
			t.Fatalf("pass %d: the re-shown request is inert behind the recorded rejection, got state=%s res=%+v", pass, u.state(), res)
		}
		if got := emptyTargetEvents(u.rec); len(got) != 0 {
			t.Fatalf("pass %d: the warning is recorded once, at the request, got %v", pass, got)
		}
	}
}

// Once the cutover has landed the unit reads Stable with its stable
// identity dropped: there is no revision to return to, and the request is
// removed with the warning that says so.
func TestReconcile_RollbackAfterTheCutoverLandedIsRefused(t *testing.T) {
	u := walkToTheCutover(t)
	u.engines(map[string]int32{"new": 8}, nil)
	if res := u.pass(t); !res.Complete || phaseOf(u.isvc) != v1beta1.RolloutPhaseStable {
		t.Fatalf("every engine on the canary revision lands the cutover, got phase=%s res=%+v", phaseOf(u.isvc), res)
	}
	eventsFrom(u.rec)
	res := u.requestRollback(t)
	if res.RolledBack || u.record().RolledBackRevisionHash != "" || phaseOf(u.isvc) != v1beta1.RolloutPhaseStable {
		t.Fatalf("a landed cutover has no canary to roll back, got record=%s res=%+v", recordString(u.record()), res)
	}
	if want := []string{constants.RolloutRollbackAnnotation}; fmt.Sprint(res.Consume) != fmt.Sprint(want) {
		t.Fatalf("the request is removed, got consume=%v", res.Consume)
	}
	if events := eventsFrom(u.rec); !hasEvent(events, EventReasonCanaryRollbackIgnored) {
		t.Fatalf("the refusal is said out loud, got %v", events)
	}
}
