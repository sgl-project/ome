// Package usage rolls observed consumption up the AcceleratorQuota tree.
//
// A quota tree is described by three distinct numbers:
//
//   - spec.budgets, the allowance an admin authorized;
//   - the root's status.capacity, the installed hardware, derived by
//     pkg/quota/capacity;
//   - status.budgets, the amount workloads hold right now, computed here.
//
// All three are needed to diagnose a tenant whose work will not start. The
// cause may be its authorized limit, or accelerators that have left the
// cluster, or no pending demand at all — three states that are
// indistinguishable from any single one of the numbers.
//
// Consumption is summed upward from the leaves. Only a leaf materializes a
// queue that can hold anything; internal nodes render as topology alone, with
// no resourceGroups of their own. Every accelerator in use is therefore
// charged to exactly one leaf, so summing a subtree cannot count the same one
// twice.
//
// That last property depends on how OME renders the tree, not on any guarantee
// from the enforcement backend — a backend free to charge an internal node
// directly would break it. It is restated at each place a sum is taken rather
// than assumed once here.
package usage

import (
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/quota/tree"
)

// Key is the (resource, flavor) pair quota is keyed by.
type Key struct {
	ResourceName   string
	ResourceFlavor string
}

// Observed is one leaf queue's reading for one Key.
type Observed struct {
	// Admitted is held by workloads the backend has fully admitted, including
	// anything borrowed.
	Admitted resource.Quantity

	// Reserved is held by workloads holding a quota reservation, whether or not
	// they are admitted yet. It is never below Admitted, and the gap is work
	// that owns the chips but has not started — a tenant can sit at its ceiling
	// with Admitted low, which reads as idle unless this is carried too.
	Reserved resource.Quantity

	// Borrowed is the part of Admitted above this queue's own nominal. It is
	// the backend's own figure, measured against the queue it actually wrote.
	Borrowed resource.Quantity
}

// Total is one node's rolled-up reading.
type Total struct {
	Admitted resource.Quantity
	Reserved resource.Quantity
	Borrowed resource.Quantity
}

// Roll sums the leaf readings onto every node of the tree.
//
// leaves is keyed by node name; a node absent from it contributes nothing,
// which is how "no queue materialized yet" differs from "a queue holding
// nothing". The result carries an entry only for nodes with something to
// report, so an idle fleet allocates nothing and writes no status.
//
// Admitted and Reserved sum freely: each leaf's figure is disjoint from its
// siblings', because a leaf is the only thing that holds quota. Borrowed does
// not sum — see below.
func Roll(t *tree.Tree, leaves map[string]map[Key]Observed) map[string]map[Key]Total {
	if t == nil || len(leaves) == 0 {
		return nil
	}

	out := make(map[string]map[Key]Total)
	for _, node := range t.Nodes() {
		name := node.Name()

		var totals map[Key]Total
		for _, d := range t.Subtree(name) {
			if !d.IsLeaf() {
				continue
			}
			for key, o := range leaves[d.Name()] {
				if totals == nil {
					totals = make(map[Key]Total)
				}
				agg := totals[key]
				add(&agg.Admitted, o.Admitted)
				add(&agg.Reserved, o.Reserved)
				if node.IsLeaf() {
					// A leaf reports the backend's own borrowed figure. Above a
					// leaf the number is recomputed rather than summed: a loan
					// between two siblings inside this subtree is internal to
					// it, and adding the children's figures would report the
					// subtree as borrowing from outside itself when it is not.
					add(&agg.Borrowed, o.Borrowed)
				}
				totals[key] = agg
			}
		}
		if totals == nil {
			continue
		}
		if !node.IsLeaf() {
			borrowedAboveNominal(node, totals)
		}
		out[name] = totals
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// borrowedAboveNominal recomputes an internal node's borrowed figure as the
// admitted quantity above what that node was authored to hold.
//
// A node with no authored budget for a pair reports no borrowing rather than
// treating an absent budget as a nominal of zero: a grouping tier sized later
// is unconstrained on that pair, not instantly and entirely in debt.
func borrowedAboveNominal(node *tree.Node, totals map[Key]Total) {
	nominal := make(map[Key]resource.Quantity, len(node.Quota.Spec.Budgets))
	for _, b := range node.Quota.Spec.Budgets {
		nominal[Key{ResourceName: b.ResourceName, ResourceFlavor: b.ResourceFlavor}] = b.Nominal
	}
	for key, t := range totals {
		n, authored := nominal[key]
		if !authored {
			t.Borrowed = resource.Quantity{}
			totals[key] = t
			continue
		}
		over := t.Admitted.DeepCopy()
		over.Sub(n)
		if over.Sign() <= 0 {
			t.Borrowed = resource.Quantity{}
			totals[key] = t
			continue
		}
		t.Borrowed = over
		totals[key] = t
	}
}

// add accumulates into dst without mutating src, which belongs to the caller.
func add(dst *resource.Quantity, src resource.Quantity) {
	if dst.IsZero() && dst.Format == "" {
		*dst = src.DeepCopy()
		return
	}
	dst.Add(src)
}

// SortedKeys orders a node's pairs so two rolls of the same tree produce the
// same status and an idle fleet does not rewrite it every pass.
func SortedKeys(totals map[Key]Total) []Key {
	out := make([]Key, 0, len(totals))
	for k := range totals {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ResourceName != out[j].ResourceName {
			return out[i].ResourceName < out[j].ResourceName
		}
		return out[i].ResourceFlavor < out[j].ResourceFlavor
	})
	return out
}
