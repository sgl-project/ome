package acceleratorclassselector

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Selector is the main interface for accelerator class selection.
// It provides methods to select, validate, and list compatible accelerator classes.
type Selector interface {
	// GetAcceleratorClass selects the best accelerator class for a given inference service, runtime, and component.
	// Returns the accelerator class spec, the accelerator class name, and an error if the fetch fails.
	GetAcceleratorClass(ctx context.Context, isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec, component v1beta1.ComponentType) (*v1beta1.AcceleratorClassSpec, string, error)
}

// AcceleratorSelection represents the selected accelerator class with metadata.
type AcceleratorSelection struct {
	// Name is the name of the selected accelerator class
	Name string

	// Spec is the accelerator class specification
	Spec *v1beta1.AcceleratorClassSpec

	// NodeSelector that should be applied to pods
	NodeSelector map[string]string

	// ResourceRequests that should be applied to pods
	ResourceRequests map[string]string
}

// AcceleratorFetcher abstracts the fetching of accelerator class resources.
type AcceleratorFetcher interface {
	// FetchAcceleratorClasses returns both namespace and cluster scoped accelerator classes.
	FetchAcceleratorClasses(ctx context.Context) (*AcceleratorCollection, error)

	// GetAcceleratorClass fetches a specific accelerator class by name.
	// It first checks namespace-scoped accelerator classes, then cluster-scoped ones.
	GetAcceleratorClass(ctx context.Context, name string) (*v1beta1.AcceleratorClassSpec, bool, error)
}

// AcceleratorCollection holds both namespace and cluster scoped accelerator classes.
type AcceleratorCollection struct {

	// ClusterAcceleratorClasses contains cluster-scoped AcceleratorClasses
	ClusterAcceleratorClasses []v1beta1.AcceleratorClass
}

// Config holds configuration for the accelerator class selector.
type Config struct {
	// Client is the Kubernetes client (uses controller-runtime cache)
	Client client.Client

	// EnableDetailedLogging enables verbose logging for debugging
	EnableDetailedLogging bool
}

// NewConfig creates a new Config with default values.
func NewConfig(client client.Client) *Config {
	return &Config{
		Client:                client,
		EnableDetailedLogging: false,
	}
}
