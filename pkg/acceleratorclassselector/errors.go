package acceleratorclassselector

import (
	"fmt"
)

// AcceleratorCompatibilityError represents an error when an accelerator class doesn't meet requirements.
type AcceleratorCompatibilityError struct {
	AcceleratorClassName string
	Component            string
	Reason               string
	DetailedError        error
}

// AcceleratorNotFoundError indicates that a specified accelerator class doesn't exist.
type AcceleratorNotFoundError struct {
	AcceleratorClassName string
}

// Error implements the error interface.
func (e *AcceleratorNotFoundError) Error() string {
	return fmt.Sprintf("accelerator class %s not found at cluster scope",
		e.AcceleratorClassName)
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

// IsAcceleratorNotFoundError checks if an error is an AcceleratorNotFoundError.
func IsAcceleratorNotFoundError(err error) bool {
	_, ok := err.(*AcceleratorNotFoundError)
	return ok
}
