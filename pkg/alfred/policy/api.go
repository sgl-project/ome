// Package policy defines the contract every Alfred policy implements: a
// pure, side-effect-free function of the ClusterSnapshot that returns ranked
// Candidates (OEP-0008 §The engine). A policy holds no client and emits no
// Event, metric, or ConfigMap entry — the engine routes its output: executable
// Candidates enter the Arbiter, advisory ones go straight to the Reporter.
package policy

import (
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// Candidate reasons (event-facing; the dispatcher maps them to the wire
// contract's lowercase reason values).
const (
	ReasonFragmentation     = "Fragmentation"
	ReasonNodeUnhealthy     = "NodeUnhealthy"
	ReasonRemediationSignal = "RemediationSignal"
)

// Advisory reasons: why an emitted Candidate is not executable. Advisory
// candidates surface through the Reporter only — they never enter arbitration
// and never consume migration budget.
const (
	// AdvisoryNoSurgeHeadroom: the surge-shaped replacement footprint fits
	// on no feasible target while the source still holds its GPUs.
	AdvisoryNoSurgeHeadroom = "NoSurgeHeadroom"
	// AdvisoryVolumePinned: an RWO/RWOP PVC pins the workload to its node;
	// no migration mechanism can move it.
	AdvisoryVolumePinned = "VolumePinned"
	// AdvisoryLWSMigrationUnsupported: LWS tears down the whole group on
	// pod restart with no surge protection; the safe fix (move the
	// workload to OMENative) is the operator's.
	AdvisoryLWSMigrationUnsupported = "LWSMigrationUnsupported"
	// AdvisoryRawDeploymentMigrationUnsupported: Alpha has no truthful
	// RawDeployment executor; every otherwise observable Raw Instance is
	// surfaced to the operator without entering arbitration.
	AdvisoryRawDeploymentMigrationUnsupported = "RawDeploymentMigrationUnsupported"
	// AdvisoryOMENativeUnavailable: no OMENative executor exists on this
	// cluster, so the migration verb has no consumer.
	AdvisoryOMENativeUnavailable = "OMENativeUnavailable"
	// AdvisoryOMENativeObservationInvalid: the checked IR/Pod join is
	// missing, stale, or structurally inconsistent.
	AdvisoryOMENativeObservationInvalid = "OMENativeObservationInvalid"
	// AdvisoryOMENativeStateIneligible: the checked OMENative state is not
	// fully steady or the workload is already busy with a transition.
	AdvisoryOMENativeStateIneligible = "OMENativeStateIneligible"
	// AdvisoryNonExecutableObservedFragmentation: fragmentation is visible
	// to the operator but cannot authorize a positive executable move.
	AdvisoryNonExecutableObservedFragmentation = "NonExecutableObservedFragmentation"
	// AdvisoryMigrationSurfaceDisabled: the component's execution surface
	// is switched off in alfred-config.
	AdvisoryMigrationSurfaceDisabled = "MigrationSurfaceDisabled"
	// AdvisoryModelUnresolved: model availability could not be resolved
	// (see ModelAvailability.ResolveError); the policy treats the model as
	// having no feasible target and surfaces the reason.
	AdvisoryModelUnresolved = "ModelUnresolved"
)

// ComponentWideInstance marks a Candidate that addresses a whole component
// rather than one Instance (advisories such as VolumePinned or the LWS
// recommendation). Component-wide candidates are never dispatched.
const ComponentWideInstance int32 = -1

// NodeRemediation is the typed desired-state payload carried by a node-health
// marker Candidate. It identifies the node incident, preserves the complete
// structured health observation, lists every affected OME GPU workload whose
// identity is known, and separately preserves unresolved OME GPU occupancy so
// Reporter never mistakes "identity unavailable" for "node drained."
type NodeRemediation struct {
	Node                   string
	Health                 snapshot.NodeHealthObservation
	Workloads              []string
	OMEGPUOccupantsPresent bool
}

// Candidate is a single proposed action — "migrate Component X's Instance Y
// off Node Z" — with explicit, cross-policy-comparable benefit and cost so
// the Arbiter can rank on a common axis instead of trusting each policy's
// self-assessment.
type Candidate struct {
	// Policy is the emitting policy's Name().
	Policy string

	Workload  types.NamespacedName
	Component v1beta1.ComponentType
	// Instance is the addressed Instance index, or ComponentWideInstance.
	Instance int32
	// Mode is the component's resolved deployment mode, which determines
	// the execution surface.
	Mode constants.DeploymentModeType

	// Reason is the event-facing cause (Fragmentation, NodeUnhealthy, ...).
	Reason string
	// Remediation is set only on node-health marker Candidates. Workload
	// findings leave it nil and use the ordinary Candidate action fields.
	Remediation *NodeRemediation

	// FromNode is the policy-selected source node for the finding. Defrag
	// selects the member holding the largest GPU share; Node Health selects
	// the lexicographically first actually unhealthy member.
	FromNode string
	// HintTargetNodes is the bounded, ranked policy-supplied target set for
	// operator-facing reporting. The scheduler still makes the final pod-level
	// placement decision.
	HintTargetNodes []string
	// PlacementTargetNodes is an internal exhaustive, ranked target set for
	// replaying a policy's successful atomic placement during arbitration. It
	// is not part of operator-facing reports. Empty means the Arbiter falls
	// back to HintTargetNodes for compatibility with pluggable policies.
	PlacementTargetNodes []string

	// Executable=false marks an advisory finding; AdvisoryReason says why.
	Executable     bool
	AdvisoryReason string

	// SurgeShaped records the simulated execution shape. Defragmentation
	// has one executable Alpha shape: OMENative place-then-free surge.
	SurgeShaped bool
	// FootprintGPUs is the instance's GPU footprint. For a surge-shaped
	// move this much headroom must exist while the source still holds its
	// GPUs; it is also the ranking tie-break (smaller moves first).
	FootprintGPUs int64

	// Benefit is the expected improvement (F_observed_before minus
	// F_observed_after on the simulated post-move state).
	Benefit float64
	// Cost is the disruption risk keyed off the migration mode.
	Cost float64
	// Score is the policy-defined rank within a priority class. Defrag uses
	// Benefit-CostWeight*Cost with optional emergency/source boosts; Node
	// Health uses the workload's numeric priority directly.
	Score float64
	// Emergency marks a candidate whose move unblocks a pending pod older
	// than emergencyPendingAgeMinutes.
	Emergency bool
}

// Policy is a pluggable decision module: a pure function of the snapshot.
type Policy interface {
	Name() string
	Evaluate(snap *snapshot.ClusterSnapshot, cfg *config.Config) []Candidate
}
