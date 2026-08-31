package runtimeselector

import (
	"math"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// DefaultRuntimeScorer implements RuntimeScorer with configurable scoring weights.
type DefaultRuntimeScorer struct {
	config *Config
}

func NewDefaultRuntimeScorer(config *Config) RuntimeScorer {
	return &DefaultRuntimeScorer{
		config: config,
	}
}

// CalculateScore picks the best-scoring supported format on the runtime.
// Each format contributes (modelFormat.weight + modelFramework.weight) *
// priority; zero weights fall back to the global defaults in Config.
func (s *DefaultRuntimeScorer) CalculateScore(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec) (int64, error) {
	var maxScore int64 = 0

	for _, supportedFormat := range runtime.SupportedModelFormats {
		if modelRequiresCacheProvider(model) && !supportedFormatSupportsModelCacheProvider(supportedFormat, s.config.ModelCacheProvider) {
			continue
		}

		if supportedFormat.AutoSelect != nil && !(*supportedFormat.AutoSelect) {
			continue
		}

		priority := int64(s.config.DefaultPriority)
		if supportedFormat.Priority != nil {
			priority = int64(*supportedFormat.Priority)
		}

		score := s.CalculateFormatScore(model, supportedFormat, priority)
		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore, nil
}

// CompareRuntimes orders by (score desc, size-range distance asc,
// namespace-scoped before cluster, then name asc) to keep selection
// deterministic when several runtimes score identically.
func (s *DefaultRuntimeScorer) CompareRuntimes(r1, r2 RuntimeMatch, model *v1beta1.BaseModelSpec) int {
	if r1.Score != r2.Score {
		return int(r1.Score - r2.Score)
	}

	// Closer ModelSizeRange wins next — protects against picking a
	// "7B-70B" runtime over a "5B-10B" one for a 7B model when both
	// score the same on format/framework match.
	if model.ModelParameterSize != nil {
		r1SizeScore := s.calculateSizeScore(r1, model)
		r2SizeScore := s.calculateSizeScore(r2, model)

		if r1SizeScore != r2SizeScore {
			return int(r2SizeScore - r1SizeScore)
		}
	}

	// Namespace runtimes are tenant overrides and should win against
	// equally-scoring cluster runtimes.
	if r1.IsCluster != r2.IsCluster {
		if r1.IsCluster {
			return -1
		}
		return 1
	}

	if r1.Name < r2.Name {
		return 1
	} else if r1.Name > r2.Name {
		return -1
	}

	return 0
}

func (s *DefaultRuntimeScorer) CalculateFormatScore(model *v1beta1.BaseModelSpec, supportedFormat v1beta1.SupportedModelFormat, priority int64) int64 {
	if modelRequiresCacheProvider(model) && !supportedFormatSupportsModelCacheProvider(supportedFormat, s.config.ModelCacheProvider) {
		return 0
	}

	modelFormatMatches := false
	if supportedFormat.ModelFormat != nil {
		if supportedFormat.ModelFormat.Name != model.ModelFormat.Name {
			return 0
		}
		if supportedFormat.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			modelFormatMatches = s.compareVersions(supportedFormat.ModelFormat, &model.ModelFormat)
			if !modelFormatMatches {
				return 0
			}
		} else {
			modelFormatMatches = true
		}
	}

	modelFrameworkMatches := false
	if supportedFormat.ModelFramework != nil && model.ModelFramework != nil {
		if supportedFormat.ModelFramework.Name != model.ModelFramework.Name {
			return 0
		}
		if supportedFormat.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			modelFrameworkMatches = s.compareFrameworkVersions(supportedFormat.ModelFramework, model.ModelFramework)
			if !modelFrameworkMatches {
				return 0
			}
		} else {
			modelFrameworkMatches = true
		}
	}

	if (modelFormatMatches || supportedFormat.ModelFormat == nil) &&
		(modelFrameworkMatches || (supportedFormat.ModelFramework == nil && model.ModelFramework == nil)) {

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

// calculateSizeScore is the sum-of-distances of the model size from each
// boundary of the runtime's supported range — lower means a tighter fit.
// Min and Max are each +optional in the CRD; an unset bound is treated as an
// open (infinitely far) edge that contributes no distance, so a partial range
// scores only on the bound it actually sets rather than dereferencing nil.
func (s *DefaultRuntimeScorer) calculateSizeScore(runtime RuntimeMatch, model *v1beta1.BaseModelSpec) float64 {
	if runtime.Spec == nil || runtime.Spec.ModelSizeRange == nil || model.ModelParameterSize == nil {
		return 0
	}

	sizeRange := runtime.Spec.ModelSizeRange
	modelSize := parseModelSize(*model.ModelParameterSize)

	var distance float64
	if sizeRange.Min != nil {
		distance += math.Abs(parseModelSize(*sizeRange.Min) - modelSize)
	}
	if sizeRange.Max != nil {
		distance += math.Abs(parseModelSize(*sizeRange.Max) - modelSize)
	}

	return distance
}

func (s *DefaultRuntimeScorer) compareVersions(supportedFormat *v1beta1.ModelFormat, modelFormat *v1beta1.ModelFormat) bool {
	return compareVersionsWithOperator(*modelFormat.Version, *supportedFormat.Version, supportedFormat.Operator)
}

func (s *DefaultRuntimeScorer) compareFrameworkVersions(supportedFramework *v1beta1.ModelFrameworkSpec, modelFramework *v1beta1.ModelFrameworkSpec) bool {
	return compareVersionsWithOperator(*modelFramework.Version, *supportedFramework.Version, supportedFramework.Operator)
}
