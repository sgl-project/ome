package query

import (
	"sort"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// HeldByPartition reports whether the hold candidate at rank (its position
// in PartitionHeldIndices' ascending candidate order) is held by the
// effective partition: the first partition candidates are held. A nil
// partition or a partition of zero or less holds nothing.
func HeldByPartition(partition *int32, rank int32) bool {
	if partition == nil || *partition <= 0 {
		return false
	}
	return rank < *partition
}

// PartitionHeldIndices returns the set of Instance indices the effective
// partition holds on their current revision, read off the observed rows
// given (a nil entry is an unobserved Instance, with nothing running to
// hold). The roll and every gate that counts a member's Instances read the
// held set through this one rule.
//
// Partition counts Instances to hold, not index values: migration and
// lowest-unused surge allocation leave sparse index sets (such as {1,2}
// for two replicas), so a raw index-below-partition compare holds the
// wrong count. The hold is keyed to revision membership, not position in
// the index set: gang surge places target-revision replacements at freed
// (often lower) indices, so a positional hold would let a replacement
// steal a held slot mid-roll and un-hold a still-old Instance, ending the
// roll with no old-revision Instance and the desired staged shape never
// reached.
//
// Hold candidates are the observed Instances off the target revision (an
// unset RunningRevision included; ReachedDesiredShape counts those as held
// too, and the two must agree on membership). Excluded from candidacy, so
// never held:
//   - Instances already converging to the target: an Update attempt in
//     flight (the row's Owner, gang-surge target markers included) or the
//     continuation of one (UpdateContinuation). An in-flight update must
//     finish: holding it strands its surge pod, which is never promoted to
//     serving and permanently consumes the surge budget, deadlocking the
//     roll on the non-held Instances. The hold falls to the next-lowest
//     old-revision Instance instead, preserving the count. A parked
//     attempt claims nothing and is held like the Failed row it stands
//     for, in either phase its pods give it; held with no pod left, the
//     Create pass rebuilds it on the revision it runs;
//   - transient migration surge Instances (a Migrate operation on a
//     non-Migrating phase): the pair's source carries the steady capacity.
//
// The lowest-indexed partition candidates are held.
func PartitionHeldIndices(partition *int32, observed []*workload.InstanceStatus, targetRevName string) map[int32]bool {
	held := make(map[int32]bool)
	if partition == nil || *partition <= 0 {
		return held
	}
	tgt := RevisionFromName(targetRevName)
	candidates := make([]int32, 0, len(observed))
	for _, s := range observed {
		if s == nil {
			continue
		}
		if RevisionFromName(s.RunningRevision).Same(tgt) {
			continue
		}
		if workload.Owner(s) == workload.OwnerUpdate || workload.UpdateContinuation(s) {
			continue
		}
		if s.Operation != nil && s.Operation.Type == workload.InstanceOperationMigrate && s.Phase != workload.InstancePhaseMigrating {
			continue
		}
		candidates = append(candidates, s.Index)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	for i, idx := range candidates {
		if !HeldByPartition(partition, int32(i)) {
			break
		}
		held[idx] = true
	}
	return held
}
