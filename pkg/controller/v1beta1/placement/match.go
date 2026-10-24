package placement

import (
	"sort"

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
// Its matches carry the primary component's weight.
func placementSelector(isvc *v1beta1.InferenceService) (*affinity.Selector, error) {
	selectors, err := componentSelectors(isvc)
	if err != nil || selectors == nil {
		return nil, err
	}
	components := placedComponents(isvc)
	if len(components) == 0 {
		return affinity.Compile(isvc.Spec.Placement.ClusterAffinity, isvc.Spec.Placement.Mode == v1beta1.PlacementModeSplit)
	}
	return selectors[components[0]], nil
}

// componentSelectors compiles the affinity once per placed component. Every
// component matches the same clusters, so the primary component's clusters
// are every component's; each selector's matches carry that component's
// weight. A nil map means the source declares no placement.
func componentSelectors(isvc *v1beta1.InferenceService) (map[v1beta1.ComponentType]*affinity.Selector, error) {
	if err := validation.ValidatePlacementIntent(isvc); err != nil {
		return nil, err
	}
	if isvc == nil || isvc.Spec.Placement == nil {
		return nil, nil
	}
	p := isvc.Spec.Placement
	components := placedComponents(isvc)
	out := make(map[v1beta1.ComponentType]*affinity.Selector, len(components))
	for _, component := range components {
		selector, err := affinity.CompileFor(p.ClusterAffinity, p.Mode == v1beta1.PlacementModeSplit, component)
		if err != nil {
			return nil, err
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
