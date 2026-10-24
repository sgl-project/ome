package placement

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/irstatus"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/placement/allocation"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workloadcluster"
	"sigs.k8s.io/ome/pkg/placement/protocol"
)

type plannedMemberObservation struct {
	Candidate         v1beta1.CandidatePlacement
	Member            *v1beta1.InferenceService
	MemberState       allocation.MemberState
	RolloutReserved   int32
	PauseAcknowledged bool
	FullPolicyReady   bool
	IdleZeroFloor     bool
	ScalingToZero     bool
	Terminal          bool
	// ComponentReady counts each scaled component's ready instances while the
	// member can serve; it is empty otherwise.
	ComponentReady map[v1beta1.ComponentType]int32
}

// observePlannedMember combines the observed ISVC with a direct workload inventory,
// re-reading it after a transient failure as MemberReadRetry allows. Component
// requirements include every declared component, even before its IR exists.
func (r *Reconciler) observePlannedMember(ctx context.Context, source *v1beta1.InferenceService, previous v1beta1.CandidatePlacement, components []v1beta1.ComponentType) (plannedMemberObservation, error) {
	return retryMemberRead(ctx, r, previous.Cluster, func(attemptCtx context.Context) (plannedMemberObservation, error) {
		return r.observePlannedMemberOnce(attemptCtx, source, previous, components)
	})
}

func (r *Reconciler) observePlannedMemberOnce(ctx context.Context, source *v1beta1.InferenceService, previous v1beta1.CandidatePlacement, components []v1beta1.ComponentType) (plannedMemberObservation, error) {
	// An unreadable member keeps the plan it last acknowledged behind an unknown
	// observation; its ready count, which routes traffic, follows the grace.
	out := plannedMemberObservation{Candidate: r.unknownCandidate(source, previous)}
	if previous.Allocation == nil || source.Status.Placement == nil || source.Status.Placement.Plan == nil || len(components) == 0 {
		return out, fmt.Errorf("planned observation requires allocation authority and resolved components")
	}
	if source.UID == "" || source.Status.Placement.Plan.SourceUID != source.UID {
		return out, fmt.Errorf("planned observation requires verified source identity")
	}
	cl, transport, err := r.plannedTransport(ctx, previous.Cluster, previous.Allocation.ClusterUID)
	if err != nil {
		return out, err
	}
	member := &v1beta1.InferenceService{}
	memberExists := true
	if err := cl.Get(ctx, client.ObjectKeyFromObject(source), member); err != nil {
		if !apierrors.IsNotFound(err) {
			return out, err
		}
		memberExists = false
	} else if !isOurDerived(member, source) || member.UID == "" || member.ResourceVersion == "" {
		return out, fmt.Errorf("member service ownership is unverified")
	}
	if memberExists {
		out.Candidate.Endpoint = member.Status.URL.DeepCopy()
		// The member's own status was read: an ingress it reports not ready
		// means the member cannot serve, even if a later read fails.
		if !member.Status.IsConditionReady(v1beta1.IngressReady) {
			out.Candidate.ReadyReplicas = 0
		}
	}
	irs := &v1beta1.InferenceReplicaList{}
	selector := client.MatchingLabels{constants.InferenceServicePodLabelKey: source.Name}
	if err := cl.List(ctx, irs, client.InNamespace(source.Namespace), selector); err != nil {
		return out, err
	}
	policy := executionPolicy(source, previous.Allocation)
	floors, err := plannedMemberFloors(source, previous, policy)
	if err != nil {
		return out, err
	}
	applied := memberExists && member.DeletionTimestamp.IsZero()
	if memberExists {
		memberPolicy, err := protocol.FromDerived(member)
		if err != nil {
			return out, err
		}
		applied = applied && equality.Semantic.DeepEqual(memberPolicy, policy)
	}
	// The list only names the components to read. Each component is read once,
	// and that snapshot carries its ownership, policy, rows and reservations.
	reads := r.instanceStatusReader(cl)
	snapshots := make([]*v1beta1.InferenceReplica, 0, len(irs.Items))
	statuses := map[v1beta1.ComponentType]*v1beta1.InferenceReplicaStatus{}
	for i := range irs.Items {
		ir := &v1beta1.InferenceReplica{}
		if _, err := irstatus.GetDecoded(ctx, reads, client.ObjectKeyFromObject(&irs.Items[i]), ir); err != nil {
			return out, err
		}
		owner := metav1.GetControllerOf(ir)
		if owner == nil || owner.APIVersion != v1beta1.SchemeGroupVersion.String() || owner.Kind != "InferenceService" || owner.Name != source.Name || owner.UID == "" || ir.ParentName() != source.Name {
			return out, fmt.Errorf("component %q has unverified service ownership", ir.Name)
		}
		if memberExists && owner.UID != member.UID {
			return out, fmt.Errorf("component %q belongs to a different member service", ir.Name)
		}
		if !memberExists {
			p := ir.Spec.PlacementExecution
			if p == nil || p.SourceUID != source.UID || p.ClusterUID != previous.Allocation.ClusterUID {
				return out, fmt.Errorf("orphan component %q has unverified source identity", ir.Name)
			}
		}
		if _, duplicate := statuses[ir.Spec.Component]; duplicate {
			return out, fmt.Errorf("multiple live components claim %q", ir.Spec.Component)
		}
		if ir.Spec.Component != v1beta1.EngineComponent && ir.Spec.Component != v1beta1.DecoderComponent && ir.Spec.Component != v1beta1.RouterComponent {
			return out, fmt.Errorf("component %q has an unsupported role", ir.Name)
		}
		statuses[ir.Spec.Component] = &ir.Status
		snapshots = append(snapshots, ir)
	}
	// Pods are listed after every component snapshot. A Pod created since a
	// snapshot is charged to its owner, and a reservation whose Pod is not
	// listed stays charged through the snapshot, so occupancy is never lost.
	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, client.InNamespace(source.Namespace), selector); err != nil {
		return out, err
	}
	byOwner := map[types.UID][]corev1.Pod{}
	for _, pod := range pods.Items {
		owner := metav1.GetControllerOf(&pod)
		if owner == nil {
			return out, fmt.Errorf("member pod %q has no component owner", pod.Name)
		}
		byOwner[owner.UID] = append(byOwner[owner.UID], pod)
	}
	counts := map[v1beta1.ComponentType]memberResourceCount{}
	idleComponents := map[v1beta1.ComponentType]bool{}
	zeroRequests := map[v1beta1.ComponentType]bool{}
	for _, ir := range snapshots {
		gangSizes, err := memberGangSizes(ctx, cl, ir, byOwner[ir.UID])
		if err != nil {
			return out, err
		}
		status, err := verifiedMemberAdmission(ir, byOwner[ir.UID], gangSizes)
		if err != nil {
			return out, err
		}
		ir.Status = *status
		count, err := countMemberResources(ir, byOwner[ir.UID], gangSizes)
		if err != nil {
			return out, err
		}
		delete(byOwner, ir.UID)
		counts[ir.Spec.Component] = count
		if ir.Spec.Replicas == nil || *ir.Spec.Replicas < 0 {
			return out, fmt.Errorf("component %q has unresolved desired replicas", ir.Name)
		}
		zeroRequests[ir.Spec.Component] = *ir.Spec.Replicas == 0 && protocol.HasZeroReplicaFloor(policy, ir.Spec.Component)
		idleComponents[ir.Spec.Component] = zeroRequests[ir.Spec.Component] && count.Occupied == 0 && count.Reserved == 0
		// Engine and Decoder occupancy is counted in primary units through each
		// component's floor. Router stays shared within each member, with its own
		// replica policy and readiness gate.
		floor, primaryFloor := componentFloor(floors, ir.Spec.Component, previous.Allocation.CurrentReplicas), floors[primaryScaleComponent(components)]
		if ir.Spec.Component != v1beta1.RouterComponent {
			// A paused limit is durable capacity authority, even if the current
			// autoscaler request is smaller. Requests above it remain deferred.
			committed := *ir.Spec.Replicas
			if ir.Spec.PlacementExecution != nil && ir.Spec.PlacementExecution.PauseSurge && ir.Spec.PlacementReplicaLimit != nil {
				committed = *ir.Spec.PlacementReplicaLimit
			}
			out.MemberState.Occupied = max(out.MemberState.Occupied, occupiedUnits(max(count.Occupied, committed), floor, primaryFloor))
			out.RolloutReserved = max(out.RolloutReserved, occupiedUnits(count.Reserved, floor, primaryFloor))
		}
		if policy.PauseSurge {
			applied = applied && ir.Spec.PlacementReplicaLimit != nil && *ir.Spec.PlacementReplicaLimit > 0
			if ir.Spec.Component != v1beta1.RouterComponent {
				applied = applied && ir.Spec.PlacementReplicaLimit != nil && *ir.Spec.PlacementReplicaLimit >= floor
			}
		}
		applied = applied && ir.Generation > 0 && ir.Status.ObservedGeneration == ir.Generation && ir.Status.PlacementObservedGeneration == ir.Generation && ir.DeletionTimestamp.IsZero() && equality.Semantic.DeepEqual(ir.Spec.PlacementExecution, policy)
		if memberExists {
			applied = applied && ir.Annotations[constants.InferenceReplicaParentGenerationAnnotationKey] == strconv.FormatInt(member.Generation, 10)
		}
		if ir.Spec.Component != v1beta1.RouterComponent {
			applied = applied && ir.Spec.Replicas != nil && *ir.Spec.Replicas >= floor
		}
	}
	if len(byOwner) > 0 {
		return out, fmt.Errorf("member pods remain without identified live components")
	}
	if !memberExists && len(irs.Items) == 0 && len(pods.Items) == 0 {
		// Absence can release allocation authority. A newly created service may
		// still be missing from the cache before it creates any components.
		direct, _ := workloadcluster.DirectClient(transport)
		if err := direct.Get(ctx, client.ObjectKeyFromObject(source), &v1beta1.InferenceService{}); !apierrors.IsNotFound(err) {
			if err != nil {
				return out, err
			}
			return out, fmt.Errorf("%w: member service exists beyond the cached observation", workloadcluster.ErrMemberCacheUnavailable)
		}
	}
	if memberExists {
		out.Member = member
	}
	scaled := make([]v1beta1.ComponentType, 0, len(components))
	readyCounts := map[v1beta1.ComponentType]int32{}
	routerReady := true
	allAdmitted, allServing := true, true
	idleZeroFloor := true
	zeroRequested := true
	for _, component := range components {
		count, exists := counts[component]
		applied = applied && exists
		admitted := componentHasAdmittedInstance(statuses[component])
		allAdmitted = allAdmitted && admitted
		if !admitted {
			idleZeroFloor = idleZeroFloor && idleComponents[component]
			zeroRequested = zeroRequested && zeroRequests[component]
		}
		if component == v1beta1.RouterComponent {
			allServing = allServing && statuses[component] != nil && statuses[component].ReadyReplicas > 0
			routerReady = routerReady && count.Ready > 0
			continue
		}
		if component != v1beta1.EngineComponent && component != v1beta1.DecoderComponent {
			return out, fmt.Errorf("unresolved placement component %q", component)
		}
		scaled = append(scaled, component)
		readyCounts[component] = count.Ready
	}
	if len(scaled) == 0 {
		return out, fmt.Errorf("placement requires an engine or decoder component")
	}
	ready := primaryUnits(scaled, readyCounts, floors)
	if !routerReady {
		ready = 0
	}
	out.MemberState.Known = true
	out.MemberState.Absent = !memberExists && len(irs.Items) == 0 && len(pods.Items) == 0
	out.MemberState.Applied = applied
	out.MemberState.Ready = ready
	out.PauseAcknowledged = applied && policy.PauseSurge
	out.FullPolicyReady = applied && len(policy.ReplicaFloors) > 0
	for _, floor := range policy.ReplicaFloors {
		out.FullPolicyReady = out.FullPolicyReady && counts[floor.Component].Ready >= floor.Replicas
	}
	if memberExists {
		out.Terminal = IsTerminallyFailed(member, statuses)
	}
	// Zero demand preserves member identity without inventing admission. Pending
	// cleanup is distinct from an idle member and cannot establish convergence.
	out.IdleZeroFloor = applied && memberExists && !out.Terminal && !allAdmitted && idleZeroFloor
	out.ScalingToZero = applied && memberExists && !out.Terminal && !allAdmitted && zeroRequested && !idleZeroFloor
	out.FullPolicyReady = out.FullPolicyReady && !out.Terminal
	out.Candidate = v1beta1.CandidatePlacement{Cluster: previous.Cluster, Allocation: previous.Allocation.DeepCopy(), ObservationKnown: true, Phase: v1beta1.CandidatePhaseAdmitting}
	if applied {
		out.Candidate.AppliedPlanID = policy.PlanID
	}
	if memberExists && member.DeletionTimestamp.IsZero() {
		if allAdmitted {
			out.Candidate.AdmittedReplicas = placementAdmittedReplicas(scaled, statuses, floors)
		}
		if out.Candidate.AdmittedReplicas > 0 {
			out.Candidate.Phase = v1beta1.CandidatePhaseAdmitted
			out.Candidate.Endpoint = endpointFor(member).DeepCopy()
		}
		if member.Status.IsConditionReady(v1beta1.IngressReady) && allAdmitted && allServing {
			out.Candidate.ReadyReplicas = placementReadyReplicas(scaled, statuses, servingFloors(source, floors))
			out.ComponentReady = readyCounts
			if count, divided := counts[v1beta1.RouterComponent]; divided && routerDivided(source) {
				out.ComponentReady[v1beta1.RouterComponent] = count.Ready
			}
		} else {
			out.MemberState.Ready = 0
		}
	} else {
		out.MemberState.Ready = 0
	}
	out.FullPolicyReady = out.FullPolicyReady && out.Candidate.ReadyReplicas > 0
	return out, nil
}

// plannedMemberFloors returns the per-component floors a planned member is held
// to: the accepted full policy when the plan carries one, the split
// apportionment over the plan's current targets otherwise. Without either,
// every scaled component is held to the member's primary target.
func plannedMemberFloors(source *v1beta1.InferenceService, previous v1beta1.CandidatePlacement, policy *v1beta1.PlacementExecutionPolicy) (map[v1beta1.ComponentType]int32, error) {
	if len(policy.ReplicaFloors) > 0 {
		return floorMap(policy.ReplicaFloors), nil
	}
	if mode := placementMode(source); mode != v1beta1.PlacementModeSplit && mode != v1beta1.PlacementModeSplitByCapacity {
		return nil, nil
	}
	floors, err := splitMemberFloors(source, source.Status.Placement.Candidates, previous)
	if err != nil {
		return nil, err
	}
	return floorMap(floors), nil
}

// componentFloor is the floor a component is held to; a component outside the
// accepted floors is held to the member's primary target.
func componentFloor(floors map[v1beta1.ComponentType]int32, component v1beta1.ComponentType, primary int32) int32 {
	if floor, known := floors[component]; known {
		return floor
	}
	return primary
}

// primaryScaleComponent is the component whose instance count is the member's
// replica unit: the engine when declared, otherwise the decoder.
func primaryScaleComponent(components []v1beta1.ComponentType) v1beta1.ComponentType {
	for _, component := range []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent} {
		if slices.Contains(components, component) {
			return component
		}
	}
	return ""
}

// plannedTrafficEvidence reads routing intent for the exact accepted allocation.
// Publisher acknowledgement describes external realization, not member authority.
func plannedTrafficEvidence(source *v1beta1.InferenceService, trafficMap *v1beta1.TrafficMap, candidate v1beta1.CandidatePlacement) (known, routable, drained bool) {
	if source == nil || source.Status.Placement == nil || source.Status.Placement.Plan == nil || candidate.Allocation == nil || trafficMap == nil || !trafficMap.DeletionTimestamp.IsZero() || !TrafficMapHasExactController(trafficMap, source) {
		return false, false, false
	}
	accepted := source.Status.Placement.Plan
	if accepted.ID == "" || accepted.Revision <= 0 || accepted.SourceUID != source.UID || accepted.ObservedGeneration != source.Generation {
		return false, false, false
	}
	if trafficMap.Status.SourceUID != source.UID || trafficMap.Spec.PlacementPlanID != source.Status.Placement.Plan.ID || trafficMap.Spec.ObservedISVCGeneration != source.Generation || trafficMap.Generation <= 0 {
		return false, false, false
	}
	var entry *v1beta1.TrafficMapEntry
	for i := range trafficMap.Spec.Entries {
		if trafficMap.Spec.Entries[i].Cluster == candidate.Cluster {
			if entry != nil {
				return false, false, false
			}
			entry = &trafficMap.Spec.Entries[i]
		}
	}
	if entry == nil || entry.Weight == 0 {
		return true, false, candidate.Allocation.DrainRequested
	}
	if entry.Weight < 0 || entry.Endpoint == nil || candidate.Endpoint == nil || entry.Endpoint.String() != candidate.Endpoint.String() {
		return false, false, false
	}
	return true, true, false
}
