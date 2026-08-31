package gangpack

import (
	"context"
	"testing"
	"time"

	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

func TestPostFilterUnwindsPreReserveFailure(t *testing.T) {
	sibling := &fakeWaitingPod{pod: gangPod("team", "pf")}
	g := &GangPack{handle: &fakeHandle{waiting: []framework.WaitingPod{sibling}}, pins: placement.New()}
	g.pins.ChooseInTopology("team/pf", testKey, topology.FreeByDomain{"a": 2}, 2)
	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", minMember: 2, topologyKey: testKey, timeout: time.Minute})

	_, status := g.PostFilter(context.Background(), state, gangPod("team", "pf"), nil)
	if status.Code() != framework.Unschedulable {
		t.Fatalf("PostFilter = %v, want Unschedulable", status)
	}
	if _, pinned := g.pins.Get("team/pf"); pinned {
		t.Fatal("pin leaked after every candidate failed before Reserve")
	}
	if !sibling.rejected {
		t.Fatal("waiting sibling must unwind with the failed member")
	}
}

func TestPostFilterRetrySkipsFailedDomain(t *testing.T) {
	g := &GangPack{handle: &fakeHandle{}, pins: placement.New()}
	gang := gangInfo{key: "team/pf", uid: "uid-1", minMember: 1, topologyKey: testKey, timeout: time.Minute}
	nodes := []framework.NodeInfo{
		nodeInfo(gpuNode("a1", "a", "4")),
		nodeInfo(gpuNode("b1", "b", "4")),
	}
	first := newCycleState()
	_, status := g.pinGang(first, nodes, gang, gangGPUPod("team", "pf", "4"))
	if !status.IsSuccess() || readPin(first).domain != "a" {
		t.Fatalf("first pin = %+v, %v, want domain a", readPin(first), status)
	}
	g.PostFilter(context.Background(), first, gangPod("team", "pf"), nil)

	retry := newCycleState()
	_, status = g.pinGang(retry, nodes, gang, gangGPUPod("team", "pf", "4"))
	if !status.IsSuccess() || readPin(retry).domain != "b" {
		t.Fatalf("retry pin = %+v, %v, want domain b after a failed", readPin(retry), status)
	}
}

func TestFailedDomainHistorySurvivesPartialReserve(t *testing.T) {
	g := &GangPack{handle: &fakeHandle{}, pins: placement.New()}
	gang := gangInfo{key: "team/pf", uid: "uid-1", minMember: 2, topologyKey: testKey, timeout: time.Minute}
	nodes := []framework.NodeInfo{
		nodeInfo(gpuNode("a1", "a", "4")), nodeInfo(gpuNode("a2", "a", "4")),
		nodeInfo(gpuNode("b1", "b", "4")), nodeInfo(gpuNode("b2", "b", "4")),
	}
	first := newCycleState()
	g.pinGang(first, nodes, gang, gangGPUPod("team", "pf", "4"))
	if readPin(first).domain != "a" {
		t.Fatalf("first domain = %q, want a", readPin(first).domain)
	}
	g.PostFilter(context.Background(), first, gangPod("team", "pf"), nil)

	second := newCycleState()
	g.pinGang(second, nodes, gang, gangGPUPod("team", "pf", "4"))
	if readPin(second).domain != "b" {
		t.Fatalf("second domain = %q, want b", readPin(second).domain)
	}
	g.Reserve(context.Background(), second, gangPod("team", "pf"), "b1")
	filtered, _ := g.withoutFailedDomains(gang, topology.FreeByDomain{"a": 2, "b": 2})
	if _, present := filtered["a"]; present {
		t.Fatal("partial Reserve cleared failed domain a before the commitment drained")
	}
	g.PostFilter(context.Background(), second, gangPod("team", "pf"), nil)
	filtered, _ = g.withoutFailedDomains(gang, topology.FreeByDomain{"a": 2, "b": 2})
	if len(filtered) != 0 {
		t.Fatalf("failed domains after a then b = %v, want both excluded", filtered)
	}
}
