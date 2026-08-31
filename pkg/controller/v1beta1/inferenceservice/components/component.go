package components

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Component can be reconciled to create underlying resources for an InferenceService
type Component interface {
	Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) (ctrl.Result, error)
}

// ComponentConfig defines the interface for component-specific configuration
type ComponentConfig interface {
	// GetComponentType returns the component type (engine, decoder, etc.)
	GetComponentType() v1beta1.ComponentType

	// GetComponentSpec returns the component extension spec
	GetComponentSpec() *v1beta1.ComponentExtensionSpec

	// GetServiceSuffix returns the suffix for the service name
	GetServiceSuffix() string

	// ValidateSpec validates the component spec
	ValidateSpec() error
}
