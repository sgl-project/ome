package acceleratorclassselector

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// DefaultAcceleratorFetcher is the default implementation of AcceleratorFetcher.
type DefaultAcceleratorFetcher struct {
	client client.Client
}

// NewDefaultAcceleratorFetcher creates a new DefaultAcceleratorFetcher.
func NewDefaultAcceleratorFetcher(client client.Client) AcceleratorFetcher {
	return &DefaultAcceleratorFetcher{
		client: client,
	}
}

// FetchAcceleratorClasses returns both namespace and cluster scoped accelerator classes.
func (f *DefaultAcceleratorFetcher) FetchAcceleratorClasses(ctx context.Context) (*AcceleratorCollection, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Fetching accelerator classes")

	collection := &AcceleratorCollection{}

	// Fetch cluster-scoped AcceleratorClasses
	var clusterAcceleratorClasses v1beta1.AcceleratorClassList
	if err := f.client.List(ctx, &clusterAcceleratorClasses); err != nil {
		return nil, fmt.Errorf("failed to list cluster-scoped accelerator classes: %w", err)
	}
	collection.ClusterAcceleratorClasses = clusterAcceleratorClasses.Items

	logger.V(1).Info("Fetched accelerator classes",
		"clusterAcceleratorClasses", len(collection.ClusterAcceleratorClasses))

	return collection, nil
}

// GetAcceleratorClass fetches a specific accelerator class by name.
// It first checks namespace-scoped accelerator classes, then cluster-scoped ones.
func (f *DefaultAcceleratorFetcher) GetAcceleratorClass(ctx context.Context, name string) (*v1beta1.AcceleratorClassSpec, bool, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Getting accelerator class", "name", name)

	// If not found in namespace, try cluster-scoped AcceleratorClass
	var clusterAcceleratorClass v1beta1.AcceleratorClass
	acceleratorClassName := client.ObjectKey{Name: name}
	if err := f.client.Get(ctx, acceleratorClassName, &clusterAcceleratorClass); err == nil {
		logger.V(1).Info("Found cluster-scoped accelerator class", "name", name)
		return &clusterAcceleratorClass.Spec, true, nil
	}

	// Not found in either scope
	return nil, false, &AcceleratorNotFoundError{
		AcceleratorClassName: name,
	}
}
