// Package runtimerevision houses the GC loop that keeps the OME-managed
// ControllerRevisions in the OME namespace bounded.
//
// The reconcile is a small, deterministic plan computation:
//
//  1. Group OME-owned revisions by source runtime
//     (label ome.io/runtime-of).
//  2. Per group, the N most recent (by creation timestamp) are kept.
//  3. Revisions referenced by ANY ISVC (spec.runtime.revision or
//     status.pinnedRevisionName) are also kept, regardless of position.
//  4. Anything else is GC-eligible:
//     - First time observed eligible → set ome.io/gc-eligible-since.
//     - Already annotated AND annotation older than the grace period
//     → delete.
//     - Already annotated but became referenced again or rose back into
//     the top N → clear the annotation.
//
// The algorithm is a pure function over (revisions, referencedNames,
// now, retention, grace) so it's exhaustively unit-testable; the
// controller wraps it with the apiserver IO.
package runtimerevision

import (
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
)

// Action describes what the controller should do with one revision
// after the plan computation.
type Action int

const (
	// Keep: revision belongs in the retain set. The controller should
	// clear any gc-eligible-since annotation that's present (the
	// revision moved back into the keep set after being marked).
	Keep Action = iota
	// Mark: unreferenced + over retention + no annotation yet. The
	// controller sets gc-eligible-since=now.
	Mark
	// Wait: unreferenced + over retention + annotated, but grace
	// period not yet elapsed. The controller does nothing.
	Wait
	// Delete: unreferenced + over retention + annotated + grace
	// elapsed. The controller removes the revision.
	Delete
)

// Decision is the per-revision result of plan computation.
type Decision struct {
	Revision *appsv1.ControllerRevision
	Action   Action
	// HadAnnotation tracks whether the revision was already annotated;
	// helps the controller decide whether a Keep needs to issue a
	// status update to CLEAR the annotation or can be a no-op.
	HadAnnotation bool
}

// Plan computes per-revision GC decisions. Pure function — no IO.
// referencedNames is the set of revision names currently pinned by
// any ISVC (spec.runtime.revision or status.pinnedRevisionName).
func Plan(
	revisions []appsv1.ControllerRevision,
	referencedNames map[string]bool,
	now time.Time,
	retentionPerRuntime int,
	gracePeriod time.Duration,
	gcEligibleSinceKey string,
) []Decision {
	// Group by source runtime (label).
	byRuntime := map[string][]*appsv1.ControllerRevision{}
	for i := range revisions {
		key := revisions[i].Labels["ome.io/runtime-of"]
		byRuntime[key] = append(byRuntime[key], &revisions[i])
	}

	out := make([]Decision, 0, len(revisions))
	for _, group := range byRuntime {
		// Newest first.
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].CreationTimestamp.After(group[j].CreationTimestamp.Time)
		})
		for i, rev := range group {
			d := Decision{Revision: rev}
			_, d.HadAnnotation = rev.Annotations[gcEligibleSinceKey]

			keepByCount := i < retentionPerRuntime
			keepByRef := referencedNames[rev.Name]
			if keepByCount || keepByRef {
				d.Action = Keep
				out = append(out, d)
				continue
			}

			// Unreferenced + over retention.
			if !d.HadAnnotation {
				d.Action = Mark
				out = append(out, d)
				continue
			}
			sinceStr := rev.Annotations[gcEligibleSinceKey]
			since, err := time.Parse(time.RFC3339, sinceStr)
			if err != nil {
				// Corrupt annotation value — re-mark so we have a fresh,
				// parsable timestamp on the next pass.
				d.Action = Mark
				out = append(out, d)
				continue
			}
			if now.Sub(since) >= gracePeriod {
				d.Action = Delete
			} else {
				d.Action = Wait
			}
			out = append(out, d)
		}
	}
	return out
}
