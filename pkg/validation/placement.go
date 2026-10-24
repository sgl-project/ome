package validation

import (
	"errors"
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/placement/affinity"
)

// placementPolicyRemedy follows every refusal of a policy other than
// ClusterAffinity, so a service that still applies the removed
// ComponentAffinity policy is told where per-component weights live.
const placementPolicyRemedy = "the ComponentAffinity policy was removed, set each component's weight in the weights block of each clusterAffinity term"

// ErrAcceptedPlanRequiresClusterAffinity refuses a service that holds an
// accepted placement plan while its spec selects another policy.
var ErrAcceptedPlanRequiresClusterAffinity = errors.New("an accepted placement plan requires spec.placement.policy ClusterAffinity; " + placementPolicyRemedy)

// ValidatePlacement checks explicit placement syntax independently of member
// health or runtime resolution. An absent block leaves local services unchanged.
func ValidatePlacement(spec *v1beta1.InferenceServiceSpec) error {
	if spec == nil || spec.Placement == nil {
		return nil
	}
	p := spec.Placement
	if !p.UsesClusterAffinity() {
		return fmt.Errorf("spec.placement.policy must be ClusterAffinity; %s", placementPolicyRemedy)
	}
	split := p.Mode == v1beta1.PlacementModeSplit || p.Mode == v1beta1.PlacementModeSplitByCapacity
	if !split && p.Mode != v1beta1.PlacementModeSingle && p.Mode != v1beta1.PlacementModeAll {
		return fmt.Errorf("spec.placement.mode must explicitly select Single, All, Split, or SplitByCapacity")
	}
	if _, err := affinity.Compile(p.ClusterAffinity, p.Mode == v1beta1.PlacementModeSplit); err != nil {
		return fmt.Errorf("spec.placement.%w", err)
	}
	if p.ReplacementTimeout != nil && (p.Mode != v1beta1.PlacementModeSingle || p.ReplacementTimeout.Duration <= 0) {
		return fmt.Errorf("spec.placement.replacementTimeout must be positive and is permitted only for Single")
	}
	if err := validateSplit(p, split); err != nil {
		return err
	}
	return validateComponentWeights(spec)
}

// validateSplit checks the fleet floor and per-cluster ceiling, which only the
// two Split modes accept.
func validateSplit(p *v1beta1.PlacementSpec, split bool) error {
	if p.Split == nil {
		return nil
	}
	if !split {
		return fmt.Errorf("spec.placement.split is permitted only for Split and SplitByCapacity")
	}
	s := p.Split
	if s.Replicas != nil && *s.Replicas < 1 {
		return fmt.Errorf("spec.placement.split.replicas must be positive when specified")
	}
	if s.MaxReplicasPerCluster < 0 {
		return fmt.Errorf("spec.placement.split.maxReplicasPerCluster must be nonnegative")
	}
	return nil
}

// validateComponentWeights requires every component a term weighs to be
// declared, and a positive router.minReplicas when a router weight divides
// the routers across clusters.
func validateComponentWeights(spec *v1beta1.InferenceServiceSpec) error {
	terms := spec.Placement.ClusterAffinity
	for i, term := range terms {
		if term.Weights == nil {
			continue
		}
		for _, weight := range []struct {
			component string
			set       bool
			declared  bool
		}{
			{component: "engine", set: term.Weights.Engine != nil, declared: spec.Engine != nil},
			{component: "decoder", set: term.Weights.Decoder != nil, declared: spec.Decoder != nil},
			{component: "router", set: term.Weights.Router != nil, declared: spec.Router != nil},
		} {
			if weight.set && !weight.declared {
				return fmt.Errorf("spec.placement.clusterAffinity[%d].weights.%s names a component the service does not declare", i, weight.component)
			}
		}
	}
	if affinity.DividesRouters(terms) && (spec.Router.MinReplicas == nil || *spec.Router.MinReplicas <= 0) {
		return fmt.Errorf("spec.placement.clusterAffinity weights.router divides router.minReplicas across clusters; set a positive router.minReplicas")
	}
	return nil
}

// ValidatePlacementIntent rejects obsolete selector annotations and requires
// an accepted allocation to retain its explicit placement policy.
func ValidatePlacementIntent(isvc *v1beta1.InferenceService) error {
	if isvc == nil {
		return nil
	}
	if !isvc.Spec.Placement.UsesClusterAffinity() {
		if isvc.Status.Placement != nil && isvc.Status.Placement.Plan != nil {
			return ErrAcceptedPlanRequiresClusterAffinity
		}
	}
	for _, key := range []string{constants.AcceleratorRequirements, constants.ClusterSelector} {
		if _, exists := isvc.Annotations[key]; exists {
			return fmt.Errorf("annotation %s is unsupported; declare spec.placement.mode and migrate restrictions to clusterAffinity", key)
		}
	}
	return ValidatePlacement(&isvc.Spec)
}

// ValidatePlacementPolicyUpdate prevents removing source ownership while
// member workloads can still hold its allocation authority.
func ValidatePlacementPolicyUpdate(oldSpec, newSpec *v1beta1.InferenceServiceSpec) error {
	if oldSpec.Placement.UsesClusterAffinity() && !newSpec.Placement.UsesClusterAffinity() {
		return fmt.Errorf("spec.placement.policy cannot be removed; drain and recreate the service to change placement ownership")
	}
	return nil
}

// placementModeOf is the mode a spec runs under: its block's mode, or none
// for a service that lives on this cluster alone.
func placementModeOf(spec *v1beta1.InferenceServiceSpec) v1beta1.PlacementMode {
	if spec == nil || spec.Placement == nil {
		return ""
	}
	return spec.Placement.Mode
}

// placementModeWord names a spec's placement mode in a refusal: the mode, or
// the absence of the block.
func placementModeWord(spec *v1beta1.InferenceServiceSpec) string {
	switch {
	case spec == nil || spec.Placement == nil:
		return "unset (no spec.placement block)"
	case spec.Placement.Mode == "":
		return "unset"
	}
	return string(spec.Placement.Mode)
}

// ValidatePlacementModeUpdate keeps spec.placement.mode as it was at create.
// The engine cannot carry a running service from one placement mode to
// another, so an update that names another mode, or adds or removes the
// whole block, is refused with both values named; a service is placed
// another way by deleting and recreating it.
func ValidatePlacementModeUpdate(oldSpec, newSpec *v1beta1.InferenceServiceSpec) error {
	if placementModeOf(oldSpec) == placementModeOf(newSpec) {
		return nil
	}
	return fmt.Errorf("spec.placement.mode is fixed for the life of the service and cannot change from %s to %s; delete and recreate the service to place it another way",
		placementModeWord(oldSpec), placementModeWord(newSpec))
}
