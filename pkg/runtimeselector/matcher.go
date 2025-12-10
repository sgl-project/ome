package runtimeselector

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	modelVer "github.com/sgl-project/ome/pkg/modelver"
)

// DefaultRuntimeMatcher implements RuntimeMatcher with comprehensive compatibility checking.
type DefaultRuntimeMatcher struct {
	config *Config
}

// NewDefaultRuntimeMatcher creates a new DefaultRuntimeMatcher.
func NewDefaultRuntimeMatcher(config *Config) RuntimeMatcher {
	return &DefaultRuntimeMatcher{
		config: config,
	}
}

// IsCompatible checks if a runtime can serve a model.
func (m *DefaultRuntimeMatcher) IsCompatible(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (bool, error) {
	// Quick checks first
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
	// Check if any supported format matches
	for _, format := range runtime.SupportedModelFormats {
		if m.compareSupportedModelFormats(model, format) {
			// Found a matching format, now check model size if specified
			if err := m.checkModelSize(runtime, model, runtimeName); err == nil {
				return true, nil
			}
			// If model size check failed, continue checking other formats
		}
	}

	// No matching format found
	return false, nil
}

// GetCompatibilityDetails returns detailed compatibility information.
func (m *DefaultRuntimeMatcher) GetCompatibilityDetails(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, isvc *v1beta1.InferenceService, runtimeName string) (*CompatibilityReport, error) {
	ctx := context.Background()
	logger := log.FromContext(ctx)

	report := &CompatibilityReport{
		IsCompatible:           false,
		IncompatibilityReasons: []string{},
		Warnings:               []string{},
		MatchDetails:           MatchDetails{},
	}

	// Check if runtime is disabled
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

	// Check supported formats (mimics original RuntimeSupportsModel logic)
	formatSupported := false
	for _, format := range runtime.SupportedModelFormats {
		if m.compareSupportedModelFormats(model, format) {
			formatSupported = true
			match := m.evaluateFormatMatch(model, format)
			report.MatchDetails = match
			break
		}
	}

	if !formatSupported {
		report.IncompatibilityReasons = append(report.IncompatibilityReasons,
			fmt.Sprintf("model format '%s' not in supported formats", getModelFormatLabel(model)))
		return report, nil
	}

	// Check model size compatibility
	if model.ModelParameterSize != nil && runtime.ModelSizeRange != nil {
		modelSize := parseModelSize(*model.ModelParameterSize)
		minSize := parseModelSize(*runtime.ModelSizeRange.Min)
		maxSize := parseModelSize(*runtime.ModelSizeRange.Max)

		if modelSize < minSize || modelSize > maxSize {
			report.IncompatibilityReasons = append(report.IncompatibilityReasons,
				fmt.Sprintf("model size %s is outside supported range [%s, %s]",
					*model.ModelParameterSize, *runtime.ModelSizeRange.Min, *runtime.ModelSizeRange.Max))
			report.MatchDetails.SizeMatch = false
			return report, nil
		}
		report.MatchDetails.SizeMatch = true
	}

	// At this point, runtime supports the model
	report.IsCompatible = true

	// Check if runtime has auto-select enabled (this is separate from compatibility)
	hasAutoSelectFormat := false
	for _, format := range runtime.SupportedModelFormats {
		if format.AutoSelect != nil && *format.AutoSelect {
			hasAutoSelectFormat = true
			break
		}
	}

	if !hasAutoSelectFormat {
		// Note: This doesn't make it incompatible, but it's tracked separately
		report.Warnings = append(report.Warnings,
			"runtime does not have auto-select enabled for any supported format")
	}

	// Add other warnings
	if model.ModelParameterSize == nil && runtime.ModelSizeRange != nil {
		report.Warnings = append(report.Warnings,
			"model does not specify size, but runtime has size constraints")
	}

	logger.V(1).Info("Compatibility check completed",
		"runtime", runtimeName,
		"model", model.ModelFormat.Name,
		"compatible", report.IsCompatible,
		"reasons", len(report.IncompatibilityReasons))

	return report, nil
}

// evaluateFormatMatch evaluates how well a supported format matches a model.
func (m *DefaultRuntimeMatcher) evaluateFormatMatch(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat) MatchDetails {
	match := MatchDetails{
		FormatMatch:       false,
		FrameworkMatch:    false,
		ArchitectureMatch: true, // Default to true if not specified
		QuantizationMatch: true, // Default to true if not specified
		SizeMatch:         true, // Will be checked separately
		Priority:          m.config.DefaultPriority,
		Weight:            0,
		Reasons:           []string{},
	}

	// Set priority
	if format.Priority != nil {
		match.Priority = *format.Priority
	}

	// Set auto-select
	if format.AutoSelect != nil {
		match.AutoSelectEnabled = *format.AutoSelect
	}

	// Check architecture
	if model.ModelArchitecture != nil && format.ModelArchitecture != nil {
		match.ArchitectureMatch = *model.ModelArchitecture == *format.ModelArchitecture
		if !match.ArchitectureMatch {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("architecture mismatch: model=%s, runtime=%s",
					*model.ModelArchitecture, *format.ModelArchitecture))
		}
	} else if (model.ModelArchitecture == nil) != (format.ModelArchitecture == nil) {
		match.ArchitectureMatch = false
		match.Reasons = append(match.Reasons, "architecture requirement mismatch")
	}

	// Check quantization
	if model.Quantization != nil && format.Quantization != nil {
		match.QuantizationMatch = *model.Quantization == *format.Quantization
		if !match.QuantizationMatch {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("quantization mismatch: model=%s, runtime=%s",
					*model.Quantization, *format.Quantization))
		}
	} else if (model.Quantization == nil) != (format.Quantization == nil) {
		match.QuantizationMatch = false
		match.Reasons = append(match.Reasons, "quantization requirement mismatch")
	}

	// Check model format
	if format.ModelFormat != nil && &model.ModelFormat != nil {
		if format.ModelFormat.Name == model.ModelFormat.Name {
			// Check version compatibility if both are specified
			if format.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
				match.FormatMatch = m.compareModelFormatVersions(format.ModelFormat, &model.ModelFormat)
			} else if format.ModelFormat.Version == nil && model.ModelFormat.Version == nil {
				match.FormatMatch = true
			} else {
				// One has version, one doesn't - not a match
				match.FormatMatch = false
			}

			if match.FormatMatch && format.ModelFormat.Weight > 0 {
				match.Weight += format.ModelFormat.Weight * int64(match.Priority)
			}
		}
	}

	// Check model framework
	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name == model.ModelFramework.Name {
			// Check version compatibility if both are specified
			if format.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
				match.FrameworkMatch = m.compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework)
			} else if format.ModelFramework.Version == nil && model.ModelFramework.Version == nil {
				match.FrameworkMatch = true
			} else {
				// One has version, one doesn't - not a match
				match.FrameworkMatch = false
			}

			if match.FrameworkMatch && format.ModelFramework.Weight > 0 {
				match.Weight += format.ModelFramework.Weight * int64(match.Priority)
			}
		}
	} else if format.ModelFramework == nil && model.ModelFramework == nil {
		match.FrameworkMatch = true
	}

	return match
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

// compareSupportedModelFormats checks if a model matches a supported format.
func (m *DefaultRuntimeMatcher) compareSupportedModelFormats(model *v1beta1.BaseModelSpec, format v1beta1.SupportedModelFormat) bool {
	// Check architecture
	if model.ModelArchitecture != nil && format.ModelArchitecture != nil {
		if *model.ModelArchitecture != *format.ModelArchitecture {
			return false
		}
	} else if (model.ModelArchitecture == nil) != (format.ModelArchitecture == nil) {
		return false
	}

	// Check quantization
	if model.Quantization != nil && format.Quantization != nil {
		if *model.Quantization != *format.Quantization {
			return false
		}
	} else if (model.Quantization == nil) != (format.Quantization == nil) {
		return false
	}

	// Check model format
	if format.ModelFormat != nil && &model.ModelFormat != nil {
		if format.ModelFormat.Name != model.ModelFormat.Name {
			return false
		}
		if format.ModelFormat.Version != nil && model.ModelFormat.Version != nil {
			if !m.compareModelFormatVersions(format.ModelFormat, &model.ModelFormat) {
				return false
			}
		} else if (format.ModelFormat.Version == nil) != (model.ModelFormat.Version == nil) {
			return false
		}
	} else if (format.ModelFormat != nil) != (&model.ModelFormat != nil) {
		return false
	}

	// Check model framework
	if format.ModelFramework != nil && model.ModelFramework != nil {
		if format.ModelFramework.Name != model.ModelFramework.Name {
			return false
		}
		if format.ModelFramework.Version != nil && model.ModelFramework.Version != nil {
			if !m.compareModelFrameworkVersions(format.ModelFramework, model.ModelFramework) {
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

// compareModelFormatVersions compares model format versions based on operator.
func (m *DefaultRuntimeMatcher) compareModelFormatVersions(supportedFormat *v1beta1.ModelFormat, modelFormat *v1beta1.ModelFormat) bool {
	baseVersion, err := modelVer.Parse(*modelFormat.Version)
	if err != nil {
		return false
	}

	supportedVersion, err := modelVer.Parse(*supportedFormat.Version)
	if err != nil {
		return false
	}

	// Check for unofficial versions
	hasUnofficial := modelVer.ContainsUnofficialVersion(baseVersion) ||
		modelVer.ContainsUnofficialVersion(supportedVersion)

	operator := getRuntimeSelectorOperator(supportedFormat.Operator)

	// If there are unofficial versions or operator is Equal, use Equal comparison
	if hasUnofficial || operator == "Equal" {
		return modelVer.Equal(supportedVersion, baseVersion)
	}

	// Check for version precision mismatch and major version prefix mismatch
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

// compareModelFrameworkVersions compares model framework versions based on operator.
func (m *DefaultRuntimeMatcher) compareModelFrameworkVersions(supportedFramework *v1beta1.ModelFrameworkSpec, modelFramework *v1beta1.ModelFrameworkSpec) bool {
	baseVersion, err := modelVer.Parse(*modelFramework.Version)
	if err != nil {
		return false
	}

	supportedVersion, err := modelVer.Parse(*supportedFramework.Version)
	if err != nil {
		return false
	}

	// Check for unofficial versions
	hasUnofficial := modelVer.ContainsUnofficialVersion(baseVersion) ||
		modelVer.ContainsUnofficialVersion(supportedVersion)

	operator := getRuntimeSelectorOperator(supportedFramework.Operator)

	// If there are unofficial versions or operator is Equal, use Equal comparison
	if hasUnofficial || operator == "Equal" {
		return modelVer.Equal(supportedVersion, baseVersion)
	}

	// Check for version precision mismatch and major version prefix mismatch
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

// checkModelSize verifies if the model size is within the runtime's supported range.
func (m *DefaultRuntimeMatcher) checkModelSize(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) error {
	if model.ModelParameterSize == nil || runtime.ModelSizeRange == nil {
		return nil
	}

	modelSize := parseModelSize(*model.ModelParameterSize)
	minSize := parseModelSize(*runtime.ModelSizeRange.Min)
	maxSize := parseModelSize(*runtime.ModelSizeRange.Max)

	if modelSize < minSize || modelSize > maxSize {
		return &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   "", // Will be filled by caller if available
			ModelFormat: model.ModelFormat.Name,
			Reason: fmt.Sprintf("model size %s is outside supported range [%s, %s]",
				*model.ModelParameterSize, *runtime.ModelSizeRange.Min, *runtime.ModelSizeRange.Max),
		}
	}

	return nil
}

// Helper functions

// getRuntimeSelectorOperator returns a string representation of the RuntimeSelectorOperator.
func getRuntimeSelectorOperator(operator *v1beta1.RuntimeSelectorOperator) string {
	if operator == nil {
		return string(v1beta1.RuntimeSelectorOpEqual)
	}
	return string(*operator)
}

// parseModelSize converts a model size string (e.g., "7B", "13B", "70B") to a float64 value.
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

// getModelFormatLabel creates a standardized label string for model formats.
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
