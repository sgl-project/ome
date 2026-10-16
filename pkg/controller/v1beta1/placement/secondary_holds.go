package placement

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// secondaryHome is one split home's view of a secondary component: the floor
// its member was last given, the floor the current step apportions, and its
// ready instances.
type secondaryHome struct {
	Applied, Next, Ready int32
	// Known means the home was observed; Serving means its ready instances
	// carry traffic.
	Known, Serving bool
}

// secondaryFloorHolds lowers each home's floor toward its next value without
// losing ready instances the fleet still needs. Unready instances above the
// next floor leave at once. Ready ones leave only while ready instances on
// serving homes stay at or above the fleet minimum, granted in name order. An
// unobserved home keeps its applied floor. Only floors above Next are returned.
func secondaryFloorHolds(minimum int32, homes map[string]secondaryHome) map[string]int32 {
	var serving int64
	for _, home := range homes {
		if home.Known && home.Serving {
			serving += int64(home.Ready)
		}
	}
	budget := max(int64(0), serving-int64(minimum))
	out := map[string]int32{}
	for _, name := range slices.Sorted(maps.Keys(homes)) {
		home := homes[name]
		if home.Applied <= home.Next {
			continue
		}
		floor := home.Applied
		if home.Known {
			floor = max(home.Next, min(home.Applied, home.Ready))
			release := int32(min(int64(floor-home.Next), budget))
			budget -= int64(release)
			floor -= release
		}
		if floor > home.Next {
			out[name] = floor
		}
	}
	return out
}

// splitSecondaryHolds returns, per split home, the secondary floors that stay
// above the step's apportionment until replacements elsewhere are ready. The
// transition gates primary targets on readiness, but a secondary floor also
// falls when the apportionment shifts under an unchanged target. A home with
// no target or asked to drain is retired by the transition and is not held.
func splitSecondaryHolds(source *v1beta1.InferenceService, observations map[string]plannedHomeObservation) (map[string]map[v1beta1.ComponentType]int32, error) {
	if mode := placementMode(source); mode != v1beta1.PlacementModeSplit && mode != v1beta1.PlacementModeSplitByCapacity {
		return nil, nil
	}
	minimums, err := splitComponentMinimums(source)
	if err != nil {
		return nil, err
	}
	secondaries := minimums.secondaries()
	if len(secondaries) == 0 {
		return nil, nil
	}
	candidates := source.Status.Placement.Candidates
	homes := map[v1beta1.ComponentType]map[string]secondaryHome{}
	for _, candidate := range candidates {
		assignment := candidate.Allocation
		if assignment == nil || assignment.CurrentHome != nil || assignment.CurrentReplicas <= 0 || assignment.DrainRequested {
			continue
		}
		floors, err := splitHomeFloors(source, candidates, candidate)
		if err != nil {
			return nil, err
		}
		next := floorMap(floors)
		observed := observations[candidate.Cluster]
		applied := memberFloors(observed.Member)
		for component := range secondaries {
			if homes[component] == nil {
				homes[component] = map[string]secondaryHome{}
			}
			homes[component][candidate.Cluster] = secondaryHome{
				Applied: applied[component], Next: next[component], Ready: observed.ComponentReady[component],
				Known: observed.Home.Known, Serving: observed.Home.Routable,
			}
		}
	}
	out := map[string]map[v1beta1.ComponentType]int32{}
	for component, minimum := range secondaries {
		for name, floor := range secondaryFloorHolds(minimum, homes[component]) {
			if out[name] == nil {
				out[name] = map[v1beta1.ComponentType]int32{}
			}
			out[name][component] = floor
		}
	}
	return out, nil
}

// memberFloors reads the floors written on a member copy, including the router
// floor that a divided router receives.
func memberFloors(member *v1beta1.InferenceService) map[v1beta1.ComponentType]int32 {
	floors := observedHomeFloors(member)
	if member != nil && member.Spec.Router != nil {
		if floor := member.Spec.Router.MinReplicas; floor != nil && *floor >= 0 && int64(*floor) <= math.MaxInt32 {
			floors[v1beta1.RouterComponent] = int32(*floor)
		}
	}
	return floors
}

// heldFloorsMessage names each held floor for the progress condition.
func heldFloorsMessage(holds map[string]map[v1beta1.ComponentType]int32) string {
	var parts []string
	for _, name := range slices.Sorted(maps.Keys(holds)) {
		for _, component := range slices.Sorted(maps.Keys(holds[name])) {
			parts = append(parts, fmt.Sprintf("%s keeps %d %s replicas", name, holds[name][component], component))
		}
	}
	return strings.Join(parts, "; ") + " until their replacements on other clusters are ready"
}
