// Package topology holds the pure, framework-free placement accounting for
// ome-scheduler: per-domain free-capacity counting and the best-fit domain
// choice. Keeping it free of the scheduler framework is deliberate — this is
// the core of placement, unit-tested in isolation; the plugin wires the live
// node/pod state into these types.
package topology

import "sort"

// FreeByDomain maps a domain (the value of the configured topology label) to its
// count of free whole nodes. Whole-node accounting is the model: a gang pod
// occupies an entire node, so capacity is just a node count and any free node in
// a domain is interchangeable (true for an NVLink clique; true for a TPU slice
// when the gang uses the whole slice).
type FreeByDomain map[string]int

// BestFit returns the domain that best fits a gang of gangSize whole nodes: the
// domain with the FEWEST free nodes that still has at least gangSize free. This
// is the packing rule — concentrate work into already-busy domains and keep
// empty domains whole for future large gangs. Ties (equal free counts) break on
// domain name so the choice is deterministic and explainable.
//
// ok is false when no domain can hold the gang, or when gangSize <= 0 (a gang
// always has at least one member; a non-positive size is a caller bug, not a
// fit).
func BestFit(free FreeByDomain, gangSize int) (domain string, ok bool) {
	if gangSize <= 0 {
		return "", false
	}

	// Iterate domains in name order so ties resolve deterministically to the
	// lowest name.
	names := make([]string, 0, len(free))
	for d := range free {
		names = append(names, d)
	}
	sort.Strings(names)

	best := ""
	bestFree := 0
	for _, d := range names {
		f := free[d]
		if f < gangSize {
			continue // domain can't hold the whole gang
		}
		if best == "" || f < bestFree {
			best, bestFree = d, f
		}
	}
	return best, best != ""
}
