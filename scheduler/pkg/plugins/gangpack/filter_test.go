package gangpack

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

// TestPluginName pins the registered plugin name referenced from the scheduler
// config.
func TestPluginName(t *testing.T) {
	if got := (&GangPack{}).Name(); got != Name {
		t.Fatalf("Name() = %q, want %q", got, Name)
	}
}

func TestFilterProtectsOutstandingGangReservationFromStandalonePod(t *testing.T) {
	g := &GangPack{pins: placement.New()}
	g.pins.ChooseInTopology("team/pf", testKey, topology.FreeByDomain{"a": 2}, 2)
	state := newCycleState() // standalone pod has no gang pin
	if status := g.Filter(context.Background(), state, gpuPod("4"), nodeInfo(gpuNode("a1", "a", "4"))); status.Code() != framework.UnschedulableAndUnresolvable {
		t.Fatalf("reserved-domain Filter = %v, want UnschedulableAndUnresolvable", status)
	}
	if status := g.Filter(context.Background(), state, gpuPod("4"), nodeInfo(gpuNode("b1", "b", "4"))); !status.IsSuccess() {
		t.Fatalf("unreserved-domain Filter = %v, want Success", status)
	}
}

// TestFilterEnforcesPinnedDomain: once a gang is pinned to a domain (PreFilter
// records it in CycleState), Filter admits only nodes in that domain and rejects
// the rest as UnschedulableAndUnresolvable — a wrong-domain node can't become
// right by preempting on it, so the scheduler shouldn't waste preemption there.
// With no pin recorded (not a pinned gang member), Filter imposes no constraint.
func TestFilterEnforcesPinnedDomain(t *testing.T) {
	g := &GangPack{}
	ctx := context.Background()
	pod := &v1.Pod{}

	// No pin recorded -> any node passes.
	st := newCycleState()
	if s := g.Filter(ctx, st, pod, nodeInfo(gpuNode("n0", "a", "4"))); !s.IsSuccess() {
		t.Fatalf("no-pin Filter = %v, want Success", s)
	}

	// Pin the gang to domain "a".
	writePin(st, "a", gangInfo{topologyKey: testKey})

	if s := g.Filter(ctx, st, pod, nodeInfo(gpuNode("n1", "a", "4"))); !s.IsSuccess() {
		t.Fatalf("in-domain Filter = %v, want Success", s)
	}

	got := g.Filter(ctx, st, pod, nodeInfo(gpuNode("n2", "b", "4")))
	if got.IsSuccess() || got.Code() != framework.UnschedulableAndUnresolvable {
		t.Fatalf("out-of-domain Filter = %v, want UnschedulableAndUnresolvable", got)
	}

	// A node with no domain label is not the pinned domain -> rejected too.
	if s := g.Filter(ctx, st, pod, nodeInfo(gpuNode("n3", "", "4"))); s.IsSuccess() {
		t.Fatalf("no-domain-label Filter = Success, want rejected")
	}
}
