package types

import (
	"time"
)

// ComponentPlan is the desired Component → Instance → Runner → Pod
// shape computed each reconcile from desired + observed state. Not
// persisted; policy fields hold the effective values after defaults.
type ComponentPlan struct {
	// Component identifies which of router / engine / decoder this plan
	// describes. Typed as the workload-side ComponentType so the
	// workload package stays free of v1beta1 imports; adapters convert
	// from v1beta1.ComponentType at the boundary.
	Component ComponentType

	// Replicas is the desired number of Instances for this Component
	// (= MinReplicas, or 1 when MinReplicas is unset).
	Replicas int32

	// Instances enumerates the desired Instances. Normally one entry
	// per index 0..Replicas-1; indices may be sparse while a surge
	// migration is in flight (the surge Instance carries an
	// out-of-band index until promotion settles the plan).
	Instances []InstancePlan

	// RestartPolicy is the effective Instance-group restart policy.
	RestartPolicy RestartPolicy

	// UpdateStrategy is the effective rollout policy across Instances.
	UpdateStrategy UpdateStrategy

	// ReadyPolicy is the effective Instance-level readiness aggregation.
	ReadyPolicy InstanceReadyPolicy

	// InstanceReadyTimeout is the wait ceiling on a newly-created
	// Instance becoming Ready.
	InstanceReadyTimeout time.Duration

	// MigrationMode is the effective migration disposition (auto / surge /
	// never).
	MigrationMode MigrationMode

	// Paused stops the dispatcher before it starts or advances Restart,
	// Migration, Update, or Create operations. Scale-down remains active so a
	// reduced desired replica count can still release capacity while paused.
	Paused bool

	// TopologyKey is the resolved gang co-location node-label key for
	// this Component (e.g. an NVLink/RDMA fabric-domain label). Empty for
	// single-pod Components or when unset. When non-empty on a multi-pod
	// Component, Render auto-generates the per-Instance worker→leader
	// podAffinity and the PodGroup carries the same key for topology-aware
	// gang schedulers. Both constraints therefore select the same configured
	// topology domain.
	TopologyKey string

	// InstanceTopologyKeys holds a temporary per-Instance override while live
	// pods still carry affinity rendered from an older revision. Missing pods in
	// that active gang must use the same immutable topology contract; fresh
	// Instances and surge indices fall back to TopologyKey.
	InstanceTopologyKeys map[int32]string
}

// TopologyKeyForInstance returns the live-safe topology contract for an
// Instance, falling back to the Component's current desired key when the group
// has no active revision-specific override.
func (p ComponentPlan) TopologyKeyForInstance(index int32) string {
	if key, ok := p.InstanceTopologyKeys[index]; ok {
		return key
	}
	return p.TopologyKey
}

// InstancePlan is the desired state for one Instance — the atomic
// unit of gang scheduling, restart, and migration.
type InstancePlan struct {
	// Index is the stable Instance ordinal. Sparse after surge
	// migration.
	Index int32

	// Incarnation is the monotonic lifecycle token for this
	// (Component, Index) pair. Initial create stamps 1; full recreate
	// / restart-on-loss bumps it; in-place update does not. Distinguishes
	// old pod materializations from current ones when stable names get
	// reused after a recreate.
	Incarnation int64

	// Runners enumerates the Runners that constitute this Instance.
	// Single-pod has one "default" Runner of size 1; multi-pod has
	// leader + worker.
	Runners []RunnerPlan

	// MigrationOverlay carries placement hints from an in-flight surge
	// migration. Render injects hard anti-affinity from the source node
	// and preferred affinity to hint target nodes. Nil for non-migration
	// paths.
	MigrationOverlay *MigrationOverlay

	// ExcludedNodes lists nodes this Instance's pods must not land on —
	// the relocation-directive memory (AutoRecover ledger entries)
	// projected through WorkloadObservedState.ExcludedNodesByInstance.
	// Render materializes each entry as the same required NotIn
	// hostname term the migration overlay uses. Empty for normal
	// Instances.
	ExcludedNodes []string

	// PeerHostnames is an optional render-time cache of the Instance's
	// ordered peer-DNS host list (one entry per pod, flat gang-rank
	// order). The list is identical for every pod in the gang, so the
	// gang-render loop computes it once and stashes it here to avoid the
	// O(gangsize^2) per-pod recompute. Nil when unset; Render then
	// derives the list from Runners as before. Transient render scratch,
	// not part of the desired state and never persisted.
	PeerHostnames []string
}

// MigrationOverlay records the per-Instance placement constraints a
// surge migration carries onto the rendered pod. Materialized as pod
// affinity in Render and intentionally absent from the canonical
// revision payload — transient operation overlay, not a steady-state
// template field.
type MigrationOverlay struct {
	// FromNode surfaces as RequiredDuringScheduling anti-affinity on
	// kubernetes.io/hostname.
	FromNode string
	// HintTargetNodes surface as PreferredDuringScheduling affinity;
	// the scheduler is free to pick another node when these are
	// unavailable.
	HintTargetNodes []string
}

// RunnerPlan is the desired state for one Runner within an Instance.
type RunnerPlan struct {
	// Name is "leader", "worker", or "default" (single-pod).
	Name string
	// Size is the number of pods of this Runner within the Instance.
	Size int32
}

// TotalPods returns the total desired pod count across all Runners —
// 1 for single-pod, 1+N for leader+worker.
func (i InstancePlan) TotalPods() int32 {
	var n int32
	for _, r := range i.Runners {
		n += r.Size
	}
	return n
}

// AllocateSurgeIndex returns the lowest int32 not present in any of
// the InstanceStatuses — the slot the migration op picks for its +1
// surge pod without colliding with steady-state indices or other
// in-flight surges.
func AllocateSurgeIndex(instances []InstanceStatus) int32 {
	used := make(map[int32]struct{}, len(instances)*2)
	for _, s := range instances {
		used[s.Index] = struct{}{}
		// Also exclude in-flight surge slots. A source mid-surge records its
		// replacement's index in Operation.SurgeIndex BEFORE the replacement
		// Instance exists (it's created a pass later, and within a single
		// reconcile pass a sibling that just claimed a slot has it recorded
		// here but not yet as a real Index). Without excluding these, every
		// Instance surging in the same pass collides on the same lowest-free
		// index; when that single shared surge becomes Ready the drain-on-ready
		// logic releases ALL sharing sources at once — a full-fleet wipe.
		if s.Operation != nil && s.Operation.SurgeIndex != nil {
			used[*s.Operation.SurgeIndex] = struct{}{}
		}
	}
	// Lowest-free exists at or below |used| (distinct ints), so an unbounded
	// scan terminates; keep it unbounded so the 2x-entry set can't off-by-one.
	for i := int32(0); ; i++ {
		if _, taken := used[i]; !taken {
			return i
		}
	}
}
