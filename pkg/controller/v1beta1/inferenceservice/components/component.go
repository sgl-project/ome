package components

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// Component can be reconciled to create underlying resources for an InferenceService
type Component interface {
	Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error)
}

// ComponentType constants for different component types
const (
	ComponentTypePredictor = "predictor"
	ComponentTypeEngine    = "engine"
	ComponentTypeDecoder   = "decoder"
)

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

// PodSpecProvider defines the interface for providing pod specifications
type PodSpecProvider interface {
	// GetPodSpec returns the pod spec for the component
	GetPodSpec() *v1beta1.PodSpec

	// GetRunnerSpec returns the runner spec if available
	GetRunnerSpec() *v1beta1.RunnerSpec

	// GetLeaderSpec returns the leader spec for multi-node deployments
	GetLeaderSpec() (*v1beta1.PodSpec, *v1beta1.RunnerSpec)

	// GetWorkerSpec returns the worker spec for multi-node deployments
	GetWorkerSpec() (*v1beta1.PodSpec, *v1beta1.RunnerSpec, *int)
}

// ComponentReconciler defines the full interface for a component reconciler
type ComponentReconciler interface {
	Component
	ComponentConfig
	PodSpecProvider
}

// DeploymentStrategy defines deployment-specific operations
type DeploymentStrategy interface {
	// SupportsDeploymentMode checks if the component supports a deployment mode
	SupportsDeploymentMode(mode constants.DeploymentModeType) bool

	// GetPreferredDeploymentMode returns the preferred deployment mode
	GetPreferredDeploymentMode() constants.DeploymentModeType
}

// StatusUpdater defines component-specific status update operations
type StatusUpdater interface {
	// UpdateComponentStatus updates the component-specific status
	UpdateComponentStatus(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta) error

	// GetPodLabelInfo returns pod label key and value for status tracking
	GetPodLabelInfo(rawDeployment bool, objectMeta metav1.ObjectMeta, statusSpec v1beta1.ComponentStatusSpec) (string, string)
}

// MetadataProvider defines operations for component metadata
type MetadataProvider interface {
	// ProcessAnnotations processes component-specific annotations
	ProcessAnnotations(isvc *v1beta1.InferenceService, existing map[string]string) (map[string]string, error)

	// ProcessLabels processes component-specific labels
	ProcessLabels(isvc *v1beta1.InferenceService, existing map[string]string) map[string]string

	// DetermineServiceName determines the service name for the component
	DetermineServiceName(isvc *v1beta1.InferenceService) (string, error)
}

// ReconcileHooks defines optional hooks for customizing reconciliation
type ReconcileHooks interface {
	// BeforeReconcile is called before main reconciliation logic
	BeforeReconcile(isvc *v1beta1.InferenceService) error

	// AfterReconcile is called after successful reconciliation
	AfterReconcile(isvc *v1beta1.InferenceService, result ctrl.Result) error

	// OnReconcileError is called when reconciliation fails
	OnReconcileError(isvc *v1beta1.InferenceService, err error) error
}

// ComponentFactory defines the interface for creating components
type ComponentFactory interface {
	// CreateComponent creates a component based on the spec
	CreateComponent(spec interface{}) (Component, error)

	// GetSupportedComponentTypes returns the component types this factory can create
	GetSupportedComponentTypes() []v1beta1.ComponentType
}
