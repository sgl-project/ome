package runtimeselector

import (
	"fmt"
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
		sb.WriteString(". Excluded runtimes: ")
		var excluded []string
		for name, reason := range e.ExcludedRuntimes {
			excluded = append(excluded, fmt.Sprintf("%s (%v)", name, reason))
		}
		sb.WriteString(strings.Join(excluded, "; "))
	}

	return sb.String()
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
