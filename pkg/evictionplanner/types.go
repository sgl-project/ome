// Package evictionplanner builds deterministic, in-memory model eviction plans.
// The caller supplies global protection decisions and must freshly revalidate
// every selected CR before executing a plan. Build performs no Kubernetes I/O.
package evictionplanner

import (
	"time"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/artifactinventory"
)

// Input is one fixed snapshot. A zero Policy uses DefaultPolicy.
type Input struct {
	Now              time.Time
	Policy           ScoringPolicy
	Nodes            []Node
	EligibilityByUID map[types.UID]Eligibility
}

// Node watermarks are usage fractions: 0 <= LowWatermark < HighWatermark <= 1.
// A nil Inventory means mapping failed or was not evaluated, never an empty
// successful mapping. InventoryError optionally explains that failure.
type Node struct {
	Name           string
	CapacityBytes  int64
	AvailableBytes int64
	HighWatermark  float64
	LowWatermark   float64
	Inventory      *artifactinventory.Inventory
	InventoryError string
}

// Eligibility is the caller's global CR eligibility snapshot, including serving,
// claims and in-flight operations. Missing entries exclude the entire group.
// Missing, zero or future IdleSince is a soft highest-weight penalty.
type Eligibility struct {
	Eligible  bool
	Reason    string
	IdleSince *time.Time
}

// ScoringPolicy is versioned by the caller. Windows increase strictly; weights
// are positive and nonincreasing. Ages equal to a window enter the next band.
type ScoringPolicy struct {
	Version      string
	IdleWindows  [3]time.Duration
	Weights      [4]float64
	SizeScaleGiB float64
}

// DefaultPolicy returns the initial engineering policy, not a reuse probability.
func DefaultPolicy() ScoringPolicy {
	return ScoringPolicy{
		Version: "v1", IdleWindows: [3]time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 14 * 24 * time.Hour},
		Weights: [4]float64{8, 4, 2, 1}, SizeScaleGiB: 64,
	}
}

// GroupID compares lexicographically by node name, then normalized local path.
type GroupID struct {
	NodeName string
	Path     string
}

// ModelIdentity deliberately carries no stale execution authorization.
type ModelIdentity struct {
	Namespace string
	Name      string
	UID       types.UID
}

// Candidate is one eligible local artifact and all known CR references from the
// supplied Ready inventory, deduplicated by UID; this is not a live census.
// Cost = Weight * (len(PotentialNodes) + EstimatedBytes / GiB / SizeScaleGiB).
// InitialScore uses the local gap when this node is considered; zero means the
// candidate was already covered or the node needed no local choice. Greedy
// selection recomputes this score after each pick. Unknown IdleSince sorts last.
type Candidate struct {
	ID             GroupID
	Models         []ModelIdentity
	EstimatedBytes int64
	IdleSince      *time.Time
	Weight         float64
	PotentialNodes []string
	Cost           float64
	InitialScore   float64
}

// Alternative retains generated order after local redundancy pruning. Cost is
// a local comparison only; it must not be added across nodes as a global cost.
type Alternative struct {
	Strategy       string
	Groups         []GroupID
	PrunedGroups   []GroupID
	Cost           float64
	ReclaimedBytes int64
	Selected       bool
}

type ExcludedArtifact struct {
	ID     GroupID
	Reason string
}

// NodeResult includes final projections after batch pruning. Status is planned,
// skipped (cross-node relief below H), below_high_watermark, or unevaluated.
// RequiredBytes and RemainingShortfallBytes are zero for unevaluated and
// initially below-H nodes: they are not batch goals or claimed benefits.
type NodeResult struct {
	Name                    string
	Status                  string
	Reason                  string
	InitialUsedBytes        int64
	RequiredBytes           int64
	ProjectedUsedBytes      int64
	ProjectedReclaimedBytes int64
	RemainingShortfallBytes int64
	Candidates              []Candidate
	ExcludedArtifacts       []ExcludedArtifact
	Alternatives            []Alternative
}

// Plan is advisory. SelectedGroups preserve the winning local insertion order
// after reverse pruning; Models is the sorted union of the final groups' UIDs.
type Plan struct {
	EvaluatedAt    time.Time
	PolicyVersion  string
	SelectedGroups []Candidate
	Models         []ModelIdentity
	Nodes          []NodeResult
	PrunedGroups   []GroupID
}
