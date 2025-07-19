package runtimeselector

import (
	"context"
	"math"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// DefaultRuntimeScorer implements RuntimeScorer with configurable scoring weights.
type DefaultRuntimeScorer struct {
	config *Config
}

// NewDefaultRuntimeScorer creates a new DefaultRuntimeScorer.
func NewDefaultRuntimeScorer(config *Config) RuntimeScorer {
	return &DefaultRuntimeScorer{
		config: config,
	}
}

// CalculateScore returns a score for how well a runtime matches a model.
// The score is calculated based on:
// 1. Model format match and weight
// 2. Model framework match and weight
// 3. Priority multiplier
// 4. Model size proximity (if applicable)
func (s *DefaultRuntimeScorer) CalculateScore(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec) (int64, error) {
	ctx := context.Background()
	logger := log.FromContext(ctx)

	var maxScore int64 = 0

	// Go through all supported model formats in runtime
	for _, supportedFormat := range runtime.SupportedModelFormats {
		// Skip if autoSelect is explicitly false
		if supportedFormat.AutoSelect != nil && !(*supportedFormat.AutoSelect) {
			continue
		}

		// Get priority for this format
		priority := int64(s.config.DefaultPriority)
		if supportedFormat.Priority != nil {
			priority = int64(*supportedFormat.Priority)
		}

		// Calculate score for this format
		score := s.calculateFormatScore(model, supportedFormat, priority)

		if score > maxScore {
			maxScore = score
		}
	}

	logger.V(2).Info("Calculated runtime score",
		"runtime", runtime,
		"model", model.ModelFormat.Name,
		"score", maxScore)

	return maxScore, nil
}

// CompareRuntimes compares two runtime matches for a given model.
// Returns positive if r1 is better, negative if r2 is better, 0 if equal.
func (s *DefaultRuntimeScorer) CompareRuntimes(r1, r2 RuntimeMatch, model *v1beta1.BaseModelSpec) int {
	// First, compare by score
	if r1.Score != r2.Score {
		return int(r1.Score - r2.Score)
	}

	// If scores are equal, compare by model size range if available
	if model.ModelParameterSize != nil {
		r1SizeScore := s.calculateSizeScore(r1, model)
		r2SizeScore := s.calculateSizeScore(r2, model)

		if r1SizeScore != r2SizeScore {
			// Lower score is better (closer to model size)
			return int(r2SizeScore - r1SizeScore)
		}
	}

	// If still equal, prefer namespace-scoped runtimes over cluster-scoped
	if r1.IsCluster != r2.IsCluster {
		if r1.IsCluster {
			return -1 // r2 is namespace-scoped, prefer it
		}
		return 1 // r1 is namespace-scoped, prefer it
	}

	// Finally, compare by name for deterministic ordering
	if r1.Name < r2.Name {
		return 1
	} else if r1.Name > r2.Name {
		return -1
	}

	return 0
}

// calculateFormatScore calculates the score for a specific supported format.
// This matches the exact logic from the original score() function.
func (s *DefaultRuntimeScorer) calculateFormatScore(model *v1beta1.BaseModelSpec, supportedFormat v1beta1.SupportedModelFormat, priority int64) int64 {
	// Compare model format
	modelFormatMatches := false
	if supportedFormat.ModelFormat != nil && &model.ModelFormat.Name != nil {
		if supportedFormat.ModelFormat.Name != model.ModelFormat.Name {
			return 0 // Format name doesn't match
		}
		// Compare versions if both are specified
		if supportedFormat.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			modelFormatMatches = s.compareVersions(supportedFormat.ModelFormat, &model.ModelFormat)
			if !modelFormatMatches {
				return 0 // Version doesn't match
			}
		} else {
			modelFormatMatches = true
		}
	}

	// Compare model framework
	modelFrameworkMatches := false
	if supportedFormat.ModelFramework != nil && model.ModelFramework != nil {
		if supportedFormat.ModelFramework.Name != model.ModelFramework.Name {
			return 0 // Framework name doesn't match
		}
		// Compare versions if both are specified
		if supportedFormat.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			modelFrameworkMatches = s.compareFrameworkVersions(supportedFormat.ModelFramework, model.ModelFramework)
			if !modelFrameworkMatches {
				return 0 // Version doesn't match
			}
		} else {
			modelFrameworkMatches = true
		}
	}

	// Check the matching condition (same as original line 223-224)
	if (modelFormatMatches || (supportedFormat.ModelFormat == nil && &model.ModelFormat == nil)) &&
		(modelFrameworkMatches || (supportedFormat.ModelFramework == nil && model.ModelFramework == nil)) {

		// Calculate weighted score
		var currentScore int64 = 0
		if modelFormatMatches && supportedFormat.ModelFormat != nil {
			weight := supportedFormat.ModelFormat.Weight
			if weight == 0 {
				weight = s.config.ModelFormatWeight
			}
			currentScore += weight * priority
		}
		if modelFrameworkMatches && supportedFormat.ModelFramework != nil {
			weight := supportedFormat.ModelFramework.Weight
			if weight == 0 {
				weight = s.config.ModelFrameworkWeight
			}
			currentScore += weight * priority
		}

		return currentScore
	}

	return 0
}

// calculateSizeScore calculates a score based on how close the model size is to the runtime's range.
// Lower scores are better (closer to the model size).
func (s *DefaultRuntimeScorer) calculateSizeScore(runtime RuntimeMatch, model *v1beta1.BaseModelSpec) float64 {
	if runtime.Spec == nil || runtime.Spec.ModelSizeRange == nil || model.ModelParameterSize == nil {
		return 0
	}

	modelSize := parseModelSize(*model.ModelParameterSize)
	minSize := parseModelSize(*runtime.Spec.ModelSizeRange.Min)
	maxSize := parseModelSize(*runtime.Spec.ModelSizeRange.Max)

	// Calculate distance from the model size to the range boundaries
	minDiff := math.Abs(minSize - modelSize)
	maxDiff := math.Abs(maxSize - modelSize)

	// Return the sum of distances (lower is better)
	return minDiff + maxDiff
}

// compareVersions is a helper that delegates to the matcher's version comparison logic.
func (s *DefaultRuntimeScorer) compareVersions(supportedFormat *v1beta1.ModelFormat, modelFormat *v1beta1.ModelFormat) bool {
	matcher := NewDefaultRuntimeMatcher(s.config)
	return matcher.(*DefaultRuntimeMatcher).compareModelFormatVersions(supportedFormat, modelFormat)
}

// compareFrameworkVersions is a helper that delegates to the matcher's version comparison logic.
func (s *DefaultRuntimeScorer) compareFrameworkVersions(supportedFramework *v1beta1.ModelFrameworkSpec, modelFramework *v1beta1.ModelFrameworkSpec) bool {
	matcher := NewDefaultRuntimeMatcher(s.config)
	return matcher.(*DefaultRuntimeMatcher).compareModelFrameworkVersions(supportedFramework, modelFramework)
}
