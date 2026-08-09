package runtimeselector

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Selector is the main interface for runtime selection.
type Selector interface {
	// SelectRuntime returns the highest-scoring runtime that supports the
	// model, or a *NoRuntimeFoundError if no compatible runtime exists.
	SelectRuntime(ctx context.Context, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService) (*RuntimeSelection, error)

	// GetCompatibleRuntimes returns all compatible runtimes sorted by score
	// (best first). Useful for debugging and listing available options.
	GetCompatibleRuntimes(ctx context.Context, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, namespace string) ([]RuntimeMatch, error)

	// ValidateRuntime returns nil if the named runtime is compatible, or an
	// error explaining why it isn't. Used to validate user-pinned runtimes.
	ValidateRuntime(ctx context.Context, runtimeName string, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService) error

	// GetRuntime fetches a runtime by name. The bool return reports whether
	// the resolved runtime was cluster-scoped.
	GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)

	// GetSupportedModelFormat picks the best SupportedModelFormat for a
	// runtime-model pair. When userSpecifiedRuntime is true all
	// SupportedModelFormats are considered; otherwise only ones with
	// AutoSelect=true are eligible (matching SelectRuntime's behaviour).
	GetSupportedModelFormat(ctx context.Context, runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, userSpecifiedRuntime bool) *v1beta1.SupportedModelFormat
}

// RuntimeSelection represents the selected runtime with metadata.
type RuntimeSelection struct {
	Name      string
	Spec      *v1beta1.ServingRuntimeSpec
	Score     int64
	IsCluster bool
}

// RuntimeMatch is a RuntimeSelection enriched with the per-dimension match
// details that drove its score, useful for debugging selection decisions.
type RuntimeMatch struct {
	RuntimeSelection
	MatchDetails MatchDetails
}

// MatchDetails records which dimensions of runtime-model compatibility
// contributed to (or excluded) a match. Fields default to true for
// dimensions that the matcher silently accepts when neither side specifies
// them (e.g. ArchitectureMatch is true when both model and runtime omit
// ModelArchitecture).
type MatchDetails struct {
	FormatMatch             bool
	FrameworkMatch          bool
	SizeMatch               bool
	ArchitectureMatch       bool
	DiffusionPipelineMatch  bool
	QuantizationMatch       bool
	ModelCacheProviderMatch bool
	Priority                int32
	Weight                  int64
	AutoSelectEnabled       bool
	Reasons                 []string
}

// RuntimeFetcher abstracts the fetching of runtime resources.
type RuntimeFetcher interface {
	FetchRuntimes(ctx context.Context, namespace string) (*RuntimeCollection, error)
	GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)
}

// RuntimeCollection holds both namespace and cluster scoped runtimes.
type RuntimeCollection struct {
	NamespaceRuntimes []v1beta1.ServingRuntime
	ClusterRuntimes   []v1beta1.ClusterServingRuntime
}

// forEach invokes fn for every runtime in the collection, namespace-scoped
// first then cluster-scoped. isCluster reports the scope of the current item.
func (c *RuntimeCollection) forEach(fn func(name string, spec *v1beta1.ServingRuntimeSpec, isCluster bool)) {
	for i := range c.NamespaceRuntimes {
		rt := &c.NamespaceRuntimes[i]
		fn(rt.Name, &rt.Spec, false)
	}
	for i := range c.ClusterRuntimes {
		rt := &c.ClusterRuntimes[i]
		fn(rt.Name, &rt.Spec, true)
	}
}

// RuntimeMatcher handles compatibility checking between runtimes and models.
type RuntimeMatcher interface {
	IsCompatible(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (bool, error)

	// GetCompatibilityDetails returns the same compatible/incompatible verdict
	// as IsCompatible, plus per-dimension match info and the
	// human-readable reasons exposed in NoRuntimeFoundError.
	GetCompatibilityDetails(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (*CompatibilityReport, error)
}

// CompatibilityReport provides detailed compatibility analysis.
type CompatibilityReport struct {
	IsCompatible           bool
	MatchDetails           MatchDetails
	IncompatibilityReasons []string
	Warnings               []string
}

// RuntimeScorer calculates scores for runtime-model pairs.
type RuntimeScorer interface {
	// CalculateScore returns a score >= 0; higher means a better match.
	CalculateScore(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec) (int64, error)

	// CompareRuntimes returns positive if r1 is better, negative if r2 is
	// better, 0 if equal. Used as a sort comparator.
	CompareRuntimes(r1, r2 RuntimeMatch, model *v1beta1.BaseModelSpec) int

	CalculateFormatScore(model *v1beta1.BaseModelSpec, supportedFormat v1beta1.SupportedModelFormat, priority int64) int64
}

// Config holds configuration for the runtime selector.
type Config struct {
	Client client.Client

	// EnableDetailedLogging is retained for backward compatibility with
	// external callers; it is currently a no-op (verbose logs use
	// log.V() levels driven by the global log config instead).
	EnableDetailedLogging bool

	// DefaultPriority is the multiplier used when a SupportedModelFormat
	// doesn't set Priority explicitly.
	DefaultPriority int32

	// ModelFormatWeight / ModelFrameworkWeight are the fallback weights used
	// when the runtime author sets weight=0 on a supported format. Tuned so
	// format matches outweigh framework matches at equal priority.
	ModelFormatWeight    int64
	ModelFrameworkWeight int64

	// ModelCacheProvider is the cluster-configured cache provider name; only
	// runtimes whose formats list this provider are eligible for sharded
	// BaseModels (Distribution == DistributionSharded).
	ModelCacheProvider string
}

// NewConfig returns a Config with the default scoring constants:
// priority=1, format weight 10, framework weight 5.
func NewConfig(client client.Client) *Config {
	return &Config{
		Client:               client,
		DefaultPriority:      1,
		ModelFormatWeight:    10,
		ModelFrameworkWeight: 5,
	}
}
