package coordination

import (
	"context"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// mkScaledUpReferenceFixture is the scale-up picture one service pass after
// the group's reference followed the new counts: 6 engines and 2 routers in
// one RatioBalanced group at tolerance 10, scaled to 7 engines and 3 routers
// while the roll settles. The reference reads 7:3, each side has its new
// Instance's row with a Create attempt open and no pod serving yet, so the
// engine serves 6 of 7 and the router 2 of 3. The router has rolled 1 of its
// 3, the engine 0 of its 7.
func mkScaledUpReferenceFixture() (*v1beta1.InferenceService, map[v1beta1.ComponentType][]v1beta1.OMENativeInstanceStatus) {
	eng, rtr := v1beta1.EngineComponent, v1beta1.RouterComponent
	isvc := mkRatioProgressFixture(10, 7, 0, 3, 1)
	isvc.Status.Components[eng] = v1beta1.ComponentStatusSpec{Lifecycle: &v1beta1.LifecycleStatus{
		Replicas: 7, ServingReplicas: 6, UpdatedReplicas: 0,
	}}
	isvc.Status.Components[rtr] = v1beta1.ComponentStatusSpec{Lifecycle: &v1beta1.LifecycleStatus{
		Replicas: 3, ServingReplicas: 2, UpdatedReplicas: 1,
	}}
	creating := func(index int32) v1beta1.OMENativeInstanceStatus {
		return v1beta1.OMENativeInstanceStatus{
			Index: index, Phase: v1beta1.OMENativeInstanceCreating,
			Operation: &v1beta1.InstanceOperation{Type: v1beta1.InstanceOperationCreate, Step: "CreatePods", TargetRevision: "rev-2"},
		}
	}
	return isvc, map[v1beta1.ComponentType][]v1beta1.OMENativeInstanceStatus{
		eng: {creating(6)},
		rtr: {creating(2)},
	}
}

// TestEvaluateUpdateGate_ScaleUpKeepsTheLockstepHoldOnceTheReferenceFollowsTheSpec
// reads the scale-up picture through the production decision site in the
// pass after the reference moved to the new counts, which is where the roll
// spends the whole time the new Instances take to come up. The router, 1 of
// 3 rolled against an engine at 0 of 7, leads by 33 points at a tolerance
// of 10: the lockstep holds its second surge, and a side that lost no pod
// keeps that hold while its new Instance is still being created, and still
// once it serves. The engine, behind, waits on the serving band only while
// the fleet is short of the counts the reference names (6:2 against 7:3)
// and proceeds once the new Instances serve. A router that lost a serving
// pod beside the one coming up is short of capacity and is still refilled.
func TestEvaluateUpdateGate_ScaleUpKeepsTheLockstepHoldOnceTheReferenceFollowsTheSpec(t *testing.T) {
	eng, rtr := v1beta1.EngineComponent, v1beta1.RouterComponent
	isvc, rows := mkScaledUpReferenceFixture()
	client := fakeClientForISVCWithInstances(isvc, rows)
	allowed, gate, reason := EvaluateUpdateGate(context.Background(), client, isvc, rtr, "", forwardTarget(isvc, rtr), nil, GroupDefaults{},
		workloadtypes.UpdateStrategySurgeThenDrain, 0, 0)
	if allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "outpace") {
		t.Fatalf("router 1 of 3 against engine 0 of 7 must stay held on gate Ratio by the lockstep while its new Instance comes up: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}
	if allowed, gate, reason := EvaluateUpdateGate(context.Background(), client, isvc, eng, "", forwardTarget(isvc, eng), nil, GroupDefaults{},
		workloadtypes.UpdateStrategySurgeThenDrain, 0, 0); allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "skew") {
		t.Fatalf("the engine at 6 of 7 serving beside a router at 2 of 3 must wait on the serving band until the new Instances serve: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}

	served, _ := mkScaledUpReferenceFixture()
	served.Status.Components[eng] = v1beta1.ComponentStatusSpec{Lifecycle: &v1beta1.LifecycleStatus{
		Replicas: 7, ServingReplicas: 7, UpdatedReplicas: 0,
	}}
	served.Status.Components[rtr] = v1beta1.ComponentStatusSpec{Lifecycle: &v1beta1.LifecycleStatus{
		Replicas: 3, ServingReplicas: 3, UpdatedReplicas: 1,
	}}
	client = fakeClientForISVC(served)
	if allowed, gate, reason := EvaluateUpdateGate(context.Background(), client, served, rtr, "", forwardTarget(served, rtr), nil, GroupDefaults{},
		workloadtypes.UpdateStrategySurgeThenDrain, 0, 0); allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "outpace") {
		t.Fatalf("router 1 of 3 against engine 0 of 7 must stay held by the lockstep once the new Instances serve: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}
	if allowed, _, reason := EvaluateUpdateGate(context.Background(), client, served, eng, "", forwardTarget(served, eng), nil, GroupDefaults{},
		workloadtypes.UpdateStrategySurgeThenDrain, 0, 0); !allowed {
		t.Fatalf("the engine, behind, must proceed once the new Instances serve: %s", reason)
	}

	lost, rows := mkScaledUpReferenceFixture()
	lost.Status.Components[rtr] = v1beta1.ComponentStatusSpec{Lifecycle: &v1beta1.LifecycleStatus{
		Replicas: 3, ServingReplicas: 1, UpdatedReplicas: 1,
	}}
	routerCtx := ResolveGateContext(context.Background(), fakeClientForISVCWithInstances(lost, rows), lost, rtr)
	if allowed, reason := routerCtx.CheckRatio(0, 0, +1); !allowed || !strings.Contains(reason, "capacity recovery") {
		t.Fatalf("a router that lost a serving pod beside its new Instance must still be refilled: allowed=%v reason=%q", allowed, reason)
	}
}
