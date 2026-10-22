package coordination

import (
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// The group unavailable budget on an engine of gangs whose roll changed
// strategy while a gang surge pair stands. The operator pushed version 2
// under SurgeThenDrain and OME built one gang's replacement as a row of its
// own beside its source, the pair ratio_surge_budget_test.go describes. The
// operator then changed the update strategy to RecreatePod, so every later
// start takes a gang out of rotation and asks the unavailable budget. That
// budget is a share of the Instance count the spec asks for, so the extra
// row must not widen it: 4 gangs at 25 percent lose one gang at a time, and
// 3 of the 4 keep serving, whether the replica publishes 4 rows or 5.

// drainFirstGangRoll is gangRollReplica after the strategy change: the rows
// as the surge wrote them, the spec now asking for RecreatePod. With
// replacementServing the open pairs' replacement gangs serve beside their
// sources, the moment before the old gangs leave, and the replica counts
// them as serving.
func drainFirstGangRoll(f gangRollFixture, replacementServing bool) *v1beta1.InferenceReplica {
	ir := gangRollReplica(f)
	ir.Spec.Lifecycle.UpdateStrategy = &v1beta1.UpdateStrategy{Type: v1beta1.UpdateStrategyRecreatePod}
	if !replacementServing {
		return ir
	}
	for i := range ir.Status.InstanceStatuses {
		row := &ir.Status.InstanceStatuses[i]
		if row.Operation == nil || row.Operation.Step != workloadtypes.UpdateStepGangSurgeTarget {
			continue
		}
		row.ReadyPodCount, row.ServingPodCount, row.AvailablePodCount = gangPods, gangPods, gangPods
		ir.Status.ServingReplicas++
		ir.Status.ReadyReplicas++
	}
	return ir
}

func drainFirstGangClient(t *testing.T, f gangRollFixture, replacementServing bool) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(pairingScheme(t)).WithObjects(drainFirstGangRoll(f, replacementServing)).Build()
}

// namedQuarterService is the service the gangs roll in when the group names
// its budget: a rollingUpdate group of the engine alone with maxUnavailable
// 25 percent and a run open. gangRollService is the group that names none
// and is read with the default quarter.
func namedQuarterService() *v1beta1.InferenceService {
	return batchService(batchGroupFixture{maxUnavailable: "25%", components: []v1beta1.ComponentType{v1beta1.EngineComponent}}, nil)
}

// expectDrainHeld asks the gate stack about taking one more engine gang out
// of rotation with inFlight gangs already taken this pass and requires the
// Budget hold with the given numbers.
func expectDrainHeld(t *testing.T, isvc *v1beta1.InferenceService, c client.Client, inFlight int32, want string) {
	t.Helper()
	allowed, gate, reason := batchGate(t, isvc, c, v1beta1.EngineComponent, workloadtypes.UpdateStrategyRecreatePod, inFlight)
	if allowed || gate != v1beta1.RolloutHoldGateBudget || !strings.Contains(reason, want) {
		t.Errorf("drain with %d out this pass: allowed=%v gate=%q reason=%q, want the Budget hold %q", inFlight, allowed, gate, reason, want)
	}
}

// expectDrainAdmitted is the admitted counterpart.
func expectDrainAdmitted(t *testing.T, isvc *v1beta1.InferenceService, c client.Client, inFlight int32) {
	t.Helper()
	if allowed, gate, reason := batchGate(t, isvc, c, v1beta1.EngineComponent, workloadtypes.UpdateStrategyRecreatePod, inFlight); !allowed {
		t.Errorf("drain with %d out this pass: held under %q with %q, want it admitted", inFlight, gate, reason)
	}
}

// TestEvaluateUpdateGate_AGangSurgeRowDoesNotWidenTheGroupUnavailableBudget
// is the operator's story: 4 engine gangs of one leader and one worker in a
// blueGreen group of the engine alone that names no budget, so the group's
// unavailable budget is the default quarter of 4 rounded up, one gang.
// Version 2 was pushed under SurgeThenDrain, one gang's replacement stands
// beside its source, 5 rows for 4 gangs, and the operator changed the
// strategy to RecreatePod. With no pair open the first drain is admitted and
// a second in the same pass is held. With the pair open the same holds, and
// the hold names the budget of one, not a quarter of the 5 rows. Once the
// replacement gang serves beside its source the replica counts 5 serving,
// and a second drain in the same pass must still be held: otherwise two
// gangs leave rotation and 2 of the 4 serve once the pair's old gang goes.
// On 8 gangs with two pairs open the budget is 2.
func TestEvaluateUpdateGate_AGangSurgeRowDoesNotWidenTheGroupUnavailableBudget(t *testing.T) {
	isvc := gangRollService(v1beta1.CoordinationPolicyBlueGreen)
	const heldAtOne = "unavailable budget 1 exhausted (current 0, in-flight 1, would become 2)"
	t.Run("no-pair-open-4-rows", func(t *testing.T) {
		c := drainFirstGangClient(t, gangRollFixture{desired: 4}, false)
		expectDrainAdmitted(t, isvc, c, 0)
		expectDrainHeld(t, isvc, c, 1, heldAtOne)
	})
	t.Run("one-pair-open-5-rows", func(t *testing.T) {
		c := drainFirstGangClient(t, gangRollFixture{desired: 4, open: 1}, false)
		expectDrainAdmitted(t, isvc, c, 0)
		expectDrainHeld(t, isvc, c, 1, heldAtOne)
	})
	t.Run("one-pair-open-replacement-serving-5-rows", func(t *testing.T) {
		c := drainFirstGangClient(t, gangRollFixture{desired: 4, open: 1}, true)
		expectDrainAdmitted(t, isvc, c, 0)
		expectDrainHeld(t, isvc, c, 1, heldAtOne)
	})
	t.Run("one-promoted-one-pair-open-5-rows", func(t *testing.T) {
		c := drainFirstGangClient(t, gangRollFixture{desired: 4, promoted: 1, open: 1}, false)
		expectDrainAdmitted(t, isvc, c, 0)
		expectDrainHeld(t, isvc, c, 1, heldAtOne)
	})
	t.Run("eight-gangs-two-pairs-open-10-rows", func(t *testing.T) {
		c := drainFirstGangClient(t, gangRollFixture{desired: 8, open: 2}, false)
		expectDrainAdmitted(t, isvc, c, 1)
		expectDrainHeld(t, isvc, c, 2, "unavailable budget 2 exhausted (current 0, in-flight 2, would become 3)")
	})
}

// TestCheckUnavailability_PercentBudgetIsSizedByTheDesiredCountNotTheRows
// asks the unavailable budget gate alone, off the same replica: the quarter
// is scaled against the 4 gangs the spec asks for, so with one pair open and
// its replacement serving (5 rows, 5 serving) the second drain of a pass is
// held, under the default quarter and under a named one alike, and a replica
// that records no desired count is sized by its rows.
func TestCheckUnavailability_PercentBudgetIsSizedByTheDesiredCountNotTheRows(t *testing.T) {
	ask := func(t *testing.T, isvc *v1beta1.InferenceService, ir *v1beta1.InferenceReplica, inFlight int32) (bool, string) {
		t.Helper()
		c := fake.NewClientBuilder().WithScheme(pairingScheme(t)).WithObjects(ir).Build()
		return ResolveGateContext(t.Context(), c, isvc, v1beta1.EngineComponent).CheckUnavailability(inFlight)
	}
	const secondHeld = "unavailable budget 1 exhausted (current 0, in-flight 1, would become 2)"
	t.Run("one-pair-open-holds-the-second-drain-of-a-pass", func(t *testing.T) {
		allowed, reason := ask(t, gangRollService(v1beta1.CoordinationPolicyBlueGreen), drainFirstGangRoll(gangRollFixture{desired: 4, open: 1}, true), 1)
		if allowed || !strings.Contains(reason, secondHeld) {
			t.Fatalf("allowed=%v reason=%q, want the budget of one spent by the first drain", allowed, reason)
		}
	})
	t.Run("a-named-quarter-reads-the-same", func(t *testing.T) {
		allowed, reason := ask(t, namedQuarterService(), drainFirstGangRoll(gangRollFixture{desired: 4, open: 1}, true), 1)
		if allowed || !strings.Contains(reason, secondHeld) {
			t.Fatalf("allowed=%v reason=%q, want the named quarter of 4 to hold the second drain", allowed, reason)
		}
	})
	t.Run("no-desired-count-recorded-sizes-by-the-rows", func(t *testing.T) {
		ir := drainFirstGangRoll(gangRollFixture{desired: 4, open: 1}, true)
		ir.Spec.Replicas = nil
		if allowed, reason := ask(t, gangRollService(v1beta1.CoordinationPolicyBlueGreen), ir, 1); !allowed {
			t.Fatalf("held with %q, want the quarter of the 5 rows, 2, to admit the second drain", reason)
		}
	})
}

// TestEvaluateUpdateGate_UnavailableBudgetHoldsWhateverPhaseTheGroupReads
// builds the pictures of the gang roll in which the group reads each rolling
// phase and Failed, the pictures the surge budget is proved on, checks the
// phase off the same replica the gate reads, and requires the budget of one
// on the next drain in every one. Where the picture has a gang down already,
// a promoted gang restarting (Waiting) or a gang whose pods died (Failed),
// the first drain of the pass is held; elsewhere the second is.
func TestEvaluateUpdateGate_UnavailableBudgetHoldsWhateverPhaseTheGroupReads(t *testing.T) {
	blueGreen, rolling := v1beta1.CoordinationPolicyBlueGreen, v1beta1.CoordinationPolicyRollingUpdate
	const secondHeld = "unavailable budget 1 exhausted (current 0, in-flight 1, would become 2)"
	const firstHeld = "unavailable budget 1 exhausted (current 1, in-flight 0, would become 2)"
	cases := []struct {
		name     string
		policy   v1beta1.CoordinationPolicy
		fixture  gangRollFixture
		phase    v1beta1.CoordinationPhase
		inFlight int32
		want     string
	}{
		{"Surging", blueGreen, gangRollFixture{desired: 4, open: 1}, v1beta1.CoordinationPhaseSurging, 1, secondHeld},
		{"Waiting", blueGreen, gangRollFixture{desired: 4, promoted: 1, open: 1, promotedUnready: true}, v1beta1.CoordinationPhaseWaiting, 0, firstHeld},
		{"Shifting", blueGreen, gangRollFixture{desired: 4, promoted: 1, open: 1}, v1beta1.CoordinationPhaseShifting, 1, secondHeld},
		{"ScalingDown", rolling, gangRollFixture{desired: 4, promoted: 1, open: 1}, v1beta1.CoordinationPhaseScalingDown, 1, secondHeld},
		{"Failed", blueGreen, gangRollFixture{desired: 4, open: 1, failed: 1}, v1beta1.CoordinationPhaseFailed, 0, firstHeld},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gangRollPhase(tc.policy, tc.fixture); got != tc.phase {
				t.Fatalf("group phase %q, want %q", got, tc.phase)
			}
			expectDrainHeld(t, gangRollService(tc.policy), drainFirstGangClient(t, tc.fixture, false), tc.inFlight, tc.want)
		})
	}
}
