package placement

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/placement/affinity"
	"sigs.k8s.io/ome/pkg/validation"
)

// MatchReason explains why placement currently has no eligible candidate.
type MatchReason string

const (
	MatchReasonNoRequirements    MatchReason = "NoPlacementIntent"
	MatchReasonMalformedSelector MatchReason = "InvalidPlacementIntent"
	MatchReasonNoReadyClusters   MatchReason = "NoReadyClusters"
	MatchReasonNoMatch           MatchReason = "NoMatch"
)

// placementSelector is shared by actuation, standing observations, and cluster
// event filtering. Health and admission capability never alter its matched set.
// Under ComponentAffinity it is the primary component's selector, and its
// weights split the primary component; every other component must match the
// same clusters (componentClustersDiffer).
func placementSelector(isvc *v1beta1.InferenceService) (*affinity.Selector, error) {
	if err := validation.ValidatePlacementIntent(isvc); err != nil {
		return nil, err
	}
	if isvc == nil || isvc.Spec.Placement == nil {
		return nil, nil
	}
	if !isvc.Spec.Placement.UsesComponentAffinity() {
		return affinity.Compile(isvc.Spec.Placement.ClusterAffinity, isvc.Spec.Placement.Mode == v1beta1.PlacementModeSplit)
	}
	selectors, err := componentSelectors(isvc)
	if err != nil {
		return nil, err
	}
	return selectors[placedComponents(isvc)[0]], nil
}

// componentSelectors compiles the terms of each placed component. Under
// ClusterAffinity every component shares the one list. A nil map means the
// source declares no placement.
func componentSelectors(isvc *v1beta1.InferenceService) (map[v1beta1.ComponentType]*affinity.Selector, error) {
	if err := validation.ValidatePlacementIntent(isvc); err != nil {
		return nil, err
	}
	if isvc == nil || isvc.Spec.Placement == nil {
		return nil, nil
	}
	p := isvc.Spec.Placement
	weighted := p.Mode == v1beta1.PlacementModeSplit
	components := placedComponents(isvc)
	out := make(map[v1beta1.ComponentType]*affinity.Selector, len(components))
	if !p.UsesComponentAffinity() {
		shared, err := affinity.Compile(p.ClusterAffinity, weighted)
		if err != nil {
			return nil, err
		}
		for _, component := range components {
			out[component] = shared
		}
		return out, nil
	}
	if len(components) == 0 {
		return nil, fmt.Errorf("spec.placement.componentAffinity requires an engine or decoder component")
	}
	for _, component := range components {
		terms := componentTerms(p.ComponentAffinity, component)
		if terms == nil {
			// Only the router may omit its terms; it then runs on every home. A
			// model component never reads as unconstrained.
			if component != v1beta1.RouterComponent {
				return nil, fmt.Errorf("spec.placement.componentAffinity.%s is required", component)
			}
			continue
		}
		selector, err := affinity.Compile(terms, weighted)
		if err != nil {
			return nil, fmt.Errorf("spec.placement.componentAffinity.%s: %w", component, err)
		}
		out[component] = selector
	}
	return out, nil
}

// placedComponents lists the declared components in placement order: the
// primary component (Engine, or Decoder without an Engine) first, then the
// Decoder and the Router.
func placedComponents(isvc *v1beta1.InferenceService) []v1beta1.ComponentType {
	var out []v1beta1.ComponentType
	if isvc.Spec.Engine != nil {
		out = append(out, v1beta1.EngineComponent)
	}
	if isvc.Spec.Decoder != nil {
		out = append(out, v1beta1.DecoderComponent)
	}
	if len(out) > 0 && isvc.Spec.Router != nil {
		out = append(out, v1beta1.RouterComponent)
	}
	return out
}

func componentTerms(terms *v1beta1.ComponentClusterAffinity, component v1beta1.ComponentType) []v1beta1.ClusterAffinityTerm {
	if terms == nil {
		return nil
	}
	switch component {
	case v1beta1.EngineComponent:
		return terms.Engine
	case v1beta1.DecoderComponent:
		return terms.Decoder
	case v1beta1.RouterComponent:
		return terms.Router
	}
	return nil
}

// componentClustersDiffer reports every registered cluster that some of the
// source's components match and others do not. A cluster holding prefill
// without decode, or model servers without their router, cannot serve, so
// placement holds on any difference instead of using the overlap.
func componentClustersDiffer(isvc *v1beta1.InferenceService, clusters []v1beta1.WorkloadCluster) error {
	if isvc == nil || !isvc.Spec.Placement.UsesComponentAffinity() {
		return nil
	}
	selectors, err := componentSelectors(isvc)
	if err != nil {
		return err
	}
	var differences []string
	sorted := slices.Clone(clusters)
	slices.SortFunc(sorted, func(a, b v1beta1.WorkloadCluster) int { return strings.Compare(a.Name, b.Name) })
	for i := range sorted {
		cluster := &sorted[i]
		if !cluster.DeletionTimestamp.IsZero() {
			continue
		}
		var matched, unmatched []string
		for _, component := range placedComponents(isvc) {
			selector, exists := selectors[component]
			if !exists {
				continue
			}
			if _, matches := selector.Match(cluster); matches {
				matched = append(matched, string(component))
			} else {
				unmatched = append(unmatched, string(component))
			}
		}
		if len(matched) > 0 && len(unmatched) > 0 {
			differences = append(differences, fmt.Sprintf("%s matches %s but not %s", cluster.Name, strings.Join(matched, " and "), strings.Join(unmatched, " and ")))
		}
	}
	if len(differences) > 0 {
		return fmt.Errorf("every component's componentAffinity must match the same clusters: %s", strings.Join(differences, "; "))
	}
	return nil
}

// MatchCandidates returns sorted Ready members of the matched set. Replica
// apportionment uses all matching registrations, independently of this gate.
func MatchCandidates(isvc *v1beta1.InferenceService, clusters []v1beta1.WorkloadCluster) ([]string, MatchReason, error) {
	selector, err := placementSelector(isvc)
	if err != nil {
		return nil, MatchReasonMalformedSelector, err
	}
	if selector == nil {
		return nil, MatchReasonNoRequirements, nil
	}
	matched := 0
	var out []string
	for i := range clusters {
		cluster := &clusters[i]
		if !cluster.DeletionTimestamp.IsZero() {
			continue
		}
		if _, matches := selector.Match(cluster); !matches {
			continue
		}
		matched++
		if clusterReady(cluster) {
			out = append(out, cluster.Name)
		}
	}
	sort.Strings(out)
	switch {
	case len(out) > 0:
		return out, "", nil
	case matched == 0:
		return nil, MatchReasonNoMatch, nil
	default:
		return nil, MatchReasonNoReadyClusters, nil
	}
}
