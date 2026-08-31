package gangpack

import (
	"context"
	"testing"
	"time"

	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

func TestPermitAdmitsCommittedGangWithoutSnapshotRead(t *testing.T) {
	pins := placement.New()
	pins.Set("team/pf", "a")
	g := &GangPack{
		handle:   &fakeHandle{},
		pins:     pins,
		pgReader: fakeReader{"team/pf": {min: 3, topo: testKey}},
	}

	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", minMember: 3, topologyKey: testKey, timeout: time.Minute})
	status, _ := g.Permit(context.Background(), state, gangPod("team", "pf"), "a3")
	if status.IsWait() {
		t.Fatal("gate wedged: committed gang with 2 bound siblings + arriving member = 3 should admit, not wait")
	}
	if !status.IsSuccess() {
		t.Fatalf("Permit = %v, want Success", status)
	}
}

func TestPermitDoesNotDoubleCountAssumedWaitingPods(t *testing.T) {
	pins := placement.New()
	_, token, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"a": 4}, map[string][]string{"a": {"a1", "a2", "a3", "a4"}}, 4)
	if !ok {
		t.Fatal("failed to reserve gang")
	}
	// Two members have reached Reserve. Both are already assumed in the snapshot,
	// and one is also in the waiting set; only two of four members have arrived.
	pins.PlaceIf("team/pf", token)
	pins.PlaceIf("team/pf", token)
	waiterPod := gangPod("team", "pf")
	waiterPod.Name = "waiter"
	current := gangPod("team", "pf")
	current.Name = "current"
	waiter := &fakeWaitingPod{pod: waiterPod}
	g := &GangPack{pins: pins, handle: &fakeHandle{
		waiting: []framework.WaitingPod{waiter},
		snapshot: []framework.NodeInfo{
			nodeInfo(gpuNode("a1", "a", "4"), waiterPod),
			nodeInfo(gpuNode("a2", "a", "4"), current),
		},
	}}
	g.rememberAttempt(waiterPod, token)
	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", minMember: 4, topologyKey: testKey, timeout: time.Minute}, token)
	status, _ := g.Permit(context.Background(), state, current, "a2")
	if !status.IsWait() {
		t.Fatalf("Permit = %v, want Wait with only two committed arrivals", status)
	}
	if waiter.allowed {
		t.Fatal("waiting sibling was released before the commitment reached minMember")
	}
}

// TestPermitFormingGangDoesNotScanBound: a still-forming gang (remaining>0)
// holds until its reservation drains, regardless of snapshot contents.
func TestPermitFormingGangDoesNotScanBound(t *testing.T) {
	pins := placement.New()
	pins.Choose("team/pf", topology.FreeByDomain{"a": 5}, 3) // pinned, remaining==3, not committed
	// A snapshot that WOULD satisfy the gate if (wrongly) counted.
	snapshot := []framework.NodeInfo{
		nodeInfo(gpuNode("a1", "a", "4"), gangPod("team", "pf")),
		nodeInfo(gpuNode("a2", "a", "4"), gangPod("team", "pf")),
	}
	g := &GangPack{
		handle:   &fakeHandle{snapshot: snapshot},
		pins:     pins,
		pgReader: fakeReader{"team/pf": {min: 3, topo: testKey}},
	}
	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", minMember: 3, topologyKey: testKey, timeout: time.Minute})
	status, _ := g.Permit(context.Background(), state, gangPod("team", "pf"), "a3")
	if !status.IsWait() {
		t.Fatalf("forming gang (remaining>0) must wait, not count bound siblings; got %v", status)
	}
}

// TestGateDecision: the gang releases when the members already waiting plus the
// arriving one reach minMember — not before.
func TestSameGang(t *testing.T) {
	if !sameGang(gangPod("team", "pf"), "team/pf") {
		t.Fatal("pod in team/pf should match gang key team/pf")
	}
	if sameGang(gangPod("team", "pf"), "team/decode") {
		t.Fatal("pod in team/pf must not match a different gang")
	}
	if sameGang(gangPod("other", "pf"), "team/pf") {
		t.Fatal("namespace must be part of the gang key")
	}
	if sameGang(gangPod("team", ""), "team/pf") {
		t.Fatal("non-gang pod matches nothing")
	}
}

func TestPermitRejectsLabeledPodWithoutValidatedState(t *testing.T) {
	g := &GangPack{}
	status, _ := g.Permit(context.Background(), newCycleState(), gangPod("team", "pf"), "n")
	if status.Code() != framework.Unschedulable {
		t.Fatalf("Permit = %v, want Unschedulable", status)
	}
}

func TestPermitIgnoresWaitingPodsFromOlderAttempt(t *testing.T) {
	pins := placement.New()
	_, oldToken, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"a": 2}, map[string][]string{"a": {"a1", "a2"}}, 2)
	if !ok || !pins.ReleaseIf("team/pf", oldToken, nil) {
		t.Fatal("failed to establish old attempt")
	}
	_, newToken, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"b": 2}, map[string][]string{"b": {"b1", "b2"}}, 2)
	if !ok {
		t.Fatal("failed to establish retry")
	}
	oldWaiter := &fakeWaitingPod{pod: gangPod("team", "pf")}
	h := &fakeHandle{waiting: []framework.WaitingPod{oldWaiter}}
	g := &GangPack{handle: h, pins: pins}
	g.rememberAttempt(oldWaiter.pod, oldToken)
	current := gangPod("team", "pf")
	state := newCycleState()
	writePin(state, "b", gangInfo{key: "team/pf", minMember: 2, topologyKey: testKey, timeout: time.Minute}, newToken)

	status, _ := g.Permit(context.Background(), state, current, "b1")
	if !status.IsWait() {
		t.Fatalf("Permit = %v, want Wait; old-attempt waiter must not complete retry", status)
	}
	if oldWaiter.allowed {
		t.Fatal("old-attempt waiter was allowed by retry")
	}
}
