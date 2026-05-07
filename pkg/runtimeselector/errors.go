package runtimeselector

import (
	"fmt"
	"sort"
	"strings"
)

// RuntimeCompatibilityError represents an error when a runtime doesn't support a model.
type RuntimeCompatibilityError struct {
	RuntimeName   string
	ModelName     string
	ModelFormat   string
	Reason        string
	DetailedError error
}

// Error implements the error interface.
func (e *RuntimeCompatibilityError) Error() string {
	if e.DetailedError != nil {
		return fmt.Sprintf("runtime %s does not support model %s: %s (%v)",
			e.RuntimeName, e.ModelName, e.Reason, e.DetailedError)
	}
	return fmt.Sprintf("runtime %s does not support model %s: %s",
		e.RuntimeName, e.ModelName, e.Reason)
}

// Unwrap returns the underlying error.
func (e *RuntimeCompatibilityError) Unwrap() error {
	return e.DetailedError
}

// NoRuntimeFoundError indicates that no compatible runtime was found for a model.
type NoRuntimeFoundError struct {
	ModelName          string
	ModelFormat        string
	Namespace          string
	ExcludedRuntimes   map[string]error // Map of runtime name to exclusion reason
	TotalRuntimes      int
	NamespacedRuntimes int
	ClusterRuntimes    int
	ClosestRuntime     string
	ClosestReason      string
}

// Error implements the error interface.
func (e *NoRuntimeFoundError) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("no runtime found to support model %s with format %s in namespace %s",
		e.ModelName, e.ModelFormat, e.Namespace))

	if e.TotalRuntimes > 0 {
		sb.WriteString(fmt.Sprintf(". Checked %d runtimes (%d namespace-scoped, %d cluster-scoped)",
			e.TotalRuntimes, e.NamespacedRuntimes, e.ClusterRuntimes))
	}

	if len(e.ExcludedRuntimes) > 0 {
		sb.WriteString(". Excluded runtimes by reason: ")
		categoryMap := make(map[string][]string)
		for _, name := range sortedRuntimeNames(e.ExcludedRuntimes) {
			reason := e.ExcludedRuntimes[name]
			category := categorizeExclusionReason(reason.Error())
			categoryMap[category] = append(categoryMap[category], fmt.Sprintf("%s (%v)", name, reason))
		}
		sb.WriteString(formatCategorizedExclusions(categoryMap))
	}

	if e.ClosestRuntime != "" {
		sb.WriteString(fmt.Sprintf(". Closest match: %s", e.ClosestRuntime))
		if e.ClosestReason != "" {
			sb.WriteString(fmt.Sprintf(" (excluded because %s)", e.ClosestReason))
		}
	}

	return sb.String()
}

func sortedRuntimeNames(excluded map[string]error) []string {
	names := make([]string, 0, len(excluded))
	for name := range excluded {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func formatCategorizedExclusions(categoryMap map[string][]string) string {
	categoryOrder := []string{
		"accelerator mismatch",
		"format mismatch",
		"size mismatch",
		"disabled",
		"other",
	}

	var groups []string
	for _, category := range categoryOrder {
		exclusions := categoryMap[category]
		if len(exclusions) == 0 {
			continue
		}
		sort.Strings(exclusions)
		groups = append(groups, fmt.Sprintf("%s: %s", category, strings.Join(exclusions, "; ")))
	}
	return strings.Join(groups, "; ")
}

func categorizeExclusionReason(reason string) string {
	lowerReason := strings.ToLower(reason)
	switch {
	case strings.Contains(lowerReason, "accelerator"):
		return "accelerator mismatch"
	case strings.Contains(lowerReason, "format") ||
		strings.Contains(lowerReason, "architecture") ||
		strings.Contains(lowerReason, "quantization") ||
		strings.Contains(lowerReason, "framework") ||
		strings.Contains(lowerReason, "pipeline"):
		return "format mismatch"
	case strings.Contains(lowerReason, "model size"):
		return "size mismatch"
	case strings.Contains(lowerReason, "disabled"):
		return "disabled"
	default:
		return "other"
	}
}

// RuntimeNotFoundError indicates that a specified runtime doesn't exist.
type RuntimeNotFoundError struct {
	RuntimeName string
	Namespace   string
}

// Error implements the error interface.
func (e *RuntimeNotFoundError) Error() string {
	return fmt.Sprintf("runtime %s not found in namespace %s or at cluster scope",
		e.RuntimeName, e.Namespace)
}

// RuntimeDisabledError indicates that a runtime is disabled.
type RuntimeDisabledError struct {
	RuntimeName string
	IsCluster   bool
}

// Error implements the error interface.
func (e *RuntimeDisabledError) Error() string {
	scope := "namespace-scoped"
	if e.IsCluster {
		scope = "cluster-scoped"
	}
	return fmt.Sprintf("%s runtime %s is disabled", scope, e.RuntimeName)
}

// ModelValidationError indicates that the model specification is invalid.
type ModelValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ModelValidationError) Error() string {
	return fmt.Sprintf("invalid model specification: %s: %s", e.Field, e.Message)
}

// ConfigurationError indicates a configuration problem.
type ConfigurationError struct {
	Component string
	Message   string
}

// Error implements the error interface.
func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in %s: %s", e.Component, e.Message)
}

// IsRuntimeCompatibilityError checks if an error is a RuntimeCompatibilityError.
func IsRuntimeCompatibilityError(err error) bool {
	_, ok := err.(*RuntimeCompatibilityError)
	return ok
}

// IsNoRuntimeFoundError checks if an error is a NoRuntimeFoundError.
func IsNoRuntimeFoundError(err error) bool {
	_, ok := err.(*NoRuntimeFoundError)
	return ok
}

// IsRuntimeNotFoundError checks if an error is a RuntimeNotFoundError.
func IsRuntimeNotFoundError(err error) bool {
	_, ok := err.(*RuntimeNotFoundError)
	return ok
}

// IsRuntimeDisabledError checks if an error is a RuntimeDisabledError.
func IsRuntimeDisabledError(err error) bool {
	_, ok := err.(*RuntimeDisabledError)
	return ok
}

// IsModelValidationError checks if an error is a ModelValidationError.
func IsModelValidationError(err error) bool {
	_, ok := err.(*ModelValidationError)
	return ok
}
