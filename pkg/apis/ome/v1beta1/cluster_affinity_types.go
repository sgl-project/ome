package v1beta1

import corev1 "k8s.io/api/core/v1"

// Cluster affinity bounds are API limits on selector expression size.
const (
	MaxClusterAffinityTerms        = 64
	MaxClusterSelectorRequirements = 64
	MaxClusterSelectorValues       = 64
)

// ClusterAffinityTerm selects WorkloadClusters by ANDing its requirements.
// Multiple terms form a union; weights assign replica shares to each match.
// +kubebuilder:validation:XValidation:rule="(has(self.matchExpressions) && size(self.matchExpressions) > 0) || (has(self.matchFields) && size(self.matchFields) > 0)",message="a cluster affinity term must contain a requirement"
// +kubebuilder:validation:XValidation:rule="!has(self.weight) || !has(self.weights) || (!has(self.weights.default) && !has(self.weights.engine))",message="weight means weights.default; it may sit beside weights.decoder and weights.router, but not beside weights.default or weights.engine"
type ClusterAffinityTerm struct {
	// Deprecated: Weight is weights.default, and is permitted only for static
	// Split. It is the only weight a manager without weights support reads, so
	// a rollback to such a manager keeps the engine shares set here.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Weight *int32 `json:"weight,omitempty"`

	// Weights sets each component's replica weight on the clusters this term
	// matches, and is permitted only for static Split. If any term gives a
	// component a weight, the weights of matching terms add and a term that
	// gives none contributes one. Weights are ratios, so they have no upper bound.
	// +optional
	Weights *ClusterAffinityWeights `json:"weights,omitempty"`

	// MatchExpressions selects the WorkloadCluster's actual labels.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	MatchExpressions []ClusterSelectorRequirement `json:"matchExpressions,omitempty"`

	// MatchFields selects the real metadata.name using In or NotIn. Multiple
	// names are allowed; a label named metadata.name does not affect this match.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	MatchFields []ClusterSelectorRequirement `json:"matchFields,omitempty"`
}

// ClusterSelectorRequirement tests one label or supported object field.
type ClusterSelectorRequirement struct {
	// Key is a label key, or metadata.name for a field requirement.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
	// Operator follows Kubernetes node-selector semantics for labels.
	// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist;Gt;Lt
	Operator corev1.NodeSelectorOperator `json:"operator"`
	// Values is nonempty for In/NotIn, empty for Exists/DoesNotExist, and a
	// single integer for Gt/Lt.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	Values []string `json:"values,omitempty"`
}

// ClusterAffinityWeights holds one term's weight per component. The engine
// takes engine, then default. The decoder takes decoder, then default, then
// the engine's weight. Routers are divided across clusters only when a term
// sets router; a term without it then contributes the engine's weight.
// +kubebuilder:validation:MinProperties=1
type ClusterAffinityWeights struct {
	// Default is the weight of every component this term does not name.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Default *int32 `json:"default,omitempty"`

	// Engine is the engine's weight.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Engine *int32 `json:"engine,omitempty"`

	// Decoder is the decoder's weight.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Decoder *int32 `json:"decoder,omitempty"`

	// Router is the router's weight. Setting it on any term divides
	// router.minReplicas across the clusters; otherwise every cluster runs the
	// full router policy.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Router *int32 `json:"router,omitempty"`
}
