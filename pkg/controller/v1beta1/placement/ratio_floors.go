package placement

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/bits"
	"slices"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/placement/affinity"
	"sigs.k8s.io/ome/pkg/placement/protocol"
)

// plannedBound fixes one component's replica band on a split member. A zero
// ceiling pins the component at its floor.
type plannedBound struct {
	Floor   int32
	Ceiling int32
}

// splitMinimums is the fleet minimum of every scalable component a split
// source apportions, keyed by component, with the component whose instance
// count is the placement unit.
type splitMinimums struct {
	primary v1beta1.ComponentType
	floors  map[v1beta1.ComponentType]int32
}

func (m splitMinimums) secondaries() map[v1beta1.ComponentType]int32 {
	out := map[v1beta1.ComponentType]int32{}
	for component, floor := range m.floors {
		if component != m.primary {
			out[component] = floor
		}
	}
	return out
}

// splitComponentMinimums reads the source's fleet minimums. The primary floor
// is split.replicas or the engine minimum. A declared decoder minimum follows
// the engine minimum in their declared ratio, scaled to the primary floor and
// rounded up; without a declared ratio it is used as declared. An undeclared
// decoder minimum pairs one decoder with each primary unit. A router divided
// across members contributes its minimum, scaled the same way.
func splitComponentMinimums(source *v1beta1.InferenceService) (splitMinimums, error) {
	primaryFloor := splitDesiredReplicas(source)
	if primaryFloor <= 0 {
		return splitMinimums{}, fmt.Errorf("ReplicaCountUnresolved: declare split.replicas or a positive engine.minReplicas")
	}
	out := splitMinimums{floors: map[v1beta1.ComponentType]int32{}}
	for _, component := range placementScaleComponents(source) {
		if component != v1beta1.EngineComponent && component != v1beta1.DecoderComponent {
			continue
		}
		if out.primary == "" {
			out.primary = component
			out.floors[component] = primaryFloor
			continue
		}
		minimum := int64(primaryFloor)
		if component == v1beta1.DecoderComponent && source.Spec.Decoder.MinReplicas != nil {
			declared := int64(*source.Spec.Decoder.MinReplicas)
			if declared < 0 || declared > math.MaxInt32 {
				return splitMinimums{}, fmt.Errorf("decoder.minReplicas must be a nonnegative replica count")
			}
			minimum = declared
			if engine := source.Spec.Engine.MinReplicas; engine != nil && *engine > 0 && int64(*engine) != int64(primaryFloor) {
				minimum = (declared*int64(primaryFloor) + int64(*engine) - 1) / int64(*engine)
			}
		}
		out.floors[component] = int32(min(minimum, math.MaxInt32))
	}
	if out.primary == "" {
		return splitMinimums{}, fmt.Errorf("placement requires an engine or decoder component")
	}
	if routerDivided(source) {
		if source.Spec.Router.MinReplicas == nil || *source.Spec.Router.MinReplicas <= 0 {
			return splitMinimums{}, fmt.Errorf("router.minReplicas must be positive to divide routers across homes")
		}
		minimum := int64(*source.Spec.Router.MinReplicas)
		var declared *int
		if out.primary == v1beta1.EngineComponent {
			declared = source.Spec.Engine.MinReplicas
		} else {
			declared = source.Spec.Decoder.MinReplicas
		}
		if declared != nil && *declared > 0 && int64(*declared) != int64(primaryFloor) {
			minimum = (minimum*int64(primaryFloor) + int64(*declared) - 1) / int64(*declared)
		}
		out.floors[v1beta1.RouterComponent] = int32(min(minimum, math.MaxInt32))
	}
	return out, nil
}

// routerDivided reports whether the source divides its routers across Split
// members: an affinity term sets a router weight. Otherwise every member runs
// the source's full router policy.
func routerDivided(source *v1beta1.InferenceService) bool {
	return source.Spec.Router != nil && placementMode(source) == v1beta1.PlacementModeSplit &&
		affinity.DividesRouters(source.Spec.Placement.ClusterAffinity)
}

// splitMemberFloors apportions every secondary component's minimum over the
// members' current primary targets, in the fleet ratio of whatever those targets
// sum to, and returns the named member's floors. The member's own allocation is
// authoritative for its target.
func splitMemberFloors(source *v1beta1.InferenceService, candidates []v1beta1.CandidatePlacement, member v1beta1.CandidatePlacement) ([]v1beta1.PlacementComponentFloor, error) {
	if member.Cluster == "" || member.Allocation == nil {
		return nil, fmt.Errorf("split home floors require an identified allocation")
	}
	minimums, err := splitComponentMinimums(source)
	if err != nil {
		return nil, err
	}
	targets := map[string]int32{}
	allocations := map[string]v1beta1.CandidateAllocationStatus{}
	for _, candidate := range candidates {
		if candidate.Allocation != nil && candidate.Cluster != "" {
			targets[candidate.Cluster] = candidate.Allocation.CurrentReplicas
			allocations[candidate.Cluster] = *candidate.Allocation
		}
	}
	targets[member.Cluster] = member.Allocation.CurrentReplicas
	allocations[member.Cluster] = *member.Allocation
	floors, err := apportionSplitFloors(minimums, targets, splitWeightsOf(source, allocations))
	if err != nil {
		return nil, err
	}
	return floors[member.Cluster], nil
}

// splitWeights holds the accepted static weights of a split plan: the primary
// component's, and each other component's that the plan records. Only matched
// members carry weights.
type splitWeights struct {
	primary    map[string]int64
	components map[v1beta1.ComponentType]map[string]int64
}

// splitWeightsOf reads the weights an accepted or proposed allocation records.
// Only static Split gives a component its own weights; every other plan
// apportions by the primary targets alone.
func splitWeightsOf(source *v1beta1.InferenceService, allocations map[string]v1beta1.CandidateAllocationStatus) splitWeights {
	out := splitWeights{primary: map[string]int64{}, components: map[v1beta1.ComponentType]map[string]int64{}}
	if placementMode(source) != v1beta1.PlacementModeSplit {
		return out
	}
	for name, allocation := range allocations {
		if allocation.Weight == nil {
			continue
		}
		out.primary[name] = *allocation.Weight
		for _, match := range allocation.ComponentMatches {
			if match.Weight == nil {
				continue
			}
			if out.components[match.Component] == nil {
				out.components[match.Component] = map[string]int64{}
			}
			out.components[match.Component][name] = *match.Weight
		}
	}
	return out
}

// ownWeights returns the components whose weights are not proportional to the
// primary component's. They are apportioned by their own weights; every other
// component follows the primary targets.
func (w splitWeights) ownWeights() map[v1beta1.ComponentType]map[string]int64 {
	out := map[v1beta1.ComponentType]map[string]int64{}
	for component, weights := range w.components {
		if !proportionalWeights(w.primary, weights) {
			out[component] = weights
		}
	}
	return out
}

// proportionalWeights reports whether two weight maps differ only by a
// constant positive factor, compared by exact cross-multiplication.
func proportionalWeights(a, b map[string]int64) bool {
	names := map[string]bool{}
	for name := range a {
		names[name] = true
	}
	for name := range b {
		names[name] = true
	}
	var refA, refB int64
	for _, name := range slices.Sorted(maps.Keys(names)) {
		if a[name] > 0 || b[name] > 0 {
			refA, refB = a[name], b[name]
			break
		}
	}
	if refA <= 0 || refB <= 0 {
		return refA == 0 && refB == 0
	}
	for name := range names {
		leftHi, leftLo := bits.Mul64(uint64(max(a[name], 0)), uint64(refB))
		rightHi, rightLo := bits.Mul64(uint64(max(b[name], 0)), uint64(refA))
		if leftHi != rightHi || leftLo != rightLo {
			return false
		}
	}
	return true
}

// servingFloors returns the floors a member's ready count is measured against
// for routing. A component apportioned by its own weights gives members
// different mixes, so every member is then measured against the service's ratio
// instead of its own floors, and traffic follows what each member can serve.
func servingFloors(source *v1beta1.InferenceService, floors map[v1beta1.ComponentType]int32) map[v1beta1.ComponentType]int32 {
	if source == nil || source.Status.Placement == nil {
		return floors
	}
	allocations := map[string]v1beta1.CandidateAllocationStatus{}
	for _, candidate := range source.Status.Placement.Candidates {
		if candidate.Allocation != nil {
			allocations[candidate.Cluster] = *candidate.Allocation
		}
	}
	if len(splitWeightsOf(source, allocations).ownWeights()) == 0 {
		return floors
	}
	minimums, err := splitComponentMinimums(source)
	if err != nil {
		return floors
	}
	return minimums.floors
}

// apportionSplitFloors apportions every secondary component's minimum over the
// members' primary targets. A component without its own weights, or whose
// weights are proportional to the primary component's, follows the primary
// targets. Any other component is apportioned by its own weights over the
// members that hold the primary component, at least one each.
func apportionSplitFloors(minimums splitMinimums, targets map[string]int32, weights splitWeights) (map[string][]v1beta1.PlacementComponentFloor, error) {
	own := weights.ownWeights()
	following := map[v1beta1.ComponentType]int32{}
	for component, minimum := range minimums.secondaries() {
		if _, independent := own[component]; !independent {
			following[component] = minimum
		}
	}
	primaryMinimum := minimums.floors[minimums.primary]
	floors, err := protocol.ApportionReplicaFloors(minimums.primary, primaryMinimum, targets, following)
	if err != nil {
		return nil, err
	}
	for _, component := range slices.Sorted(maps.Keys(own)) {
		minimum, declared := minimums.floors[component]
		if !declared || component == minimums.primary {
			continue
		}
		shares, err := protocol.ApportionWeightedReplicaFloor(component, primaryMinimum, targets, minimum, own[component])
		if err != nil {
			return nil, err
		}
		for name := range targets {
			floors[name] = append(floors[name], v1beta1.PlacementComponentFloor{Component: component, Replicas: shares[name]})
			slices.SortFunc(floors[name], func(a, b v1beta1.PlacementComponentFloor) int { return cmp.Compare(a.Component, b.Component) })
		}
	}
	return floors, nil
}

// splitPlannedBounds derives the member replica band of each scalable
// component on a split member. The local ceiling is declared in primary
// replicas; a secondary component's ceiling is that ceiling in its own ratio,
// rounded up.
func splitPlannedBounds(source *v1beta1.InferenceService, candidates []v1beta1.CandidatePlacement, member v1beta1.CandidatePlacement) (map[v1beta1.ComponentType]plannedBound, error) {
	floors, err := splitMemberFloors(source, candidates, member)
	if err != nil {
		return nil, err
	}
	minimums, err := splitComponentMinimums(source)
	if err != nil {
		return nil, err
	}
	var ceiling int32
	if source.Spec.Placement != nil && source.Spec.Placement.Split != nil {
		ceiling = source.Spec.Placement.Split.MaxReplicasPerCluster
	}
	out := make(map[v1beta1.ComponentType]plannedBound, len(floors))
	primaryMinimum := int64(minimums.floors[minimums.primary])
	for _, floor := range floors {
		bound := plannedBound{Floor: floor.Replicas, Ceiling: ceiling}
		if ceiling > 0 && floor.Component != minimums.primary {
			scaled := (int64(ceiling)*int64(minimums.floors[floor.Component]) + primaryMinimum - 1) / primaryMinimum
			bound.Ceiling = int32(min(scaled, math.MaxInt32))
		}
		out[floor.Component] = bound
	}
	return out, nil
}

// setPlannedReplicas fixes member bounds at the allocation unless an explicit
// local ceiling permits autoscaling. A retained floor can exceed a reduced cap
// until its replacement serves. Components without a bound keep their policy.
func setPlannedReplicas(member *v1beta1.InferenceService, bounds map[v1beta1.ComponentType]plannedBound) {
	apply := func(component *v1beta1.ComponentExtensionSpec, bound plannedBound) {
		floor := int(bound.Floor)
		component.MinReplicas = &floor
		if bound.Ceiling > 0 {
			component.MaxReplicas = int(max(bound.Ceiling, bound.Floor))
		} else {
			component.MaxReplicas = floor
		}
	}
	if bound, ok := bounds[v1beta1.EngineComponent]; ok && member.Spec.Engine != nil {
		apply(&member.Spec.Engine.ComponentExtensionSpec, bound)
	}
	if bound, ok := bounds[v1beta1.DecoderComponent]; ok && member.Spec.Decoder != nil {
		apply(&member.Spec.Decoder.ComponentExtensionSpec, bound)
	}
	if bound, ok := bounds[v1beta1.RouterComponent]; ok && member.Spec.Router != nil {
		apply(&member.Spec.Router.ComponentExtensionSpec, bound)
	}
}

// floorMap indexes accepted floors by component.
func floorMap(floors []v1beta1.PlacementComponentFloor) map[v1beta1.ComponentType]int32 {
	if len(floors) == 0 {
		return nil
	}
	out := make(map[v1beta1.ComponentType]int32, len(floors))
	for _, floor := range floors {
		out[floor.Component] = floor.Replicas
	}
	return out
}

// primaryUnits expresses per-component counts in whole primary units: each
// component covers count/floor of its share and the member has the smallest
// coverage. Equal or unknown floors reduce to the minimum count; a component
// with a zero floor is skipped.
func primaryUnits(comps []v1beta1.ComponentType, counts map[v1beta1.ComponentType]int32, floors map[v1beta1.ComponentType]int32) int32 {
	if len(comps) == 0 {
		return 0
	}
	primaryFloor := floors[comps[0]]
	units := int32(math.MaxInt32)
	for _, component := range comps {
		count := counts[component]
		floor, known := floors[component]
		switch {
		case primaryFloor <= 0 || !known:
			units = min(units, count)
		case floor <= 0:
			continue
		default:
			units = min(units, int32(min(int64(count)*int64(primaryFloor)/int64(floor), math.MaxInt32)))
		}
	}
	return units
}

// everyRequiredComponentPresent reports whether a member has an instance of every
// component primaryUnits measures: the primary component, and each other
// component without a known zero floor. It is false when the primary floor is
// not positive, because primaryUnits then already counts whole instances.
func everyRequiredComponentPresent(comps []v1beta1.ComponentType, counts map[v1beta1.ComponentType]int32, floors map[v1beta1.ComponentType]int32) bool {
	if len(comps) == 0 || floors[comps[0]] <= 0 {
		return false
	}
	for _, component := range comps {
		if floor, known := floors[component]; known && floor <= 0 {
			continue
		}
		if counts[component] < 1 {
			return false
		}
	}
	return true
}

// occupiedUnits converts one component's occupancy into whole primary units,
// rounding up so a partially consumed unit still counts against the budget.
// Without a positive floor on both sides the raw count is kept.
func occupiedUnits(count, floor, primaryFloor int32) int32 {
	if count <= 0 || floor <= 0 || primaryFloor <= 0 {
		return count
	}
	return int32(min((int64(count)*int64(primaryFloor)+int64(floor)-1)/int64(floor), math.MaxInt32))
}
