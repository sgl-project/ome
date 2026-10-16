package coordination

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// EvaluatePairingDrain is the drain-time half of the pair floor. A surge
// update asks it right before its source leaves rotation, with the
// replacement already serving, and the source stays in rotation while any
// of these holds:
//   - the replacement cannot pair yet and the peer Component updates by
//     SurgeThenDrain: no serving pod of the peer runs the target cohort (or
//     the empty protocol, which pairs with anything), so retiring a source
//     that does pair would trade serving capacity for none, and the peer's
//     replacements come up without waiting on this drain, so the first one
//     to serve releases the hold;
//   - the drain would push the peer below its pairable floor: fewer than
//     desired minus maxUnavailable of its Instances would keep a serving pod
//     with a partner, because the pods leaving are the last partners of the
//     peer's old-cohort Instances. Those Instances recover a partner on
//     their own, through a replacement of their own in rotation or their
//     own move, so the hold cannot deadlock;
//   - the drain would push this Component below its own floor, because the
//     replacement cannot pair yet, and the peer's new cohort is already
//     coming up without this promote: the peer surges, or an Instance of a
//     drain-first or in-place peer is in an update attempt toward the
//     target cohort (targetCohortComingUp). That attempt needs nothing from
//     this drain: the start-time gate admits it on the replacement's
//     serving capacity, so the first pod it brings into rotation releases
//     the hold. An attempt the escalation has parked or failed is not
//     coming up, so a first pod that never serves cannot hold this drain
//     for good; the fresh attempt a retry opens arms the hold again. A
//     drain-first peer that has not opened its first step is not waited on
//     (the window is then the one a single-replica recreate costs without
//     pairing);
//   - the source is the last pod keeping any pairable engine+decoder pair
//     alive.
//
// An Instance counts as pairable while any of its routable serving pods has
// a partner, the surge replacement included from the moment it serves, so
// an old pod whose replacement already pairs never costs its Instance.
//
// It reads pods, not InferenceReplica status: a single-pod surge keeps its
// replacement inside the source's Instance until the promote, so only the
// pods' cohort labels tell the two cohorts apart. A serving pod is PodReady
// (the serving gate is folded into it) and routable; workers never carry
// traffic. targetRevision is the ControllerRevision the Component's Instances
// move to in this pass, the same one the start-time gate orients by. Fails
// closed on a list error, like the start-time gate.
func EvaluatePairingDrain(ctx context.Context, reads client.Reader, isvc *v1beta1.InferenceService, component v1beta1.ComponentType, targetRevision *appsv1.ControllerRevision, defaults GroupDefaults, sourcePods []string) (allowed bool, gate v1beta1.RolloutHoldGate, reason string) {
	gateCtx := ResolveGateContextWithDefaults(ctx, reads, isvc, component, defaults)
	gateCtx.TargetRevision = targetRevision
	if gateCtx.ShortCircuit {
		return true, "", gateCtx.ShortReason
	}
	if !groupPairsEngineAndDecoder(gateCtx.Group) {
		return true, "", "group does not pair engine and decoder"
	}
	if component != v1beta1.EngineComponent && component != v1beta1.DecoderComponent {
		return true, "", "component does not participate in pairing"
	}
	target, ok, reason := gateCtx.targetPairingProtocol()
	if !ok {
		return false, v1beta1.RolloutHoldGatePairing, reason
	}
	if target == "" {
		return true, "", "target revision declares no pairing protocol"
	}

	// Each pairing Component's pods are listed under its own replica prefix:
	// a referenced peer's pods carry the peer replica's name, not the
	// service's.
	serving := map[v1beta1.ComponentType]map[string]int32{
		v1beta1.EngineComponent:  {},
		v1beta1.DecoderComponent: {},
	}
	// instances lists, per Component and Instance, the cohorts of its
	// routable serving pods.
	instances := map[v1beta1.ComponentType]map[string][]string{
		v1beta1.EngineComponent:  {},
		v1beta1.DecoderComponent: {},
	}
	protoByPod := map[string]string{}
	instanceByPod := map[string]string{}
	transition := false
	for _, comp := range pairingComponents {
		prefix := prefixFor(isvc, comp)
		pods := &corev1.PodList{}
		if err := reads.List(ctx, pods, client.InNamespace(isvc.Namespace), client.MatchingLabels{
			constants.InferenceServicePodLabelKey: prefix,
			constants.OMEComponentLabel:           string(comp),
			query.LabelManagedBy:                  query.ManagedByOMENative,
		}); err != nil {
			return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf("pairing drain gate: cannot list %s pods, failing closed: %v", comp, err)
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.DeletionTimestamp != nil || !query.RoutedRunner(pod) {
				continue
			}
			proto := pod.Labels[query.LabelPairingProtocol]
			if !podreadiness.IsPodReady(pod) || query.RoutedServiceForPod(prefix, workloadtypes.ComponentType(comp), pod) == "" {
				continue
			}
			serving[comp][proto]++
			key := instanceKeyForPod(pod)
			instances[comp][key] = append(instances[comp][key], proto)
			protoByPod[pod.Name] = proto
			instanceByPod[pod.Name] = key
			if proto != "" && proto != target {
				transition = true
			}
		}
	}
	if !transition {
		return true, "", "no pairing protocol transition in flight"
	}
	if !pairableServingPairExists(serving) {
		return true, "", fmt.Sprintf("no serving engine+decoder pair exists to protect; allowing progress toward cohort %q", target)
	}

	after := copyServing(serving)
	afterInstances := copyInstances(instances)
	leaving := map[string]struct{}{}
	for _, name := range sourcePods {
		proto, ok := protoByPod[name]
		if !ok {
			continue // not serving, or a worker: nothing leaves rotation
		}
		after[component][proto]--
		leaving[proto] = struct{}{}
		key := instanceByPod[name]
		afterInstances[component][key] = withoutOneCohort(afterInstances[component][key], proto)
	}
	peer := pairingPeer(component)

	// Both pairing replicas, read once: the peer's update strategy, the
	// attempts its Instances carry and both sides' desired counts and
	// budgets come from them. A missing replica leaves the one-pair rule;
	// an unreadable one fails closed.
	replicas := map[v1beta1.ComponentType]*v1beta1.InferenceReplica{}
	for _, comp := range pairingComponents {
		ir, _, err := irprojector.DecodedComponentIRFor(ctx, reads, isvc, comp)
		if err != nil {
			return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf("pairing drain gate: cannot read the %s replica, failing closed: %v", comp, err)
		}
		replicas[comp] = ir
	}
	peerSurges := surgeStrategy(replicas[peer])
	peerMoving := false
	if !peerSurges {
		var reason string
		if peerMoving, reason = targetCohortComingUp(ctx, reads, isvc.Namespace, replicas[peer], target); reason != "" {
			return false, v1beta1.RolloutHoldGatePairing, reason
		}
	}
	replacementPairs := pairsWithPeer(serving[peer], target)
	if !replacementPairs && anyPairsWithPeer(serving[peer], leaving) && peerSurges {
		return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf(
			"pairing protocol transition to %q: the replacement %s cannot pair because no serving %s runs cohort %q yet; holding the drain of the serving %s of cohort %s until one does",
			target, component, peer, target, component, cohortList(leaving))
	}
	if floor := resolvePairFloor(replicas[peer], gateCtx.Group.Pacing); floor.Applies() {
		before := pairableInstanceCount(instances[peer], serving[component])
		left := pairableInstanceCount(afterInstances[peer], after[component])
		if left < floor.floor && left < before {
			return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf(
				"pairing protocol transition to %q: draining the serving %s pods of cohort %s would leave the %s with %d pairable serving Instance(s), below its %s; holding the drain until the %s Instances still on cohort %s have another partner or have moved",
				target, component, cohortList(leaving), peer, left, floor, peer, cohortList(leaving))
		}
	}
	if floor := resolvePairFloor(replicas[component], gateCtx.Group.Pacing); floor.Applies() && !replacementPairs && (peerSurges || peerMoving) {
		before := pairableInstanceCount(instances[component], serving[peer])
		left := pairableInstanceCount(afterInstances[component], serving[peer])
		if left < floor.floor && left < before {
			return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf(
				"pairing protocol transition to %q: the replacement %s cannot pair because no serving %s runs cohort %q yet, and retiring the serving %s of cohort %s would leave the %s with %d pairable serving Instance(s), below its %s; holding the drain until one does",
				target, component, peer, target, component, cohortList(leaving), component, left, floor)
		}
	}
	if pairableServingPairExists(after) {
		return true, "", ""
	}
	return false, v1beta1.RolloutHoldGatePairing, fmt.Sprintf(
		"pairing protocol transition to %q: draining the serving %s pods of cohort %s would leave no pairable serving engine+decoder pair; holding the drain until cohort %q serves on both Components",
		target, component, cohortList(leaving), target)
}

// targetCohortComingUp reports whether a drain-first or in-place replica is
// bringing a pod of the target cohort into rotation on its own: one of its
// Instances is in an update attempt pinned to a revision of that cohort, or
// of the empty protocol, which pairs with anything. A Failed Instance and a
// parked attempt are not coming up: the escalation ended that attempt, and
// the roll opens a fresh one over it when it retries. Fails closed (a
// non-empty reason) when the revision such an attempt names cannot be read,
// like the rest of the gate; a revision that is gone names no cohort.
func targetCohortComingUp(ctx context.Context, reads client.Reader, namespace string, ir *v1beta1.InferenceReplica, target string) (bool, string) {
	if ir == nil {
		return false, ""
	}
	protoByRevision := map[string]string{}
	for i := range ir.Status.InstanceStatuses {
		row := &ir.Status.InstanceStatuses[i]
		op := row.Operation
		if row.Phase == v1beta1.OMENativeInstanceFailed || op == nil || op.Type != v1beta1.InstanceOperationUpdate ||
			op.Step == workloadtypes.UpdateStepParked || op.TargetRevision == "" {
			continue
		}
		proto, ok := protoByRevision[op.TargetRevision]
		if !ok {
			p, found, err := pairingProtocolForRevisionName(ctx, reads, namespace, op.TargetRevision)
			if err != nil {
				return false, fmt.Sprintf("pairing drain gate: cannot read revision %s of the %s attempt on instance %d, failing closed: %v",
					op.TargetRevision, ir.Spec.Component, row.Index, err)
			}
			if !found {
				continue
			}
			protoByRevision[op.TargetRevision] = p
			proto = p
		}
		if proto == "" || proto == target {
			return true, ""
		}
	}
	return false, ""
}

// instanceKeyForPod names the Instance a pod belongs to, by its index label;
// a pod without one stands alone.
func instanceKeyForPod(pod *corev1.Pod) string {
	if idx, ok := query.InstanceIdxFromLabels(pod); ok {
		return strconv.Itoa(int(idx))
	}
	return "pod/" + pod.Name
}

func copyInstances(in map[v1beta1.ComponentType]map[string][]string) map[v1beta1.ComponentType]map[string][]string {
	out := make(map[v1beta1.ComponentType]map[string][]string, len(in))
	for comp, byInstance := range in {
		m := make(map[string][]string, len(byInstance))
		for key, protos := range byInstance {
			m[key] = append([]string(nil), protos...)
		}
		out[comp] = m
	}
	return out
}

// withoutOneCohort drops one entry of proto from an Instance's cohort list.
func withoutOneCohort(protos []string, proto string) []string {
	for i, p := range protos {
		if p == proto {
			return append(append([]string(nil), protos[:i]...), protos[i+1:]...)
		}
	}
	return protos
}

// pairingPeer is the other pairing Component.
func pairingPeer(component v1beta1.ComponentType) v1beta1.ComponentType {
	if component == v1beta1.DecoderComponent {
		return v1beta1.EngineComponent
	}
	return v1beta1.DecoderComponent
}

// pairsWithPeer reports whether a pod of cohort proto can pair with some
// serving pod of the peer: equal protocols, or either side empty.
func pairsWithPeer(peerServing map[string]int32, proto string) bool {
	for p, n := range peerServing {
		if n > 0 && (p == "" || proto == "" || p == proto) {
			return true
		}
	}
	return false
}

// anyPairsWithPeer reports whether any of the cohorts pairs with the peer.
func anyPairsWithPeer(peerServing map[string]int32, cohorts map[string]struct{}) bool {
	for proto := range cohorts {
		if pairsWithPeer(peerServing, proto) {
			return true
		}
	}
	return false
}

// cohortList renders cohorts for a hold reason, sorted and quoted.
func cohortList(cohorts map[string]struct{}) string {
	out := make([]string, 0, len(cohorts))
	for proto := range cohorts {
		out = append(out, fmt.Sprintf("%q", proto))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// groupPairsEngineAndDecoder reports whether the resolved group spans both
// pairing Components; the pair floor is inert otherwise.
func groupPairsEngineAndDecoder(group ResolvedGroup) bool {
	spansEngine, spansDecoder := false, false
	for _, c := range group.Components {
		switch c {
		case v1beta1.EngineComponent:
			spansEngine = true
		case v1beta1.DecoderComponent:
			spansDecoder = true
		}
	}
	return spansEngine && spansDecoder
}
