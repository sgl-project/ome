package coordination

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/revision"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// PairingProtocolForRevision resolves the P/D pairing protocol one revision of
// one Component was minted under, by reading the annotation (or stored
// payload) of the ControllerRevision named `<prefix>-<component>-<hash>`,
// where prefix is the Component's replica prefix (prefixFor).
//
// found=false reports a NotFound CR. Callers choose the degrade: traffic /
// Service-label producers treat it as "" (a swept or pre-pairing revision
// pairs with anything), while the pair-floor gate fails closed — retention
// protects every live revision, so a missing CR behind a SERVING instance is
// an anomaly the gate must not guess about.
func PairingProtocolForRevision(ctx context.Context, reads client.Reader, namespace, prefix string, component v1beta1.ComponentType, hash string) (protocol string, found bool, err error) {
	if hash == "" {
		return "", false, nil
	}
	return pairingProtocolForRevisionName(ctx, reads, namespace, fmt.Sprintf("%s-%s-%s", prefix, component, hash))
}

// pairingProtocolForRevisionName is PairingProtocolForRevision keyed by the
// full ControllerRevision name (the form InstanceStatus.RunningRevision
// records).
func pairingProtocolForRevisionName(ctx context.Context, reads client.Reader, namespace, name string) (protocol string, found bool, err error) {
	if reads == nil || name == "" {
		return "", false, nil
	}
	cr := &appsv1.ControllerRevision{}
	if gerr := reads.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cr); gerr != nil {
		if apierrors.IsNotFound(gerr) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get ControllerRevision %s: %w", name, gerr)
	}
	return revision.PairingProtocolFromRevision(cr), true, nil
}

// AttachPairingProtocols fills RevisionWeight.PairingProtocol on each weight
// from its revision's ControllerRevision under the Component's replica
// prefix, so BuildTrafficTargets publishes the cohort token routing
// consumers pair on. A missing CR yields "" (a swept or pre-pairing revision
// pairs with anything); read errors propagate.
func AttachPairingProtocols(ctx context.Context, reads client.Reader, namespace, prefix string, component v1beta1.ComponentType, weights []RevisionWeight) error {
	for i := range weights {
		proto, _, err := PairingProtocolForRevision(ctx, reads, namespace, prefix, component, weights[i].RevisionHash)
		if err != nil {
			return err
		}
		weights[i].PairingProtocol = proto
	}
	return nil
}

// pairingComponents are the Components that participate in P/D pairing. The
// router fronts both cohorts and never constrains a pair.
var pairingComponents = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}

// CheckPairing is the pair-floor gate for pairing-protocol transitions: while
// a rollout is changing spec.rollout.pairingProtocol, every per-Instance
// update step must leave at least one pairable serving (engine, decoder) pair
// — a serving engine and a serving decoder whose cohort protocols are equal,
// or either empty (empty pairs with anything). Without it, per-Instance
// progressions can drain the last old-cohort engine while the new cohort has
// no serving decoder yet; every serving engine then speaks a protocol no
// serving decoder does and routing has no valid pair to place a request on.
//
// A step that takes its pod out of rotation at once (a drain-first recreate,
// or an in-place patch that returns the pod on the new cohort) must also
// keep both Components at their pairable floor: at least desired minus
// maxUnavailable Instances with a serving pod that has a partner on the
// other side (resolvePairFloor). A surge start takes nothing out of rotation,
// so it answers the one-pair rule alone and the drain-time gate
// (EvaluatePairingDrain) holds the floor when its source would leave.
//
// The target cohort is the protocol of ctx.TargetRevision, the revision the
// Component's Instances move to in this pass — the rollback revision while a
// rollback is pinned, else the spec target. A canary revert is therefore
// simulated toward the stable cohort while the live spec field still names
// the rejected protocol; the live field alone would orient it the wrong way.
//
// Inactive (allows) unless ALL of: the resolved group spans engine AND
// decoder, ctx.Component is one of the pairing Components, the target
// revision carries a non-empty pairing protocol, and some serving instance
// still runs a different non-empty protocol (a transition in flight). A
// transition to or from the empty protocol never activates the gate — empty
// pairs with anything, so every intermediate mix is routable.
//
// When active it simulates this step's end state against authoritative IR
// status, instance-granular (a serving instance is one with
// ServingPodCount>0): the Instance being started leaves the cohort of
// fromRevision, the ControllerRevision its row runs now. An Instance
// stranded on an intermediate cohort that pairs with nothing may therefore
// move on while its sibling on the last pairable cohort is held. An empty
// fromRevision, or one whose ControllerRevision is gone, names no cohort, so
// every old cohort is simulated losing the instance, the pessimistic
// reading. A surge in flight serves two cohorts at once: its source keeps
// the running cohort and its replacement, once it serves, counts for the
// cohort of the revision the surge is pinned to (servingRevisions), so a
// peer's step sees that capacity before the promote and never waits on a
// drain that waits on it. Surge and in-place strategies credit the target
// cohort with the replacement — a SurgeThenDrain op only drains after its
// replacement is Ready, and an in-place op returns the same pod — while
// drain-first strategies do not. In-place is NOT bypassed the way CheckRatio
// bypasses it: its net capacity change is ~0, but the instance still crosses
// cohorts, which is exactly what this gate paces. In-flight starts from the
// same wake-up are charged pessimistically against each cohort under
// simulation.
//
// Three liveness escapes keep the gate deadlock-free:
//   - When no pairable pair exists BEFORE the step, there is nothing left to
//     protect and progress toward the target cohort is the only way back to
//     one — allow.
//   - When both pairing Components are down to a single serving instance with
//     no target-cohort capacity anywhere (the 1×1 mutual wall), denying both
//     first movers would deadlock the rollout permanently, so the step is
//     allowed. For surge strategies the drain-time check
//     (EvaluatePairingDrain) then holds each source until the peer's target
//     cohort serves, so admitting the first movers costs no unpaired window;
//     a drain-first strategy has no such point and keeps the window.
//   - When the acting Component is down to its last serving instance and the
//     peer already serves the target cohort (the last-hop shape), the step is
//     the acting side's final unavoidable hop and no other step can create
//     the pairing a denial would wait for — allow. The transient unpaired
//     window equals what a single-replica update costs WITHOUT pairing (the
//     gate must not be stricter than non-pairing behavior for the last
//     mover), and the incoming instance pairs with the peer's target
//     capacity the moment it serves.
//
// Fails closed (deny, retryable reason) when the target revision is unknown
// or IR status or a serving instance's ControllerRevision cannot be read —
// approving a cohort-draining step on unreadable protocol data is how the
// last pair dies silently.
func (ctx GateContext) CheckPairing(fromRevision string, strategy workloadtypes.UpdateStrategyType, inFlightSurge, inFlightUnavail int32) (allowed bool, reason string) {
	if ctx.ShortCircuit {
		return true, ctx.ShortReason
	}
	if !groupPairsEngineAndDecoder(ctx.Group) {
		return true, "group does not pair engine and decoder"
	}
	component := ctx.Component
	if component != v1beta1.EngineComponent && component != v1beta1.DecoderComponent {
		return true, "component does not participate in pairing"
	}
	target, ok, reason := ctx.targetPairingProtocol()
	if !ok {
		return false, reason
	}
	if target == "" {
		return true, "target revision declares no pairing protocol"
	}

	// Serving instances per (Component, protocol), from authoritative IR
	// status. An instance with no recorded RunningRevision predates revision
	// tracking entirely and therefore predates pairing protocols: "".
	serving := map[v1beta1.ComponentType]map[string]int32{}
	replicas := map[v1beta1.ComponentType]*v1beta1.InferenceReplica{}
	protoByRevision := map[string]string{}
	// servingProtocol resolves the cohort of a revision a serving instance
	// runs, once per revision. A revision that cannot be read, or is gone,
	// fails the gate closed: retention protects every live revision, so a
	// missing one behind a serving instance is an anomaly the gate must not
	// guess about.
	servingProtocol := func(revision string, comp v1beta1.ComponentType, index int32) (proto, reason string) {
		if p, ok := protoByRevision[revision]; ok {
			return p, ""
		}
		p, found, gerr := pairingProtocolForRevisionName(ctx.Ctx, ctx.Reads, ctx.ISVC.Namespace, revision)
		if gerr != nil {
			return "", fmt.Sprintf("pairing gate: cannot read revision %s for serving %s instance %d, failing closed: %v",
				revision, comp, index, gerr)
		}
		if !found {
			return "", fmt.Sprintf("pairing gate: revision %s for serving %s instance %d not found, failing closed",
				revision, comp, index)
		}
		protoByRevision[revision] = p
		return p, ""
	}
	transition := false
	for _, comp := range pairingComponents {
		serving[comp] = map[string]int32{}
		ir, _, err := irprojector.DecodedComponentIRFor(ctx.Ctx, ctx.Reads, ctx.ISVC, comp)
		if err != nil {
			return false, fmt.Sprintf("pairing gate: cannot read %s IR status, failing closed: %v", comp, err)
		}
		if ir == nil {
			continue
		}
		replicas[comp] = ir
		for i := range ir.Status.InstanceStatuses {
			inst := &ir.Status.InstanceStatuses[i]
			if inst.ServingPodCount <= 0 {
				continue
			}
			for _, revision := range servingRevisions(inst) {
				proto := ""
				if revision != "" {
					var reason string
					if proto, reason = servingProtocol(revision, comp, inst.Index); reason != "" {
						return false, reason
					}
				}
				serving[comp][proto]++
				if proto != "" && proto != target {
					transition = true
				}
			}
		}
	}
	if !transition {
		return true, "no pairing protocol transition in flight"
	}
	if !pairableServingPairExists(serving) {
		return true, fmt.Sprintf("no serving engine+decoder pair exists to protect; allowing progress toward cohort %q", target)
	}

	// Replacement credit: a surge op drains only after its replacement is
	// Ready; an in-place op returns the same pod. Drain-first strategies get
	// none — the instance leaves serving before anything comes back.
	credit := int32(0)
	switch strategy {
	case workloadtypes.UpdateStrategySurgeThenDrain, "",
		workloadtypes.UpdateStrategyInPlaceIfPossible, workloadtypes.UpdateStrategyInPlaceOnly:
		credit = 1
	}
	// A surge start leaves its source in rotation until the drain-time gate
	// admits the drain; every other start takes the pod out now and answers
	// the pairable floor here.
	leavesRotationNow := strategy != workloadtypes.UpdateStrategySurgeThenDrain && strategy != ""
	peer := pairingPeer(component)
	floors := map[v1beta1.ComponentType]pairFloor{
		component: resolvePairFloor(replicas[component], ctx.Group.Pacing),
		peer:      resolvePairFloor(replicas[peer], ctx.Group.Pacing),
	}
	// The cohort the started instance leaves. A row that records no revision,
	// or one whose revision is gone, names no cohort, and every old cohort is
	// then simulated losing the instance.
	fromProto, fromKnown := "", false
	if fromRevision != "" {
		p, ok := protoByRevision[fromRevision]
		if !ok {
			var gerr error
			p, ok, gerr = pairingProtocolForRevisionName(ctx.Ctx, ctx.Reads, ctx.ISVC.Namespace, fromRevision)
			if gerr != nil {
				return false, fmt.Sprintf("pairing gate: cannot read revision %s of the starting %s instance, failing closed: %v",
					fromRevision, component, gerr)
			}
		}
		fromProto, fromKnown = p, ok
	}
	// In-flight fresh starts from this wake-up haven't reached IR status yet;
	// charge them against the cohort under simulation (worst case they all
	// came from it). Surge in-flights carry their own replacement credit.
	inFlightLoss := inFlightSurge + inFlightUnavail
	for proto, n := range serving[component] {
		if proto == target || n <= 0 {
			continue
		}
		loss := inFlightLoss
		if !fromKnown || proto == fromProto {
			loss++
		}
		if loss == 0 {
			continue
		}
		sim := copyServing(serving)
		sim[component][proto] -= loss
		if sim[component][proto] < 0 {
			sim[component][proto] = 0
		}
		sim[component][target] += credit + inFlightSurge
		if pairableServingPairExists(sim) {
			if !leavesRotationNow {
				continue
			}
			if held, reason := startBreaksPairFloor(serving, sim, component, peer, floors, target, proto); held {
				return false, reason
			}
			continue
		}
		if mutualPairingWall(serving, component, peer, target) {
			return true, fmt.Sprintf("both %s and %s are down to their last serving instance with no cohort %q capacity; allowing the step rather than deadlocking the transition",
				component, peer, target)
		}
		if lastHopEscape(serving, component, peer, target) {
			return true, fmt.Sprintf("%s is down to its last serving instance and %s already serves cohort %q; allowing the final hop (the unpaired window equals a single-replica update without pairing)",
				component, peer, target)
		}
		return false, fmt.Sprintf("pairing protocol transition to %q: updating a serving %s of cohort %q would leave no pairable serving engine+decoder pair; holding until cohort %q serves on both Components",
			target, component, proto, target)
	}
	return true, ""
}

// servingRevisions names the revisions a serving Instance's pods run, one
// entry per cohort in rotation. A settled row runs its RunningRevision; an
// empty one is the empty cohort, which pairs with anything. A single-pod
// surge keeps its replacement inside the source's Instance until the
// promote, so once both pods are in rotation the row serves the running
// revision and the one the surge is pinned to, and past the hand-over, with
// the source out of rotation, the replacement is the row's serving pod. A
// gang surge's replacement is its own row: it runs nothing yet and serves
// the pinned revision.
func servingRevisions(inst *v1beta1.OMENativeInstanceStatus) []string {
	op := inst.Operation
	if op == nil || op.Type != v1beta1.InstanceOperationUpdate || op.TargetRevision == "" {
		return []string{inst.RunningRevision}
	}
	switch op.Step {
	case workloadtypes.UpdateStepGangSurgeTarget:
		return []string{op.TargetRevision}
	case workloadtypes.UpdateStepSurge:
		if op.SurgeIndex == nil && inst.ServingPodCount >= 2 {
			return []string{inst.RunningRevision, op.TargetRevision}
		}
	case workloadtypes.UpdateStepSurgeDrain, workloadtypes.UpdateStepSurgeDrainSettle:
		if op.SurgeIndex == nil {
			if inst.ServingPodCount >= 2 {
				return []string{inst.RunningRevision, op.TargetRevision}
			}
			return []string{op.TargetRevision}
		}
	}
	return []string{inst.RunningRevision}
}

// startBreaksPairFloor reports whether the simulated step pushes a pairing
// Component below its pairable floor: fewer of its serving Instances have a
// partner after the step than its floor allows, and fewer than before it. A
// Component already short for another reason is not held on that shortfall.
func startBreaksPairFloor(serving, sim map[v1beta1.ComponentType]map[string]int32, component, peer v1beta1.ComponentType, floors map[v1beta1.ComponentType]pairFloor, target, fromProto string) (bool, string) {
	for _, side := range []v1beta1.ComponentType{peer, component} {
		floor := floors[side]
		if !floor.Applies() {
			continue
		}
		other := pairingPeer(side)
		before := pairableCohortCount(serving[side], serving[other])
		after := pairableCohortCount(sim[side], sim[other])
		if after >= floor.floor || after >= before {
			continue
		}
		return true, fmt.Sprintf("pairing protocol transition to %q: updating a serving %s of cohort %q would leave the %s with %d pairable serving Instance(s), below its %s; holding until cohort %q serves on both Components",
			target, component, fromProto, side, after, floor, target)
	}
	return false, ""
}

// targetPairingProtocol is the cohort protocol a step of this Component
// moves toward: the protocol of the revision its Instances are rolling to.
// A nil TargetRevision fails closed — without the revision the unit is
// moving to, the gate cannot tell which way the cohorts move.
func (ctx GateContext) targetPairingProtocol() (protocol string, ok bool, reason string) {
	if ctx.TargetRevision == nil {
		return "", false, "pairing gate: no target revision for this step, failing closed"
	}
	return revision.PairingProtocolFromRevision(ctx.TargetRevision), true, ""
}

// pairableServingPairExists reports whether some serving engine and some
// serving decoder can pair: equal protocols, or either side empty (empty
// pairs with anything).
func pairableServingPairExists(serving map[v1beta1.ComponentType]map[string]int32) bool {
	for pe, engines := range serving[v1beta1.EngineComponent] {
		if engines <= 0 {
			continue
		}
		for pd, decoders := range serving[v1beta1.DecoderComponent] {
			if decoders <= 0 {
				continue
			}
			if pe == "" || pd == "" || pe == pd {
				return true
			}
		}
	}
	return false
}

// mutualPairingWall reports the 1×1 deadlock shape: both pairing Components
// have at most one serving instance and neither serves the target cohort, so
// denying either Component's step would deny both forever.
func mutualPairingWall(serving map[v1beta1.ComponentType]map[string]int32, component, peer v1beta1.ComponentType, target string) bool {
	if serving[component][target] > 0 || serving[peer][target] > 0 {
		return false
	}
	return totalServing(serving[component]) <= 1 && totalServing(serving[peer]) <= 1
}

// lastHopEscape reports the asymmetric deadlock shape: the acting Component
// is down to at most one serving instance while the peer already serves the
// target cohort. The step is the acting side's final unavoidable hop — every
// path to convergence moves that instance, and no other step can create the
// pairing a denial would wait for (the peer's remaining old instances are
// themselves held by the ordinary simulation until the acting side flips).
// A Component with two or more serving instances never satisfies this: its
// old cohort keeps being paced instance-by-instance. Disjoint from the
// mutual wall, which covers the peer-has-no-target-capacity shape.
func lastHopEscape(serving map[v1beta1.ComponentType]map[string]int32, component, peer v1beta1.ComponentType, target string) bool {
	return totalServing(serving[component]) <= 1 && serving[peer][target] > 0
}

func totalServing(byProto map[string]int32) int32 {
	var total int32
	for _, n := range byProto {
		total += n
	}
	return total
}

func copyServing(in map[v1beta1.ComponentType]map[string]int32) map[v1beta1.ComponentType]map[string]int32 {
	out := make(map[v1beta1.ComponentType]map[string]int32, len(in))
	for comp, byProto := range in {
		m := make(map[string]int32, len(byProto)+1)
		for p, n := range byProto {
			m[p] = n
		}
		out[comp] = m
	}
	return out
}
