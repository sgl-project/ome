/*
InferenceReplica is the per-(InferenceService, Component) workload CRD
that owns the OMENative lifecycle subtree.

Access model: this is NOT a user-facing API. The InferenceService
controller is the sole writer of InferenceReplica specs; the
InferenceReplica controller is the sole writer of statuses. A
validating webhook (pkg/webhook/admission/inferencereplica) rejects
direct user writes that lack the ome.io/controller-write annotation.
Operators may kubectl get inferencereplicas for debugging.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// InferenceReplica is the per-Component workload abstraction for
// OMENative-managed InferenceService Components. One InferenceReplica
// exists per (ISVC, Component) tuple. The ISVC controller writes the
// spec; the InferenceReplica controller writes the status.
//
// The scale subresource lets HPA/KEDA target the InferenceReplica
// directly rather than indirecting through the parent ISVC.
//
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
// +kubebuilder:resource:path=inferencereplicas,shortName=irep,singular=inferencereplica
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component"
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type InferenceReplica struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceReplicaSpec   `json:"spec,omitempty"`
	Status InferenceReplicaStatus `json:"status,omitempty"`
}

// InferenceReplicaList contains a list of InferenceReplica objects.
//
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type InferenceReplicaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceReplica `json:"items"`
}

// InferenceReplicaSpec is the desired state of one (ISVC, Component)
// workload. The InferenceService controller is the sole writer; the
// admission webhook rejects writes from other actors that lack the
// ome.io/controller-write annotation.
type InferenceReplicaSpec struct {
	// ParentRef names the InferenceService that owns this replica.
	// Set by the ISVC controller at create time; immutable thereafter.
	ParentRef ParentReference `json:"parentRef"`

	// Component is one of engine | decoder | router. Immutable;
	// moving a workload between Component slots requires recreating
	// the InferenceReplica.
	Component ComponentType `json:"component"`

	// Replicas is the desired Instance count. The HPA / KEDA scale
	// subresource writes this field. Defaults to 1 when omitted.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// MinReadySeconds is propagated from the ISVC spec.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// TopologyKey is the resolved gang co-location node-label key for this
	// Component, projected verbatim from the effective ISVC↔runtime
	// component spec (spec.<component>.topologyKey, else the runtime
	// component-config value). When set on a multi-node Component, the
	// InferenceReplica controller auto-generates the per-Instance
	// worker→leader podAffinity that co-locates every worker onto its
	// gang's leader on a node sharing this label value. Nil means no
	// auto-generated gang affinity.
	// +optional
	TopologyKey *string `json:"topologyKey,omitempty"`

	// Runners is the fully-rendered set of pod templates per Instance.
	// MUST be non-empty. Single-pod Instances have one Runner with
	// Name="default" and Size=1; multi-node Instances typically have
	// Name="leader" (Size=1) plus Name="worker" (Size=N).
	//
	// The InferenceService controller is the sole writer of this
	// field. The InferenceReplica controller treats each
	// Runner.Template as opaque input (same contract
	// appsv1.Deployment.spec.template uses).
	//
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Runners []Runner `json:"runners"`

	// Lifecycle holds the OMENative lifecycle policies (RestartPolicy,
	// UpdateStrategy, ReadyPolicy, InstanceReadyTimeout,
	// MigrationPolicy). Reuses the existing LifecycleSpec
	// type so the projection from ISVC.spec.<component>.lifecycle is
	// a verbatim copy.
	// +optional
	Lifecycle *LifecycleSpec `json:"lifecycle,omitempty"`

	// Pacing is the InferenceService controller's projection of the
	// active RolloutCoordinationGroup pacing for this replica.
	// Written by the ISVC controller; read by the InferenceReplica
	// controller. Includes Partition (canary hold) and MaxUnavailable
	// (rollout budget). Nil means independent rollout.
	// +optional
	Pacing *InferenceReplicaPacing `json:"pacing,omitempty"`

	// Autoscaler is the live autoscaler configuration that downstream
	// scalers (HPA / KEDA / external) target. The ISVC controller projects
	// the per-Component autoscaler defaults from ISVC.spec.<component>.autoscaler
	// onto the corresponding IR at create time + on subsequent reconciles.
	// External autoscalers may also write directly
	// to this field via the /scale subresource without going through the
	// ISVC controller.
	// +optional
	Autoscaler *ComponentAutoscaler `json:"autoscaler,omitempty"`

	// Paused, when true, stops the InferenceReplica controller from
	// initiating any new Update / Restart / Create operations.
	// In-flight operations resume on unpause.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// RevisionHistoryLimit caps how many non-live ControllerRevisions
	// the InferenceReplica controller retains for this replica,
	// projected by the InferenceService controller from the parent
	// ISVC's ome.io/revision-history-limit annotation. Live revisions
	// (CurrentRevision / UpdateRevision and every per-Instance
	// running/target revision) are never deleted regardless of the
	// limit. Nil falls back to the operator-level
	// lifecycle.revisionHistoryLimit config; when that is also absent,
	// no revisions are pruned.
	// +optional
	// +kubebuilder:validation:Minimum=1
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
}

// ParentReference holds the InferenceService owner reference info.
// Distinct from the OwnerReference list because the InferenceReplica
// controller composes the parent's name into per-IR label and Service
// names without re-reading the metadata field.
type ParentReference struct {
	// Name is the name of the parent InferenceService.
	Name string `json:"name"`
}

// RunnerName is the enum of recognized Runner roles within an
// Instance. Matches the established Runner vocabulary so existing
// ome.io/runner label semantics carry through.
// +kubebuilder:validation:Enum=default;leader;worker
type RunnerName string

const (
	// RunnerNameDefault is the single-pod Instance Runner role.
	RunnerNameDefault RunnerName = "default"
	// RunnerNameLeader is the multi-node Instance leader Runner role.
	RunnerNameLeader RunnerName = "leader"
	// RunnerNameWorker is the multi-node Instance worker Runner role.
	RunnerNameWorker RunnerName = "worker"
)

// Runner is one logical pod role within an Instance.
//
// Single-pod Instances declare one Runner named "default" with Size=1.
// Multi-node Instances declare "leader" (Size=1) + "worker" (Size>=1).
//
// The revision hash includes every Runner's Template; a worker-only
// image bump still triggers a coordinated rollout across the whole
// Instance because tensor-parallel weights require leader+workers to
// move together.
type Runner struct {
	// Name is the Runner role within the Instance.
	Name RunnerName `json:"name"`

	// Size is the pod count for this Runner per Instance.
	// Constraints:
	//   - "default" Size MUST equal 1
	//   - "leader" Size MUST equal 1
	//   - "worker" Size MUST be >= 1
	// +kubebuilder:validation:Minimum=1
	Size int32 `json:"size"`

	// Template is the fully-rendered pod template for this Runner.
	// Labels include ome.io/runner=<name>. The InferenceReplica
	// controller stamps OME_RUNNER (leader|worker|default) and,
	// for non-leader pods, OME_LEADER_ADDRESS (short-form DNS,
	// matching LWS conventions) onto the rendered pod at create.
	Template corev1.PodTemplateSpec `json:"template"`
}

// InferenceReplicaPacing is the per-replica projection of the active
// RolloutCoordinationGroup pacing. Mirrors the corresponding
// RollingUpdate subset so the projection is a verbatim
// copy.
type InferenceReplicaPacing struct {
	// Partition holds back updates for Instances whose index is less
	// than Partition. Mirrors RollingUpdate.Partition. 0
	// (the default) updates all Instances. Used for canary holds.
	// +optional
	// +kubebuilder:validation:Minimum=0
	Partition *int32 `json:"partition,omitempty"`

	// MaxUnavailable caps in-rollout disruption. Accepts either a
	// raw count or a percent string. When nil, the InferenceReplica
	// controller falls back to its own default budget.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// RollbackToRevision, when set, names a ControllerRevision the
	// InferenceReplica must roll every Instance back to — overriding the
	// rendered desired template with that revision's stored pod template (and
	// using it as the update target). The InferenceService controller sets this
	// during a canary rollback so the forward-roll machinery drains the canary
	// pods back onto the stable revision. Empty in steady state.
	// +optional
	RollbackToRevision *string `json:"rollbackToRevision,omitempty"`
}

// InferenceReplicaStatus is the observed state of one
// InferenceReplica. The InferenceReplica controller is the sole
// writer. Fields mirror the LifecycleStatus shape so the ISVC's
// aggregated per-Component status is a verbatim projection of this
// block.
// +kubebuilder:validation:XValidation:rule="(!has(self.instanceStatusEncoding) && !has(self.instanceStatusColumns)) || (has(self.instanceStatusEncoding) && self.instanceStatusEncoding == 'ColumnarV2' && has(self.instanceStatusColumns) && !has(self.instanceStatuses))",message="instance status encoding and payload must form a valid DenseV1 or ColumnarV2 representation"
type InferenceReplicaStatus struct {
	// ObservedGeneration is the InferenceReplica.metadata.generation
	// the most recent status flush reflects.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the total Instance count (any phase).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of Instances whose every pod is Ready.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// ServingReplicas is the number of Instances whose every pod is
	// both ContainersReady AND has serving=True on the
	// controller-managed readiness gate (i.e., actually in the load
	// balancer rotation). MaxUnavailable budgets are computed
	// against this count, not ReadyReplicas, so "missing from
	// traffic" reflects reality during in-place updates.
	// +optional
	ServingReplicas int32 `json:"servingReplicas,omitempty"`

	// AvailableReplicas is the number of Instances that have been
	// Ready for at least MinReadySeconds.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// UpdatedReplicas is the number of Instances on UpdateRevision.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// UpdatedReadyReplicas is the number of Instances on
	// UpdateRevision that are fully Ready. UpdatedReadyReplicas /
	// Replicas drives MaxUnavailable rollout pacing.
	// +optional
	UpdatedReadyReplicas int32 `json:"updatedReadyReplicas,omitempty"`

	// CurrentRevision names the ControllerRevision currently serving
	// traffic. Set by the InferenceReplica controller.
	// +optional
	CurrentRevision string `json:"currentRevision,omitempty"`

	// UpdateRevision names the ControllerRevision being rolled out.
	// +optional
	UpdateRevision string `json:"updateRevision,omitempty"`

	// CollisionCount is the salt folded into ControllerRevision hash
	// input after a name collision is observed, so retries produce a
	// different revision name. Same pattern as StatefulSet.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`

	// LabelSelector identifies pods of this InferenceReplica for the
	// HPA scale subresource. Uses the same selector formula as the
	// Component's pod labels, so HPAs targeting either the ISVC or the
	// InferenceReplica resolve to the same pod set.
	// +optional
	LabelSelector string `json:"labelSelector,omitempty"`

	// InstanceStatuses is the DenseV1 per-Instance representation. It may be
	// present only when InstanceStatusEncoding is absent.
	// +optional
	// +listType=map
	// +listMapKey=index
	InstanceStatuses []OMENativeInstanceStatus `json:"instanceStatuses,omitempty"`

	// InstanceStatusEncoding selects a non-dense per-Instance representation.
	// Its absence identifies DenseV1.
	// +optional
	InstanceStatusEncoding *InstanceStatusEncoding `json:"instanceStatusEncoding,omitempty"`

	// InstanceStatusColumns contains the ColumnarV2 per-Instance
	// representation and is present together with InstanceStatusEncoding.
	// +optional
	InstanceStatusColumns *InstanceStatusColumns `json:"instanceStatusColumns,omitempty"`

	// RetryBlocks records per-target-revision retry authority for update
	// attempts. Revision-scoped: one block per failed target revision,
	// shared by every Instance attempting it — allocating a new Instance
	// cannot evade the block.
	// +optional
	// +listType=map
	// +listMapKey=targetRevision
	RetryBlocks []RetryBlock `json:"retryBlocks,omitempty"`

	// Migrations is the authoritative set of migration records for
	// this InferenceReplica — one entry per request UUID. Manual
	// entries are resumable processes born Accepted; Auto entries are
	// born-terminal Relocated records. Executors select on
	// non-terminal phase only — terminal entries are records, never
	// work. Non-terminal entries are never trimmed; terminal entries
	// are pruned once older than the migration-capacity rate window,
	// so the list stays bounded. Full history remains in the audit
	// ledger and events.
	// +optional
	// +listType=map
	// +listMapKey=requestUUID
	// +patchStrategy=merge
	// +patchMergeKey=requestUUID
	Migrations []MigrationStatus `json:"migrations,omitempty" patchStrategy:"merge" patchMergeKey:"requestUUID"`

	// Conditions reports InferenceReplica-level conditions. Defined
	// types include Available, Progressing, FailedScale,
	// FailedUpdate, GangSchedulingUnavailable, OperationStuck.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Traffic reports per-revision traffic weights for this
	// InferenceReplica. Each entry corresponds to one revision
	// currently receiving traffic. The ISVC controller mirrors this
	// onto ISVC.status.components[c].traffic[] for the
	// HTTPRoute builder.
	// +optional
	// +listType=map
	// +listMapKey=revisionName
	Traffic []ComponentTrafficTarget `json:"traffic,omitempty"`

	// CoordinationGroupRef back-references the
	// RolloutCoordinationGroup this replica is currently bound into
	// (if any). Lets operators find the owning coord group from the
	// child without parsing the ISVC spec.
	// +optional
	CoordinationGroupRef string `json:"coordinationGroupRef,omitempty"`
}

// RetryBlockState is the retry authority state for one target revision.
// +kubebuilder:validation:Enum=Backoff;Held;RetryInProgress
type RetryBlockState string

const (
	// RetryBlockBackoff: attempts remain; the next one is authorized at
	// NextRetryAt.
	RetryBlockBackoff RetryBlockState = "Backoff"
	// RetryBlockHeld: attempts exhausted (or retry unconfigured). Only a
	// new target revision releases the hold.
	RetryBlockHeld RetryBlockState = "Held"
	// RetryBlockRetryInProgress: exactly one retry attempt is authorized
	// and running.
	RetryBlockRetryInProgress RetryBlockState = "RetryInProgress"
)

// RetryBlock is the persisted per-revision retry authority. The
// subject is the owning IR plus TargetRevision.
type RetryBlock struct {
	// TargetRevision is the ControllerRevision name this block scopes.
	TargetRevision string `json:"targetRevision"`

	State RetryBlockState `json:"state"`

	// AttemptsStarted counts lifecycle attempts at this revision.
	// Kubelet container restarts are diagnostic and not counted here.
	AttemptsStarted int32 `json:"attemptsStarted"`

	// NextRetryAt is the persisted resume time while State=Backoff.
	// Never recomputed from process-local history (restart-safe).
	// +optional
	NextRetryAt *metav1.Time `json:"nextRetryAt,omitempty"`

	// +optional
	FirstFailureAt *metav1.Time `json:"firstFailureAt,omitempty"`
	// +optional
	LastFailureAt *metav1.Time `json:"lastFailureAt,omitempty"`

	// Reason is the operator-facing last-failure summary.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// MigrationTrigger identifies who initiated a migration entry.
// +kubebuilder:validation:Enum=Manual;Auto
type MigrationTrigger string

const (
	// MigrationTriggerManual marks an operator-requested migration.
	// Manual entries are resumable processes: born Accepted, they
	// walk the phase chain to a terminal phase.
	MigrationTriggerManual MigrationTrigger = "Manual"
	// MigrationTriggerAuto marks a controller-initiated relocation.
	// Auto entries are born-terminal Relocated records — visibility
	// mirrors of a relocation directive, never resumable work.
	MigrationTriggerAuto MigrationTrigger = "Auto"
)

// MigrationPhase is the lifecycle phase of a migration.
//
// Manual migrations walk Accepted -> SurgePending -> SurgeReady ->
// Draining -> Completed | Failed. Auto relocations are born terminal
// (Relocated). Terminal phases are Completed, Failed, and Relocated —
// executors select work on non-terminal phase only; terminal entries
// are records, never work.
//
// Pending and InProgress are legacy values used only by the ISVC
// status migrationHistory window.
// +kubebuilder:validation:Enum=Accepted;SurgePending;SurgeReady;Draining;Completed;Failed;Relocated;Pending;InProgress
type MigrationPhase string

const (
	// MigrationPhaseAccepted: the request was validated and admitted;
	// no surge Instance is allocated yet.
	MigrationPhaseAccepted MigrationPhase = "Accepted"
	// MigrationPhaseSurgePending: the surge Instance is allocated and
	// waiting to become Ready.
	MigrationPhaseSurgePending MigrationPhase = "SurgePending"
	// MigrationPhaseSurgeReady: the surge Instance is Ready; the
	// source may begin draining.
	MigrationPhaseSurgeReady MigrationPhase = "SurgeReady"
	// MigrationPhaseDraining: the source Instance is draining out of
	// rotation.
	MigrationPhaseDraining MigrationPhase = "Draining"
	// MigrationPhaseCompleted: terminal — the migration succeeded.
	MigrationPhaseCompleted MigrationPhase = "Completed"
	// MigrationPhaseFailed: terminal — the migration did not complete.
	MigrationPhaseFailed MigrationPhase = "Failed"
	// MigrationPhaseRelocated: terminal — a born-terminal Auto
	// relocation record.
	MigrationPhaseRelocated MigrationPhase = "Relocated"

	// MigrationPhasePending is a legacy migrationHistory-only value.
	MigrationPhasePending MigrationPhase = "Pending"
	// MigrationPhaseInProgress is a legacy migrationHistory-only value.
	MigrationPhaseInProgress MigrationPhase = "InProgress"
)

// Terminal reports whether p is a terminal phase (Completed, Failed,
// or Relocated). Executors select migrations on non-terminal phase
// only — terminal entries are records, never work.
func (p MigrationPhase) Terminal() bool {
	switch p {
	case MigrationPhaseCompleted, MigrationPhaseFailed, MigrationPhaseRelocated:
		return true
	}
	return false
}

// MigrationStatus is one migration request's authoritative record on
// the InferenceReplica. status.migrations is the single source of
// truth for migration work: executors resume from non-terminal
// entries; the audit ledger and events are history, not work.
//
// Lifecycle contract:
//   - Manual entries (Trigger=Manual) are resumable processes: born
//     Accepted, they walk Accepted -> SurgePending -> SurgeReady ->
//     Draining -> Completed | Failed.
//   - Auto entries (Trigger=Auto) are born-terminal Relocated records
//     mirroring a relocation directive; Succeeded is stamped post-hoc
//     when the relocated Instance next reaches Ready.
//   - Executors select on non-terminal phase only — terminal entries
//     are records, never work.
type MigrationStatus struct {
	// RequestUUID uniquely identifies the migration request. Manual:
	// supplied by the requester. Auto: generated by the controller.
	RequestUUID string `json:"requestUUID"`

	// Trigger records who initiated the migration.
	Trigger MigrationTrigger `json:"trigger"`

	// SourceInstance is the Instance index being migrated away from.
	SourceInstance int32 `json:"sourceInstance"`

	// SurgeInstance is the Instance index allocated for the surge
	// replacement. Manual only; nil until allocated. Distinct from
	// index 0, which is a valid surge index.
	// +optional
	SurgeInstance *int32 `json:"surgeInstance,omitempty"`

	// AllocatedAt is when the surge Instance index was allocated —
	// the start of execution. Nil while the entry is queued (Accepted
	// but not yet picked up by the executor). The capacity gate counts
	// execution from this stamp, never queued intent.
	// +optional
	AllocatedAt *metav1.Time `json:"allocatedAt,omitempty"`

	// FromNode is the node the source is being moved off.
	// Auto: the excluded node.
	// +optional
	FromNode string `json:"fromNode,omitempty"`

	// HintTargetNodes are requester-supplied preferred placement
	// targets for the surge replacement. Rendered as preferred (soft)
	// node affinity; the scheduler may pick other nodes.
	// +optional
	// +listType=atomic
	HintTargetNodes []string `json:"hintTargetNodes,omitempty"`

	// Phase is the migration lifecycle phase. See MigrationPhase for
	// the Manual chain, the Auto born-terminal rule, and the terminal
	// set.
	Phase MigrationPhase `json:"phase"`

	// Attempt is the relocation attempt ordinal.
	// Auto: N of autoMigrate.maxAttempts.
	// +optional
	Attempt int32 `json:"attempt,omitempty"`

	// Reason is the requester-supplied reason (Manual) or the
	// disposition branch that produced the entry (Auto).
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message describes the current blocker (non-terminal) or the
	// terminal outcome.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt is when the entry was created.
	StartedAt metav1.Time `json:"startedAt"`

	// Deadline is when a non-terminal entry expires.
	// Manual: accept time plus InstanceReadyTimeout.
	Deadline metav1.Time `json:"deadline"`

	// CompletedAt is when the entry reached a terminal phase.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Succeeded reports the post-hoc outcome. Auto: stamped true when
	// the relocated Instance next reaches Ready.
	// +optional
	Succeeded *bool `json:"succeeded,omitempty"`
}

func init() {
	SchemeBuilder.Register(&InferenceReplica{}, &InferenceReplicaList{})
}
