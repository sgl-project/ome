package placement

import (
	"testing"

	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

// TestReconcileReclaimsLeakedReservation is the reservation-leak invariant. A
// reservation drains per-Place only when a member is assumed, but a group can pin
// a domain (reserving gangSize) and then fail its scheduling cycle before any
// member is assumed — Filter rejects every candidate, so no Reserve/Unreserve runs
// and the reservation is never returned. It then subtracts capacity forever,
// starving other groups from a domain that is physically free. Reconcile, given
// the reservation the group can still justify against ground truth (gangSize minus
// its placed members), returns the leaked slots.
//
// Group A pins "a" reserving 3 but places none of its members (its cycle keeps
// failing), so a fresh group of 3 sees eff = 3 - 3 = 0 and cannot fit. Reconcile
// with A's true need of 0 (nothing placed, and the caller has decided the
// reservation is stale) returns the capacity, so the fresh group fits.
func TestReconcileReclaimsLeakedReservation(t *testing.T) {
	p := New()
	if d, ok := p.Choose("A", topology.FreeByDomain{"a": 3}, 3); !ok || d != "a" {
		t.Fatalf("A Choose = %q,%v want a,true", d, ok)
	}
	// Before reconcile the leaked reservation blocks a fresh group entirely.
	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 3); ok || d != "" {
		t.Fatalf("precondition: B Choose = %q,%v want \"\",false (leak blocks it)", d, ok)
	}

	p.Reconcile(map[string]int{"A": 0}) // A justifies no reservation now

	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 3); !ok || d != "a" {
		t.Fatalf("B Choose after reconcile = %q,%v want a,true (leaked reservation reclaimed)", d, ok)
	}
	// The reservation aggregate is truly gone, not merely masked: A holds no pin-time
	// reservation, so a third group also fits the (now smaller) free view.
	if _, pinned := p.Get("A"); !pinned {
		t.Fatal("Reconcile must not drop the pin itself, only shrink its reservation")
	}
}

// TestReconcileShrinksToPlacedRemainder: a partially-formed gang keeps exactly the
// reservation for members not yet placed. Reconcile to gangSize-placed leaves the
// live claim intact while returning the drained portion.
func TestReconcileShrinksToPlacedRemainder(t *testing.T) {
	p := New()
	p.Choose("A", topology.FreeByDomain{"a": 5}, 3) // reserves 3 in a
	// A placed 2 members (say via the snapshot); it still needs 1 reserved.
	p.Reconcile(map[string]int{"A": 1})
	// Any remaining heterogeneous claim owns the domain exclusively.
	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 2); ok || d != "" {
		t.Fatalf("B Choose = %q,%v want empty,false while A is still forming", d, ok)
	}
	p.Reconcile(map[string]int{"A": 0})
	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 2); !ok || d != "a" {
		t.Fatalf("B Choose after A commits = %q,%v want a,true", d, ok)
	}
}

// TestReconcileClampsNegative: a want below zero (more members placed than the
// gang size, e.g. a surplus pod) clamps to a zero reservation, never a negative
// that would inflate a domain's effective free.
func TestReconcileClampsNegative(t *testing.T) {
	p := New()
	p.Choose("A", topology.FreeByDomain{"a": 3}, 3)
	p.Reconcile(map[string]int{"A": -2})
	// A now reserves nothing; a fresh gang sees the full 3 free (not 3 - (-2) = 5).
	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 3); !ok || d != "a" {
		t.Fatalf("B Choose = %q,%v want a,true", d, ok)
	}
	if d, ok := p.Choose("C", topology.FreeByDomain{"a": 3}, 3); ok || d != "" {
		t.Fatalf("C Choose = %q,%v want \"\",false (negative reservation must not inflate free)", d, ok)
	}
}

// TestReconcileSkipsUnknownGroup: a group absent from want is untouched, so a
// partial view (some PodGroups unresolvable this pass) never reclaims a live
// reservation.
func TestReconcileSkipsUnknownGroup(t *testing.T) {
	p := New()
	p.Choose("A", topology.FreeByDomain{"a": 3}, 3)
	p.Reconcile(map[string]int{}) // A absent
	if d, ok := p.Choose("B", topology.FreeByDomain{"a": 3}, 1); ok || d != "" {
		t.Fatalf("B Choose = %q,%v want \"\",false (A's reservation preserved)", d, ok)
	}
}

// TestDomainsSnapshot: Domains reflects exactly the pinned groups and their
// domains, and drops a released group.
func TestDomainsSnapshot(t *testing.T) {
	p := New()
	p.Choose("A", topology.FreeByDomain{"a": 3}, 2)
	p.Choose("B", topology.FreeByDomain{"b": 3}, 2)
	d := p.Domains()
	if len(d) != 2 || d["A"] != "a" || d["B"] != "b" {
		t.Fatalf("Domains = %v, want {A:a, B:b}", d)
	}
	p.Release("A")
	if d := p.Domains(); len(d) != 1 || d["B"] != "b" {
		t.Fatalf("Domains after release = %v, want {B:b}", d)
	}
}
