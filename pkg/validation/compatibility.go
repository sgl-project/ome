package validation

import (
	"fmt"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	modelVer "sigs.k8s.io/ome/pkg/modelver"
)

// CompatibilityOptions configures model-to-format compatibility checks.
type CompatibilityOptions struct {
	ModelCacheProvider string
}

// CompatibilityOption is a functional option for CompareModelToFormat and
// GetFormatMismatchReason.
type CompatibilityOption func(*CompatibilityOptions)

// WithModelCacheProvider sets the cluster's model cache provider name. When
// set, sharded-distribution models are checked against the format's supported
// providers. When omitted, the model-cache check is skipped (suitable for
// offline validation where cluster config is unavailable).
func WithModelCacheProvider(provider string) CompatibilityOption {
	return func(o *CompatibilityOptions) { o.ModelCacheProvider = provider }
}

func buildOptions(opts []CompatibilityOption) CompatibilityOptions {
	var o CompatibilityOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// CompareModelToFormat checks if a model matches a supported format entry. It
// mirrors the logic of DefaultRuntimeMatcher.compareSupportedModelFormats but
// is a pure function with no receiver or config dependency.
func CompareModelToFormat(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat, opts ...CompatibilityOption) bool {
	o := buildOptions(opts)

	if o.ModelCacheProvider != "" {
		if modelRequiresCacheProvider(model) && !supportedFormatSupportsModelCacheProvider(format, o.ModelCacheProvider) {
			return false
		}
	}

	if ok, _ := compareDiffusionPipeline(model.DiffusionPipeline, format.DiffusionPipeline); !ok {
		return false
	}

	// architecture
	if model.ModelArchitecture != nil && format.ModelArchitecture != nil {
		if *model.ModelArchitecture != *format.ModelArchitecture {
			return false
		}
	} else if (model.ModelArchitecture == nil) != (format.ModelArchitecture == nil) {
		return false
	}

	// quantization
	if model.Quantization != nil && format.Quantization != nil {
		if *model.Quantization != *format.Quantization {
			return false
		}
	} else if (model.Quantization == nil) != (format.Quantization == nil) {
		return false
	}

	// model format
	if format.ModelFormat != nil {
		if format.ModelFormat.Name != model.ModelFormat.Name {
			return false
		}
		if format.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			if !compareModelFormatVersions(format.ModelFormat, &model.ModelFormat) {
				return false
			}
		} else if (format.ModelFormat.Version == nil) != (model.ModelFormat.Version == nil) {
			return false
		}
	} else {
		return false
	}

	// model framework
	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name != model.ModelFramework.Name {
			return false
		}
		if format.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			if !compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework) {
				return false
			}
		} else if (format.ModelFramework.Version == nil) != (model.ModelFramework.Version == nil) {
			return false
		}
	} else if (format.ModelFramework != nil) != (model.ModelFramework != nil) {
		return false
	}

	return true
}

// GetFormatMismatchReason returns a human-readable reason why a model doesn't
// match a supported format. Returns "unknown mismatch" if no specific reason
// can be determined.
func GetFormatMismatchReason(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat, opts ...CompatibilityOption) string {
	o := buildOptions(opts)
	var reasons []string

	if o.ModelCacheProvider != "" {
		if modelRequiresCacheProvider(model) && !supportedFormatSupportsModelCacheProvider(format, o.ModelCacheProvider) {
			reasons = append(reasons, modelCacheProviderMismatchReason(o.ModelCacheProvider, format))
		}
	}

	if ok, reason := compareDiffusionPipeline(model.DiffusionPipeline, format.DiffusionPipeline); !ok {
		if reason != "" {
			reasons = append(reasons, reason)
		} else {
			reasons = append(reasons, "diffusion pipeline mismatch")
		}
	}

	// architecture
	if model.ModelArchitecture != nil && format.ModelArchitecture != nil {
		if *model.ModelArchitecture != *format.ModelArchitecture {
			reasons = append(reasons, fmt.Sprintf("architecture mismatch (model=%s, runtime=%s)",
				*model.ModelArchitecture, *format.ModelArchitecture))
		}
	} else if (model.ModelArchitecture == nil) != (format.ModelArchitecture == nil) {
		if model.ModelArchitecture == nil {
			reasons = append(reasons, fmt.Sprintf("model has no architecture but runtime requires %s",
				*format.ModelArchitecture))
		} else {
			reasons = append(reasons, fmt.Sprintf("model has architecture %s but runtime has no architecture requirement",
				*model.ModelArchitecture))
		}
	}

	// quantization
	if model.Quantization != nil && format.Quantization != nil {
		if *model.Quantization != *format.Quantization {
			reasons = append(reasons, fmt.Sprintf("quantization mismatch (model=%s, runtime=%s)",
				*model.Quantization, *format.Quantization))
		}
	} else if (model.Quantization == nil) != (format.Quantization == nil) {
		if model.Quantization == nil {
			reasons = append(reasons, fmt.Sprintf("model has no quantization but runtime requires %s",
				*format.Quantization))
		} else {
			reasons = append(reasons, fmt.Sprintf("model has quantization %s but runtime has no quantization requirement",
				*model.Quantization))
		}
	}

	// model format
	if format.ModelFormat != nil {
		if format.ModelFormat.Name != model.ModelFormat.Name {
			reasons = append(reasons, fmt.Sprintf("format name mismatch (model=%s, runtime=%s)",
				model.ModelFormat.Name, format.ModelFormat.Name))
		} else if format.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			if !compareModelFormatVersions(format.ModelFormat, &model.ModelFormat) {
				reasons = append(reasons, fmt.Sprintf("format version mismatch (model=%s, runtime=%s)",
					*model.ModelFormat.Version, *format.ModelFormat.Version))
			}
		} else if (format.ModelFormat.Version == nil) != (model.ModelFormat.Version == nil) {
			if model.ModelFormat.Version == nil {
				reasons = append(reasons, fmt.Sprintf("model has no format version but runtime requires %s",
					*format.ModelFormat.Version))
			} else {
				reasons = append(reasons, "model has format version but runtime has no version requirement")
			}
		}
	}

	// model framework
	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name != model.ModelFramework.Name {
			reasons = append(reasons, fmt.Sprintf("framework name mismatch (model=%s, runtime=%s)",
				model.ModelFramework.Name, format.ModelFramework.Name))
		} else if format.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			if !compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework) {
				reasons = append(reasons, fmt.Sprintf("framework version mismatch (model=%s, runtime=%s)",
					*model.ModelFramework.Version, *format.ModelFramework.Version))
			}
		} else if (format.ModelFramework.Version == nil) != (model.ModelFramework.Version == nil) {
			if model.ModelFramework.Version == nil {
				reasons = append(reasons, fmt.Sprintf("model has no framework version but runtime requires %s",
					*format.ModelFramework.Version))
			} else {
				reasons = append(reasons, "model has framework version but runtime has no version requirement")
			}
		}
	} else if (format.ModelFramework != nil) != (model.ModelFramework != nil) {
		if model.ModelFramework == nil {
			reasons = append(reasons, fmt.Sprintf("model has no framework but runtime requires %s",
				format.ModelFramework.Name))
		} else {
			reasons = append(reasons, fmt.Sprintf("model has framework %s but runtime has no framework requirement",
				model.ModelFramework.Name))
		}
	}

	if len(reasons) == 0 {
		return "unknown mismatch"
	}
	return strings.Join(reasons, ", ")
}

// --- version comparison helpers ---

func compareModelFormatVersions(supportedFormat *v1beta1.ModelFormat, modelFormat *v1beta1.ModelFormat) bool {
	baseVersion, err := modelVer.Parse(*modelFormat.Version)
	if err != nil {
		return false
	}

	supportedVersion, err := modelVer.Parse(*supportedFormat.Version)
	if err != nil {
		return false
	}

	hasUnofficial := modelVer.ContainsUnofficialVersion(baseVersion) ||
		modelVer.ContainsUnofficialVersion(supportedVersion)

	operator := getRuntimeSelectorOperator(supportedFormat.Operator)

	if hasUnofficial || operator == "Equal" {
		return modelVer.Equal(supportedVersion, baseVersion)
	}

	if baseVersion.Precision != supportedVersion.Precision ||
		strings.Compare(baseVersion.MajorPrefix, supportedVersion.MajorPrefix) != 0 {
		return false
	}

	switch operator {
	case "GreaterThan":
		return modelVer.GreaterThan(supportedVersion, baseVersion)
	case "GreaterThanOrEqual":
		return modelVer.GreaterThanOrEqual(supportedVersion, baseVersion)
	default:
		return modelVer.Equal(supportedVersion, baseVersion)
	}
}

func compareModelFrameworkVersions(supportedFramework *v1beta1.ModelFrameworkSpec, modelFramework *v1beta1.ModelFrameworkSpec) bool {
	baseVersion, err := modelVer.Parse(*modelFramework.Version)
	if err != nil {
		return false
	}

	supportedVersion, err := modelVer.Parse(*supportedFramework.Version)
	if err != nil {
		return false
	}

	hasUnofficial := modelVer.ContainsUnofficialVersion(baseVersion) ||
		modelVer.ContainsUnofficialVersion(supportedVersion)

	operator := getRuntimeSelectorOperator(supportedFramework.Operator)

	if hasUnofficial || operator == "Equal" {
		return modelVer.Equal(supportedVersion, baseVersion)
	}

	if baseVersion.Precision != supportedVersion.Precision ||
		strings.Compare(baseVersion.MajorPrefix, supportedVersion.MajorPrefix) != 0 {
		return false
	}

	switch operator {
	case "GreaterThan":
		return modelVer.GreaterThan(supportedVersion, baseVersion)
	case "GreaterThanOrEqual":
		return modelVer.GreaterThanOrEqual(supportedVersion, baseVersion)
	default:
		return modelVer.Equal(supportedVersion, baseVersion)
	}
}

func getRuntimeSelectorOperator(operator *v1beta1.RuntimeSelectorOperator) string {
	if operator == nil {
		return string(v1beta1.RuntimeSelectorOpEqual)
	}
	return string(*operator)
}

// --- diffusion pipeline helpers ---

func compareDiffusionPipeline(model *v1beta1.DiffusionPipelineSpec, format *v1beta1.DiffusionPipelineSpec) (bool, string) {
	if format == nil {
		return true, ""
	}

	if model == nil {
		return false, "diffusion pipeline required by runtime but not specified in model"
	}

	if format.ClassName != nil {
		if model.ClassName == nil || *format.ClassName != *model.ClassName {
			modelClassName := "<nil>"
			if model.ClassName != nil {
				modelClassName = *model.ClassName
			}
			return false, fmt.Sprintf("pipeline class mismatch (model=%s, runtime=%s)",
				modelClassName, *format.ClassName)
		}
	}

	componentChecks := []struct {
		name          string
		model, format *v1beta1.DiffusionComponentSpec
	}{
		{name: "scheduler", model: model.Scheduler, format: format.Scheduler},
		{name: "textEncoder", model: model.TextEncoder, format: format.TextEncoder},
		{name: "tokenizer", model: model.Tokenizer, format: format.Tokenizer},
		{name: "transformer", model: model.Transformer, format: format.Transformer},
		{name: "vae", model: model.VAE, format: format.VAE},
	}

	for _, check := range componentChecks {
		if ok, reason := compareDiffusionComponent(check.name, check.model, check.format); !ok {
			return false, reason
		}
	}

	if len(format.AdditionalComponents) > 0 {
		if len(model.AdditionalComponents) == 0 {
			return false, "diffusion pipeline missing required additional components"
		}
		for key, formatComponent := range format.AdditionalComponents {
			modelComponent, exists := model.AdditionalComponents[key]
			if !exists {
				return false, fmt.Sprintf("diffusion component %s missing in model", key)
			}
			runtimeComponent := formatComponent
			if ok, reason := compareDiffusionComponent(key, &modelComponent, &runtimeComponent); !ok {
				return false, reason
			}
		}
	}

	return true, ""
}

func compareDiffusionComponent(name string, model *v1beta1.DiffusionComponentSpec, runtime *v1beta1.DiffusionComponentSpec) (bool, string) {
	if runtime == nil {
		return true, ""
	}
	if model == nil {
		return false, fmt.Sprintf("component %s required by runtime but not specified in model", name)
	}
	if runtime.Library != "" && runtime.Library != model.Library {
		return false, fmt.Sprintf("%s library mismatch (model=%s, runtime=%s)",
			name, model.Library, runtime.Library)
	}
	if runtime.Type != "" && runtime.Type != model.Type {
		return false, fmt.Sprintf("%s type mismatch (model=%s, runtime=%s)",
			name, model.Type, runtime.Type)
	}
	return true, ""
}

// --- model cache provider helpers ---

func modelRequiresCacheProvider(model *v1beta1.BaseModelSpec) bool {
	return model != nil &&
		model.Distribution != nil &&
		*model.Distribution == v1beta1.DistributionSharded
}

func supportedFormatSupportsModelCacheProvider(format v1beta1.SupportedModelFormat, provider string) bool {
	provider = strings.TrimSpace(provider)
	if provider == "" || len(format.ModelCacheProviders) == 0 {
		return false
	}
	for _, supported := range format.ModelCacheProviders {
		if string(supported) == provider {
			return true
		}
	}
	return false
}

func modelCacheProviderMismatchReason(provider string, format v1beta1.SupportedModelFormat) string {
	provider = strings.TrimSpace(provider)
	if len(format.ModelCacheProviders) == 0 {
		if provider == "" {
			return "no model cache provider is configured for sharded model loading"
		}
		return fmt.Sprintf("runtime format does not support model cache provider %q", provider)
	}
	if provider == "" {
		return "no model cache provider is configured for sharded model loading"
	}

	supported := make([]string, 0, len(format.ModelCacheProviders))
	for _, cacheProvider := range format.ModelCacheProviders {
		supported = append(supported, string(cacheProvider))
	}
	return fmt.Sprintf("runtime format supports model cache providers [%s], not %q", strings.Join(supported, ", "), provider)
}
