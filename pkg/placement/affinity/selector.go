// Package affinity compiles cluster selection independently of member health.
package affinity

import (
	"cmp"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Match records why a cluster matched and its effective replica weight.
type Match struct {
	TermIndexes []int32
	Weight      int64
}

// Selector is an immutable compiled affinity. Its zero value matches nothing.
type Selector struct {
	unconstrained bool
	weighted      bool
	terms         []term
}

type term struct {
	labels labels.Selector
	fields []v1beta1.ClusterSelectorRequirement
	weight int64
}

// Compile validates and snapshots an affinity whose matches carry the engine's
// weight. Nil means no restriction; an explicitly empty list is invalid.
// allowWeights is true only for static Split.
func Compile(terms []v1beta1.ClusterAffinityTerm, allowWeights bool) (*Selector, error) {
	return CompileFor(terms, allowWeights, v1beta1.EngineComponent)
}

// CompileFor is Compile with matches that carry the named component's weight.
// Every component matches the same clusters.
func CompileFor(terms []v1beta1.ClusterAffinityTerm, allowWeights bool, component v1beta1.ComponentType) (*Selector, error) {
	if terms == nil {
		return &Selector{unconstrained: true}, nil
	}
	if len(terms) == 0 || len(terms) > v1beta1.MaxClusterAffinityTerms {
		return nil, fmt.Errorf("clusterAffinity must contain between 1 and %d terms", v1beta1.MaxClusterAffinityTerms)
	}
	selector := &Selector{}
	for i, input := range terms {
		compiled, err := compileTerm(input, allowWeights)
		if err != nil {
			return nil, fmt.Errorf("clusterAffinity[%d]: %w", i, err)
		}
		if weight := termWeight(input, component); weight != nil {
			selector.weighted = true
			compiled.weight = int64(*weight)
		}
		selector.terms = append(selector.terms, compiled)
	}
	return selector, nil
}

// termWeight returns the weight a term gives a component, or nil when it gives
// none. The deprecated weight reads as weights.default. The engine takes
// engine, then default; the decoder takes decoder, then default, then the
// engine's; the router takes router, then the engine's.
func termWeight(input v1beta1.ClusterAffinityTerm, component v1beta1.ComponentType) *int32 {
	weights := input.Weights
	if weights == nil {
		weights = &v1beta1.ClusterAffinityWeights{}
	}
	fallback := cmp.Or(weights.Default, scalarWeight(input))
	engine := cmp.Or(weights.Engine, fallback)
	switch component {
	case v1beta1.EngineComponent:
		return engine
	case v1beta1.DecoderComponent:
		return cmp.Or(weights.Decoder, fallback, engine)
	case v1beta1.RouterComponent:
		return cmp.Or(weights.Router, engine)
	}
	return nil
}

// scalarWeight reads the deprecated weight, which means weights.default. It is
// the only reader, so every other caller goes through the weights rules here.
func scalarWeight(input v1beta1.ClusterAffinityTerm) *int32 {
	return input.Weight //nolint:staticcheck // The deprecated weight still means weights.default.
}

// SetsWeights reports whether any term holds a weights block, which can give
// the decoder and router weights other than the engine's.
func SetsWeights(terms []v1beta1.ClusterAffinityTerm) bool {
	return slices.ContainsFunc(terms, func(t v1beta1.ClusterAffinityTerm) bool { return t.Weights != nil })
}

// DividesRouters reports whether any term sets a router weight. Routers are
// then divided across the matched clusters; otherwise each runs the full
// router policy.
func DividesRouters(terms []v1beta1.ClusterAffinityTerm) bool {
	return slices.ContainsFunc(terms, func(t v1beta1.ClusterAffinityTerm) bool { return t.Weights != nil && t.Weights.Router != nil })
}

func compileTerm(input v1beta1.ClusterAffinityTerm, allowWeights bool) (term, error) {
	if len(input.MatchExpressions)+len(input.MatchFields) == 0 {
		return term{}, fmt.Errorf("term must contain a label or field requirement")
	}
	if len(input.MatchExpressions) > v1beta1.MaxClusterSelectorRequirements || len(input.MatchFields) > v1beta1.MaxClusterSelectorRequirements {
		return term{}, fmt.Errorf("matchExpressions and matchFields each allow at most %d requirements", v1beta1.MaxClusterSelectorRequirements)
	}
	out := term{labels: labels.Everything(), weight: 1}
	if weight := scalarWeight(input); weight != nil {
		if !allowWeights {
			return term{}, fmt.Errorf("weight is permitted only in Split mode")
		}
		if *weight < 1 {
			return term{}, fmt.Errorf("weight must be positive")
		}
	}
	if err := validateWeights(input, allowWeights); err != nil {
		return term{}, err
	}
	for i, req := range input.MatchExpressions {
		if len(req.Values) > v1beta1.MaxClusterSelectorValues {
			return term{}, fmt.Errorf("matchExpressions[%d]: at most %d values are allowed", i, v1beta1.MaxClusterSelectorValues)
		}
		op, ok := labelOperators[req.Operator]
		if !ok {
			return term{}, fmt.Errorf("matchExpressions[%d]: unsupported operator %q", i, req.Operator)
		}
		parsed, err := labels.NewRequirement(req.Key, op, slices.Clone(req.Values))
		if err != nil {
			return term{}, fmt.Errorf("matchExpressions[%d]: %w", i, err)
		}
		out.labels = out.labels.Add(*parsed)
	}
	for i, req := range input.MatchFields {
		if req.Key != metav1.ObjectNameField || (req.Operator != corev1.NodeSelectorOpIn && req.Operator != corev1.NodeSelectorOpNotIn) {
			return term{}, fmt.Errorf("matchFields[%d]: only metadata.name with In or NotIn is supported", i)
		}
		if len(req.Values) == 0 || len(req.Values) > v1beta1.MaxClusterSelectorValues {
			return term{}, fmt.Errorf("matchFields[%d]: between 1 and %d names are required", i, v1beta1.MaxClusterSelectorValues)
		}
		for _, name := range req.Values {
			if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
				return term{}, fmt.Errorf("matchFields[%d]: invalid WorkloadCluster name %q: %v", i, name, problems)
			}
		}
		req.Values = slices.Clone(req.Values)
		out.fields = append(out.fields, req)
	}
	return out, nil
}

// validateWeights checks a term's weights block against the same rules the
// schema enforces, for objects admitted while a rule was absent.
func validateWeights(input v1beta1.ClusterAffinityTerm, allowWeights bool) error {
	w := input.Weights
	if w == nil {
		return nil
	}
	if !allowWeights {
		return fmt.Errorf("weights is permitted only in Split mode")
	}
	set := 0
	for _, field := range []struct {
		name  string
		value *int32
	}{{"default", w.Default}, {"engine", w.Engine}, {"decoder", w.Decoder}, {"router", w.Router}} {
		if field.value == nil {
			continue
		}
		set++
		if *field.value < 1 {
			return fmt.Errorf("weights.%s must be positive", field.name)
		}
	}
	if set == 0 {
		return fmt.Errorf("weights must set at least one component")
	}
	if scalarWeight(input) != nil && (w.Default != nil || w.Engine != nil) {
		return fmt.Errorf("weight means weights.default; it may sit beside weights.decoder and weights.router, but not beside weights.default or weights.engine")
	}
	return nil
}

var labelOperators = map[corev1.NodeSelectorOperator]selection.Operator{
	corev1.NodeSelectorOpIn:           selection.In,
	corev1.NodeSelectorOpNotIn:        selection.NotIn,
	corev1.NodeSelectorOpExists:       selection.Exists,
	corev1.NodeSelectorOpDoesNotExist: selection.DoesNotExist,
	corev1.NodeSelectorOpGt:           selection.GreaterThan,
	corev1.NodeSelectorOpLt:           selection.LessThan,
}

// Match considers labels and object identity, including disconnected clusters.
// Member preflight decides whether a matched cluster is currently eligible.
func (s *Selector) Match(cluster *v1beta1.WorkloadCluster) (Match, bool) {
	if s == nil || cluster == nil {
		return Match{}, false
	}
	if s.unconstrained {
		return Match{Weight: 1}, true
	}
	var matched Match
	for i, term := range s.terms {
		if !term.matches(cluster) {
			continue
		}
		matched.TermIndexes = append(matched.TermIndexes, int32(i))
		if s.weighted {
			matched.Weight += term.weight
		} else {
			matched.Weight = 1
		}
	}
	return matched, len(matched.TermIndexes) > 0
}

func (t term) matches(cluster *v1beta1.WorkloadCluster) bool {
	if !t.labels.Matches(labels.Set(cluster.Labels)) {
		return false
	}
	for _, field := range t.fields {
		contains := slices.Contains(field.Values, cluster.Name)
		if (field.Operator == corev1.NodeSelectorOpIn) != contains {
			return false
		}
	}
	return true
}
