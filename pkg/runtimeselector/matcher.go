package runtimeselector

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	modelVer "sigs.k8s.io/ome/pkg/modelver"
)

// DefaultRuntimeMatcher implements RuntimeMatcher with comprehensive compatibility checking.
type DefaultRuntimeMatcher struct {
	config *Config
}

func NewDefaultRuntimeMatcher(config *Config) RuntimeMatcher {
	return &DefaultRuntimeMatcher{
		config: config,
	}
}

func (m *DefaultRuntimeMatcher) IsCompatible(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (bool, error) {
	if runtime.IsDisabled() {
		return false, &RuntimeDisabledError{RuntimeName: runtimeName}
	}

	// Check accelerator class compatibility
	if !m.compareAcceleratorClass(runtime, isvc) {
		return false, &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   "", // Will be filled by caller if available
			ModelFormat: model.ModelFormat.Name,
			Reason:      "runtime does not support the required accelerator class",
		}
	}
	// Apply component-level deployment mode constraints
	if ok, reason := m.compareDeploymentMode(runtime, isvc); !ok {
		return false, &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   "", // Will be filled by caller if available
			ModelFormat: model.ModelFormat.Name,
			Reason:      reason,
		}
	}

	for _, format := range runtime.SupportedModelFormats {
		if !m.compareSupportedModelFormats(model, format) {
			continue
		}
		// A matching format is only useful if the model size fits — keep
		// scanning the rest in case a different supported format pairs
		// with a wider ModelSizeRange.
		if err := m.checkModelSize(runtime, model, runtimeName); err == nil {
			return true, nil
		}
	}

	return false, nil
}

func (m *DefaultRuntimeMatcher) GetCompatibilityDetails(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (*CompatibilityReport, error) {
	report := &CompatibilityReport{
		IncompatibilityReasons: []string{},
		Warnings:               []string{},
	}

	if runtime.IsDisabled() {
		report.IncompatibilityReasons = append(report.IncompatibilityReasons, "runtime is disabled")
		return report, nil
	}

	// Check if accelerator class is compatible
	if !m.compareAcceleratorClass(runtime, isvc) {
		report.IncompatibilityReasons = append(report.IncompatibilityReasons,
			"runtime does not support the required accelerator class")
		return report, nil
	}

	// Apply component-level deployment mode constraints
	if ok, reason := m.compareDeploymentMode(runtime, isvc); !ok {
		report.IncompatibilityReasons = append(report.IncompatibilityReasons, reason)
		return report, nil
	}

	formatSupported := false
	var formatMismatchReasons []string
	for _, format := range runtime.SupportedModelFormats {
		if m.compareSupportedModelFormats(model, format) {
			formatSupported = true
			report.MatchDetails = m.evaluateFormatMatch(model, format)
			break
		}
		formatMismatchReasons = append(formatMismatchReasons, m.getFormatMismatchReason(model, format))
	}

	if !formatSupported {
		if len(formatMismatchReasons) > 0 {
			report.IncompatibilityReasons = append(report.IncompatibilityReasons,
				fmt.Sprintf("model format '%s' not in supported formats: %s",
					getModelFormatLabel(model), strings.Join(formatMismatchReasons, "; ")))
		} else {
			report.IncompatibilityReasons = append(report.IncompatibilityReasons,
				fmt.Sprintf("model format '%s' not in supported formats: no supported formats defined",
					getModelFormatLabel(model)))
		}
		return report, nil
	}

	if model.ModelParameterSize != nil && runtime.ModelSizeRange != nil {
		if !modelSizeInRange(*model.ModelParameterSize, runtime.ModelSizeRange) {
			report.IncompatibilityReasons = append(report.IncompatibilityReasons,
				fmt.Sprintf("model size %s is outside supported range %s",
					*model.ModelParameterSize, modelSizeRangeLabel(runtime.ModelSizeRange)))
			report.MatchDetails.SizeMatch = false
			return report, nil
		}
		report.MatchDetails.SizeMatch = true
	}

	report.IsCompatible = true

	// AutoSelect=false makes the runtime ineligible for auto-selection but
	// the runtime can still be picked explicitly via spec.runtime.name —
	// hence "warning", not "incompatible".
	if !runtimeHasAutoSelectFormat(runtime) {
		report.Warnings = append(report.Warnings,
			"runtime does not have auto-select enabled for any supported format")
	}

	if model.ModelParameterSize == nil && runtime.ModelSizeRange != nil {
		report.Warnings = append(report.Warnings,
			"model does not specify size, but runtime has size constraints")
	}

	return report, nil
}

// evaluateFormatMatch runs only after compareSupportedModelFormats has
// already confirmed compatibility, so the cross-cutting matches (cache
// provider, diffusion pipeline, architecture, quantization) start true
// and the reason-collection branches act as defense in depth in case a
// future caller invokes this directly.
func (m *DefaultRuntimeMatcher) evaluateFormatMatch(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat) MatchDetails {
	match := MatchDetails{
		ArchitectureMatch:       true,
		DiffusionPipelineMatch:  true,
		QuantizationMatch:       true,
		ModelCacheProviderMatch: true,
		SizeMatch:               true,
		Priority:                m.config.DefaultPriority,
		Reasons:                 []string{},
	}

	if format.Priority != nil {
		match.Priority = *format.Priority
	}
	if format.AutoSelect != nil {
		match.AutoSelectEnabled = *format.AutoSelect
	}

	pipelineMatch, pipelineReason := m.compareDiffusionPipeline(model.DiffusionPipeline, format.DiffusionPipeline)
	match.DiffusionPipelineMatch = pipelineMatch
	if !pipelineMatch && pipelineReason != "" {
		match.Reasons = append(match.Reasons, pipelineReason)
	}

	match.ArchitectureMatch = optionalEqual(model.ModelArchitecture, format.ModelArchitecture)
	if !match.ArchitectureMatch {
		if model.ModelArchitecture != nil && format.ModelArchitecture != nil {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("architecture mismatch: model=%s, runtime=%s",
					*model.ModelArchitecture, *format.ModelArchitecture))
		} else {
			match.Reasons = append(match.Reasons, "architecture requirement mismatch")
		}
	}

	match.QuantizationMatch = optionalEqual(model.Quantization, format.Quantization)
	if !match.QuantizationMatch {
		if model.Quantization != nil && format.Quantization != nil {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("quantization mismatch: model=%s, runtime=%s",
					*model.Quantization, *format.Quantization))
		} else {
			match.Reasons = append(match.Reasons, "quantization requirement mismatch")
		}
	}

	if format.ModelFormat != nil && format.ModelFormat.Name == model.ModelFormat.Name {
		match.FormatMatch = matchOptionalVersions(model.ModelFormat.Version, format.ModelFormat.Version, func() bool {
			return m.compareModelFormatVersions(format.ModelFormat, &model.ModelFormat)
		})
		if match.FormatMatch && format.ModelFormat.Weight > 0 {
			match.Weight += format.ModelFormat.Weight * int64(match.Priority)
		}
	}

	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name == model.ModelFramework.Name {
			match.FrameworkMatch = matchOptionalVersions(model.ModelFramework.Version, format.ModelFramework.Version, func() bool {
				return m.compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework)
			})
			if match.FrameworkMatch && format.ModelFramework.Weight > 0 {
				match.Weight += format.ModelFramework.Weight * int64(match.Priority)
			}
		}
	} else if format.ModelFramework == nil && model.ModelFramework == nil {
		match.FrameworkMatch = true
	}

	return match
}

func (m *DefaultRuntimeMatcher) compareSupportedModelFormats(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat) bool {

	if ok, _ := m.compareDiffusionPipeline(model.DiffusionPipeline, format.DiffusionPipeline); !ok {
		return false
	}

	if !optionalEqual(model.ModelArchitecture, format.ModelArchitecture) {
		return false
	}
	if !optionalEqual(model.Quantization, format.Quantization) {
		return false
	}

	if format.ModelFormat == nil || format.ModelFormat.Name != model.ModelFormat.Name {
		return false
	}
	if !matchOptionalVersions(model.ModelFormat.Version, format.ModelFormat.Version, func() bool {
		return m.compareModelFormatVersions(format.ModelFormat, &model.ModelFormat)
	}) {
		return false
	}

	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name != model.ModelFramework.Name {
			return false
		}
		if !matchOptionalVersions(model.ModelFramework.Version, format.ModelFramework.Version, func() bool {
			return m.compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework)
		}) {
			return false
		}
	} else if (format.ModelFramework != nil) != (model.ModelFramework != nil) {
		return false
	}

	return true
}

// getFormatMismatchReason produces an operator-friendly mismatch summary
// like "architecture mismatch (model=LlamaForCausalLM, runtime=MistralForCausalLM)"
// or a comma-joined chain when multiple attributes diverge.
func (m *DefaultRuntimeMatcher) getFormatMismatchReason(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat) string {
	var reasons []string

	if ok, reason := m.compareDiffusionPipeline(model.DiffusionPipeline, format.DiffusionPipeline); !ok {
		if reason != "" {
			reasons = append(reasons, reason)
		} else {
			reasons = append(reasons, "diffusion pipeline mismatch")
		}
	}

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

	// model.ModelFormat is a non-pointer struct so it's always present;
	// only format.ModelFormat needs a nil guard.
	if format.ModelFormat != nil {
		if format.ModelFormat.Name != model.ModelFormat.Name {
			reasons = append(reasons, fmt.Sprintf("format name mismatch (model=%s, runtime=%s)",
				model.ModelFormat.Name, format.ModelFormat.Name))
		} else if format.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			if !m.compareModelFormatVersions(format.ModelFormat, &model.ModelFormat) {
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

	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name != model.ModelFramework.Name {
			reasons = append(reasons, fmt.Sprintf("framework name mismatch (model=%s, runtime=%s)",
				model.ModelFramework.Name, format.ModelFramework.Name))
		} else if format.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			if !m.compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework) {
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

func (m *DefaultRuntimeMatcher) compareDiffusionPipeline(model *v1beta1.DiffusionPipelineSpec, format *v1beta1.DiffusionPipelineSpec) (bool, string) {
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

func (m *DefaultRuntimeMatcher) compareModelFormatVersions(supportedFormat *v1beta1.ModelFormat, modelFormat *v1beta1.ModelFormat) bool {
	return compareVersionsWithOperator(*modelFormat.Version, *supportedFormat.Version, supportedFormat.Operator)
}

func (m *DefaultRuntimeMatcher) compareModelFrameworkVersions(supportedFramework *v1beta1.ModelFrameworkSpec, modelFramework *v1beta1.ModelFrameworkSpec) bool {
	return compareVersionsWithOperator(*modelFramework.Version, *supportedFramework.Version, supportedFramework.Operator)
}

// compareVersionsWithOperator returns false if either version fails to parse.
// Unofficial (pre-release) versions always force equality regardless of the
// operator, since ordering semantics on dev/alpha tags aren't well-defined.
func compareVersionsWithOperator(modelVersion, supportedVersion string, op *v1beta1.RuntimeSelectorOperator) bool {
	base, err := modelVer.Parse(modelVersion)
	if err != nil {
		return false
	}
	supported, err := modelVer.Parse(supportedVersion)
	if err != nil {
		return false
	}

	operator := v1beta1.RuntimeSelectorOpEqual
	if op != nil {
		operator = *op
	}

	if operator == v1beta1.RuntimeSelectorOpEqual ||
		modelVer.ContainsUnofficialVersion(base) ||
		modelVer.ContainsUnofficialVersion(supported) {
		return modelVer.Equal(supported, base)
	}

	// Ordering operators only make sense when the two versions share the
	// same precision (e.g. both "1.2" or both "1.2.3") and major prefix
	// (e.g. both "v"-prefixed or both bare).
	if base.Precision != supported.Precision || base.MajorPrefix != supported.MajorPrefix {
		return false
	}

	switch operator {
	case v1beta1.RuntimeSelectorOpGreaterThan:
		return modelVer.GreaterThan(supported, base)
	case v1beta1.RuntimeSelectorOpGreaterThanOrEqual:
		return modelVer.GreaterThanOrEqual(supported, base)
	default:
		return modelVer.Equal(supported, base)
	}
}

func (m *DefaultRuntimeMatcher) checkModelSize(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) error {
	if model.ModelParameterSize == nil || runtime.ModelSizeRange == nil {
		return nil
	}

	if !modelSizeInRange(*model.ModelParameterSize, runtime.ModelSizeRange) {
		return &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelFormat: model.ModelFormat.Name,
			Reason: fmt.Sprintf("model size %s is outside supported range %s",
				*model.ModelParameterSize, modelSizeRangeLabel(runtime.ModelSizeRange)),
		}
	}

	return nil
}

// modelSizeInRange reports whether the parsed model size falls within the
// runtime's ModelSizeRange. Min and Max are each +optional in the CRD: a nil
// Min means "no lower bound" and a nil Max means "no upper bound", so a range
// with only one bound set constrains only that side rather than panicking on a
// nil dereference. Callers must guard against a nil sizeRange themselves.
func modelSizeInRange(modelSizeStr string, sizeRange *v1beta1.ModelSizeRangeSpec) bool {
	modelSize := parseModelSize(modelSizeStr)
	if sizeRange.Min != nil && modelSize < parseModelSize(*sizeRange.Min) {
		return false
	}
	if sizeRange.Max != nil && modelSize > parseModelSize(*sizeRange.Max) {
		return false
	}
	return true
}

// modelSizeRangeLabel renders a ModelSizeRange for error messages, using an
// open-interval bracket for whichever bound is unset, e.g. "[1B, 13B]",
// "[1B, inf)", or "(-inf, 13B]". Safe to call with nil Min/Max.
func modelSizeRangeLabel(sizeRange *v1beta1.ModelSizeRangeSpec) string {
	lower := "(-inf"
	if sizeRange.Min != nil {
		lower = "[" + *sizeRange.Min
	}
	upper := "inf)"
	if sizeRange.Max != nil {
		upper = *sizeRange.Max + "]"
	}
	return fmt.Sprintf("%s, %s", lower, upper)
}

func runtimeHasAutoSelectFormat(runtime *v1beta1.ServingRuntimeSpec) bool {
	for _, format := range runtime.SupportedModelFormats {
		if format.AutoSelect != nil && *format.AutoSelect {
			return true
		}
	}
	return false
}

// optionalEqual treats "both nil" and "both equal" as matches; a mixed
// presence (one side requires the field, the other doesn't specify it) is
// considered a mismatch — opposite of standard pointer equality.
func optionalEqual[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// matchOptionalVersions: both versions set -> compare via cmp; both nil ->
// match; exactly one set -> no match. Shared between ModelFormat.Version and
// ModelFrameworkSpec.Version.
func matchOptionalVersions(modelVersion, supportedVersion *string, cmp func() bool) bool {
	if modelVersion != nil && supportedVersion != nil {
		return cmp()
	}
	return modelVersion == nil && supportedVersion == nil
}

// parseModelSize converts the HuggingFace-style parameter count suffix
// ("7B", "350M", "1.5T") to a raw count. Returns 0 on parse error so callers
// can treat unknown sizes as out-of-range without panicking.
func parseModelSize(sizeStr string) float64 {
	var multiplier float64 = 1

	switch {
	case strings.HasSuffix(sizeStr, "T"):
		multiplier = 1_000_000_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "T")
	case strings.HasSuffix(sizeStr, "B"):
		multiplier = 1_000_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	case strings.HasSuffix(sizeStr, "M"):
		multiplier = 1_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	}

	size, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0
	}

	return size * multiplier
}

// getModelFormatLabel composes the "mt:" label used in user-facing error
// messages to identify a model's format/architecture/framework signature.
func getModelFormatLabel(model *v1beta1.BaseModelSpec) string {
	label := "mt:" + model.ModelFormat.Name
	if model.ModelFormat.Version != nil {
		label += ":" + *model.ModelFormat.Version
	}
	if model.ModelArchitecture != nil {
		label += ":" + *model.ModelArchitecture
	}
	if model.Quantization != nil {
		label += ":" + string(*model.Quantization)
	}
	if model.ModelFramework != nil {
		label += ":" + model.ModelFramework.Name
		if model.ModelFramework.Version != nil {
			label += ":" + *model.ModelFramework.Version
		}
	}
	return label
}

// compareAcceleratorClass checks if the runtime supports the required accelerator class.
func (m *DefaultRuntimeMatcher) compareAcceleratorClass(runtime *v1beta1.ServingRuntimeSpec, isvc *v1beta1.InferenceService) bool {
	// if inferenceService is nil, we assume no accelerator requirement
	if isvc == nil {
		return true
	}

	// Collect all unique accelerator requirements from the InferenceService
	requiredClasses := make(map[string]struct{})
	if class, ok := isvc.Annotations["ome.io/accelerator-class"]; ok {
		requiredClasses[class] = struct{}{}
	}
	if isvc.Spec.AcceleratorSelector != nil && isvc.Spec.AcceleratorSelector.AcceleratorClass != nil {
		requiredClasses[*isvc.Spec.AcceleratorSelector.AcceleratorClass] = struct{}{}
	}
	if isvc.Spec.Engine != nil && isvc.Spec.Engine.AcceleratorOverride != nil && isvc.Spec.Engine.AcceleratorOverride.AcceleratorClass != nil {
		requiredClasses[*isvc.Spec.Engine.AcceleratorOverride.AcceleratorClass] = struct{}{}
	}
	if isvc.Spec.Decoder != nil && isvc.Spec.Decoder.AcceleratorOverride != nil && isvc.Spec.Decoder.AcceleratorOverride.AcceleratorClass != nil {
		requiredClasses[*isvc.Spec.Decoder.AcceleratorOverride.AcceleratorClass] = struct{}{}
	}

	// If ISVC has no accelerator requirements, it's compatible from this perspective.
	if len(requiredClasses) == 0 {
		return true
	}

	// If ISVC has requirements, the runtime must support them.
	if runtime.AcceleratorRequirements == nil || len(runtime.AcceleratorRequirements.AcceleratorClasses) == 0 {
		return false // Runtime supports no accelerators, but ISVC requires one.
	}

	supportedClasses := runtime.AcceleratorRequirements.AcceleratorClasses
	for reqClass := range requiredClasses {
		if !slices.Contains(supportedClasses, reqClass) {
			return false
		}
	}

	return true
}

// compareDeploymentMode rejects a runtime only when both the InferenceService
// component and the matching runtime component explicitly declare different deployment modes.
func (m *DefaultRuntimeMatcher) compareDeploymentMode(runtime *v1beta1.ServingRuntimeSpec, isvc *v1beta1.InferenceService) (bool, string) {
	isvcEngineDeploymentMode, hasIsvcEngineDeploymentMode := inferenceServiceEngineDeploymentMode(isvc)
	runtimeEngineDeploymentMode, hasRuntimeEngineDeploymentMode := getRuntimeComponentDeploymentMode(runtime, v1beta1.EngineComponent)
	if ok, reason := compareComponentDeploymentMode(
		"engine",
		isvcEngineDeploymentMode,
		hasIsvcEngineDeploymentMode,
		runtimeEngineDeploymentMode,
		hasRuntimeEngineDeploymentMode,
	); !ok {
		return false, reason
	}

	isvcDecoderDeploymentMode, hasIsvcDecoderDeploymentMode := inferenceServiceDecoderDeploymentMode(isvc)
	runtimeDecoderDeploymentMode, hasRuntimeDecoderDeploymentMode := getRuntimeComponentDeploymentMode(runtime, v1beta1.DecoderComponent)
	if ok, reason := compareComponentDeploymentMode(
		"decoder",
		isvcDecoderDeploymentMode,
		hasIsvcDecoderDeploymentMode,
		runtimeDecoderDeploymentMode,
		hasRuntimeDecoderDeploymentMode,
	); !ok {
		return false, reason
	}

	return true, ""
}

func compareComponentDeploymentMode(
	componentName string,
	isvcDeploymentMode constants.DeploymentModeType,
	hasIsvcDeploymentMode bool,
	runtimeDeploymentMode constants.DeploymentModeType,
	hasRuntimeDeploymentMode bool) (bool, string) {
	// Keep the runtime unless both sides explicitly declare deployment modes and those values differ.
	if !hasIsvcDeploymentMode || !hasRuntimeDeploymentMode || isvcDeploymentMode == runtimeDeploymentMode {
		return true, ""
	}

	return false, fmt.Sprintf("runtime %s deployment mode %s does not match requested %s deployment mode %s",
		componentName, runtimeDeploymentMode, componentName, isvcDeploymentMode)
}

func inferenceServiceEngineDeploymentMode(isvc *v1beta1.InferenceService) (constants.DeploymentModeType, bool) {
	if isvc == nil || isvc.Spec.Engine == nil {
		return "", false
	}

	return deploymentModeFromAnnotations(isvc.Spec.Engine.Annotations)
}

func inferenceServiceDecoderDeploymentMode(isvc *v1beta1.InferenceService) (constants.DeploymentModeType, bool) {
	if isvc == nil || isvc.Spec.Decoder == nil {
		return "", false
	}

	return deploymentModeFromAnnotations(isvc.Spec.Decoder.Annotations)
}

func getRuntimeComponentDeploymentMode(runtime *v1beta1.ServingRuntimeSpec, componentType v1beta1.ComponentType) (constants.DeploymentModeType, bool) {
	if runtime == nil {
		return "", false
	}

	switch componentType {
	case v1beta1.EngineComponent:
		if runtime.EngineConfig == nil {
			return "", false
		}
		return deploymentModeFromAnnotations(runtime.EngineConfig.Annotations)
	case v1beta1.DecoderComponent:
		if runtime.DecoderConfig == nil {
			return "", false
		}
		return deploymentModeFromAnnotations(runtime.DecoderConfig.Annotations)
	default:
		return "", false
	}
}

func deploymentModeFromAnnotations(annotations map[string]string) (constants.DeploymentModeType, bool) {
	if annotations == nil {
		return "", false
	}
	if mode, exists := annotations[constants.DeploymentMode]; exists {
		deploymentMode := constants.DeploymentModeType(mode)
		if deploymentMode.IsValid() {
			return deploymentMode, true
		}
	}
	return "", false
}

// compareSupportedModelFormats checks if a model matches a supported format.
