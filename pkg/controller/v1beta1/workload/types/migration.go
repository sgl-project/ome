package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workload-side mirror of the InferenceReplica MigrationStatus entry.
// The adapter converts field-for-field (like RetryBlock / InstanceStatus);
// workload code never sees the CRD type. status.migrations on the owner
// is the single source of truth for migration work: the dispatcher
// selects work from non-terminal Manual records and the Migrate executor
// resumes from the record's SurgeInstance + Phase.

// MigrationTrigger identifies who initiated a migration record.
type MigrationTrigger string

const (
	// MigrationTriggerManual marks an operator-requested migration —
	// a resumable process born Accepted.
	MigrationTriggerManual MigrationTrigger = "Manual"
	// MigrationTriggerAuto marks a controller-initiated relocation —
	// a born-terminal Relocated record, never resumable work.
	MigrationTriggerAuto MigrationTrigger = "Auto"
)

// MigrationPhase is the lifecycle phase of a migration record. Manual
// records walk Accepted -> SurgePending -> SurgeReady -> Draining ->
// Completed | Failed; Auto records are born terminal (Relocated).
type MigrationPhase string

const (
	MigrationPhaseAccepted     MigrationPhase = "Accepted"
	MigrationPhaseSurgePending MigrationPhase = "SurgePending"
	MigrationPhaseSurgeReady   MigrationPhase = "SurgeReady"
	MigrationPhaseDraining     MigrationPhase = "Draining"
	MigrationPhaseCompleted    MigrationPhase = "Completed"
	MigrationPhaseFailed       MigrationPhase = "Failed"
	MigrationPhaseRelocated    MigrationPhase = "Relocated"
)

// Terminal reports whether p is a terminal phase. Executors and the
// dispatcher select work on non-terminal phase only — terminal records
// are records, never work.
func (p MigrationPhase) Terminal() bool {
	switch p {
	case MigrationPhaseCompleted, MigrationPhaseFailed, MigrationPhaseRelocated:
		return true
	}
	return false
}

// migrationPhaseRank orders the Manual phase chain for forward-only
// advancement. Terminal phases rank above every transient phase. An
// unrecognized phase has no rank and reports ok=false.
func migrationPhaseRank(p MigrationPhase) (int, bool) {
	switch p {
	case MigrationPhaseAccepted:
		return 0, true
	case MigrationPhaseSurgePending:
		return 1, true
	case MigrationPhaseSurgeReady:
		return 2, true
	case MigrationPhaseDraining:
		return 3, true
	case MigrationPhaseCompleted, MigrationPhaseFailed, MigrationPhaseRelocated:
		return 4, true
	default:
		return 0, false
	}
}

// MigrationPhaseAtOrPast reports whether p has already reached (or
// passed) the given phase in the Manual chain — the guard the executor's
// forward-only phase advancement uses so a stale write can never move a
// record backward.
//
// An unrecognized phase on either side reports false. Neither answer is
// knowable for a phase outside the chain, and false is the recoverable
// one: it lets the executor drive the record forward, where true would
// report every advancement as already done and wedge it permanently.
func MigrationPhaseAtOrPast(p, target MigrationPhase) bool {
	pRank, pOK := migrationPhaseRank(p)
	targetRank, targetOK := migrationPhaseRank(target)
	if !pOK || !targetOK {
		return false
	}
	return pRank >= targetRank
}

// MigrationRecord mirrors v1beta1.MigrationStatus field-for-field.
type MigrationRecord struct {
	// RequestUUID uniquely identifies the migration request.
	RequestUUID string

	Trigger MigrationTrigger

	// SourceInstance is the Instance index being migrated away from.
	SourceInstance int32

	// SurgeInstance is the allocated surge Instance index; nil until
	// the executor allocates it (0 is a valid surge index).
	SurgeInstance *int32

	// AllocatedAt is when the surge index was allocated — execution
	// start. Nil while the record is queued (Accepted, not yet picked
	// up). Capacity counts execution from this stamp.
	AllocatedAt *metav1.Time

	// FromNode is the node the source is being moved off.
	FromNode string

	// HintTargetNodes are preferred placement targets for the surge.
	HintTargetNodes []string

	Phase MigrationPhase

	// Attempt is the relocation attempt ordinal (Auto records).
	Attempt int32

	// Reason is the requester-supplied reason (Manual) or disposition
	// branch (Auto).
	Reason string

	// Message describes the current blocker (non-terminal) or the
	// terminal outcome.
	Message string

	StartedAt metav1.Time

	// Deadline is when a non-terminal record expires.
	Deadline metav1.Time

	CompletedAt *metav1.Time

	Succeeded *bool
}

// FindMigrationRecord returns a pointer to the record for requestUUID
// (aliasing the slice element), or nil.
func FindMigrationRecord(records []MigrationRecord, requestUUID string) *MigrationRecord {
	for i := range records {
		if records[i].RequestUUID == requestUUID {
			return &records[i]
		}
	}
	return nil
}

// NextManualMigration selects the migration the dispatcher should drive
// this pass: the oldest-StartedAt Manual record whose phase is
// non-terminal. Auto records are excluded structurally — born terminal,
// they never rank. Returns nil when no work exists.
func NextManualMigration(records []MigrationRecord) *MigrationRecord {
	var picked *MigrationRecord
	for i := range records {
		r := &records[i]
		if r.Trigger != MigrationTriggerManual || r.Phase.Terminal() {
			continue
		}
		if picked == nil || r.StartedAt.Time.Before(picked.StartedAt.Time) {
			picked = r
		}
	}
	return picked
}
