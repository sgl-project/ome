package runtimeselector

import (
	"context"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

type defaultSelector struct {
	config  *Config
	fetcher RuntimeFetcher
	matcher RuntimeMatcher
	scorer  RuntimeScorer
}

func New(client client.Client) Selector {
	return NewWithConfig(NewConfig(client))
}

func NewWithConfig(config *Config) Selector {
	return &defaultSelector{
		config:  config,
		fetcher: NewDefaultRuntimeFetcher(config.Client),
		matcher: NewDefaultRuntimeMatcher(config),
		scorer:  NewDefaultRuntimeScorer(config),
	}
}

func (s *defaultSelector) SelectRuntime(ctx context.Context, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService) (*RuntimeSelection, error) {
	namespace := isvc.Namespace
	logger := log.FromContext(ctx)

	if err := s.validateModel(model); err != nil {
		return nil, err
	}

	matches, err := s.GetCompatibleRuntimes(ctx, model, isvc, namespace)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		collection, _ := s.fetcher.FetchRuntimes(ctx, namespace)
		excludedRuntimes := make(map[string]error)
		collection.forEach(func(name string, spec *v1beta1.ServingRuntimeSpec, _ bool) {
			if compatible, _ := s.matcher.IsCompatible(spec, model, isvc, name); compatible {
				return
			}
			report, _ := s.matcher.GetCompatibilityDetails(spec, model, isvc, name)
			if report != nil && len(report.IncompatibilityReasons) > 0 {
				excludedRuntimes[name] = fmt.Errorf("%s", report.IncompatibilityReasons[0])
			}
		})

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

	best := matches[0]
	logger.Info("Selected runtime",
		"runtime", best.Name,
		"score", best.Score,
		"isCluster", best.IsCluster,
		"model", model.ModelFormat.Name,
		"namespace", namespace,
		"candidates", len(matches))

	return &best.RuntimeSelection, nil
}

func (s *defaultSelector) GetCompatibleRuntimes(ctx context.Context, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, namespace string) ([]RuntimeMatch, error) {
	logger := log.FromContext(ctx)

	if err := s.validateModel(model); err != nil {
		return nil, err
	}

	collection, err := s.fetcher.FetchRuntimes(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch runtimes: %w", err)
	}

	var namespaceMatches, clusterMatches []RuntimeMatch
	collection.forEach(func(name string, spec *v1beta1.ServingRuntimeSpec, isCluster bool) {
		match := s.evaluateRuntime(ctx, spec, model, isvc, name, isCluster)
		if match == nil {
			return
		}
		if isCluster {
			clusterMatches = append(clusterMatches, *match)
		} else {
			namespaceMatches = append(namespaceMatches, *match)
		}
	})

	// Namespace-scoped runtimes always win against cluster runtimes — sort
	// each list independently, then concatenate.
	s.sortMatches(namespaceMatches, model)
	s.sortMatches(clusterMatches, model)
	matches := append(namespaceMatches, clusterMatches...)

	logger.V(1).Info("Compatible runtimes evaluated",
		"model", model.ModelFormat.Name,
		"namespace", namespace,
		"matches", len(matches),
		"considered", len(collection.NamespaceRuntimes)+len(collection.ClusterRuntimes))

	return matches, nil
}

func (s *defaultSelector) ValidateRuntime(ctx context.Context, runtimeName string, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService) error {
	namespace := isvc.Namespace

	if err := s.validateModel(model); err != nil {
		return err
	}

	runtimeSpec, isCluster, err := s.fetcher.GetRuntime(ctx, runtimeName, namespace)
	if err != nil {
		return err
	}

	if runtimeSpec.IsDisabled() {
		return &RuntimeDisabledError{
			RuntimeName: runtimeName,
			IsCluster:   isCluster,
		}
	}

	compatible, err := s.matcher.IsCompatible(runtimeSpec, model, isvc, runtimeName)
	if err != nil {
		return err
	}

	if !compatible {
		report, _ := s.matcher.GetCompatibilityDetails(runtimeSpec, model, isvc, runtimeName)
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

	return nil
}

func (s *defaultSelector) evaluateRuntime(ctx context.Context, spec *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, name string, isCluster bool) *RuntimeMatch {
	logger := log.FromContext(ctx)

	if spec.IsDisabled() {
		logger.V(2).Info("Skipping disabled runtime", "runtime", name)
		return nil
	}

	report, err := s.matcher.GetCompatibilityDetails(spec, model, isvc, name)
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

	if !runtimeHasAutoSelectFormat(spec) {
		logger.V(2).Info("Runtime does not have auto-select enabled", "runtime", name)
		return nil
	}

	score, err := s.scorer.CalculateScore(spec, model)
	if err != nil {
		logger.Error(err, "Failed to calculate score", "runtime", name)
		return nil
	}

	// Zero score means none of the supported formats produced a match
	// (format/framework name or version mismatch). Exclude these so they
	// don't pollute the ranked candidate list.
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

func (s *defaultSelector) sortMatches(matches []RuntimeMatch, model *v1beta1.BaseModelSpec) {
	sort.Slice(matches, func(i, j int) bool {
		return s.scorer.CompareRuntimes(matches[i], matches[j], model) > 0
	})
}

func (s *defaultSelector) GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error) {
	return s.fetcher.GetRuntime(ctx, name, namespace)
}

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

func getModelName(model *v1beta1.BaseModelSpec) string {
	if model.ModelFormat.Name != "" {
		return model.ModelFormat.Name
	}
	return "unknown"
}

func (s *defaultSelector) GetSupportedModelFormat(ctx context.Context, runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, userSpecifiedRuntime bool) *v1beta1.SupportedModelFormat {
	// Lean path (no model): downstream components treat a nil
	// SupportedModelFormat as "no model-format metadata available", which is
	// the right shape for runtime-only ISVCs that don't use model parsing.
	if model == nil || runtime == nil || runtime.SupportedModelFormats == nil {
		return nil
	}
	maxScore := int64(0)
	bestSupportedFormat := v1beta1.SupportedModelFormat{}
	for _, supportedFormat := range runtime.SupportedModelFormats {
		if !userSpecifiedRuntime && (supportedFormat.AutoSelect == nil || !*supportedFormat.AutoSelect) {
			continue
		}
		score := s.scorer.CalculateFormatScore(model, supportedFormat, int64(s.config.DefaultPriority))
		if score > maxScore {
			maxScore = score
			bestSupportedFormat = supportedFormat
		}
	}
	if maxScore > 0 {
		return &bestSupportedFormat
	}
	return nil
}
