// plan_decision.go — the decision layer of the workload reconcile.
// Plan evaluates the pass triggers against the reconcile's single
// observation and produces a Decision; Execute (reconcile.go) applies
// it through the workload/ops state machines. Every SELECTION lives
// here, every EFFECT lives in Execute — so a Decision is unit-testable
// with a fake clock and recorder-instrumented callbacks.
package workload

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	workloadops "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/ops"
)

// ActionKind names one pass-level action of the reconcile pipeline.
type ActionKind string

const (
	// ActionScaleDown deletes the InstanceStatus indices the plan no
	// longer covers.
	ActionScaleDown ActionKind = "ScaleDown"
	// ActionRestart advances the Restart state machine for the selected
	// Instances.
	ActionRestart ActionKind = "Restart"
	// ActionMigrateExpiry consumes expired Manual migration records.
	ActionMigrateExpiry ActionKind = "MigrateExpiry"
	// ActionMigrate drives the selected Manual migration record.
	ActionMigrate ActionKind = "Migrate"
	// ActionUpdate runs the update pass over the selected Instances.
	ActionUpdate ActionKind = "Update"
	// ActionCreate materializes missing Instances (the op self-detects
	// missing pods per planned Instance).
	ActionCreate ActionKind = "Create"
)

// RestartSelection is one Instance the restart pass advances, with the
// trigger reason that lands on Operation.Reason on the first pass.
type RestartSelection struct {
	Instance InstancePlan
	Reason   string
}

// MigrationSelection is the Manual migration record the migrate pass
// drives this reconcile (a copy of the ObservedState record).
type MigrationSelection struct {
	Record MigrationRecord
}

// UpdateItem is one Instance the update pass touches, in plan order.
type UpdateItem struct {
	Instance InstancePlan
	// AdoptRevision: stamp Ready-on-target (the RunningRevision
	// backfill) instead of running the Update op — the Instance's
	// runtime-ready pods already match the target.
	AdoptRevision bool
	// StartingFresh: this Instance STARTS a new update rather than
	// continuing an in-flight one, so it is subject to the
	// per-Component budget and the coordination UpdateGate before the
	// op runs, and it charges the within-pass counters once dispatched.
	// An Instance whose in-flight UPDATE escalated to Phase=Failed
	// (Update operation preserved for retry) is a CONTINUATION of that
	// surge/drain: its in-flight pod already counts against the budget
	// (CurrentUnavailableInFlight charges Failed+Update), so charging
	// it again would project over budget and gate the recovery forever.
	// Failed with any OTHER preserved operation is not budget-charged
	// as in-flight and starts fresh — the exemption and the charge must
	// cover the same set, or unbudgeted recreates skew the gate.
	StartingFresh bool
	// CoordGateExempt: this fresh start skips the coordination
	// UpdateGate consult (the per-Component budget still applies). Set
	// for a Failed Instance with zero serving pods on a non-surge
	// strategy: the gate's unavailability accounting is serving-based
	// (desired - ServingReplicas), so the Instance's outage is already
	// inside the gate's current count — charging its own recreate +1 on
	// top projects over budget on every pass and starves the recovery
	// (nothing else can raise ServingReplicas; the denied recreate IS
	// the recovery). Surge strategies keep the consult: their gates
	// count surge pods, not serving loss, and the recreate genuinely
	// adds one.
	CoordGateExempt bool
	// CleanupOnly: the update trigger declined (zero revision distance —
	// the corrective roll-back — or a third-party-revision leftover) but
	// the instance carries superseded-revision wreckage
	// (ops.EvaluateWreckage). Execute routes the item into
	// ops.CleanupWreckage — abandon toward the current desired state —
	// instead of the Update op. Never budget-charged and never gated:
	// cleanup only removes dead pods and frees capacity, so gating it
	// would re-wedge the recovery the cleanup exists to unblock.
	CleanupOnly bool
}

// UpdateSelection carries the update pass's per-Instance selections
// plus the pure budget inputs the executor's within-pass counters
// project against.
type UpdateSelection struct {
	// Items in plan order. Instances held by RollingUpdate.Partition
	// (canary) and Instances with no trigger are not listed.
	Items []UpdateItem
	// Strategy is the resolved update strategy (empty Type defaults to
	// SurgeThenDrain — matches workload.BuildPlan's defaulting).
	// SurgeThenDrain doesn't take pods offline, so the executor gates
	// on the budget that actually applies to the strategy.
	Strategy UpdateStrategyType
	// SurgeBudget / UnavailBudget are the per-Component caps
	// (BudgetNoLimit when uncapped) — a distinct layer from the
	// cross-Component coordination-group gate (input.UpdateGate). Both
	// layers must allow before an Instance starts a fresh update; see
	// budget.go's package comment for the composition rule:
	//   effective_cap = min(group_cap, per_component_cap)
	SurgeBudget   int32
	UnavailBudget int32
	// PriorSurgeInFlight / PriorUnavailInFlight anchor the executor's
	// within-pass counters: operations in flight from prior wake-ups,
	// counted ONCE. Without this anchor the per-Component check would
	// re-count every iteration and double-charge the budget against
	// the same Instance.
	PriorSurgeInFlight   int32
	PriorUnavailInFlight int32
}

// PlannedAction is one pass-level action selected for this reconcile.
// Exactly the field matching Kind is populated.
type PlannedAction struct {
	Kind ActionKind

	// Extras are the scale-down target indices (ActionScaleDown).
	Extras []int32

	// Restarts are the Instances the restart pass advances
	// (ActionRestart).
	Restarts []RestartSelection

	// Migration is the record the migrate pass drives (ActionMigrate).
	Migration *MigrationSelection

	// Update is the update pass's selection (ActionUpdate).
	Update *UpdateSelection
}

// Decision is the outcome of Plan for one reconcile: the ordered
// pass-level actions to run. Ordering is the pass precedence
// (scale-down > restart > migration expiry > migration > update >
// create); Execute runs the actions in order and stops at the first
// one whose op outcome requires a requeue, so the ordering IS the
// precedence. The ops are step machines advanced once per reconcile —
// a Decision selects WHICH op advances for WHICH instances, not the
// op's internal effect sequence.
type Decision struct {
	// Actions are the selected pass-level actions,
	// first-precedence-first. A paused plan truncates the list after
	// scale-down: a deliberate replica reduction still releases
	// capacity, but no lifecycle operation that can create, replace,
	// or migrate pods is started or advanced.
	Actions []PlannedAction

	// RequeueAfter is the earliest not-yet-due Backoff RetryBlock
	// wake-up reported by the update-trigger evaluation — the one
	// requeue the decision layer owns without running an op. The
	// executor folds it additively into the pass result
	// (foldRetryAfter) so the gate is re-evaluated on time; it only
	// ever ADDS a wake-up, never delays one an op scheduled.
	RequeueAfter time.Duration

	// Escalate gates the terminal-failure escalation pass
	// (escalation.go). False while paused: escalation is suspended with the
	// rest of the lifecycle machinery (deadlines are parked by the adapter
	// while paused). The executor separately guards status-commit boundaries
	// and excludes deferred scale-down extras.
	Escalate bool
}

// Plan is the pure decision layer: it evaluates the pass triggers in
// precedence order and returns the Decision Execute applies.
//
// PURITY CONTRACT: Plan performs NO mutations of any kind — no client
// writes, no events, no MutateInstance / ApplyInstanceMutations /
// MutateRetryBlock / MutateMigration / AppendMigration / RemoveInstance
// calls, no expectation records. It reads ONLY the snapshot's pod buckets,
// input.ObservedState + input.DesiredSpec, and the injected clock
// (input.Now). input.UpdateGate is a decision input by contract, but
// Execute consults it at the update pass's position so the gate
// observes live peer/self state after earlier passes' effects have
// landed, exactly as the pass pipeline did.
func Plan(ctx context.Context, input ReconcileInput, plan ComponentPlan, target *appsv1.ControllerRevision, snapshot *ObservedSnapshot) (Decision, error) {
	var d Decision

	// Scale-down selection: indices observed but no longer planned.
	// Runs before everything else so excess pods don't run alongside
	// in-flight drains.
	if extras := ExtraInstanceIndices(input.ObservedState.InstanceStatuses, plan, false); len(extras) > 0 || hasDeleteOwnedInstance(input.ObservedState.InstanceStatuses) {
		d.Actions = append(d.Actions, PlannedAction{Kind: ActionScaleDown, Extras: extras})
	}

	// Paused is an operator circuit breaker: scale-down above still
	// releases capacity, but nothing after it is planned. Pod and spec
	// watches enqueue the workload again after unpause; no periodic
	// requeue is needed while the desired state is intentionally held.
	if plan.Paused {
		return d, nil
	}
	d.Escalate = true

	// Restart selection: per-Instance pod-loss / pod-Failed triggers,
	// evaluated against the LIVE pod read — restart is destructive and
	// must not select from a stale cache. In-flight restarts
	// (Phase=Restarting) re-select so their state machine advances.
	if plan.RestartPolicy == RestartPolicyRecreateInstance {
		liveByInstance, lerr := snapshot.LivePods(ctx)
		if lerr != nil {
			return Decision{}, fmt.Errorf("workload.Reconcile: list pods for restart pass (component=%s): %w", plan.Component, lerr)
		}
		var restarts []RestartSelection
		for _, inst := range plan.Instances {
			if needs, reason := workloadops.DetectRestartTriggerWithPods(input, plan, inst, liveByInstance[inst.Index]); needs {
				restarts = append(restarts, RestartSelection{Instance: inst, Reason: reason})
			}
		}
		if len(restarts) > 0 {
			d.Actions = append(d.Actions, PlannedAction{Kind: ActionRestart, Restarts: restarts})
		}
	}

	// Migration expiry selection — the deadline consumer. Planned
	// BEFORE the drive pass so an expired record is consumed before it
	// can be driven (re-stamped) again, and regardless of MigrationMode
	// so a mode flip to Never can never strand a non-terminal record.
	if workloadops.HasExpiredMigrationCandidate(input.ObservedState.Migrations, input.Now()) {
		d.Actions = append(d.Actions, PlannedAction{Kind: ActionMigrateExpiry})
	}

	// Migration drive selection. Work comes from the owner's
	// status.migrations records: the oldest non-terminal Manual record
	// is driven, one per pass. Terminal records and Auto records (born
	// terminal) are excluded structurally — records, never work. Skip
	// when mode is Never.
	if plan.MigrationMode != MigrationModeNever {
		if rec := NextManualMigration(input.ObservedState.Migrations); rec != nil {
			// A fresh migration adds one surge Instance. Let an existing update
			// surge finish first so the two operations do not stack extra capacity.
			// Allocated migrations always resume from their durable record.
			allocated := rec.SurgeInstance != nil && *rec.SurgeInstance >= 0
			if allocated || CurrentSurgeInFlight(input.ObservedState.InstanceStatuses) == 0 {
				d.Actions = append(d.Actions, PlannedAction{Kind: ActionMigrate, Migration: &MigrationSelection{Record: *rec}})
			}
		}
	}

	// Update selection. A nil target short-circuits the pass
	// (DesiredSpec.PodSpec nil / MinReplicas=0).
	if target != nil {
		sel, retryBlockWait, uerr := planUpdateSelection(ctx, input, plan, target, snapshot)
		if uerr != nil {
			return Decision{}, uerr
		}
		d.RequeueAfter = retryBlockWait
		if len(sel.Items) > 0 {
			d.Actions = append(d.Actions, PlannedAction{Kind: ActionUpdate, Update: &sel})
		}
	}

	// Create — always planned on a non-paused reconcile: the op is the
	// pass's own detector (it lists and diffs per planned Instance) and
	// a nil target returns immediately.
	d.Actions = append(d.Actions, PlannedAction{Kind: ActionCreate})

	return d, nil
}

// planUpdateSelection evaluates the per-Instance update triggers
// (pure) over the snapshot's cached pods and returns the selection
// plus the earliest not-yet-due RetryBlock wake-up across denied
// Instances.
func planUpdateSelection(ctx context.Context, input ReconcileInput, plan ComponentPlan, target *appsv1.ControllerRevision, snapshot *ObservedSnapshot) (UpdateSelection, time.Duration, error) {
	var rollingUpdate *RollingUpdate
	if plan.UpdateStrategy.RollingUpdate != nil {
		rollingUpdate = plan.UpdateStrategy.RollingUpdate
	}
	strategy := plan.UpdateStrategy.Type
	if strategy == "" {
		strategy = UpdateStrategySurgeThenDrain
	}
	sel := UpdateSelection{
		Strategy:             strategy,
		SurgeBudget:          PerComponentMaxSurgeBudget(rollingUpdate, plan.Replicas),
		UnavailBudget:        PerComponentMaxUnavailableBudget(rollingUpdate, plan.Replicas),
		PriorSurgeInFlight:   CurrentSurgeInFlight(input.ObservedState.InstanceStatuses),
		PriorUnavailInFlight: CurrentUnavailableInFlight(input.ObservedState.InstanceStatuses),
	}
	// One cached List + bucket for the whole Component (the snapshot's
	// non-destructive read source) — an absent bucket is an empty pod
	// set.
	updateByInstance, lerr := snapshot.CachedPods(ctx)
	if lerr != nil {
		return UpdateSelection{}, 0, fmt.Errorf("workload.Reconcile: list pods for update pass (component=%s): %w", plan.Component, lerr)
	}
	heldIndices := PartitionHeldIndices(rollingUpdate, input.ObservedState.InstanceStatuses, plan.Instances, target.Name)
	var retryBlockWait time.Duration
	for _, inst := range plan.Instances {
		// RollingUpdate.Partition (canary): hold the selected Instances
		// on their current revision — skip their update. Membership is
		// keyed to revision, not index position, and Instances already
		// converging to target are never held (they must finish); see
		// PartitionHeldIndices for the full candidacy rule.
		if heldIndices[inst.Index] {
			continue
		}
		dec := workloadops.EvaluateUpdateTrigger(input, inst, target, input.DesiredSpec.PodSpec, updateByInstance[inst.Index])
		if dec.AdoptRevision {
			sel.Items = append(sel.Items, UpdateItem{Instance: inst, AdoptRevision: true})
			continue
		}
		if !dec.Trigger {
			// Denied by a not-yet-due Backoff RetryBlock — keep the
			// earliest re-evaluation time across Instances.
			if dec.RetryAfter > 0 && (retryBlockWait == 0 || dec.RetryAfter < retryBlockWait) {
				retryBlockWait = dec.RetryAfter
			}
			// Wreckage scan: the trigger is revision-diff-keyed, so an
			// instance at zero revision distance (corrective roll-back)
			// or carrying third-party-revision debris never dispatches —
			// yet its superseded-revision wreckage must still be
			// abandoned toward the current desired state. Pure snapshot
			// read; the effect runs in Execute (ops.CleanupWreckage).
			if s := findObservedInstanceStatus(input.ObservedState.InstanceStatuses, inst.Index); workloadops.EvaluateWreckage(s, target, updateByInstance[inst.Index]) {
				sel.Items = append(sel.Items, UpdateItem{Instance: inst, CleanupOnly: true})
			}
			continue
		}
		s := findObservedInstanceStatus(input.ObservedState.InstanceStatuses, inst.Index)
		startingFresh := s == nil ||
			(s.Phase != InstancePhaseUpdating &&
				!(s.Phase == InstancePhaseFailed && s.Operation != nil && s.Operation.Type == InstanceOperationUpdate))
		gateExempt := startingFresh && strategy != UpdateStrategySurgeThenDrain &&
			s != nil && s.Phase == InstancePhaseFailed && s.ServingPodCount == 0
		sel.Items = append(sel.Items, UpdateItem{Instance: inst, StartingFresh: startingFresh, CoordGateExempt: gateExempt})
	}
	return sel, retryBlockWait, nil
}
