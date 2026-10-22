package coordination

import (
	"time"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ComponentObservation captures the inputs the state machine reads for
// one Component in a group: replica counts, per-revision pod counts,
// in-flight failure / pause hints. Populated by the caller from
// the status aggregate plus live pod reads.
type ComponentObservation struct {
	// Component identifies this observation.
	Component v1beta1.ComponentType

	// DesiredReplicas is the replica's status.replicas: the Instance rows it
	// manages, in any phase. Zero until its status is first written.
	DesiredReplicas int32

	// SpecReplicas is the Instance count the replica's spec asks for now, the
	// operator's latest intent (minReplicas, or an autoscaler's count), read
	// as at least one since a count of zero cannot anchor a ratio. Zero only
	// while the replica does not exist yet.
	SpecReplicas int32

	// TotalPods is the live pod count, any revision.
	TotalPods int32

	// ReadyPods is the live pod count reporting Ready=True.
	ReadyPods int32

	// ServingPods is the live pod count actually in load-balancer
	// rotation (serving=True). Diverges from ReadyPods during in-place
	// drains where the serving gate is flipped before containers
	// restart. RatioBalanced math projects against this — operators
	// care about live traffic capacity, not rollout progress.
	ServingPods int32

	// NewRevisionPods is the live pod count on the target
	// ControllerRevision (UpdateRevision in LifecycleStatus
	// terms). When zero, the Component is not yet surging.
	NewRevisionPods int32

	// NewRevisionReadyPods is the subset of NewRevisionPods that
	// reports Ready=True.
	NewRevisionReadyPods int32

	// TargetRevisionHash is the ControllerRevision hash the Component
	// is converging toward. Empty when no rollout is in flight (the
	// Component is already at its desired revision).
	TargetRevisionHash string

	// CurrentRevisionHash is the ControllerRevision hash the
	// Component is currently serving.
	CurrentRevisionHash string

	// RolloutInFlight is true when DesiredRevisionHash !=
	// CurrentRevisionHash. The state machine routes off this flag
	// rather than recomputing it from the two hashes so callers can
	// override (e.g., for tests).
	RolloutInFlight bool

	// Partition is the component's desired static rollingUpdate.partition
	// (instances with index < Partition are held on the prior revision).
	// 0 = full rollout. Resolved only for non-canary groups; canary-owned
	// components leave it 0 (they use the canary reconciler's own path).
	Partition int32

	// AtDesiredShape is true when the component rests at its staged shape
	// (status.ReachedDesiredShape): every Instance Ready, the ones still on
	// the prior revision held by the partition, at most Partition of them
	// and every one of them when fewer than Partition remain. The state
	// machine rests (Staged) and Sequential hands off on this.
	AtDesiredShape bool

	// ReadyAt is the instant the Component's replica last reported its roll
	// done: the Ready condition's last transition to True, which the replica
	// writes once every Instance is Ready on the target or, under a
	// partition, at its staged shape. Zero while the condition is not True.
	// The Sequential wait after this Component is measured from it, the one
	// clock the replica-side gate reads too.
	ReadyAt time.Time

	// Failed is true when the Component's OMENative reconciler
	// flagged a terminal failure (Ready timeout, persistent op error).
	Failed bool

	// FailureMessage is the operator-facing explanation when Failed.
	FailureMessage string
}

// GroupObservation collects per-Component observations plus group-wide
// hints the state machine reads.
type GroupObservation struct {
	// Group is the resolved coordination group.
	Group ResolvedGroup

	// Components maps Component → observation. Includes every
	// Component listed in Group.Components.
	Components map[v1beta1.ComponentType]ComponentObservation

	// OriginalReplicas is the snapshot the RatioBalanced pacing math
	// reads. Snapshotted at rollout start; nil when no rollout has
	// started yet.
	OriginalReplicas map[v1beta1.ComponentType]int32

	// PausedGlobal is true when the operator paused the whole
	// rollout via the ome.io/rollout-paused annotation on the ISVC
	// (either accepted value — pause and freeze hold coordination
	// identically). The state machine honors this by leaving every
	// group in Paused until the annotation is removed.
	PausedGlobal bool

	// Now is the reference time the state machine compares against
	// when evaluating time-based gates (Sequential soak). Caller
	// passes time.Now() at the top of the reconcile pass so all gates
	// see the same instant.
	Now time.Time

	// PreviousPhaseEnteredAt is the timestamp the group last entered
	// its current Phase, as recorded in the previously-written status.
	// The phase-dwell metric reads it; no transition is decided on it,
	// since it moves on every base-phase change. Zero when no prior
	// status is available (first reconcile).
	PreviousPhaseEnteredAt time.Time
}

// GroupTransition is the state machine's decision for one group this
// reconcile. The caller applies the decision (writes status, surges
// pods, drains, etc.) without re-reading the state.
type GroupTransition struct {
	// Phase is the next base phase.
	Phase v1beta1.CoordinationPhase

	// CompositePhase is the operator-facing composite phase string
	// (e.g., `decoder.Surging` for Sequential).
	CompositePhase string

	// CurrentComponent names the Component the group is actively
	// reconciling (Sequential only).
	CurrentComponent v1beta1.ComponentType

	// PreviousComponent names the most recently completed Component
	// (Sequential only).
	PreviousComponent v1beta1.ComponentType

	// Message is the operator-facing explanation of the phase.
	Message string

	// RatioSkewRejected is true when at least one Component's surge
	// budget was reduced to zero by RatioBalanced pacing this reconcile.
	// Surfaces the EvaluateSurge SkewRejected signal so callers can
	// emit the RatioSkewRejected event + increment the ratio_skew_total
	// counter. Always false for PerComponent pacing.
	RatioSkewRejected bool
}

// ComputeTransition walks the group's state machine for this reconcile
// and returns the decision. Pure function — no I/O, no status writes.
// Status writers translate the decision; tests assert against this
// shape directly.
func ComputeTransition(obs GroupObservation) GroupTransition {
	if obs.PausedGlobal {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhasePaused,
			CompositePhase: string(v1beta1.CoordinationPhasePaused),
			Message:        "rollout paused via ome.io/rollout-paused annotation",
		}
	}
	switch obs.Group.Policy {
	case v1beta1.CoordinationPolicyBlueGreen:
		return blueGreenTransition(obs)
	case v1beta1.CoordinationPolicyRollingUpdate:
		return rollingUpdateTransition(obs)
	case v1beta1.CoordinationPolicySequential:
		return sequentialTransition(obs)
	case v1beta1.CoordinationPolicyIndependent:
		fallthrough
	default:
		return independentTransition(obs)
	}
}

// blueGreenTransition implements the gang-style state machine: surge all
// Components together, wait for all Ready, shift traffic together,
// drain, scale down. Failure in any Component fails the whole group.
func blueGreenTransition(obs GroupObservation) GroupTransition {
	allFailed := false
	for _, c := range obs.Components {
		if c.Failed {
			allFailed = true
			break
		}
	}
	if allFailed {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseFailed,
			CompositePhase: string(v1beta1.CoordinationPhaseFailed),
			Message:        "one or more Components in the BlueGreen group failed",
		}
	}

	rolling := false
	for _, c := range obs.Components {
		if c.RolloutInFlight {
			rolling = true
			break
		}
	}
	if !rolling {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseIdle,
			CompositePhase: string(v1beta1.CoordinationPhaseIdle),
			Message:        "no rollout in flight",
		}
	}

	allHaveSurge := true
	allNewReady := true
	for _, c := range obs.Components {
		if c.NewRevisionPods == 0 {
			allHaveSurge = false
		}
		if c.NewRevisionReadyPods < c.NewRevisionPods || c.NewRevisionPods == 0 {
			allNewReady = false
		}
	}

	switch {
	case !allHaveSurge:
		_, skew := computeSurgeBudgetWithRatio(obs)
		return GroupTransition{
			Phase:             v1beta1.CoordinationPhaseSurging,
			CompositePhase:    string(v1beta1.CoordinationPhaseSurging),
			Message:           "creating new-revision pods",
			RatioSkewRejected: skew,
		}
	case !allNewReady:
		_, skew := computeSurgeBudgetWithRatio(obs)
		return GroupTransition{
			Phase:             v1beta1.CoordinationPhaseWaiting,
			CompositePhase:    string(v1beta1.CoordinationPhaseWaiting),
			Message:           "waiting for new-revision pods to be Ready",
			RatioSkewRejected: skew,
		}
	}

	// Converged to a partitioned target: every component reached its desired
	// staged shape and at least one intentionally holds old-revision pods.
	// Rest at Staged instead of churning Shifting/ScalingDown forever.
	stagedAll, anyPartition := true, false
	for _, c := range obs.Components {
		if !c.AtDesiredShape {
			stagedAll = false
			break
		}
		if c.Partition > 0 {
			anyPartition = true
		}
	}
	if stagedAll && anyPartition {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseStaged,
			CompositePhase: string(v1beta1.CoordinationPhaseStaged),
			Message:        "converged to partitioned target; holding old-revision pods by design",
		}
	}

	// All Components have surge pods AND every surge pod is Ready.
	// Walk Shifting → Draining → ScalingDown. We collapse the
	// post-Ready phases into a single Shifting return here so the
	// status writer (which sees the traffic delta) can advance to
	// Draining/ScalingDown based on whether the producer has
	// already written and whether old pods still exist.
	oldStillUp := false
	for _, c := range obs.Components {
		if c.TotalPods > c.NewRevisionPods {
			oldStillUp = true
			break
		}
	}
	if oldStillUp {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseShifting,
			CompositePhase: string(v1beta1.CoordinationPhaseShifting),
			Message:        "shifting traffic; old-revision pods still serving",
		}
	}
	return GroupTransition{
		Phase:          v1beta1.CoordinationPhaseScalingDown,
		CompositePhase: string(v1beta1.CoordinationPhaseScalingDown),
		Message:        "scaling down old-revision pods",
	}
}

// rollingUpdateTransition implements RollingUpdate: per-Component
// state machines walk in parallel; the group phase is the union (any
// Component still surging means the group is Surging; otherwise
// Waiting → Shifting → ScalingDown → Idle).
func rollingUpdateTransition(obs GroupObservation) GroupTransition {
	failed := false
	for _, c := range obs.Components {
		if c.Failed {
			failed = true
			break
		}
	}
	if failed {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseFailed,
			CompositePhase: string(v1beta1.CoordinationPhaseFailed),
			Message:        "one or more Components in the RollingUpdate group failed",
		}
	}

	anyRolling := false
	for _, c := range obs.Components {
		if c.RolloutInFlight {
			anyRolling = true
			break
		}
	}
	if !anyRolling {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseIdle,
			CompositePhase: string(v1beta1.CoordinationPhaseIdle),
			Message:        "no rollout in flight",
		}
	}

	anySurging := false
	anyWaiting := false
	anyScalingDown := false
	for _, c := range obs.Components {
		if !c.RolloutInFlight {
			continue
		}
		if c.NewRevisionPods == 0 {
			anySurging = true
			continue
		}
		if c.NewRevisionReadyPods < c.NewRevisionPods {
			anyWaiting = true
			continue
		}
		if c.TotalPods > c.NewRevisionPods {
			anyScalingDown = true
		}
	}
	// Converged to a partitioned target (see blueGreen): if every rolling
	// component reached its desired staged shape and a partition holds old
	// pods, rest at Staged rather than ScalingDown forever.
	if !anySurging && !anyWaiting {
		stagedAll, anyPartition := true, false
		for _, c := range obs.Components {
			if !c.RolloutInFlight {
				continue
			}
			if !c.AtDesiredShape {
				stagedAll = false
				break
			}
			if c.Partition > 0 {
				anyPartition = true
			}
		}
		if stagedAll && anyPartition {
			return GroupTransition{
				Phase:          v1beta1.CoordinationPhaseStaged,
				CompositePhase: string(v1beta1.CoordinationPhaseStaged),
				Message:        "converged to partitioned target; holding old-revision pods by design",
			}
		}
	}
	switch {
	case anySurging:
		_, skew := computeSurgeBudgetWithRatio(obs)
		return GroupTransition{
			Phase:             v1beta1.CoordinationPhaseSurging,
			CompositePhase:    string(v1beta1.CoordinationPhaseSurging),
			Message:           "at least one Component is surging",
			RatioSkewRejected: skew,
		}
	case anyWaiting:
		_, skew := computeSurgeBudgetWithRatio(obs)
		return GroupTransition{
			Phase:             v1beta1.CoordinationPhaseWaiting,
			CompositePhase:    string(v1beta1.CoordinationPhaseWaiting),
			Message:           "waiting for new-revision pods to be Ready",
			RatioSkewRejected: skew,
		}
	case anyScalingDown:
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseScalingDown,
			CompositePhase: string(v1beta1.CoordinationPhaseScalingDown),
			Message:        "scaling down old-revision pods",
		}
	default:
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseShifting,
			CompositePhase: string(v1beta1.CoordinationPhaseShifting),
			Message:        "writing per-Component traffic weights",
		}
	}
}

// independentTransition collapses every rolling Component into the
// group's phase but skips the gang-readiness check. Used both for
// explicit Independent groups and as the implicit default when no
// group is declared (callers that drive an InferenceService without a
// rolloutCoordination block hit this branch).
func independentTransition(obs GroupObservation) GroupTransition {
	anyRolling := false
	failed := false
	for _, c := range obs.Components {
		if c.Failed {
			failed = true
		}
		if c.RolloutInFlight {
			anyRolling = true
		}
	}
	if failed {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseFailed,
			CompositePhase: string(v1beta1.CoordinationPhaseFailed),
			Message:        "one or more Components in the Independent group failed",
		}
	}
	if !anyRolling {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseIdle,
			CompositePhase: string(v1beta1.CoordinationPhaseIdle),
			Message:        "no rollout in flight",
		}
	}
	return GroupTransition{
		Phase:          v1beta1.CoordinationPhaseShifting,
		CompositePhase: string(v1beta1.CoordinationPhaseShifting),
		Message:        "independent per-Component rollouts in flight",
	}
}

// sequentialTransition implements Sequential: walk the group's
// Order one Component at a time. The next Component cannot enter
// Surging until the previous Component is back at Idle on its new
// revision.
func sequentialTransition(obs GroupObservation) GroupTransition {
	// Failure block: while any Component in the Order is Failed, the
	// group reports Sequential.Failed. The phase is a pure rollup of
	// per-Component state — it exits as soon as a corrective spec edit
	// (roll-forward to a fixed revision or roll-back to the prior one)
	// reconciles the failed Component back to health; no group-level
	// state is latched.
	for _, c := range obs.Group.Order {
		comp, ok := obs.Components[c]
		if !ok {
			continue
		}
		if comp.Failed {
			return GroupTransition{
				Phase:             v1beta1.CoordinationPhaseFailed,
				CompositePhase:    v1beta1.CompositePhaseSequentialFailed,
				CurrentComponent:  c,
				PreviousComponent: previousCompletedComponent(obs, c),
				Message:           "Sequential group blocked by Failed Component: " + string(c),
			}
		}
	}

	// Pick the Component in the Order that's currently rolling, if any.
	// Sequential's invariant: at most one Component rolls at a time.
	if c, ok := activeSequentialComponent(obs.Group.Order, obs.Components); ok {
		comp := obs.Components[c]
		// The wait after a member: while the operator's window since the
		// previous Component finished is still running, this Component
		// reads Sequential.Awaiting. The window is measured from that
		// Component's Ready transition, the instant its roll was recorded
		// done and the clock the replica-side gate reads, so it is measured
		// once after each member finishes: the group's own transition time
		// moves on every base-phase change and would restart the window on
		// each phase of this Component's roll, or on a pause. No Ready
		// transition means no window to measure, and the Component's phase
		// shows.
		if prev := previousCompletedComponent(obs, c); obs.Group.Soak > 0 && prev != "" {
			if finished := obs.Components[prev].ReadyAt; !finished.IsZero() {
				elapsed := obs.Now.Sub(finished)
				if elapsed < obs.Group.Soak {
					return GroupTransition{
						Phase:             v1beta1.CoordinationPhaseIdle,
						CompositePhase:    v1beta1.CompositePhaseSequentialAwaiting,
						CurrentComponent:  c,
						PreviousComponent: prev,
						Message: "Sequential soak: " + (obs.Group.Soak - elapsed).String() +
							" remaining before " + string(c) + " starts",
					}
				}
			}
		}
		// Found the active Component — drive its per-Component phase
		// as the group phase.
		sub := blueGreenTransition(GroupObservation{
			Group: ResolvedGroup{
				Name:       obs.Group.Name,
				Index:      obs.Group.Index,
				Components: []v1beta1.ComponentType{c},
				Policy:     v1beta1.CoordinationPolicyBlueGreen,
				Pacing:     obs.Group.Pacing,
			},
			Components: map[v1beta1.ComponentType]ComponentObservation{c: comp},
		})
		return GroupTransition{
			Phase:             sub.Phase,
			CompositePhase:    string(c) + "." + string(sub.Phase),
			CurrentComponent:  c,
			PreviousComponent: previousCompletedComponent(obs, c),
			Message:           sub.Message,
		}
	}

	// No Component currently rolling. Determine whether we're at Idle
	// (everything done in this order) or in the operator-controlled
	// soak window between Components.
	completed := completedSequentialComponents(obs)
	if completed >= len(obs.Group.Order) {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseIdle,
			CompositePhase: string(v1beta1.CoordinationPhaseIdle),
			Message:        "Sequential group complete",
		}
	}
	if completed == 0 {
		return GroupTransition{
			Phase:          v1beta1.CoordinationPhaseIdle,
			CompositePhase: string(v1beta1.CoordinationPhaseIdle),
			Message:        "Sequential group awaiting first Component spec bump",
		}
	}
	prev := obs.Group.Order[completed-1]
	next := obs.Group.Order[completed]
	return GroupTransition{
		Phase:             v1beta1.CoordinationPhaseIdle,
		CompositePhase:    v1beta1.CompositePhaseSequentialAwaiting,
		CurrentComponent:  next,
		PreviousComponent: prev,
		Message:           "Sequential group in soak window between Components",
	}
}

// activeSequentialComponent returns the first Component in `order` that
// is still genuinely unfinished for handoff. A Component is "done for
// handoff" — and thus skipped — when its rollout is not in flight OR it
// has converged to its desired staged shape (AtDesiredShape). The latter
// releases a partitioned Component that keeps RolloutInFlight=true forever
// by design (old pods held by the partition): once it reaches its staged
// shape it must not block the next Component. The state machine
// routes Sequential's "which Component is currently rolling" decision off
// this; the dispatcher's CheckSequentialGate consumes the same helper
// so the two surfaces never disagree on what's active. Returns ok=false
// when no Component in `order` is still unfinished (Sequential is idle).
func activeSequentialComponent(order []v1beta1.ComponentType, components map[v1beta1.ComponentType]ComponentObservation) (v1beta1.ComponentType, bool) {
	for _, c := range order {
		comp, ok := components[c]
		if !ok || !comp.RolloutInFlight || comp.AtDesiredShape {
			continue
		}
		return c, true
	}
	return "", false
}

// completedSequentialComponents counts the number of leading Components
// in the Order that are done for handoff. A Component counts as completed
// when its rollout is not in flight OR it has converged to its desired
// staged shape (!RolloutInFlight || AtDesiredShape). The AtDesiredShape
// clause advances the count past a partitioned leading Component that
// holds old pods by design and never clears RolloutInFlight. The
// count drives Sequential's "which Component is next" decision.
func completedSequentialComponents(obs GroupObservation) int {
	count := 0
	for _, c := range obs.Group.Order {
		comp, ok := obs.Components[c]
		if !ok {
			return count
		}
		if comp.RolloutInFlight && !comp.AtDesiredShape {
			return count
		}
		count++
	}
	return count
}

// previousCompletedComponent returns the Component immediately
// preceding c in the Order whose rollout has completed (or empty when
// c is the first Order entry).
func previousCompletedComponent(obs GroupObservation, c v1beta1.ComponentType) v1beta1.ComponentType {
	for i, oc := range obs.Group.Order {
		if oc != c {
			continue
		}
		if i == 0 {
			return ""
		}
		return obs.Group.Order[i-1]
	}
	return ""
}

// computeSurgeBudget returns the per-Component MaxSurge budget for the
// group's pacing, keyed by Component. Caller plumbs each value into
// the per-Component surge logic so the operator's MaxSurge bound is
// respected end-to-end. PerComponent-pacing only — for RatioBalanced,
// use computeSurgeBudgetWithRatio.
func computeSurgeBudget(obs GroupObservation) map[v1beta1.ComponentType]int32 {
	pacing := obs.Group.Pacing
	out := make(map[v1beta1.ComponentType]int32, len(obs.Components))
	for c, comp := range obs.Components {
		replicas := comp.DesiredReplicas
		if replicas <= 0 {
			replicas = 1
		}
		out[c] = MaxSurgeBudget(pacing, replicas)
	}
	return out
}

// computeSurgeBudgetWithRatio returns SurgeBudget and a skew-rejected
// flag. For PerComponent pacing this is identical to computeSurgeBudget
// + skew=false. For RatioBalanced pacing, EvaluateSurge gates the
// MaxSurge budget — when the ratio band would be exceeded by even one
// surge, the budget is zeroed and skew=true so the caller can fire
// RatioSkewRejected metrics/events.
//
// Baseline: populates state.Serving from obs.Components.ServingPods so
// EvaluateSurge projects against live serving capacity, NOT against the
// zero-initialized NewPods that exist at rollout start. Without Serving
// populated EvaluateSurge falls back to NewPods=0 and pairwise band
// checks see a zero-serving peer (mass-drain failure mode), rejecting
// every surge attempt the moment a Component first touches RatioBalanced.
// That spurious rejection produces an unbroken stream of RatioSkewRejected
// events with no progress; the dispatcher's CheckRatioGate may still
// allow drain because it reads ServingReplicas from status, but this
// state-machine-side gating fires the noise + zeros SurgeBudget map.
func computeSurgeBudgetWithRatio(obs GroupObservation) (map[v1beta1.ComponentType]int32, bool) {
	budget := computeSurgeBudget(obs)
	if obs.Group.Pacing.Type != v1beta1.CoordinationPacingRatioBalanced {
		return budget, false
	}
	if len(obs.OriginalReplicas) == 0 {
		// No anchor yet — let MaxSurge govern alone, no skew possible
		// until the next reconcile when SnapshotOriginal has run.
		return budget, false
	}
	state := RatioState{
		Original: obs.OriginalReplicas,
		Current:  make(map[v1beta1.ComponentType]int32, len(obs.Components)),
		NewPods:  make(map[v1beta1.ComponentType]int32, len(obs.Components)),
		Serving:  make(map[v1beta1.ComponentType]int32, len(obs.Components)),
	}
	for c, comp := range obs.Components {
		state.Current[c] = comp.TotalPods
		state.NewPods[c] = comp.NewRevisionPods
		state.Serving[c] = comp.ServingPods
	}
	var skew bool
	for c, proposed := range budget {
		if proposed <= 0 {
			continue
		}
		d := EvaluateSurge(obs.Group.Pacing, state, c, proposed)
		budget[c] = d.AllowedSurgeDelta
		if d.SkewRejected {
			skew = true
		}
	}
	return budget, skew
}
