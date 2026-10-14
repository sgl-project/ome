package v1beta1

import corev1 "k8s.io/api/core/v1"

// Cluster affinity bounds are API limits on selector expression size.
const (
	MaxClusterAffinityTerms        = 64
	MaxClusterSelectorRequirements = 64
	MaxClusterSelectorValues       = 64
)

// ClusterAffinityTerm selects WorkloadClusters by ANDing its requirements.
// Multiple terms form a union; a weight assigns replica shares to each match.
// +kubebuilder:validation:XValidation:rule="(has(self.matchExpressions) && size(self.matchExpressions) > 0) || (has(self.matchFields) && size(self.matchFields) > 0)",message="a cluster affinity term must contain a requirement"
type ClusterAffinityTerm struct {
	// Weight is permitted only for static Split. If any term has an explicit
	// weight, matching term weights add and omitted weights contribute one.
	// Omission must be preserved to distinguish unweighted overlapping terms.
	// Weights are replica ratios, so they have no upper bound.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Weight *int32 `json:"weight,omitempty"`

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

// ComponentClusterAffinity holds one affinity term list per component. Every
// listed component must match the same clusters. In Split, each list's weights
// set that component's shares; in Single and All the lists only select.
// +structType=atomic
// +kubebuilder:validation:MinProperties=1
type ComponentClusterAffinity struct {
	// Engine selects clusters for the engine and, in Split, weights its shares.
	// It is required when the service declares an engine.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Engine []ClusterAffinityTerm `json:"engine,omitempty"`

	// Decoder selects clusters for the decoder and, in Split, weights its shares
	// over the clusters that hold engine replicas. It is required when the
	// service declares a decoder.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Decoder []ClusterAffinityTerm `json:"decoder,omitempty"`

	// Router selects clusters for the router. In Split, it divides
	// router.minReplicas across the clusters that hold engine replicas; without
	// it, each such cluster runs the full router policy.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Router []ClusterAffinityTerm `json:"router,omitempty"`
}
