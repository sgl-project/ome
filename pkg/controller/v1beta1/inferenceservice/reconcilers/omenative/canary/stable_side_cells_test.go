package canary

import (
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/rollout"
)

// The fixtures, events and readers of the grid rows about the stable side of
// a split: a stable pod retired by the staging or lost, a pod lost while the
// operator's pause holds the unit, a resume while the fleet already stands
// past the first step, and a push landing before the cutover. Whether a
// split's weight should move with the capacity that serves it is not decided
// yet; each row records the current behaviour and is updated when it changes.

func init() {
	for id, reader := range map[string]func(t *testing.T, isvc *v1beta1.InferenceService){
		"canary/Staging/stableGone":                                noSplitWritten,
		"canary/Staging[serving]/stableGone":                       splitStandsAtQuarter,
		"canary/Staging[serving]/stableGone[last]":                 splitAllOnCanary,
		"canary/Staging[final,serving]/stableGone":                 splitStandsAtHalf,
		"canary/Staging[final,serving]/stableGone[last]":           splitAllOnCanary,
		"canary/Staging[final,pd]/stableGone":                      splitKeeps,
		"canary/Staging[final,pd]/stableGone[last]":                engineAloneOnCanary,
		"canary/Serving/stableGone":                                splitStandsAtHalf,
		"canary/Serving/stableGone[last]":                          splitAllOnCanary,
		"canary/Serving[T]/stableGone":                             splitStandsAtHalf,
		"canary/Serving[T]/stableGone[last]":                       splitAllOnCanary,
		"canary/Serving[A]/stableGone":                             splitStandsAtHalf,
		"canary/Serving[A]/stableGone[last]":                       splitAllOnCanary,
		"canary/Serving[pd]/stableGone[last]":                      engineAloneOnCanary,
		"canary/Serving[pd]/stableGone[decoderLast]":               decoderAloneOnCanary,
		"canary/PreHold/stableGone":                                splitStandsAtHalf,
		"canary/PreHold/stableGone[last]":                          splitAllOnCanary,
		"canary/Draining[retiring]/stableGone":                     splitAllOnCanary,
		"canary/Draining/stableGone[last]":                         splitAllOnCanary,
		"canary/Draining[M,retiring]/stableGone":                   splitAllOnCanary,
		"canary/Draining[M]/stableGone[last]":                      splitAllOnCanary,
		"canary/RollingBack/stableGone":                            splitAllOnStable,
		"canary/RolledBack/stableGone":                             splitAllOnStable,
		"canary/Serving[paused]/capacityDropped":                   splitStandsAtHalfUnclocked,
		"canary/Serving[T,paused]/capacityDropped":                 splitStandsAtHalfUnclocked,
		"canary/Serving[A,paused]/capacityDropped":                 splitStandsAtHalfUnclocked,
		"canary/PreHold[paused]/capacityDropped":                   splitStandsAtHalfUnclocked,
		"canary/Staging[final,paused]/capacityDropped":             splitStandsAtHalf,
		"canary/Serving[pd,paused]/capacityDropped":                splitKeeps,
		"canary/Draining[paused]/capacityDropped":                  splitAllOnCanaryUnclocked,
		"canary/Draining[M,paused]/capacityDropped":                splitAllOnCanaryUnclocked,
		"canary/Failed[fleetAhead]/resume[nextPass]":               splitStandsAtHalf,
		"canary/Failed[stableMissing,fleetAhead]/resume[nextPass]": splitStandsAtHalf,
		"canary/Draining[pd]/newTarget":                            splitsOnTheHeldStable,
	} {
		cellAlso[id] = reader
	}
}

// pausedFixture holds a fixture under the operator's pause for one pass with
// every pod serving, so the split stands as the step wrote it.
func pausedFixture(fixture func(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs)) func(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
	return func(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
		isvc, in := fixture(t)
		evPause(isvc, &in)
		mustReconcile(t, isvc, in)
		return isvc, in
	}
}

var (
	fixtureServingManualPaused       = pausedFixture(fixtureServingManual)
	fixtureServingTimedPaused        = pausedFixture(fixtureServingTimed)
	fixtureServingAnalysisPaused     = pausedFixture(fixtureServingAnalysis)
	fixturePreHoldPaused             = pausedFixture(fixturePreHold)
	fixtureStagingFinalServingPaused = pausedFixture(fixtureStagingFinalServing)
	fixtureServingManualPDPaused     = pausedFixture(fixtureServingManualPD)
	fixtureDrainingPaused            = pausedFixture(fixtureDraining)
	fixtureDrainingHeldPaused        = pausedFixture(fixtureDrainingHeld)
)

// drainingRetiring walks the given final step to 100 percent with an old pod
// still serving beside the held Instance, as SurgeThenDrain keeps one until
// its replacement serves: three canary Instances meet the step and two pods
// still serve the stable revision.
func drainingRetiring(t *testing.T, steps []v1beta1.RolloutGroupStep, scaleDownDelay *int32) (*v1beta1.InferenceService, ReconcileInputs) {
	isvc := pinActiveRun(canaryISVC(steps, scaleDownDelay))
	in := baseInputs(isvc, map[string]int32{"new": 2, "old": 2})
	in.TargetID = "t1"
	mustReconcile(t, isvc, in) // the first split serves under its manual hold
	annotate(isvc, "ome.io/rollout-promote", "new")
	mustReconcile(t, isvc, in) // the promote advances to the final step
	delete(isvc.Annotations, "ome.io/rollout-promote")
	in.PerRevisionPods = map[string]int32{"new": 3, "old": 2}
	mustReconcile(t, isvc, in) // 100 percent on the canary revision; an old pod still retiring
	return isvc, in
}

func fixtureDrainingRetiring(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
	return drainingRetiring(t, twoStep(), i32(3600))
}

func fixtureDrainingHeldRetiring(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
	return drainingRetiring(t, twoStepHeldFinal(), nil)
}

// fixtureFailedFleetAhead is the final step stopped on the ready timeout of
// its capacity wait after the lost canary pod came back: three of four
// Instances serve the canary revision, more than the first step stages.
func fixtureFailedFleetAhead(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
	isvc, in := fixtureFailedAtTheRelease(t)
	in.PerRevisionPods = map[string]int32{"new": 3, "old": 1}
	return isvc, in
}

// fixtureFailedStableMissingFleetAhead is the stable-missing stop with three
// of four Instances on the rejected revision, more than the first step stages.
func fixtureFailedStableMissingFleetAhead(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs) {
	isvc, in := fixtureFailedStableMissing(t)
	in.PerRevisionPods = map[string]int32{"new": 3, "old": 1}
	return isvc, in
}

// A stable pod leaves rotation. The staging and serving fixtures run two
// canary pods beside two stable ones; the drain fixtures three beside the
// held one.
func evStablePodGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 2, "old": 1}
}
func evStablePodsGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 2}
}
func evStagingStablePodGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 0, "old": 3}
}
func evDrainStablePodGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 3, "old": 1}
}
func evDrainStablePodsGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 3}
}
func evRolledBackStablePodGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"old": 3}
}
func evRolledBackStablePodsGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{}
}

// The prefill/decode unit's stable pods, read off each member's own replica.
func evEngineStablePodsGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 2}
}
func evEngineStablePodsGoneAtTheFinalStep(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 3}
}
func evDecoderStablePodsGone(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	decoderServes(in, 2, 0, true)
}
func evDecoderStablePodGoneShort(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	decoderServes(in, 2, 1, false)
}

// evResumeOwnThenPass: the resume starts the unit over at step 0 and the pass
// under test is the first one the step machine runs from the new record.
func evResumeOwnThenPass(isvc *v1beta1.InferenceService, in *ReconcileInputs) {
	evResumeOwn(isvc, in)
	reconcileOnce(isvc, in)
}

// evNewTargetOverTheDrainPD: version 3 is pushed on both members while the
// drain holds one Instance per member on version 1 and the other three serve
// version 2; no pod of version 3 exists yet.
func evNewTargetOverTheDrainPD(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.CanaryRevisionHash, in.TargetID = "newer", "t2"
	in.Secondaries[v1beta1.DecoderComponent] = MemberRevisions{
		CanaryRevisionHash: "decnewer", StableRevisionHash: "decold",
		CanaryPairingProtocol: pdCanaryProtocol, StablePairingProtocol: pdStableProtocol,
		DesiredReplicas: 4,
	}
	in.SecondaryCapacityReady = false
}

func noSplitWritten(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	if traffic := isvc.Status.Components[v1beta1.EngineComponent].Traffic; len(traffic) != 0 {
		t.Errorf("step 0 has written no split yet, got %+v", traffic)
	}
}

// splitStandsAtHalf reads the manual step's 50/50 split standing.
func splitStandsAtHalf(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{
		"new": {percent: 50, latest: true},
		"old": {percent: 50},
	})
}

// splitStandsAtQuarter reads the widening step list's first split standing.
func splitStandsAtQuarter(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{
		"new": {percent: 25, latest: true},
		"old": {percent: 75},
	})
}

// splitAllOnCanary reads the whole split on the canary revision: a stable
// revision with no serving pod carries no weight.
func splitAllOnCanary(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{"new": {percent: 100, latest: true}})
}

func splitAllOnStable(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{"old": {percent: 100}})
}

func splitStandsAtHalfUnclocked(t *testing.T, isvc *v1beta1.InferenceService) {
	splitStandsAtHalf(t, isvc)
	waitCleared(t, isvc)
}

func splitAllOnCanaryUnclocked(t *testing.T, isvc *v1beta1.InferenceService) {
	splitAllOnCanary(t, isvc)
	waitCleared(t, isvc)
}

// engineAloneOnCanary reads the engine's split wholly on its canary revision
// while the decoder keeps the step's split on its own pods: the stable side
// of a split is read per member.
func engineAloneOnCanary(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{"new": {percent: 100, protocol: pdCanaryProtocol, latest: true}})
	expectTargets(t, isvc, v1beta1.DecoderComponent, map[string]trafficWant{
		"decnew": {percent: 50, protocol: pdCanaryProtocol, latest: true},
		"decold": {percent: 50, protocol: pdStableProtocol},
	})
}

func decoderAloneOnCanary(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{
		"new": {percent: 50, protocol: pdCanaryProtocol, latest: true},
		"old": {percent: 50, protocol: pdStableProtocol},
	})
	expectTargets(t, isvc, v1beta1.DecoderComponent, map[string]trafficWant{"decnew": {percent: 100, protocol: pdCanaryProtocol, latest: true}})
}

// splitsOnTheHeldStable reads both members' splits wholly on their stable
// revision after a push before the cutover: version 1 is still the stable
// identity, so each member's one version 1 pod carries everything while the
// three version 2 pods beside it carry nothing.
func splitsOnTheHeldStable(t *testing.T, isvc *v1beta1.InferenceService) {
	t.Helper()
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{"old": {percent: 100, protocol: pdStableProtocol}})
	expectTargets(t, isvc, v1beta1.DecoderComponent, map[string]trafficWant{"decold": {percent: 100, protocol: pdStableProtocol}})
}

// TestTheSplitStandsOnARevisionWithNoServingPod reads the cells where the
// executor leaves a weighted target on a revision none of the member's pods
// serve, which the grid's own pass check refuses: a step's split under the
// operator's pause with every canary pod lost, and a hold that does not
// rewrite its split when the last stable pod leaves. Whether the share should
// move is not decided yet; each case records the current behaviour and is
// updated when it changes.
func TestTheSplitStandsOnARevisionWithNoServingPod(t *testing.T) {
	cases := []struct {
		id      string
		fixture func(t *testing.T) (*v1beta1.InferenceService, ReconcileInputs)
		event   func(isvc *v1beta1.InferenceService, in *ReconcileInputs)
		state   rollout.CanaryState
		comp    v1beta1.ComponentType
		hash    string
		weight  int32
		why     string
	}{
		{"Serving[paused]/capacityDropped[allLost]", fixtureServingManualPaused, evCanaryPodsAllLost, rollout.CanaryStateServing,
			v1beta1.EngineComponent, "new", 50, "the paused pass writes nothing, so the step's share stays on the canary revision while no pod of it serves"},
		{"Serving[pd,paused]/capacityDropped[allLost]", fixtureServingManualPDPaused, evDecoderCapacityLost, rollout.CanaryStateServing,
			v1beta1.DecoderComponent, "decnew", 50, "the paused pass writes nothing, so the decoder's share stays on its canary revision while no pod of it serves"},
		{"Failed[release]/stableGone[last]", fixtureFailedAtTheRelease, evStablePodsGone, rollout.CanaryStateFailed,
			v1beta1.EngineComponent, "old", 100, "the stopped unit does not rewrite the split it returned to the stable revision when the last stable pod leaves"},
		{"RollingBack/stableGone[last]", fixtureRollingBack, evStablePodsGone, rollout.CanaryStateRollingBack,
			v1beta1.EngineComponent, "old", 100, "the revert moves traffic only onto capacity that serves and leaves the split as it stands while the stable revision has no pod"},
		{"RolledBack/stableGone[last]", fixtureRolledBack, evRolledBackStablePodsGone, rollout.CanaryStateRolledBack,
			v1beta1.EngineComponent, "old", 100, "the settled hold writes the stable split again only while a stable pod serves"},
		{"Failed[stableMissing]/stableGone[last]", fixtureFailedStableMissing, evStablePodsGone, rollout.CanaryStateFailed,
			v1beta1.EngineComponent, "old", 100, "the stopped unit neither drains nor rewrites the split the revert left"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			isvc, in := c.fixture(t)
			c.event(isvc, &in)
			mustReconcile(t, isvc, in)
			cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
			steps := len(isvc.Spec.Rollout.Groups[0].Canary.Steps)
			if got := rollout.StateOf(cs, phaseOf(isvc), steps); got != c.state {
				t.Fatalf("state %q, want %q", got, c.state)
			}
			serving := in.PerRevisionPods
			if c.comp != v1beta1.EngineComponent {
				serving = in.GroupReadyPerRevisionPods[c.comp]
			}
			if serving[c.hash] != 0 {
				t.Fatalf("the picture is wrong: %s must serve no pod of %q, got %v", c.comp, c.hash, serving)
			}
			if got := weightOn(isvc, c.comp, c.hash); got != c.weight {
				t.Fatalf("%s sends %d percent to %q, which has no serving pod; the current behaviour is %d: %s; update this case when it changes", c.comp, got, c.hash, c.weight, c.why)
			}
		})
	}
}

// evCanaryPodsAllLost: every canary pod of the step is lost at once while the
// stable pods serve.
func evCanaryPodsAllLost(_ *v1beta1.InferenceService, in *ReconcileInputs) {
	in.PerRevisionPods = map[string]int32{"new": 0, "old": 2}
}
