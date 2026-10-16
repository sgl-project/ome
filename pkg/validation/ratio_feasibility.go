package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// RatioDrainDefaults carries the operator-configured fills the drain
// feasibility rule reads, so admission judges a group as the rollout will
// run it. Both come from the operator ConfigMap, read once at startup.
type RatioDrainDefaults struct {
	// TolerancePercent fills a group that sets maintainRatio but omits its
	// tolerance (the coordination block). Nil means no default is
	// configured: such a group rolls with no drift bound and is not judged.
	TolerancePercent *int32
	// MinReplicas fills a Component that sets no minReplicas (the
	// deploy.replicas block), the fill the reconciler applies before it
	// projects the Instance count. Nil means unconfigured, and an unset
	// count then resolves to the API default of one.
	MinReplicas *int
}

// ValidateRatioDrainFeasibility rejects a maintainRatio group whose drain-first
// member could never take its first rollout step. The ratio check at runtime
// reads a drain as one Instance leaving rotation and admits it only while the
// live serving ratio of every member pair stays within the tolerance of the
// ratio the rollout started at, or, for a single drain from a fully serving
// group, within twice the tolerance; it never admits a drain that empties a
// member's side. A member whose single drain fails even the relaxed rule is
// held before its first step and the rollout parks for good, so admission
// refuses the spec instead.
//
// A group whose members all roll drain-first is paced by the batch step
// instead of the ratio check: each member's drains are counted in batches of
// its maxUnavailable and no member opens its next batch until every member's
// previous batch serves, which keeps the ratio at every batch boundary. The
// ratio check is never consulted for such a group, so nothing here refuses it
// (batchStepGoverns). The refusal stays for a group that mixes a drain-first
// member with one that surges or leaves its strategy unset.
//
// A member rolls drain-first when its update strategy is RecreatePod, or an
// in-place strategy on a Component whose Instances are multi-node gangs (an
// in-place update rebuilds the whole gang, and the ratio check reads the
// rebuild as a drain). SurgeThenDrain members and single-pod in-place members
// add or keep capacity and are not checked. An unset strategy is not classified
// here: it resolves through the runtime and operator config, which admission
// does not read.
//
// Replicas are read the way the rollout sizes a member: minReplicas when set
// and positive, else maxReplicas when set, else the count the reconciler
// projects for an unset field: the configured minReplicas default, else the
// API default of one. A Component the group names but the spec does not
// declare is left out of the pairwise check.
func ValidateRatioDrainFeasibility(spec *v1beta1.InferenceServiceSpec, defaults RatioDrainDefaults) error {
	groups := spec.GetRolloutGroups()
	for i := range groups {
		g := &groups[i]
		if g.MaintainRatio == nil || rolloutGroupKind(g) == v1beta1.RolloutProgressionCanary {
			continue
		}
		tolerance := g.MaintainRatio.Tolerance
		if tolerance == nil {
			tolerance = defaults.TolerancePercent
		}
		if tolerance == nil {
			continue
		}
		members := ratioGroupMembers(spec, g.Components, defaults.MinReplicas)
		if len(members) < 2 || batchStepGoverns(members) {
			continue
		}
		for _, m := range members {
			if m.drainFirst == "" {
				continue
			}
			if err := ratioDrainFeasible(i, m, members, *tolerance); err != nil {
				return err
			}
		}
	}
	return nil
}

// ratioMember is one group member as the feasibility rule sees it: its
// replica count, the unit a drain removes, and why it rolls drain-first
// (empty when it does not).
type ratioMember struct {
	component  v1beta1.ComponentType
	replicas   int32
	unit       string
	drainFirst string
}

// batchStepGoverns reports whether the rollout paces the group by the batch
// step rather than the ratio check: two or more members, every one of them
// rolling drain-first. It is the admission reading of the engine's rule, taken
// from the declared strategies and shapes rather than the replicas.
func batchStepGoverns(members []ratioMember) bool {
	for _, m := range members {
		if m.drainFirst == "" {
			return false
		}
	}
	return len(members) >= 2
}

// ratioGroupMembers lists the group's distinct declared Components, in
// declaration order, each at the replica count the rollout would size it at.
func ratioGroupMembers(spec *v1beta1.InferenceServiceSpec, components []v1beta1.ComponentType, defaultMinReplicas *int) []ratioMember {
	seen := make(map[v1beta1.ComponentType]struct{}, len(components))
	members := make([]ratioMember, 0, len(components))
	for _, c := range components {
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		replicas := componentReplicaBasis(spec, c, defaultMinReplicas)
		if replicas <= 0 {
			continue
		}
		strategy, gang := componentUpdateShape(spec, c)
		m := ratioMember{component: c, replicas: replicas, unit: "pod"}
		if gang {
			m.unit = "gang"
		}
		switch {
		case strategy == v1beta1.UpdateStrategyRecreatePod:
			m.drainFirst = string(strategy)
		case gang && (strategy == v1beta1.UpdateStrategyInPlaceIfPossible || strategy == v1beta1.UpdateStrategyInPlaceOnly):
			m.drainFirst = fmt.Sprintf("%s on multi-node gangs, which rebuilds the whole gang", strategy)
		}
		members = append(members, m)
	}
	return members
}

// ratioDrainFeasible projects one drain of m against every peer exactly as
// the runtime ratio check does: the starting pairwise ratio is the larger
// count over the smaller, the projected ratio the same with m one unit down,
// and the projection must stay within twice the tolerance of the start. A
// member of one has no admissible drain at all.
func ratioDrainFeasible(groupIdx int, m ratioMember, members []ratioMember, tolerancePercent int32) error {
	if m.replicas == 1 {
		return fmt.Errorf("spec.rollout.groups[%d]: the %s rolls drain-first (%s) in a maintainRatio group with a single replica, so its only drain leaves it with nothing serving, which the ratio check refuses, and the rollout would park for good; give the %s the SurgeThenDrain update strategy, add %s replicas, or roll every member of the group drain-first, which hands the pacing to the batch step (%s)",
			groupIdx, m.component, m.drainFirst, m.component, m.component, ReasonRatioDrainInfeasible)
	}
	band := 2 * float64(tolerancePercent) / 100.0
	projected := m.replicas - 1
	for _, p := range members {
		if p.component == m.component {
			continue
		}
		if ratioWithinBand(m.replicas, p.replicas, projected, p.replicas, band) {
			continue
		}
		offPoints := ratioOffPoints(m.replicas, p.replicas, projected, p.replicas)
		return fmt.Errorf("spec.rollout.groups[%d]: the %s rolls drain-first (%s) in a maintainRatio group, and its first step could never be admitted: with %s %d and %s %d, one %s %s out reads %s %d : %s %d, %d points off the starting ratio, and the ratio check lets a single drain go at most %d points (twice the tolerance of %d), so the rollout would park for good; widen maintainRatio.tolerance to at least %d, give the %s the SurgeThenDrain update strategy, add %s replicas, or roll every member of the group drain-first, which hands the pacing to the batch step (%s)",
			groupIdx, m.component, m.drainFirst, m.component, m.replicas, p.component, p.replicas,
			m.component, m.unit, m.component, projected, p.component, p.replicas, offPoints,
			2*tolerancePercent, tolerancePercent, ratioToleranceNeeded(m.replicas, p.replicas, projected, p.replicas),
			m.component, m.component, ReasonRatioDrainInfeasible)
	}
	return nil
}

// ratioWithinBand is the runtime ratio check's pairwise test on one pair:
// the starting ratio is the larger original count over the smaller, the
// projected ratio the larger projected count over the smaller, and the
// projection passes when it lies within start times one plus or minus band,
// bounds included. A side projected to nothing serving is out.
func ratioWithinBand(origA, origB, projA, projB int32, band float64) bool {
	if origA < origB {
		origA, origB = origB, origA
	}
	origRatio := float64(origA) / float64(origB)
	if projA < projB {
		projA, projB = projB, projA
	}
	if projB <= 0 {
		return false
	}
	projRatio := float64(projA) / float64(projB)
	return projRatio >= origRatio*(1-band) && projRatio <= origRatio*(1+band)
}

// ratioOffPoints is how far, in percent of the starting ratio, the projected
// pairwise ratio sits from it, rounded to the nearest point.
func ratioOffPoints(origA, origB, projA, projB int32) int32 {
	if origA < origB {
		origA, origB = origB, origA
	}
	if projA < projB {
		projA, projB = projB, projA
	}
	origRatio := float64(origA) / float64(origB)
	projRatio := float64(projA) / float64(projB)
	off := (projRatio - origRatio) / origRatio
	if off < 0 {
		off = -off
	}
	return int32(off*100 + 0.5)
}

// ratioToleranceNeeded is the smallest whole tolerance under which the
// projected pair passes the relaxed single-drain rule, found with the same
// band test the rejection used so the suggestion is one that admits.
func ratioToleranceNeeded(origA, origB, projA, projB int32) int32 {
	for t := int32(1); t <= 100; t++ {
		if ratioWithinBand(origA, origB, projA, projB, 2*float64(t)/100.0) {
			return t
		}
	}
	return 100
}

// componentReplicaBasis is the replica count the rollout sizes a Component
// at: minReplicas when set and positive, else maxReplicas when set, else the
// count the reconciler projects for a Component that sets neither. That
// projection fills an unset minReplicas from the configured default before
// it reads the field, and reads a still-unset or non-positive field as the
// API default of one. Zero only for a Component the spec does not declare.
func componentReplicaBasis(spec *v1beta1.InferenceServiceSpec, c v1beta1.ComponentType, defaultMinReplicas *int) int32 {
	ext := componentExtension(spec, c)
	if ext == nil {
		return 0
	}
	if ext.MinReplicas != nil && *ext.MinReplicas > 0 {
		return int32(*ext.MinReplicas)
	}
	if ext.MaxReplicas > 0 {
		return int32(ext.MaxReplicas)
	}
	if defaultMinReplicas != nil && *defaultMinReplicas > 0 {
		return int32(*defaultMinReplicas)
	}
	return int32(constants.DefaultMinReplicas)
}

// componentUpdateShape reports the Component's declared update strategy
// (empty when unset) and whether its Instances are multi-pod gangs.
func componentUpdateShape(spec *v1beta1.InferenceServiceSpec, c v1beta1.ComponentType) (v1beta1.UpdateStrategyType, bool) {
	ext := componentExtension(spec, c)
	if ext == nil {
		return "", false
	}
	var strategy v1beta1.UpdateStrategyType
	if ext.Lifecycle != nil && ext.Lifecycle.UpdateStrategy != nil {
		strategy = ext.Lifecycle.UpdateStrategy.Type
	}
	switch c {
	case v1beta1.EngineComponent:
		return strategy, DeclaredMultiPod(engineLeader(spec.Engine), engineWorker(spec.Engine))
	case v1beta1.DecoderComponent:
		return strategy, DeclaredMultiPod(decoderLeader(spec.Decoder), decoderWorker(spec.Decoder))
	}
	return strategy, false
}

// componentExtension returns the Component's extension spec, nil when the
// Component is not declared.
func componentExtension(spec *v1beta1.InferenceServiceSpec, c v1beta1.ComponentType) *v1beta1.ComponentExtensionSpec {
	if spec == nil {
		return nil
	}
	switch c {
	case v1beta1.EngineComponent:
		if spec.Engine != nil {
			return &spec.Engine.ComponentExtensionSpec
		}
	case v1beta1.DecoderComponent:
		if spec.Decoder != nil {
			return &spec.Decoder.ComponentExtensionSpec
		}
	case v1beta1.RouterComponent:
		if spec.Router != nil {
			return &spec.Router.ComponentExtensionSpec
		}
	}
	return nil
}

// DeclaredMultiPod reports whether a Component declares a Leader+Worker pair
// whose size is unset or positive; an unset size resolves to one worker at
// reconcile time, so every such Instance is a gang of more than one pod.
func DeclaredMultiPod(leader *v1beta1.LeaderSpec, worker *v1beta1.WorkerSpec) bool {
	return leader != nil && worker != nil && (worker.Size == nil || *worker.Size > 0)
}
