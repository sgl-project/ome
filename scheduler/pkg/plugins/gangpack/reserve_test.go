package gangpack

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

func TestReserveIsNoop(t *testing.T) {
	g := &GangPack{}
	if s := g.Reserve(context.Background(), nil, &v1.Pod{}, "n1"); !s.IsSuccess() {
		t.Fatalf("Reserve = %v, want Success", s)
	}
}

// TestReserveDrainsReservation: reserving a gang member (it just got a real node)
// drains one whole node from its domain reservation, so the capacity another gang
// sees frees up in step — the hand-off from reserved to placed.
func TestReserveDrainsReservation(t *testing.T) {
	g := &GangPack{
		pins:     placement.New(),
		pgReader: fakeReader{"team/pf": {min: 2, topo: "clique"}},
	}
	// PreFilter would have pinned pf into domain d over 2 free nodes, reserving 2.
	g.pins.Choose("team/pf", topology.FreeByDomain{"d": 2}, 2)
	_, token, _, _ := g.pins.GetOwned("team/pf")
	// A second gang can't fit: d is fully reserved (2 raw - 2 reserved = 0).
	if _, ok := g.pins.Choose("team/other", topology.FreeByDomain{"d": 2}, 1); ok {
		t.Fatal("precondition: d should be fully reserved before Reserve")
	}

	state := newCycleState()
	writePin(state, "d", gangInfo{key: "team/pf", minMember: 2, topologyKey: "clique", timeout: time.Minute}, token)
	// One pf member lands: its heterogeneous claim still owns the domain.
	if s := g.Reserve(context.Background(), state, gangPod("team", "pf"), "node1"); !s.IsSuccess() {
		t.Fatalf("Reserve = %v, want Success", s)
	}
	if _, ok := g.pins.Choose("team/other", topology.FreeByDomain{"d": 2}, 1); ok {
		t.Fatal("another gang must remain blocked while any reservation is outstanding")
	}
	if s := g.Reserve(context.Background(), state, gangPod("team", "pf"), "node2"); !s.IsSuccess() {
		t.Fatalf("second Reserve = %v, want Success", s)
	}
	if d, ok := g.pins.Choose("team/other", topology.FreeByDomain{"d": 2}, 1); !ok || d != "d" {
		t.Fatalf("after reservation drains, other Choose = %q,%v want d,true", d, ok)
	}
}

// TestUnreserveUnwindsGang: unreserving one member releases the domain pin and
// rejects the siblings still waiting at the gate (but not pods of other gangs).
func TestUnreserveUnwindsGang(t *testing.T) {
	sibling := &fakeWaitingPod{pod: gangPod("team", "pf")}
	other := &fakeWaitingPod{pod: gangPod("team", "decode")}
	h := &fakeHandle{waiting: []framework.WaitingPod{sibling, other}}
	g := &GangPack{
		handle:   h,
		pins:     placement.New(),
		pgReader: fakeReader{"team/pf": {min: 3, topo: "clique"}},
	}
	g.pins.Set("team/pf", "d1")
	_, token, _, _ := g.pins.GetOwned("team/pf")
	g.rememberAttempt(sibling.pod, token)
	state := newCycleState()
	writePin(state, "d1", gangInfo{key: "team/pf", topologyKey: "clique"}, token)

	g.Unreserve(context.Background(), state, gangPod("team", "pf"), "node1")

	if _, pinned := g.pins.Get("team/pf"); pinned {
		t.Fatal("pin should be released so the retry re-plans")
	}
	if !sibling.rejected {
		t.Fatal("waiting sibling of the same gang should be rejected")
	}
	if other.rejected {
		t.Fatal("a pod of a different gang must not be rejected")
	}
}

// TestUnreserveNonGangNoop: a pod with no pod-group label is not a gang member,
// so Unreserve does nothing.
func TestUnreserveNonGangNoop(t *testing.T) {
	victim := &fakeWaitingPod{pod: gangPod("team", "pf")}
	h := &fakeHandle{waiting: []framework.WaitingPod{victim}}
	g := &GangPack{handle: h, pins: placement.New(), pgReader: fakeReader{}}

	g.Unreserve(context.Background(), nil, gangPod("team", ""), "node1") // no label

	if victim.rejected {
		t.Fatal("non-gang Unreserve must not reject anyone")
	}
}

// TestUnreserveReleasesWhenPodGroupGone: if the PodGroup was deleted mid-flight
// (the reader can no longer resolve it), Unreserve must STILL release the pin and
// reservation — the key comes from the pod's own label, not a PodGroup lookup —
// so nothing leaks.
func TestUnreserveReleasesWhenPodGroupGone(t *testing.T) {
	g := &GangPack{
		handle:   &fakeHandle{},
		pins:     placement.New(),
		pgReader: fakeReader{}, // empty: the PodGroup is gone
	}
	g.pins.Set("team/pf", "d1") // pinned before the PodGroup vanished
	_, token, _, _ := g.pins.GetOwned("team/pf")
	state := newCycleState()
	writePin(state, "d1", gangInfo{key: "team/pf", topologyKey: "clique"}, token)

	g.Unreserve(context.Background(), state, gangPod("team", "pf"), "node1")

	if _, pinned := g.pins.Get("team/pf"); pinned {
		t.Fatal("pin must be released even though the PodGroup is gone (else it leaks)")
	}
}

func TestStaleUnreserveCannotReleaseOrRejectRetry(t *testing.T) {
	pins := placement.New()
	_, oldToken, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"a": 1}, map[string][]string{"a": {"a1"}}, 1)
	if !ok || !pins.ReleaseIf("team/pf", oldToken, nil) {
		t.Fatal("failed to establish and release old attempt")
	}
	_, newToken, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"b": 1}, map[string][]string{"b": {"b1"}}, 1)
	if !ok {
		t.Fatal("failed to establish retry")
	}
	retryWaiter := &fakeWaitingPod{pod: gangPod("team", "pf")}
	h := &fakeHandle{waiting: []framework.WaitingPod{retryWaiter}}
	g := &GangPack{handle: h, pins: pins}
	g.rememberAttempt(retryWaiter.pod, newToken)
	oldState := newCycleState()
	writePin(oldState, "a", gangInfo{key: "team/pf", topologyKey: testKey}, oldToken)

	g.Unreserve(context.Background(), oldState, gangPod("team", "pf"), "a1")

	if domain, _, _, pinned := pins.GetOwned("team/pf"); !pinned || domain.Name != "b" {
		t.Fatalf("retry pin = %+v,%v, want b,true", domain, pinned)
	}
	if retryWaiter.rejected {
		t.Fatal("stale Unreserve rejected a waiter owned by the retry")
	}
}

func TestReservationDrainActivatesBlockedStandalonePod(t *testing.T) {
	pins := placement.New()
	_, token, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"a": 1}, map[string][]string{"a": {"a1"}}, 1)
	if !ok {
		t.Fatal("failed to reserve domain")
	}
	h := &fakeHandle{}
	g := &GangPack{handle: h, pins: pins}
	standalone := gpuPod("4")
	standalone.Namespace, standalone.Name = "team", "standalone"
	standalone.UID = "standalone-uid"
	if status := g.Filter(context.Background(), newCycleState(), standalone,
		nodeInfo(gpuNode("a1", "a", "4"))); status.Code() != framework.UnschedulableAndUnresolvable {
		t.Fatalf("Filter = %v, want reservation rejection", status)
	}
	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", topologyKey: testKey}, token)
	g.Reserve(context.Background(), state, gangPod("team", "pf"), "a1")
	if h.activated[string(standalone.UID)] != standalone {
		t.Fatalf("activated = %v, want blocked standalone pod", h.activated)
	}
}

func TestPostBindCleansAttemptWithoutReleasingGangPin(t *testing.T) {
	pins := placement.New()
	_, token, ok := pins.ChooseInTopologyOnNodes("team/pf", testKey,
		topology.FreeByDomain{"a": 2}, map[string][]string{"a": {"a1", "a2"}}, 2)
	if !ok {
		t.Fatal("failed to establish gang pin")
	}
	pod := gangPod("team", "pf")
	pod.Name, pod.UID = "member-0", "member-0-uid"
	g := &GangPack{pins: pins}
	g.rememberAttempt(pod, token)
	state := newCycleState()
	writePin(state, "a", gangInfo{key: "team/pf", topologyKey: testKey}, token)

	g.PostBind(context.Background(), state, pod, "a1")

	if domain, currentToken, _, pinned := pins.GetOwned("team/pf"); !pinned || domain.Name != "a" || currentToken != token {
		t.Fatalf("PostBind changed gang pin: domain=%+v token=%d pinned=%v", domain, currentToken, pinned)
	}
	if got := g.attemptFor(pod); got != 0 {
		t.Fatalf("PostBind left pod attempt token %d", got)
	}
}
