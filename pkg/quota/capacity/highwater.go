package capacity

import (
	"sort"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Track folds a fresh observation into a running high-water mark.
//
// Budget checks compare against the mark rather than the instantaneous value,
// because capacity dips for reasons that say nothing about how much hardware a
// tenant is entitled to: a drain, a rolling reboot, a device plugin restarting.
// Following those down would mark budgets Degraded — and, once materialization
// exists, freeze them — every time a rack was patched.
//
// Growth is believed immediately: new hardware is usable the moment it reports.
// A drop is believed only once it exceeds bandPercent of the mark, which is
// what distinguishes a decommission from a reboot. It is deliberately a
// magnitude band and not a duration: a fleet that genuinely shrank should stop
// claiming capacity it no longer has, and waiting a timeout to say so only
// delays the same conclusion.
//
// bandPercent at or below zero disables damping, so the mark follows the
// observation exactly — absent config disables rather than assuming a band. At
// 100 or above it never lowers the mark, which is what that number reads as.
//
// `observed` is installed capacity when called from TrackAll; see there for why
// it is not the schedulable figure.
func Track(mark, observed resource.Quantity, bandPercent int32) resource.Quantity {
	if observed.Cmp(mark) >= 0 {
		return observed
	}
	if bandPercent <= 0 {
		return observed
	}
	if bandPercent >= 100 {
		// The reading an operator typing 100 intends: never lower the mark.
		// Treating it as "no damping" would invert the knob at its extreme,
		// which is the one place a mistake is least likely to be noticed.
		return mark
	}
	// Compare observed*100 against mark*(100-band) so the threshold needs no
	// division and no rounding decision.
	scaled := new(inf.Dec).Mul(observed.AsDec(), inf.NewDec(100, 0))
	threshold := new(inf.Dec).Mul(mark.AsDec(), inf.NewDec(int64(100-bandPercent), 0))
	if scaled.Cmp(threshold) < 0 {
		return observed
	}
	return mark
}

// TrackAll folds a fresh capacity reading into the marks carried in status.
//
// The mark tracks **installed** capacity — allocatable plus what is parked on
// cordoned or NotReady nodes — not the schedulable figure. Tracking allocatable
// would defeat the point: draining a quarter of a fleet is the archetypal dip
// the mark exists to absorb, and it moves allocatable far past any sane band, so
// the mark would follow it down and budgets would Degrade during exactly the
// maintenance the damping was for. A cordon leaves installed capacity untouched,
// which is the honest reading — the hardware is still in the rack.
//
// Entries are keyed by (resource, flavor), the granularity Kueue keys quota by.
// A pair that has disappeared from the observation keeps its mark and reports
// zero allocatable: hardware that stops reporting is the other case the mark
// exists for, and dropping the entry would lose the only record that the pair
// was ever larger.
func TrackAll(previous []Mark, observed []Capacity, bandPercent int32) []Mark {
	type key struct{ resource, flavor string }
	marks := make(map[key]resource.Quantity, len(previous))
	order := make([]key, 0, len(previous)+len(observed))
	for _, p := range previous {
		k := key{p.ResourceName, p.ResourceFlavor}
		if _, dup := marks[k]; dup {
			continue
		}
		marks[k] = p.HighWaterMark
		order = append(order, k)
	}

	out := make([]Mark, 0, len(order)+len(observed))
	seen := map[key]struct{}{}
	for _, c := range observed {
		k := key{c.ResourceName, c.ResourceFlavor}
		out = append(out, Mark{
			ResourceName:   c.ResourceName,
			ResourceFlavor: c.ResourceFlavor,
			Allocatable:    c.Allocatable,
			HighWaterMark:  Track(marks[k], installed(c), bandPercent),
		})
		seen[k] = struct{}{}
	}
	for _, k := range order {
		if _, ok := seen[k]; ok {
			continue
		}
		// Carried forward so a pair that stops reporting still shows what it
		// once held — but dropped once the mark itself has reached zero, or a
		// renamed flavor would leave a dead entry in status for the life of the
		// cluster.
		mark := marks[k]
		if mark.IsZero() {
			continue
		}
		out = append(out, Mark{
			ResourceName:   k.resource,
			ResourceFlavor: k.flavor,
			HighWaterMark:  mark,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ResourceName != out[j].ResourceName {
			return out[i].ResourceName < out[j].ResourceName
		}
		return out[i].ResourceFlavor < out[j].ResourceFlavor
	})
	return out
}

// installed is what the pair physically has: schedulable now plus parked on a
// cordoned or NotReady node. Summing without mutating either input, because both
// belong to the caller's Capacity.
func installed(c Capacity) resource.Quantity {
	total := c.Allocatable.DeepCopy()
	total.Add(c.Unavailable)
	return total
}

// Mark is one (resource, flavor) pair's current and peak capacity.
type Mark struct {
	ResourceName   string
	ResourceFlavor string
	Allocatable    resource.Quantity
	HighWaterMark  resource.Quantity
}
