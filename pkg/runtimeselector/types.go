package runtimeselector

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Selector is the main interface for runtime selection.
// It provides methods to select, validate, and list compatible runtimes for models.
type Selector interface {
	// SelectRuntime finds the best runtime for a given model.
	// It returns the highest scoring runtime that supports the model.
	// If no compatible runtime is found, it returns an error.
	SelectRuntime(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) (*RuntimeSelection, error)

	// GetCompatibleRuntimes returns all compatible runtimes sorted by priority.
	// This is useful for debugging and for showing available options.
	GetCompatibleRuntimes(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) ([]RuntimeMatch, error)

	// ValidateRuntime checks if a specific runtime supports a model.
	// It returns nil if the runtime is compatible, or an error explaining why it's not.
	ValidateRuntime(ctx context.Context, runtimeName string, model *v1beta1.BaseModelSpec, namespace string) error

	// GetRuntime fetches a specific runtime by name.
	// Returns the runtime spec and whether it's cluster-scoped.
	GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)
}

// RuntimeSelection represents the selected runtime with metadata.
type RuntimeSelection struct {
	// Name is the name of the selected runtime
	Name string

	// Spec is the runtime specification
	Spec *v1beta1.ServingRuntimeSpec

	// Score is the calculated compatibility score
	Score int64

	// IsCluster indicates if this is a ClusterServingRuntime (true) or namespace-scoped ServingRuntime (false)
	IsCluster bool
}

// RuntimeMatch represents a runtime that matches the model with detailed scoring information.
type RuntimeMatch struct {
	// Embedded RuntimeSelection provides basic runtime info
	RuntimeSelection

	// MatchDetails provides detailed information about why this runtime matches
	MatchDetails MatchDetails
}

// MatchDetails contains detailed information about runtime-model compatibility.
type MatchDetails struct {
	// FormatMatch indicates if the model format is compatible
	FormatMatch bool

	// FrameworkMatch indicates if the model framework is compatible
	FrameworkMatch bool

	// SizeMatch indicates if the model size is within the runtime's supported range
	SizeMatch bool

	// ArchitectureMatch indicates if the model architecture is compatible
	ArchitectureMatch bool

	// QuantizationMatch indicates if the model quantization is compatible
	QuantizationMatch bool

	// Priority is the runtime's priority for this model format
	Priority int32

	// Weight is the total weight used in scoring
	Weight int64

	// AutoSelectEnabled indicates if this runtime can be auto-selected
	AutoSelectEnabled bool

	// Reasons contains human-readable reasons for match/mismatch
	Reasons []string
}

// RuntimeFetcher abstracts the fetching of runtime resources.
type RuntimeFetcher interface {
	// FetchRuntimes returns both namespace and cluster scoped runtimes.
	// The implementation should use cached client for efficiency.
	FetchRuntimes(ctx context.Context, namespace string) (*RuntimeCollection, error)

	// GetRuntime fetches a specific runtime by name.
	// It first checks namespace-scoped runtimes, then cluster-scoped ones.
	GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)
}

// RuntimeCollection holds both namespace and cluster scoped runtimes.
type RuntimeCollection struct {
	// NamespaceRuntimes contains namespace-scoped ServingRuntimes
	NamespaceRuntimes []v1beta1.ServingRuntime

	// ClusterRuntimes contains cluster-scoped ClusterServingRuntimes
	ClusterRuntimes []v1beta1.ClusterServingRuntime
}

// RuntimeMatcher handles compatibility checking between runtimes and models.
type RuntimeMatcher interface {
	// IsCompatible checks if a runtime can serve a model.
	// Returns true if compatible, false otherwise.
	IsCompatible(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) (bool, error)

	// GetCompatibilityDetails returns detailed compatibility information.
	// This includes specific reasons for compatibility or incompatibility.
	GetCompatibilityDetails(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) (*CompatibilityReport, error)
}

// CompatibilityReport provides detailed compatibility analysis.
type CompatibilityReport struct {
	// IsCompatible indicates overall compatibility
	IsCompatible bool

	// MatchDetails provides detailed matching information
	MatchDetails MatchDetails

	// IncompatibilityReasons lists specific reasons why the runtime is incompatible
	IncompatibilityReasons []string

	// Warnings lists non-critical compatibility concerns
	Warnings []string
}

// RuntimeScorer calculates scores for runtime-model pairs.
type RuntimeScorer interface {
	// CalculateScore returns a score for how well a runtime matches a model.
	// Higher scores indicate better matches.
	CalculateScore(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec) (int64, error)

	// CompareRuntimes compares two runtimes for a given model.
	// Returns positive if r1 is better, negative if r2 is better, 0 if equal.
	CompareRuntimes(r1, r2 RuntimeMatch, model *v1beta1.BaseModelSpec) int
}

// Config holds configuration for the runtime selector.
type Config struct {
	// Client is the Kubernetes client (uses controller-runtime cache)
	Client client.Client

	// EnableDetailedLogging enables verbose logging for debugging
	EnableDetailedLogging bool

	// DefaultPriority is used when a runtime doesn't specify priority
	DefaultPriority int32

	// ModelFormatWeight is the default weight for model format matching
	ModelFormatWeight int64

	// ModelFrameworkWeight is the default weight for model framework matching
	ModelFrameworkWeight int64
}

// NewConfig creates a new Config with default values.
func NewConfig(client client.Client) *Config {
	return &Config{
		Client:                client,
		EnableDetailedLogging: false,
		DefaultPriority:       1,
		ModelFormatWeight:     10,
		ModelFrameworkWeight:  5,
	}
}
