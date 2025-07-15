package runtimeselector

import (
	"context"
	"fmt"
	"sort"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// defaultSelector is the default implementation of the Selector interface.
type defaultSelector struct {
	config  *Config
	fetcher RuntimeFetcher
	matcher RuntimeMatcher
	scorer  RuntimeScorer
}

// New creates a new Selector with default implementations.
func New(client client.Client) Selector {
	config := NewConfig(client)
	return NewWithConfig(config)
}

// NewWithConfig creates a new Selector with the provided configuration.
func NewWithConfig(config *Config) Selector {
	return &defaultSelector{
		config:  config,
		fetcher: NewDefaultRuntimeFetcher(config.Client),
		matcher: NewDefaultRuntimeMatcher(config),
		scorer:  NewDefaultRuntimeScorer(config),
	}
}

// SelectRuntime finds the best runtime for a given model.
func (s *defaultSelector) SelectRuntime(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) (*RuntimeSelection, error) {
	logger := log.FromContext(ctx)
	logger.Info("Selecting runtime for model",
		"model", model.ModelFormat.Name,
		"namespace", namespace)

	// Validate model
	if err := s.validateModel(model); err != nil {
		return nil, err
	}

	// Get all compatible runtimes
	matches, err := s.GetCompatibleRuntimes(ctx, model, namespace)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		// Build detailed error with exclusion reasons
		collection, _ := s.fetcher.FetchRuntimes(ctx, namespace)
		excludedRuntimes := make(map[string]error)

		// Check namespace runtimes
		for _, rt := range collection.NamespaceRuntimes {
			if compatible, _ := s.matcher.IsCompatible(&rt.Spec, model, rt.Name); !compatible {
				report, _ := s.matcher.GetCompatibilityDetails(&rt.Spec, model, rt.Name)
				if report != nil && len(report.IncompatibilityReasons) > 0 {
					excludedRuntimes[rt.Name] = fmt.Errorf("%s", report.IncompatibilityReasons[0])
				}
			}
		}

		// Check cluster runtimes
		for _, rt := range collection.ClusterRuntimes {
			if compatible, _ := s.matcher.IsCompatible(&rt.Spec, model, rt.Name); !compatible {
				report, _ := s.matcher.GetCompatibilityDetails(&rt.Spec, model, rt.Name)
				if report != nil && len(report.IncompatibilityReasons) > 0 {
					excludedRuntimes[rt.Name] = fmt.Errorf("%s", report.IncompatibilityReasons[0])
				}
			}
		}

		return nil, &NoRuntimeFoundError{
			ModelName:          getModelName(model),
			ModelFormat:        model.ModelFormat.Name,
			Namespace:          namespace,
			ExcludedRuntimes:   excludedRuntimes,
			TotalRuntimes:      len(collection.NamespaceRuntimes) + len(collection.ClusterRuntimes),
			NamespacedRuntimes: len(collection.NamespaceRuntimes),
			ClusterRuntimes:    len(collection.ClusterRuntimes),
		}
	}

	// Return the best match (first one after sorting)
	best := matches[0]
	logger.Info("Selected runtime",
		"runtime", best.Name,
		"score", best.Score,
		"isCluster", best.IsCluster)

	return &best.RuntimeSelection, nil
}

// GetCompatibleRuntimes returns all compatible runtimes sorted by priority.
func (s *defaultSelector) GetCompatibleRuntimes(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) ([]RuntimeMatch, error) {
	logger := log.FromContext(ctx)

	// Validate model
	if err := s.validateModel(model); err != nil {
		return nil, err
	}

	// Fetch all runtimes
	collection, err := s.fetcher.FetchRuntimes(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch runtimes: %w", err)
	}

	var namespaceMatches []RuntimeMatch
	var clusterMatches []RuntimeMatch

	// Process namespace-scoped runtimes
	for _, runtime := range collection.NamespaceRuntimes {
		if match := s.evaluateRuntime(ctx, &runtime.Spec, model, runtime.Name, false); match != nil {
			namespaceMatches = append(namespaceMatches, *match)
		}
	}

	// Process cluster-scoped runtimes
	for _, runtime := range collection.ClusterRuntimes {
		if match := s.evaluateRuntime(ctx, &runtime.Spec, model, runtime.Name, true); match != nil {
			clusterMatches = append(clusterMatches, *match)
		}
	}

	// Sort namespace and cluster matches separately
	s.sortMatches(namespaceMatches, model)
	s.sortMatches(clusterMatches, model)

	// Append cluster matches after namespace matches (namespace-scoped have priority)
	matches := append(namespaceMatches, clusterMatches...)

	logger.Info("Found compatible runtimes",
		"model", model.ModelFormat.Name,
		"count", len(matches))

	return matches, nil
}

// ValidateRuntime checks if a specific runtime supports a model.
func (s *defaultSelector) ValidateRuntime(ctx context.Context, runtimeName string, model *v1beta1.BaseModelSpec, namespace string) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Validating runtime",
		"runtime", runtimeName,
		"model", model.ModelFormat.Name,
		"namespace", namespace)

	// Validate model
	if err := s.validateModel(model); err != nil {
		return err
	}

	// Get the specific runtime
	runtimeSpec, isCluster, err := s.fetcher.GetRuntime(ctx, runtimeName, namespace)
	if err != nil {
		return err
	}

	// Check if runtime is disabled
	if runtimeSpec.IsDisabled() {
		return &RuntimeDisabledError{
			RuntimeName: runtimeName,
			IsCluster:   isCluster,
		}
	}

	// Check compatibility
	compatible, err := s.matcher.IsCompatible(runtimeSpec, model, runtimeName)
	if err != nil {
		return err
	}

	if !compatible {
		// Get detailed compatibility report for better error message
		report, _ := s.matcher.GetCompatibilityDetails(runtimeSpec, model, runtimeName)

		reason := "incompatible model format"
		if report != nil && len(report.IncompatibilityReasons) > 0 {
			reason = report.IncompatibilityReasons[0]
		}

		return &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   getModelName(model),
			ModelFormat: model.ModelFormat.Name,
			Reason:      reason,
		}
	}

	// Check if runtime has auto-select enabled
	hasAutoSelect := false
	for _, format := range runtimeSpec.SupportedModelFormats {
		if format.AutoSelect != nil && *format.AutoSelect {
			hasAutoSelect = true
			break
		}
	}

	if !hasAutoSelect {
		logger.V(1).Info("Runtime does not have auto-select enabled but is compatible",
			"runtime", runtimeName)
	}

	return nil
}

// evaluateRuntime evaluates a single runtime for compatibility and scoring.
func (s *defaultSelector) evaluateRuntime(ctx context.Context, spec *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, name string, isCluster bool) *RuntimeMatch {
	logger := log.FromContext(ctx)

	// Skip disabled runtimes
	if spec.IsDisabled() {
		logger.V(2).Info("Skipping disabled runtime", "runtime", name)
		return nil
	}

	// Check basic compatibility (mimics RuntimeSupportsModel)
	report, err := s.matcher.GetCompatibilityDetails(spec, model, name)
	if err != nil {
		logger.Error(err, "Failed to get compatibility details", "runtime", name)
		return nil
	}

	if !report.IsCompatible {
		logger.V(2).Info("Runtime not compatible",
			"runtime", name,
			"reasons", report.IncompatibilityReasons)
		return nil
	}

	// Check if runtime has auto-select enabled for at least one supported format
	hasAutoSelect := false
	for _, format := range spec.SupportedModelFormats {
		if format.AutoSelect != nil && *format.AutoSelect {
			hasAutoSelect = true
			break
		}
	}

	if !hasAutoSelect {
		logger.V(2).Info("Runtime does not have auto-select enabled", "runtime", name)
		return nil
	}

	// Calculate score
	score, err := s.scorer.CalculateScore(spec, model)
	if err != nil {
		logger.Error(err, "Failed to calculate score", "runtime", name)
		return nil
	}

	// Skip runtimes with score <= 0 (indicates no format/framework match or autoselect is false)
	if score <= 0 {
		logger.V(2).Info("Runtime has non-positive score", "runtime", name, "score", score)
		return nil
	}

	return &RuntimeMatch{
		RuntimeSelection: RuntimeSelection{
			Name:      name,
			Spec:      spec,
			Score:     score,
			IsCluster: isCluster,
		},
		MatchDetails: report.MatchDetails,
	}
}

// sortMatches sorts runtime matches by score and other criteria.
func (s *defaultSelector) sortMatches(matches []RuntimeMatch, model *v1beta1.BaseModelSpec) {
	sort.Slice(matches, func(i, j int) bool {
		comparison := s.scorer.CompareRuntimes(matches[i], matches[j], model)
		return comparison > 0
	})
}

// GetRuntime fetches a specific runtime by name.
func (s *defaultSelector) GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error) {
	return s.fetcher.GetRuntime(ctx, name, namespace)
}

// validateModel performs basic validation on the model specification.
func (s *defaultSelector) validateModel(model *v1beta1.BaseModelSpec) error {
	if model == nil {
		return &ModelValidationError{
			Field:   "model",
			Message: "model specification is nil",
		}
	}

	if model.ModelFormat.Name == "" {
		return &ModelValidationError{
			Field:   "modelFormat.name",
			Message: "model format name is required",
		}
	}

	return nil
}

// getModelName extracts a name from the model spec if available.
func getModelName(model *v1beta1.BaseModelSpec) string {
	// This is a placeholder - in practice, you might get this from annotations or other fields
	if model.ModelFormat.Name != "" {
		return model.ModelFormat.Name
	}
	return "unknown"
}
