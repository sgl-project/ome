package coordination

import (
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Edge-state coverage for the RatioBalanced gate: degenerate tolerance
// bands, members at zero serving, deliberate mid-roll scale changes,
// and anchor bookkeeping for group-membership changes.

// A zero tolerance band admits only projections that exactly preserve
// the anchored ratio. The discrete surge tiebreaker still applies: a
// balanced fleet at full original serving may take exactly ONE
// transient +1 surge (a surge only adds capacity), so a SurgeThenDrain
// roll is not wedged even by the tightest possible band.
func TestEvaluateSurge_ZeroToleranceAllowsOnlyMinimalSurge(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(0),
	}
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
	}
	d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, 2)
	if d.AllowedSurgeDelta != 1 {
		t.Errorf("zero tolerance on a balanced 4:4 fleet: surge must be trimmed to the minimal +1 tiebreaker step; got %+v", d)
	}
}

// Drain stays strict under a zero band: the drain tiebreaker relaxes
// the band by one extra tolerance width, and twice zero is still zero,
// so a drain-first strategy is refused. This is the honest-deadlock
// posture — capacity is never removed when the operator asked for
// exact ratio hold.
func TestEvaluateSurge_ZeroToleranceRefusesDrain(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(0),
	}
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
	}
	d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, -1)
	if d.AllowedSurgeDelta != 0 || !d.SkewRejected {
		t.Errorf("zero tolerance must refuse a -1 drain (projected 4/3 breaks an exact-ratio band): got %+v", d)
	}
}

// A negative tolerance clamps to zero rather than inverting the band.
// An inverted band (low > high) would reject even the exact-ratio
// baseline and disable the surge tiebreaker; the clamp keeps the
// semantics identical to tolerance zero.
func TestEvaluateSurge_NegativeToleranceClampsToZero(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(-25),
	}
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
	}
	if d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, 1); d.AllowedSurgeDelta != 1 {
		t.Errorf("negative tolerance must behave as zero (minimal +1 surge allowed on a balanced baseline), not as an inverted reject-all band: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, -1); d.AllowedSurgeDelta != 0 {
		t.Errorf("negative tolerance must behave as zero for drains (refused): got %+v", d)
	}
}

// The band is inclusive at its edges: a projection landing EXACTLY on
// originalRatio*(1+band) passes the strict band check (not the
// tiebreaker). A requested multi-pod surge is trimmed down to the
// largest delta that stays on or inside the edge.
func TestEvaluateSurge_UpperBandEdgeInclusiveAndTrimsDelta(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(25),
	}
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
	}
	// Requested +2 projects 6/4 = 1.5 > 1.25 (out); trimmed +1 projects
	// 5/4 = 1.25 == bandHigh (in, inclusive).
	d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, 2)
	if d.AllowedSurgeDelta != 1 {
		t.Fatalf("surge must be trimmed to +1 (5/4 = 1.25 sits exactly on the band edge): got %+v", d)
	}
	if d.Reason != "within tolerance" {
		t.Errorf("a projection on the inclusive band edge must pass the strict band check, not fall through to the tiebreaker: reason=%q", d.Reason)
	}
}

// A member at zero live serving makes every pairwise ratio against it
// undefined, and the gate fails safe: no drain of the healthy peer, no
// single-pod re-entry of the drained member. Restoring serving is the
// workload controller's job; the pacing gate only admits a step once
// the projection lands back inside the band — which is why a surge
// proposal large enough to fully restore the drained member IS
// admitted.
func TestEvaluateSurge_MemberAtZeroServingFailsSafe(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(25),
	}
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 0,
		},
	}

	if d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, -1); d.AllowedSurgeDelta != 0 || !d.SkewRejected {
		t.Errorf("draining the healthy member while its peer serves zero must be refused: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.DecoderComponent, 1); d.AllowedSurgeDelta != 0 || !d.SkewRejected {
		t.Errorf("a +1 surge of the zero-serving member projects 4:1 (out of band) and its baseline is below original, so the tiebreaker must not fire: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.DecoderComponent, 4); d.AllowedSurgeDelta != 4 {
		t.Errorf("a surge proposal that fully restores the drained member projects 4:4 (in band) and must be admitted whole: got %+v", d)
	}
}

// A deliberate mid-roll scale-down of one member leaves the anchor at
// the rollout-start counts (anchor preservation is the snapshot
// contract), so the live fleet sits outside the band. The gate holds
// every step that keeps or worsens the skew — including surges of the
// leading member — and admits only steps that restore the anchored
// ratio.
func TestEvaluateSurge_MidRollScaleDownHoldsUntilRatioRestored(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{
		Type:                  v1beta1.CoordinationPacingRatioBalanced,
		RatioTolerancePercent: int32Ptr(25),
	}
	// Anchor 4:4; decoder deliberately halved to 2 live serving.
	state := RatioState{
		Original: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 4,
		},
		Serving: map[v1beta1.ComponentType]int32{
			v1beta1.EngineComponent:  4,
			v1beta1.DecoderComponent: 2,
		},
	}

	if d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, -1); d.AllowedSurgeDelta != 0 {
		t.Errorf("engine drain projects 3:2 (still outside band [0.75, 1.25]) and the baseline is already skewed, so it must be refused: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.EngineComponent, 1); d.AllowedSurgeDelta != 0 {
		t.Errorf("engine surge widens the skew (5:2) and the off-band baseline disables the tiebreaker, so it must be refused: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.DecoderComponent, 1); d.AllowedSurgeDelta != 0 {
		t.Errorf("a single decoder surge projects 4:3 = 1.33 (still out of band) and the below-original baseline disables the tiebreaker: got %+v", d)
	}
	if d := EvaluateSurge(pacing, state, v1beta1.DecoderComponent, 2); d.AllowedSurgeDelta != 2 {
		t.Errorf("a decoder surge restoring 4:4 lands back in band and must be admitted whole: got %+v", d)
	}
}

// An ObservedRatio whose Original map is present but EMPTY is the same
// "no snapshot yet" state as a missing ObservedRatio: the gate lets the
// step through and the next reconcile records the real anchor. Gating
// against an empty anchor would treat every member as absent and
// deadlock the first step of every rollout.
func TestCheckRatioGate_EmptyOriginalMapIsNoSnapshot(t *testing.T) {
	tol := int32(25)
	isvc := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Rollout: &v1beta1.RolloutSpec{
				Groups: []v1beta1.RolloutGroup{{
					Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
					RollingUpdate: &v1beta1.GroupRollingUpdate{},
					MaintainRatio: &v1beta1.MaintainRatio{Tolerance: &tol},
				}},
			},
		},
		Status: v1beta1.InferenceServiceStatus{
			Components: map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
				v1beta1.EngineComponent: {
					Lifecycle: &v1beta1.LifecycleStatus{Replicas: 4, ServingReplicas: 4},
				},
				v1beta1.DecoderComponent: {
					Lifecycle: &v1beta1.LifecycleStatus{Replicas: 2, ServingReplicas: 2},
				},
			},
			RolloutCoordination: &v1beta1.RolloutCoordinationStatus{
				Groups: []v1beta1.RolloutCoordinationGroupStatus{{
					Name:          "0",
					Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
					Policy:        v1beta1.CoordinationPolicyBlueGreen,
					ObservedRatio: &v1beta1.RolloutCoordinationRatio{Original: map[v1beta1.ComponentType]int32{}},
				}},
			},
		},
	}
	pinActiveRun(isvc)
	allowed, reason := CheckRatioGate(isvc, v1beta1.EngineComponent, 0)
	if !allowed {
		t.Fatalf("empty Original map must read as no-snapshot-yet (allow, snapshot next pass); got denied: %s", reason)
	}
	if !strings.Contains(reason, "no ObservedRatio snapshot") {
		t.Errorf("reason should name the missing snapshot: %q", reason)
	}
}

// When a member is removed from the group, its stale anchor entry in
// ObservedRatio.Original must stop constraining the band. The gate
// builds its state from the group's CURRENT membership; a departed
// member (which no longer reports serving pods) would otherwise read
// as a zero-serving peer and veto every step forever.
func TestCheckRatioGate_RemovedMemberStaleAnchorIgnored(t *testing.T) {
	tol := int32(25)
	isvc := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Rollout: &v1beta1.RolloutSpec{
				Groups: []v1beta1.RolloutGroup{{
					// Router is no longer a member; its anchor entry below is stale.
					Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
					RollingUpdate: &v1beta1.GroupRollingUpdate{},
					MaintainRatio: &v1beta1.MaintainRatio{Tolerance: &tol},
				}},
			},
		},
		Status: v1beta1.InferenceServiceStatus{
			Components: map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
				v1beta1.EngineComponent: {
					Lifecycle: &v1beta1.LifecycleStatus{Replicas: 4, ServingReplicas: 4},
				},
				v1beta1.DecoderComponent: {
					Lifecycle: &v1beta1.LifecycleStatus{Replicas: 2, ServingReplicas: 2},
				},
			},
			RolloutCoordination: &v1beta1.RolloutCoordinationStatus{
				Groups: []v1beta1.RolloutCoordinationGroupStatus{{
					Name:       "0",
					Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
					Policy:     v1beta1.CoordinationPolicyBlueGreen,
					ObservedRatio: &v1beta1.RolloutCoordinationRatio{
						Original: map[v1beta1.ComponentType]int32{
							v1beta1.EngineComponent:  4,
							v1beta1.DecoderComponent: 2,
							v1beta1.RouterComponent:  8,
						},
					},
				}},
			},
		},
	}
	pinActiveRun(isvc)
	// Engine drain projects 3:2 = 1.5, exactly the lower edge of the
	// 2.0 +/- 25% band — allowed IF the stale router entry (which has no
	// serving pods) is excluded from the pairwise check.
	allowed, reason := CheckRatioGate(isvc, v1beta1.EngineComponent, 0)
	if !allowed {
		t.Fatalf("stale anchor entry for a removed member must not constrain the band (engine drain 3:2 = 1.5 sits on the band edge); got denied: %s", reason)
	}
}
