package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/placement/affinity"
)

// ValidatePlacement checks explicit placement syntax independently of member
// health or runtime resolution. An absent block leaves local services unchanged.
func ValidatePlacement(spec *v1beta1.InferenceServiceSpec) error {
	if spec == nil || spec.Placement == nil {
		return nil
	}
	p := spec.Placement
	if !p.UsesAffinityPolicy() {
		return fmt.Errorf("spec.placement.policy must explicitly select ClusterAffinity or ComponentAffinity")
	}
	split := p.Mode == v1beta1.PlacementModeSplit || p.Mode == v1beta1.PlacementModeSplitByCapacity
	if !split && p.Mode != v1beta1.PlacementModeSingle && p.Mode != v1beta1.PlacementModeAll {
		return fmt.Errorf("spec.placement.mode must explicitly select Single, All, Split, or SplitByCapacity")
	}
	if p.UsesComponentAffinity() {
		if err := validateComponentAffinity(spec); err != nil {
			return err
		}
	} else {
		if p.ComponentAffinity != nil {
			return fmt.Errorf("spec.placement.componentAffinity requires the ComponentAffinity policy")
		}
		if _, err := affinity.Compile(p.ClusterAffinity, p.Mode == v1beta1.PlacementModeSplit); err != nil {
			return fmt.Errorf("spec.placement.%w", err)
		}
	}
	if p.ReplacementTimeout != nil && (p.Mode != v1beta1.PlacementModeSingle || p.ReplacementTimeout.Duration <= 0) {
		return fmt.Errorf("spec.placement.replacementTimeout must be positive and is permitted only for Single")
	}
	if err := validateSplit(p, split); err != nil {
		return err
	}
	if p.UsesComponentAffinity() && split && p.ComponentAffinity.Router != nil && spec.Router != nil && (spec.Router.MinReplicas == nil || *spec.Router.MinReplicas <= 0) {
		return fmt.Errorf("spec.placement.componentAffinity.router divides router.minReplicas across homes in Split modes; set a positive router.minReplicas")
	}
	return nil
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

// validateComponentAffinity checks per-component terms. The ComponentAffinity
// policy reads them instead of clusterAffinity; every declared engine and
// decoder needs its own list; a list may name only a declared component; and
// weights are accepted only in Split. An explicit null decodes to an empty
// struct, which names no component and is therefore invalid.
func validateComponentAffinity(spec *v1beta1.InferenceServiceSpec) error {
	p := spec.Placement
	if p.ClusterAffinity != nil {
		return fmt.Errorf("spec.placement.clusterAffinity is not read by the ComponentAffinity policy; use componentAffinity")
	}
	c := p.ComponentAffinity
	if c == nil {
		return fmt.Errorf("spec.placement.componentAffinity is required by the ComponentAffinity policy")
	}
	lists := []struct {
		name     string
		terms    []v1beta1.ClusterAffinityTerm
		declared bool
		required bool
	}{
		{name: "engine", terms: c.Engine, declared: spec.Engine != nil, required: true},
		{name: "decoder", terms: c.Decoder, declared: spec.Decoder != nil, required: true},
		{name: "router", terms: c.Router, declared: spec.Router != nil},
	}
	if c.Engine == nil && c.Decoder == nil && c.Router == nil {
		return fmt.Errorf("spec.placement.componentAffinity must name at least one component")
	}
	for _, list := range lists {
		if list.terms == nil {
			if list.declared && list.required {
				return fmt.Errorf("spec.placement.componentAffinity.%s is required because the service declares that component", list.name)
			}
			continue
		}
		if !list.declared {
			return fmt.Errorf("spec.placement.componentAffinity.%s names a component the service does not declare", list.name)
		}
		if _, err := affinity.Compile(list.terms, p.Mode == v1beta1.PlacementModeSplit); err != nil {
			return fmt.Errorf("spec.placement.componentAffinity.%s: %w", list.name, err)
		}
	}
	return nil
}

// ValidatePlacementIntent rejects obsolete selector annotations and requires
// an accepted allocation to retain its explicit placement policy.
func ValidatePlacementIntent(isvc *v1beta1.InferenceService) error {
	if isvc == nil {
		return nil
	}
	if !isvc.Spec.Placement.UsesAffinityPolicy() {
		if isvc.Status.Placement != nil && isvc.Status.Placement.Plan != nil {
			return fmt.Errorf("an accepted placement plan requires spec.placement.policy ClusterAffinity or ComponentAffinity")
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
	if oldSpec.Placement.UsesAffinityPolicy() && !newSpec.Placement.UsesAffinityPolicy() {
		return fmt.Errorf("spec.placement.policy cannot be removed; drain and recreate the service to change placement ownership")
	}
	return nil
}
