package placement

import (
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

// TestChooseAssignsThenFollows is the core assignment contract: the first pod of a
// placement group chooses a domain via best-fit; every later call for the same
// group returns that assigned domain regardless of current free counts — the
// group is committed. This is what makes per-pod scheduling a group decision.
func TestChooseAssignsThenFollows(t *testing.T) {
	p := New()
	d, ok := p.Choose("svc", topology.FreeByDomain{"a": 18, "b": 6, "c": 4}, 4)
	if !ok || d != "c" { // fullest that fits
		t.Fatalf("first Choose = %q,%v want c,true", d, ok)
	}
	// A later member follows the assignment even though c isn't even offered now.
	d2, ok := p.Choose("svc", topology.FreeByDomain{"a": 18}, 4)
	if !ok || d2 != "c" {
		t.Fatalf("follow Choose = %q,%v want c,true (assigned)", d2, ok)
	}
}

// TestChooseNoFitLeavesNoAssignment: when nothing fits and the group isn't assigned yet, no
// domain is returned and no assignment is recorded (the gang waits, unassigned).
func TestChooseNoFitLeavesNoAssignment(t *testing.T) {
	p := New()
	if d, ok := p.Choose("svc", topology.FreeByDomain{"a": 2}, 4); ok || d != "" {
		t.Fatalf("no-fit Choose = %q,%v want \"\",false", d, ok)
	}
	if _, assigned := p.Get("svc"); assigned {
		t.Fatal("group must not be assigned when nothing fit")
	}
}

// TestSetRebuildsAssignmentOnFailover: Set seeds an assignment without best-fit — used on leader
// failover to rebuild "group -> domain" from where bound pods already sit.
func TestSetRebuildsAssignmentOnFailover(t *testing.T) {
	p := New()
	p.Set("svc", "b")
	// Would best-fit to "a", but the rebuilt assignment wins.
	if d, ok := p.Choose("svc", topology.FreeByDomain{"a": 18}, 4); !ok || d != "b" {
		t.Fatalf("Choose after Set = %q,%v want b,true", d, ok)
	}
}

// TestReleaseAllowsRechoose: Release drops the assignment (gang completed, or timed out
// and unwound), so the next Choose re-runs best-fit and may pick differently.
func TestReleaseAllowsRechoose(t *testing.T) {
	p := New()
	if d, _ := p.Choose("svc", topology.FreeByDomain{"a": 8, "b": 4}, 4); d != "b" {
		t.Fatalf("initial Choose = %q want b", d)
	}
	p.Release("svc")
	if _, assigned := p.Get("svc"); assigned {
		t.Fatal("Release must forget the assignment")
	}
	if d, ok := p.Choose("svc", topology.FreeByDomain{"a": 8}, 4); !ok || d != "a" {
		t.Fatalf("re-Choose after Release = %q,%v want a,true", d, ok)
	}
}

// TestChooseReservesAgainstOtherGangs: a gang's assignment reserves its whole-node
// capacity, so a second gang best-fits over what's LEFT — it won't pile into a
// domain the first gang has already committed to but not yet filled. Without the
// reservation both gangs would pick the same fullest domain and over-commit it.
func TestChooseReservesAgainstOtherGangs(t *testing.T) {
	p := New()
	raw := topology.FreeByDomain{"d": 3, "e": 3}

	// Gang A takes d (tie d/e -> "d") and reserves 3 there.
	if d, ok := p.Choose("A", raw, 3); !ok || d != "d" {
		t.Fatalf("A Choose = %q,%v want d,true", d, ok)
	}
	// Gang B sees d as full (3 raw - 3 reserved = 0) and must pick e instead —
	// not d again, which would over-commit the domain.
	if d, ok := p.Choose("B", raw, 3); !ok || d != "e" {
		t.Fatalf("B Choose = %q,%v want e,true (d reserved by A)", d, ok)
	}
}

// TestReservationBlocksWhenNoRoomLeft: if the only fitting domain is fully
// reserved by another gang, the newcomer gets no fit and stays unassigned — it
// waits rather than over-committing.
func TestReservationBlocksWhenNoRoomLeft(t *testing.T) {
	p := New()
	raw := topology.FreeByDomain{"d": 3}
	if d, ok := p.Choose("A", raw, 3); !ok || d != "d" {
		t.Fatalf("A Choose = %q,%v want d,true", d, ok)
	}
	if d, ok := p.Choose("B", raw, 1); ok || d != "" {
		t.Fatalf("B Choose = %q,%v want \"\",false (d fully reserved)", d, ok)
	}
}

// TestPlaceDrainsReservationNoDoubleCount: as a gang's members land, Place drains
// its reservation so a later gang isn't charged twice — once by the shrinking
// reservation and once by the members now real in the (smaller) raw free.
func TestPlaceDrainsReservationNoDoubleCount(t *testing.T) {
	p := New()
	// A domain of 5, gang A of 3 takes it (reserves 3). Room for a gang of 2 too.
	if d, _ := p.Choose("A", topology.FreeByDomain{"d": 5}, 3); d != "d" {
		t.Fatalf("A Choose = %q want d", d)
	}
	// A's three members land: raw free drops to 2, reservation drains to 0.
	p.Place("A")
	p.Place("A")
	p.Place("A")
	// A new gang of 2 sees the real 2 free (2 raw - 0 reserved), not 2 - 3 = -1.
	if d, ok := p.Choose("C", topology.FreeByDomain{"d": 2}, 2); !ok || d != "d" {
		t.Fatalf("C Choose = %q,%v want d,true (reservation drained, no double count)", d, ok)
	}
}

// TestPlaceFloorsAtZero: Place never drives a reservation negative, and is a
// no-op on an unassigned group.
func TestPlaceFloorsAtZero(t *testing.T) {
	p := New()
	p.Place("ghost") // unassigned: no panic, no effect
	p.Choose("A", topology.FreeByDomain{"d": 1}, 1)
	p.Place("A")
	p.Place("A") // already drained; must not go negative
	// A gang of 1 sees the full 1 free (no negative reservation inflating it).
	if d, ok := p.Choose("B", topology.FreeByDomain{"d": 1}, 1); !ok || d != "d" {
		t.Fatalf("B Choose = %q,%v want d,true", d, ok)
	}
}

// TestReleaseFreesReservation: releasing a gang returns its reserved capacity, so
// a later gang can best-fit into the domain again.
func TestReleaseFreesReservation(t *testing.T) {
	p := New()
	raw := topology.FreeByDomain{"d": 3}
	p.Choose("A", raw, 3) // reserves all of d
	p.Release("A")
	if d, ok := p.Choose("B", raw, 3); !ok || d != "d" {
		t.Fatalf("B Choose after release = %q,%v want d,true", d, ok)
	}
}

// TestSetHoldsNoReservation: a failover-rebuilt assignment reflects already-bound pods
// (real occupancy in the snapshot), so it holds no reservation and doesn't shrink
// what another gang sees.
func TestSetHoldsNoReservation(t *testing.T) {
	p := New()
	p.Set("A", "d") // rebuilt: A's pods already bound in d
	// B best-fits over raw free unmodified by any A reservation.
	if d, ok := p.Choose("B", topology.FreeByDomain{"d": 3}, 3); !ok || d != "d" {
		t.Fatalf("B Choose = %q,%v want d,true (Set holds no reservation)", d, ok)
	}
}

// TestRetainOnlyReleasesDeadGangs: the GC hook drops the assignments of gangs absent from
// the live set and returns their reserved capacity, while keeping live ones.
func TestRetainOnlyReleasesDeadGangs(t *testing.T) {
	p := New()
	p.Choose("live", topology.FreeByDomain{"a": 5}, 2) // reserves 2 in a
	p.Choose("dead", topology.FreeByDomain{"b": 5}, 2) // reserves 2 in b

	if released := p.RetainOnly(map[string]bool{"live": true}); released != 1 {
		t.Fatalf("RetainOnly released %d, want 1", released)
	}
	if _, ok := p.Get("dead"); ok {
		t.Fatal("dead gang's assignment should be released")
	}
	if _, ok := p.Get("live"); !ok {
		t.Fatal("live gang's assignment must be retained")
	}
	// dead's reservation was returned, so b can be claimed again.
	if d, ok := p.Choose("new", topology.FreeByDomain{"b": 5}, 3); !ok || d != "b" {
		t.Fatalf("new Choose = %q,%v want b,true (dead reservation reclaimed)", d, ok)
	}
}

// TestAssignedGroupFollows verifies repeat decisions for one gang. Partner gang
// identities are rejected by the plugin until a multi-gang reservation contract
// is implemented.
func TestAssignedGroupFollows(t *testing.T) {
	p := New()
	free := topology.FreeByDomain{"x": 20, "y": 8}
	d1, _ := p.Choose("svc-a", free, 8)  // prefill gang -> y (fullest that fits 8)
	d2, ok := p.Choose("svc-a", free, 4) // decode gang, same group, smaller
	if !ok || d2 != d1 {
		t.Fatalf("partner Choose = %q,%v want %q (co-located via shared group)", d2, ok, d1)
	}
}

func TestReservationsConflictOnOverlappingPhysicalNodes(t *testing.T) {
	p := New()
	if d, _, ok := p.ChooseInTopologyOnNodes("A", "fabric-a", topology.FreeByDomain{"a": 2}, map[string][]string{"a": {"n1", "n2"}}, 2); !ok || d != "a" {
		t.Fatalf("A Choose = %q,%v want shared,true", d, ok)
	}
	if d, _, ok := p.ChooseInTopologyOnNodes("B", "fabric-b", topology.FreeByDomain{"b": 2}, map[string][]string{"b": {"n1", "n2"}}, 1); ok || d != "" {
		t.Fatalf("B Choose = %q,%v want no fit; topology keys overlap the same physical nodes", d, ok)
	}
	if d, _, ok := p.ChooseInTopologyOnNodes("C", "fabric-b", topology.FreeByDomain{"c": 2}, map[string][]string{"c": {"n3", "n4"}}, 1); !ok || d != "c" {
		t.Fatalf("C Choose = %q,%v want c,true for disjoint physical nodes", d, ok)
	}
}

func TestReleaseIfRejectsStaleCommitmentOwner(t *testing.T) {
	p := New()
	_, first, ok := p.ChooseInTopologyOnNodes("gang", "fabric", topology.FreeByDomain{"a": 1}, map[string][]string{"a": {"n1"}}, 1)
	if !ok {
		t.Fatal("first assignment failed")
	}
	if !p.ReleaseIf("gang", first, nil) {
		t.Fatal("first owner should release its commitment")
	}
	_, second, ok := p.ChooseInTopologyOnNodes("gang", "fabric", topology.FreeByDomain{"b": 1}, map[string][]string{"b": {"n2"}}, 1)
	if !ok || second == first {
		t.Fatal("retry must receive a distinct ownership token")
	}
	if p.ReleaseIf("gang", first, nil) {
		t.Fatal("stale owner released the retry commitment")
	}
	if domain, _, _, assigned := p.GetOwned("gang"); !assigned || domain.Name != "b" {
		t.Fatalf("retry assignment = %+v,%v, want b,true", domain, assigned)
	}
}

func TestCommitmentDoesNotMatchRecreatedOwner(t *testing.T) {
	p := New()
	_, oldToken, ok := p.ChooseForOwnerInTopologyOnNodes("team/pf", "old-uid", "fabric",
		topology.FreeByDomain{"a": 1}, map[string][]string{"a": {"n1"}}, 1)
	if !ok {
		t.Fatal("old owner failed to take a domain")
	}
	_, _, _, found, ownerMatch := p.GetOwnedBy("team/pf", "new-uid")
	if !found || ownerMatch {
		t.Fatalf("GetOwnedBy recreated owner = found %v match %v, want true,false", found, ownerMatch)
	}
	if _, _, ok := p.ChooseForOwnerInTopologyOnNodes("team/pf", "new-uid", "fabric",
		topology.FreeByDomain{"b": 1}, map[string][]string{"b": {"n2"}}, 1); ok {
		t.Fatal("new owner inherited old same-name commitment")
	}
	if !p.ReleaseIf("team/pf", oldToken, nil) {
		t.Fatal("old owned commitment should be releasable")
	}
}

func TestReleaseIfSerializesSiblingRejectionBeforeReassignment(t *testing.T) {
	p := New()
	_, token, ok := p.ChooseInTopologyOnNodes("gang", "fabric", topology.FreeByDomain{"a": 1}, map[string][]string{"a": {"n1"}}, 1)
	if !ok {
		t.Fatal("initial assignment failed")
	}
	callbackStarted := make(chan struct{})
	allowCallbackReturn := make(chan struct{})
	releaseDone := make(chan struct{})
	go func() {
		p.ReleaseIf("gang", token, func() {
			close(callbackStarted)
			<-allowCallbackReturn
		})
		close(releaseDone)
	}()
	<-callbackStarted

	reassignDone := make(chan struct{})
	reassignStarted := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(reassignStarted)
		p.ChooseInTopologyOnNodes("gang", "fabric", topology.FreeByDomain{"b": 1}, map[string][]string{"b": {"n2"}}, 1)
		close(reassignDone)
	}()
	<-reassignStarted
	select {
	case <-reassignDone:
		t.Fatal("retry assigned before old-attempt sibling rejection completed")
	case <-time.After(25 * time.Millisecond):
	}
	close(allowCallbackReturn)
	<-releaseDone
	wg.Wait()
	if domain, _, _, assigned := p.GetOwned("gang"); !assigned || domain.Name != "b" {
		t.Fatalf("retry assignment = %+v,%v, want b,true", domain, assigned)
	}
}

// TestChooseRecordsSoleFit: a commitment remembers whether the planner had a
// choice. Two free domains leave one; a single fitting domain, a second domain
// held by another forming gang, or an assignment rebuilt from bound members do not. A
// stale id reads no commitment at all.
func TestChooseRecordsSoleFit(t *testing.T) {
	nodes := map[string][]string{"a": {"a1", "a2"}, "b": {"b1", "b2"}}

	p := New()
	_, id, ok := p.ChooseInTopologyOnNodes("team/pf", "k", topology.FreeByDomain{"a": 2, "b": 2}, nodes, 2)
	if !ok || p.SoleFit("team/pf", id) {
		t.Fatalf("two fitting domains: ok=%v soleFit=%v, want assigned with a choice", ok, p.SoleFit("team/pf", id))
	}
	if p.SoleFit("team/pf", id+1) {
		t.Fatal("a stale id must not read the commitment's sole fit")
	}

	p = New()
	_, id, ok = p.ChooseInTopologyOnNodes("team/pf", "k", topology.FreeByDomain{"a": 2, "b": 1}, nodes, 2)
	if !ok || !p.SoleFit("team/pf", id) {
		t.Fatalf("one fitting domain: ok=%v soleFit=%v, want assigned without a choice", ok, p.SoleFit("team/pf", id))
	}

	p = New()
	if _, _, ok := p.ChooseInTopologyOnNodes("team/other", "k", topology.FreeByDomain{"b": 2}, nodes, 2); !ok {
		t.Fatal("failed to reserve b for another gang")
	}
	_, id, ok = p.ChooseInTopologyOnNodes("team/pf", "k", topology.FreeByDomain{"a": 2, "b": 2}, nodes, 2)
	if !ok || !p.SoleFit("team/pf", id) {
		t.Fatalf("other domain reserved: ok=%v soleFit=%v, want assigned without a choice", ok, p.SoleFit("team/pf", id))
	}

	p = New()
	id = p.SetReservationOnNodes("team/pf", "k", "a", 1, nodes["a"])
	if !p.SoleFit("team/pf", id) {
		t.Fatal("an assignment rebuilt from bound members must report no alternative")
	}
}
