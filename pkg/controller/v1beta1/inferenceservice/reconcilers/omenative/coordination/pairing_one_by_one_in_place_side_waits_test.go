package coordination

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// A service of one prefill patched in place beside one decoder that
// surges, with no side named first, changes its pairing protocol twice:
// pair-v1 to pair-v2, and to pair-v3 before the prefill has moved. Whenever
// both sides rest on the same old protocol and nothing serves the new one,
// both sides' next starts look alike to the pairing gate: each would leave
// no pair for a while. They are not alike. The decoder's surge keeps its old
// pod in rotation until a prefill serves the new protocol, so the pair on
// the old protocol serves through the whole change. The prefill's patch
// takes its only pod out of rotation at once, so a prefill on the new
// protocol beside a decoder still on the old one leaves no request with a
// pair until the decoder's replacement serves (44 seconds with a slow
// image). The side whose move keeps the pair goes first, whichever side's
// pass runs first: the prefill's patch waits under the Pairing gate until a
// decoder serves the protocol the patch moves to, and runs beside the kept
// old pod once one does.
//
// The walk, picture by picture: on the settled pair at the first change
// the decoder's surge is admitted and the prefill's patch waits; the
// pair-v2 replacement serves beside the kept pod and the patch to pair-v2
// is admitted; the second change lands first, so the patch, now to pair-v3,
// waits, the old pod stays, and the pair-v2 replacement, which pairs with
// nothing, may leave; on the settled pair again the decoder's surge is
// admitted and the patch waits, whether the prefill's pass runs before the
// decoder's or after it while the replacement is still coming up; the
// pair-v3 replacement serves beside the kept pod and the patch is admitted
// while the old pod still stays; the prefill serves pair-v3 and the old pod
// may leave. A side named first is ordered by the batch step, and that
// reading does not move.
func TestPairingGates_SecondChangeOnOneByOneWithNoSideNamedFirst_PrefillWaitsForTheDecoderToSurge(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	inPlace, surge := v1beta1.UpdateStrategyInPlaceIfPossible, v1beta1.UpdateStrategySurgeThenDrain
	patch, surgeStart := workloadtypes.UpdateStrategyInPlaceIfPossible, workloadtypes.UpdateStrategySurgeThenDrain
	oldPod, replacement := "llama-decoder-0-default-0", "llama-decoder-0-default-1"
	noSideFirst := GroupDefaults{}
	chart := GroupDefaults{RatioTolerancePercent: ptr.To(int32(5)), PrefillDecode: PrefillDecodeRules{KeepRatio: true, FirstSide: dec}}

	// decoder is the surging side's replica: one Instance, maxSurge 1, none
	// allowed out of pairing, its row as given.
	decoder := func(row v1beta1.OMENativeInstanceStatus) *v1beta1.InferenceReplica {
		ir := rolledPairingIR(dec, surge, 1, 0, "v1")
		ir.Spec.Lifecycle.UpdateStrategy.RollingUpdate.MaxSurge = ptr.To(intstr.FromInt(1))
		ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{row}
		return ir
	}
	// prefill is the in-place side's replica: one Instance serving the
	// given revision, maxUnavailable 1.
	prefill := func(hash string) *v1beta1.InferenceReplica {
		return rolledPairingIR(eng, inPlace, 1, 1, hash)
	}
	// pods are the serving prefill pod, the old decode pod on pair-v1 and,
	// when named, the decoder's replacement: serving, or still coming up.
	pods := func(prefillProto, replacementProto string, replacementServes bool) []client.Object {
		out := []client.Object{
			drainPod("llama", eng, "llama-engine-0-default-0", "default", prefillProto, true),
			drainPod("llama", dec, oldPod, "default", "pair-v1", true),
		}
		if replacementProto != "" {
			out = append(out, drainPod("llama", dec, replacement, "default", replacementProto, replacementServes))
		}
		return out
	}
	picture := func(isvc *v1beta1.InferenceService, prefillHash, prefillProto string, row v1beta1.OMENativeInstanceStatus, replacementProto string, replacementServes bool) []client.Object {
		objs := append(threeCohortCRs(isvc.Spec.RolloutPairingProtocol()), prefill(prefillHash), decoder(row))
		return append(objs, pods(prefillProto, replacementProto, replacementServes)...)
	}
	// afterPatch is the picture the pairing gate reads once the prefill's
	// patch has run on a decoder row still on pair-v1: rows and revisions,
	// which are all the start-time gate reads.
	afterPatch := func(isvc *v1beta1.InferenceService, row v1beta1.OMENativeInstanceStatus) []client.Object {
		return append(threeCohortCRs(isvc.Spec.RolloutPairingProtocol()), prefill("target"), decoder(row))
	}
	settled := settledRow("llama", dec, 0, "v1")
	// requirePatchWaitsForTheSurge holds the prefill's patch to the Pairing
	// gate's wait for the decoder's surge. Were the patch admitted, the
	// picture after it says what the service would look like.
	requirePatchWaitsForTheSurge := func(isvc *v1beta1.InferenceService, objs []client.Object, after []client.Object, why string) {
		t.Helper()
		allowed, gate, reason := decoderFirstStart(t, isvc, noSideFirst, eng, patch, objs...)
		if allowed {
			_, afterReason := pairingGate(t, isvc, dec, after...).CheckPairing("llama-decoder-v1", surgeStart, 0, 0)
			t.Errorf("%s: the prefill's patch was admitted; after it the pairing gate reads the service as: %q", why, afterReason)
			return
		}
		if gate != v1beta1.RolloutHoldGatePairing || !strings.Contains(reason, "would leave no pairable serving engine+decoder pair") || !strings.Contains(reason, "the decoder surges") {
			t.Fatalf("%s: the patch must wait under the Pairing gate for the decoder's surge: gate=%q reason=%q", why, gate, reason)
		}
	}

	// The first change, pair-v1 to pair-v2, on the settled pair: the
	// decoder's surge is admitted, the prefill's patch waits. A prefill that
	// flipped first would speak pair-v2 beside a decoder on pair-v1.
	first := pairingISVC("pair-v2")
	objs := picture(first, "v1", "pair-v1", settled, "", false)
	if allowed, gate, reason := decoderFirstStart(t, first, noSideFirst, dec, surgeStart, objs...); !allowed {
		t.Fatalf("first change: the decoder's surge keeps its pair-v1 pod serving until its replacement pairs, so it may open; held under %s: %s", gate, reason)
	}
	requirePatchWaitsForTheSurge(first, objs, afterPatch(first, settled), "first change, the prefill's pass first: no decoder serves pair-v2")

	// The pair-v2 replacement serves beside the kept pod: the patch to
	// pair-v2 may run, and the old pod stays until a prefill serves pair-v2.
	replacementOnV2 := surgingRow("llama", dec, 0, "v1", "v2", workloadtypes.UpdateStepSurge, 2)
	objs = picture(first, "v1", "pair-v1", replacementOnV2, "pair-v2", true)
	if allowed, gate, reason := decoderFirstStart(t, first, noSideFirst, eng, patch, objs...); !allowed {
		t.Fatalf("first change: a decoder serves pair-v2, so the prefill's patch to pair-v2 may run beside the kept pod; held under %s: %s", gate, reason)
	}
	allowed, gate, reason := drainGate(t, first, dec, []string{oldPod}, objs...)
	requireHeld(t, allowed, gate, reason, "first change: no prefill serves pair-v2, so the old decode pod stays")

	// The second change lands before the patch ran: the patch, now to
	// pair-v3, waits, the old pod stays, and the pair-v2 replacement may
	// leave.
	second := pairingISVC("pair-v3")
	objs = picture(second, "v1", "pair-v1", replacementOnV2, "pair-v2", true)
	allowed, gate, reason = decoderFirstStart(t, second, noSideFirst, eng, patch, objs...)
	if allowed || gate != v1beta1.RolloutHoldGatePairing || !strings.Contains(reason, "would leave no pairable serving engine+decoder pair") {
		t.Fatalf("second change: no decoder serves pair-v3, so the prefill's patch to pair-v3 must wait: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}
	allowed, gate, reason = drainGate(t, second, dec, []string{oldPod}, objs...)
	requireHeld(t, allowed, gate, reason, "second change: the replacement on pair-v2 has no partner, so the old decode pod stays")
	if allowed, _, reason := drainGate(t, second, dec, []string{replacement}, objs...); !allowed {
		t.Fatalf("second change: the pair-v2 replacement pairs with nothing and the pair-v1 pods keep the only pair, so the surge may remove it: %s", reason)
	}

	// The replacement is gone and both sides rest on pair-v1 with the
	// target on pair-v3. The prefill's pass runs first: its patch waits. The
	// decoder's surge is admitted.
	objs = picture(second, "v1", "pair-v1", settled, "", false)
	requirePatchWaitsForTheSurge(second, objs, afterPatch(second, settled), "second change, the prefill's pass first: no decoder serves pair-v3")
	if allowed, gate, reason := decoderFirstStart(t, second, noSideFirst, dec, surgeStart, objs...); !allowed {
		t.Fatalf("second change: the decoder's surge toward pair-v3 keeps the pair-v1 pod serving until its replacement pairs, so it may open; held under %s: %s", gate, reason)
	}
	// The decoder's pass runs first: its surge is open and its replacement
	// is still coming up when the prefill's pass runs. The patch waits.
	surgeOpen := surgingRow("llama", dec, 0, "v1", "target", workloadtypes.UpdateStepSurge, 1)
	objs = picture(second, "v1", "pair-v1", surgeOpen, "pair-v3", false)
	requirePatchWaitsForTheSurge(second, objs, afterPatch(second, surgeOpen), "second change, the decoder's pass first and its replacement not yet serving")
	// With the decoder named first the batch step orders the sides, and the
	// prefill's wait on the settled pair stays the batch step's.
	objs = picture(second, "v1", "pair-v1", settled, "", false)
	if allowed, gate, reason := decoderFirstStart(t, second, chart, eng, patch, objs...); allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "waits for decoder batch 1 to serve first") {
		t.Fatalf("second change with the decoder named first: the prefill's patch waits for the decoder's first batch under the Ratio gate: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}

	// The pair-v3 replacement serves beside the kept pod: the patch may
	// run, and the old pod stays until the prefill serves pair-v3.
	replacementOnV3 := surgingRow("llama", dec, 0, "v1", "target", workloadtypes.UpdateStepSurge, 2)
	objs = picture(second, "v1", "pair-v1", replacementOnV3, "pair-v3", true)
	if allowed, gate, reason := decoderFirstStart(t, second, noSideFirst, eng, patch, objs...); !allowed {
		t.Fatalf("second change: a decoder serves pair-v3, so the prefill's patch to pair-v3 may run beside the kept pod; held under %s: %s", gate, reason)
	}
	allowed, gate, reason = drainGate(t, second, dec, []string{oldPod}, objs...)
	requireHeld(t, allowed, gate, reason, "second change: no prefill serves pair-v3 yet, so the old decode pod stays")
	if !strings.Contains(reason, `"pair-v3"`) {
		t.Errorf("the hold must name pair-v3 as the protocol no prefill serves: %s", reason)
	}

	// The prefill serves pair-v3: the old pod may leave.
	objs = picture(second, "target", "pair-v3", replacementOnV3, "pair-v3", true)
	if allowed, _, reason := drainGate(t, second, dec, []string{oldPod}, objs...); !allowed {
		t.Fatalf("second change: the prefill serves pair-v3, so the replacement pairs and the old decode pod may leave: %s", reason)
	}
}

// The mirror: one prefill that surges beside one decoder patched in place,
// with no side named first. The decoder's patch waits for a prefill that
// serves the new protocol, and the prefill's surge is admitted. Two sides
// that both take their pod out of rotation at once have no such order:
// either side's first move is admitted, as before.
func TestPairingGates_OneByOneWithNoSideNamedFirst_TheSideThatKeepsItsPodMovesFirst(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	isvc := pairingISVC("pair-v2")
	noSideFirst := GroupDefaults{}
	onPairV1 := []client.Object{
		drainPod("llama", eng, "llama-engine-0-default-0", "default", "pair-v1", true),
		drainPod("llama", dec, "llama-decoder-0-default-0", "default", "pair-v1", true),
	}

	surgingPrefill := rolledPairingIR(eng, v1beta1.UpdateStrategySurgeThenDrain, 1, 0, "v1")
	surgingPrefill.Spec.Lifecycle.UpdateStrategy.RollingUpdate.MaxSurge = ptr.To(intstr.FromInt(1))
	mirror := append(append(threeCohortCRs("pair-v2"), surgingPrefill, rolledPairingIR(dec, v1beta1.UpdateStrategyInPlaceIfPossible, 1, 1, "v1")), onPairV1...)
	if allowed, gate, reason := decoderFirstStart(t, isvc, noSideFirst, dec, workloadtypes.UpdateStrategyInPlaceIfPossible, mirror...); allowed || gate != v1beta1.RolloutHoldGatePairing || !strings.Contains(reason, "the engine surges") {
		t.Fatalf("the decoder's patch must wait for the prefill's surge: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}
	if allowed, gate, reason := decoderFirstStart(t, isvc, noSideFirst, eng, workloadtypes.UpdateStrategySurgeThenDrain, mirror...); !allowed {
		t.Fatalf("the prefill's surge keeps its pod serving until its replacement pairs, so it may open; held under %s: %s", gate, reason)
	}

	bothInPlace := append(threeCohortCRs("pair-v2"),
		rolledPairingIR(eng, v1beta1.UpdateStrategyInPlaceIfPossible, 1, 1, "v1"),
		rolledPairingIR(dec, v1beta1.UpdateStrategyInPlaceIfPossible, 1, 1, "v1"))
	for _, comp := range pairingComponents {
		gate := pairingGate(t, isvc, comp, bothInPlace...)
		if allowed, reason := gate.CheckPairing("llama-"+string(comp)+"-v1", workloadtypes.UpdateStrategyInPlaceIfPossible, 0, 0); !allowed || !strings.Contains(reason, "down to their last serving instance") {
			t.Fatalf("%s: with both sides patched in place neither move keeps the pair, so the first move is admitted: allowed=%v reason=%q", comp, allowed, reason)
		}
	}
}
