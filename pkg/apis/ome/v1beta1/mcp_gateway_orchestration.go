package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// OrchestrationConfig defines tool selection strategies and workflow orchestration support.
type OrchestrationConfig struct {
	// Enabled controls whether orchestration is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ToolSelection defines tool selection strategies.
	// +optional
	ToolSelection *ToolSelectionConfig `json:"toolSelection,omitempty"`

	// MaxSteps defines the maximum number of steps in a workflow.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxSteps *int32 `json:"maxSteps,omitempty"`

	// StepTimeout defines the timeout for individual workflow steps.
	// +kubebuilder:default="5m"
	// +optional
	StepTimeout *metav1.Duration `json:"stepTimeout,omitempty"`

	// WorkflowTimeout defines the overall workflow timeout.
	// +kubebuilder:default="30m"
	// +optional
	WorkflowTimeout *metav1.Duration `json:"workflowTimeout,omitempty"`

	// Storage defines workflow state storage configuration.
	// +optional
	Storage *WorkflowStorageConfig `json:"storage,omitempty"`

	// Engine defines the workflow engine to use.
	// +kubebuilder:validation:Enum=Simple;Temporal;Argo
	// +kubebuilder:default=Simple
	// +optional
	Engine WorkflowEngine `json:"engine,omitempty"`
}

// ToolSelectionConfig defines tool selection strategies.
type ToolSelectionConfig struct {
	// Strategy defines the tool selection strategy.
	// +kubebuilder:validation:Enum=FirstMatch;BestMatch;Consensus;AIGuided
	// +kubebuilder:default=BestMatch
	// +optional
	Strategy ToolSelectionStrategy `json:"strategy,omitempty"`

	// CapabilityMatching defines how to match tool capabilities.
	// +optional
	CapabilityMatching *CapabilityMatchingConfig `json:"capabilityMatching,omitempty"`

	// Consensus defines consensus-based tool selection.
	// +optional
	Consensus *ConsensusConfig `json:"consensus,omitempty"`
}

// ToolSelectionStrategy defines tool selection strategies.
type ToolSelectionStrategy string

const (
	ToolSelectionStrategyFirstMatch ToolSelectionStrategy = "FirstMatch"
	ToolSelectionStrategyBestMatch  ToolSelectionStrategy = "BestMatch"
	ToolSelectionStrategyConsensus  ToolSelectionStrategy = "Consensus"
	ToolSelectionStrategyAIGuided   ToolSelectionStrategy = "AIGuided"
)

// CapabilityMatchingConfig defines capability matching configuration.
type CapabilityMatchingConfig struct {
	// WeightByRelevance controls whether to weight matches by relevance.
	// +kubebuilder:default=true
	// +optional
	WeightByRelevance *bool `json:"weightByRelevance,omitempty"`

	// RequireExactMatch controls whether to require exact capability matches.
	// +kubebuilder:default=false
	// +optional
	RequireExactMatch *bool `json:"requireExactMatch,omitempty"`

	// SimilarityThreshold defines the minimum similarity threshold for matches.
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.7
	// +optional
	SimilarityThreshold *float64 `json:"similarityThreshold,omitempty"`
}

// ConsensusConfig defines consensus-based tool selection.
type ConsensusConfig struct {
	// MinServers defines the minimum number of servers for consensus.
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:default=3
	// +optional
	MinServers *int32 `json:"minServers,omitempty"`

	// Timeout defines the consensus timeout.
	// +kubebuilder:default="30s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// AgreementThreshold defines the required agreement percentage.
	// +kubebuilder:validation:Minimum=0.5
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.67
	// +optional
	AgreementThreshold *float64 `json:"agreementThreshold,omitempty"`
}

// WorkflowEngine defines supported workflow engines.
type WorkflowEngine string

const (
	WorkflowEngineSimple   WorkflowEngine = "Simple"
	WorkflowEngineTemporal WorkflowEngine = "Temporal"
	WorkflowEngineArgo     WorkflowEngine = "Argo"
)

// DecisionStrategy defines decision making strategies.
type DecisionStrategy string

const (
	DecisionStrategyGreedyBest     DecisionStrategy = "GreedyBest"
	DecisionStrategyExploreExploit DecisionStrategy = "ExploreExploit"
	DecisionStrategyContextAware   DecisionStrategy = "ContextAware"
)
