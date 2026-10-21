package coordination

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// drainFirstRoll plays a roll in which every side recreates one Instance at
// a time through the serving band alone, the way a prefill/decode pair rolls
// where the batch step does not govern it. Each round asks for one drain of
// each side in order; an admitted drain's replacement serves before the next
// question, so serving is back at the reference between steps and only the
// new-revision counts move. The play stops when every side is on the new
// revision or a whole round admits nothing. It returns the admitted starts
// in order, the decision that admitted each side's first start, and the
// last refusal of each side.
func drainFirstRoll(pacing v1beta1.CoordinationPacing, state RatioState, order []v1beta1.ComponentType) (starts []string, first map[v1beta1.ComponentType]RatioDecision, holds map[v1beta1.ComponentType]string) {
	first = map[v1beta1.ComponentType]RatioDecision{}
	holds = map[v1beta1.ComponentType]string{}
	var total int32
	for _, n := range state.Original {
		total += n
	}
	for round := int32(0); round <= total; round++ {
		moved := false
		for _, c := range order {
			if state.NewPods[c] >= state.Original[c] {
				continue
			}
			d := EvaluateSurge(pacing, state, c, -1)
			if d.AllowedSurgeDelta == 0 {
				holds[c] = d.Reason
				continue
			}
			if _, seen := first[c]; !seen {
				first[c] = d
			}
			delete(holds, c)
			state.NewPods[c]++
			starts = append(starts, string(c))
			moved = true
		}
		if !moved || int32(len(starts)) == total {
			break
		}
	}
	return starts, first, holds
}

func ratioBalancedAt(tolerance int32) v1beta1.CoordinationPacing {
	return v1beta1.CoordinationPacing{Type: v1beta1.CoordinationPacingRatioBalanced, RatioTolerancePercent: ptr.To(tolerance)}
}

// balancedFleet is a group serving every Instance of the counts the spec
// asks for, nothing on the new revision yet, with the reference already
// following those counts.
func balancedFleet(counts map[v1beta1.ComponentType]int32) RatioState {
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{},
		Current:  map[v1beta1.ComponentType]int32{},
		Desired:  map[v1beta1.ComponentType]int32{},
		Serving:  map[v1beta1.ComponentType]int32{},
		NewPods:  map[v1beta1.ComponentType]int32{},
	}
	for c, n := range counts {
		state.Original[c], state.Current[c], state.Desired[c], state.Serving[c] = n, n, n, n
		state.NewPods[c] = 0
	}
	return state
}

// TestEvaluateSurge_DrainAdmitsTheOneStepWhenNoneFitsTheTolerance is a
// roll that would hold for ever: 8 prefill beside 4 decode, both recreating one
// Instance at a time under maintainRatio tolerance 20, the decode side
// scaled to 2 by its autoscaler through the scale subresource, which asks
// no admission. One decode pod out reads 8:1, 100 points off 8:2, past
// twice the tolerance, so the band refused every decode drain; the prefill
// side rolled two, led by 25 points, and the lockstep held it; both sides
// read rolloutHold.gate=Ratio for good. A hold with no way out is never
// the right answer: when no single step of a side fits the tolerance the
// band admits the one step, from a balanced fleet at its reference with
// nothing of that side out of rotation, and says so with the numbers. The
// lockstep still paces the sides against each other, so both finish.
//
// The same shape at 15 prefill beside 15 decode under tolerance 5 with one
// side scaled to 8 (one pod out of the smaller side reads 14 points off),
// and a side scaled to one, whose one drain leaves it serving nothing for
// the length of the recreate, the strategy the operator chose for it.
func TestEvaluateSurge_DrainAdmitsTheOneStepWhenNoneFitsTheTolerance(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	pictures := []struct {
		name       string
		tolerance  int32
		counts     map[v1beta1.ComponentType]int32
		side       v1beta1.ComponentType
		wantReason string
	}{
		{
			name: "8 prefill beside 4 decode, the decoder scaled to 2, tolerance 20", tolerance: 20,
			counts: map[v1beta1.ComponentType]int32{eng: 8, dec: 2}, side: dec,
			wantReason: "decoder 1 of 2 serving after the drain; no single step fits tolerance 20 at 8:2; admitting one",
		},
		{
			name: "15 prefill beside 15 decode, the decoder scaled to 8, tolerance 5", tolerance: 5,
			counts: map[v1beta1.ComponentType]int32{eng: 15, dec: 8}, side: dec,
			wantReason: "decoder 7 of 8 serving after the drain; no single step fits tolerance 5 at 15:8; admitting one",
		},
		{
			name: "15 prefill beside 15 decode, the prefill side scaled to 8, tolerance 5", tolerance: 5,
			counts: map[v1beta1.ComponentType]int32{eng: 8, dec: 15}, side: eng,
			wantReason: "engine 7 of 8 serving after the drain; no single step fits tolerance 5 at 8:15; admitting one",
		},
		{
			name: "6 prefill beside 2 decode, the decoder scaled to 1, tolerance 20", tolerance: 20,
			counts: map[v1beta1.ComponentType]int32{eng: 6, dec: 1}, side: dec,
			wantReason: "decoder 0 of 1 serving after the drain; no single step fits tolerance 20 at 6:1; admitting one",
		},
	}
	for _, p := range pictures {
		t.Run(p.name, func(t *testing.T) {
			total := p.counts[eng] + p.counts[dec]
			starts, first, holds := drainFirstRoll(ratioBalancedAt(p.tolerance), balancedFleet(p.counts), []v1beta1.ComponentType{eng, dec})
			if int32(len(starts)) != total {
				t.Fatalf("the roll stopped after %d of %d starts (%s); engine holds on %q, decoder holds on %q",
					len(starts), total, strings.Join(starts, ","), holds[eng], holds[dec])
			}
			d := first[p.side]
			if d.Reason != p.wantReason {
				t.Errorf("the %s side's first drain must be admitted with the numbers: got %q, want %q", p.side, d.Reason, p.wantReason)
			}
			if !d.OutsideTolerance {
				t.Errorf("the %s side's first drain leaves the tolerance and must be read so, for the event: %+v", p.side, d)
			}
			other := eng
			if p.side == eng {
				other = dec
			}
			if first[other].OutsideTolerance {
				t.Errorf("the %s side has a step inside the tolerance or within twice it and must not be read as outside: %+v", other, first[other])
			}
		})
	}
}

// TestEvaluateSurge_TheAdmittedStepIsOneAtATime holds the admitted step to
// the bounds the band's one-pod allowance has always had: one pod of the
// side out at a time, from a fleet serving its reference in band. 8 prefill
// beside 2 decode under tolerance 20, the decoder's first drain open.
func TestEvaluateSurge_TheAdmittedStepIsOneAtATime(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	pacing := ratioBalancedAt(20)
	counts := map[v1beta1.ComponentType]int32{eng: 8, dec: 2}

	open := balancedFleet(counts)
	open.Serving[dec] = 1
	open.InFlightUnavail = map[v1beta1.ComponentType]int32{dec: 1}
	if d := EvaluateSurge(pacing, open, dec, -1); d.AllowedSurgeDelta != 0 || !strings.Contains(d.Reason, "skew") {
		t.Errorf("a second decode drain while the first is open would empty the side; it must wait: %+v", d)
	}
	if d := EvaluateSurge(pacing, open, eng, -1); d.AllowedSurgeDelta != 0 || !strings.Contains(d.Reason, "skew") {
		t.Errorf("the prefill side waits for the decode replacement: 8:1 is out of band, so no balanced baseline admits its drain: %+v", d)
	}

	// The second pod of one pass: the caller nets its own drain out of the
	// baseline before the rows record it.
	samePass := balancedFleet(counts)
	samePass.Serving[dec] = 1
	if d := EvaluateSurge(pacing, samePass, dec, -1); d.AllowedSurgeDelta != 0 {
		t.Errorf("the second decode pod of one pass sees a baseline below the reference and must be refused: %+v", d)
	}

	// A side short of a pod with nothing in flight: the repair brings the
	// pod back; the roll does not take the other one down.
	short := balancedFleet(counts)
	short.Serving[dec] = 1
	short.Current[dec] = 2
	if d := EvaluateSurge(pacing, short, dec, -1); d.AllowedSurgeDelta != 0 {
		t.Errorf("a decode side serving 1 of 2 with no drain open is not at its reference; its drain must wait for the repair: %+v", d)
	}

	// The reference still at the old counts, one ISVC pass after the scale:
	// the fleet reads skewed against 8:4 and nothing is admitted until the
	// reference follows the spec.
	stale := balancedFleet(counts)
	stale.Original = map[v1beta1.ComponentType]int32{eng: 8, dec: 4}
	if d := EvaluateSurge(pacing, stale, dec, -1); d.AllowedSurgeDelta != 0 {
		t.Errorf("against a reference of 8:4 the fleet at 8:2 is out of band, so the one step waits for the reference to follow the spec: %+v", d)
	}
}

// TestEvaluateUpdateGate_AnnouncesTheStepOutsideTheTolerance asks the
// production decision site on the wiring that reaches the band:
// 8 prefill patched in place beside 2 decode recreated one at a time, one
// rollingUpdate group under maintainRatio tolerance 20, the reference at
// 8:2. Two prefill Instances are on the new revision, so the lockstep holds
// the prefill side (25 points ahead of 0 of 2). The decode side's first
// drain is admitted and the service gets a RatioStepOutsideTolerance event
// with the numbers; a second decode drain in the same pass is refused and
// raises nothing.
func TestEvaluateUpdateGate_AnnouncesTheStepOutsideTheTolerance(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	members := map[v1beta1.ComponentType]batchMemberFixture{
		eng: {desired: 8, step: "1", strategy: v1beta1.UpdateStrategyInPlaceIfPossible, promoted: 2},
		dec: {desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod},
	}
	isvc := batchService(batchGroupFixture{maxUnavailable: "1", tolerance: ptr.To(int32(20))}, members)
	c := batchClient(t, members)
	rec := record.NewFakeRecorder(10)
	ask := func(comp v1beta1.ComponentType, mechanism workloadtypes.UpdateStrategyType, inFlightUnavail int32) (bool, v1beta1.RolloutHoldGate, string) {
		return EvaluateUpdateGate(context.Background(), c, isvc, comp, "", forwardTarget(isvc, comp), rec, GroupDefaults{}, mechanism, 0, inFlightUnavail)
	}
	event := func() string {
		select {
		case e := <-rec.Events:
			return e
		default:
			return ""
		}
	}

	if allowed, gate, reason := ask(eng, workloadtypes.UpdateStrategyInPlaceIfPossible, 0); allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "outpace") {
		t.Fatalf("the prefill side at 2 of 8 beside 0 of 2 decode leads by 25 points and must wait: allowed=%v gate=%s reason=%q", allowed, gate, reason)
	}
	if e := event(); e != "" {
		t.Fatalf("a held start raises nothing: %q", e)
	}

	allowed, gate, reason := ask(dec, workloadtypes.UpdateStrategyRecreatePod, 0)
	if !allowed {
		t.Fatalf("the decode side's first drain is the one step the band admits when none fits the tolerance: gate=%s reason=%q", gate, reason)
	}
	want := fmt.Sprintf("%s: decoder 1 of 2 serving after the drain; no single step fits tolerance 20 at 8:2; admitting one", EventReasonRatioStepOutsideTolerance)
	if e := event(); !strings.Contains(e, want) {
		t.Fatalf("the service must get the event with the numbers: got %q, want it to contain %q", e, want)
	}

	if allowed, gate, reason := ask(dec, workloadtypes.UpdateStrategyRecreatePod, 1); allowed || gate != v1beta1.RolloutHoldGateRatio {
		t.Fatalf("a second decode drain in the same pass would empty the side and must be refused: allowed=%v gate=%s reason=%q", allowed, gate, reason)
	}
	if e := event(); e != "" {
		t.Fatalf("a refused drain raises nothing: %q", e)
	}
}

// TestCheckRatio_TheOneStepWaitsForTheReferenceToFollowTheSpec is the pass
// between the scale and the reference rewrite: the decoder of 8 prefill and
// 4 decode is scaled to 2, the fleet serves 8:2, and the group status still
// records 8:4. Against the stale reference the fleet reads out of band, so
// no step is admitted; once the ISVC pass rewrites the reference to 8:2 the
// decode side's drain is the one step the band admits. The ratio consult is
// asked directly, as on a pair the batch step does not govern.
func TestCheckRatio_TheOneStepWaitsForTheReferenceToFollowTheSpec(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	isvc := mkRatioPairFixture(20, map[v1beta1.ComponentType]int32{eng: 8, dec: 4}, map[v1beta1.ComponentType]v1beta1.LifecycleStatus{
		eng: {Replicas: 8, ServingReplicas: 8, UpdatedReplicas: 0},
		dec: {Replicas: 2, ServingReplicas: 2, UpdatedReplicas: 0},
	})
	client := fakeClientForISVCWithDesiredAndInstances(isvc, map[v1beta1.ComponentType]*int32{eng: int32Ptr(8), dec: int32Ptr(2)}, nil)

	if allowed, reason := ResolveGateContext(context.Background(), client, isvc, dec).CheckRatio(0, 0, -1); allowed || !strings.Contains(reason, "skew") {
		t.Fatalf("against the stale 8:4 the fleet at 8:2 is out of band and the decode drain waits: allowed=%v reason=%q", allowed, reason)
	}

	g := ratioGroup(eng, dec)
	obs, err := buildGroupObservation(context.Background(), client, isvc, g, map[v1beta1.ComponentType]map[string]int32{})
	if err != nil {
		t.Fatalf("buildGroupObservation: %v", err)
	}
	state := buildRatioState(isvc, g, obs, map[v1beta1.ComponentType]map[string]int32{})
	if state.Original[eng] != 8 || state.Original[dec] != 2 {
		t.Fatalf("the reference must follow the scale to 8:2; got %v", state.Original)
	}
	isvc.Status.RolloutCoordination.Groups[0].ObservedRatio.Original = state.Original

	allowed, reason := ResolveGateContext(context.Background(), client, isvc, dec).CheckRatio(0, 0, -1)
	if !allowed || reason != "decoder 1 of 2 serving after the drain; no single step fits tolerance 20 at 8:2; admitting one" {
		t.Fatalf("against 8:2 the decode side's first drain is the one step the band admits: allowed=%v reason=%q", allowed, reason)
	}
}
