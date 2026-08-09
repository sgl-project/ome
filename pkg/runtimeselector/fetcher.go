package runtimeselector

import (
	"context"
	"sort"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

// DefaultRuntimeFetcher implements RuntimeFetcher using the controller-runtime client.
// It leverages the client's built-in caching for efficient runtime retrieval.
type DefaultRuntimeFetcher struct {
	client client.Client
}

func NewDefaultRuntimeFetcher(client client.Client) RuntimeFetcher {
	return &DefaultRuntimeFetcher{
		client: client,
	}
}

func (f *DefaultRuntimeFetcher) FetchRuntimes(ctx context.Context, namespace string) (*RuntimeCollection, error) {
	logger := log.FromContext(ctx)

	runtimes := &v1beta1.ServingRuntimeList{}
	if err := f.client.List(ctx, runtimes, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	clusterRuntimes := &v1beta1.ClusterServingRuntimeList{}
	if err := f.client.List(ctx, clusterRuntimes); err != nil {
		return nil, err
	}

	sortServingRuntimeList(runtimes)
	sortClusterServingRuntimeList(clusterRuntimes)

	logger.V(1).Info("Fetched runtimes",
		"namespace", namespace,
		"namespaceRuntimes", len(runtimes.Items),
		"clusterRuntimes", len(clusterRuntimes.Items))

	return &RuntimeCollection{
		NamespaceRuntimes: runtimes.Items,
		ClusterRuntimes:   clusterRuntimes.Items,
	}, nil
}

// GetRuntime resolves a runtime to its merged (inheritance-resolved) spec. Namespace-scoped
// takes precedence over cluster-scoped on name collision.
func (f *DefaultRuntimeFetcher) GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error) {
	if _, err := f.getNamespacedHead(ctx, namespace, name); err == nil {
		spec, _, resolveErr := runtimeinheritance.ResolveNamespacedRuntime(ctx, f.client, namespace, name)
		return spec, false, resolveErr
	} else if !errors.IsNotFound(err) {
		return nil, false, err
	}

	if _, err := f.getClusterHead(ctx, name); err == nil {
		spec, _, resolveErr := runtimeinheritance.ResolveClusterRuntime(ctx, f.client, name)
		return spec, true, resolveErr
	} else if errors.IsNotFound(err) {
		return nil, false, &RuntimeNotFoundError{
			RuntimeName: name,
			Namespace:   namespace,
		}
	} else {
		return nil, false, err
	}
}

func (f *DefaultRuntimeFetcher) getNamespacedHead(ctx context.Context, namespace, name string) (*v1beta1.ServingRuntime, error) {
	sr := &v1beta1.ServingRuntime{}
	if err := f.client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, sr); err != nil {
		return nil, err
	}
	return sr, nil
}

func (f *DefaultRuntimeFetcher) getClusterHead(ctx context.Context, name string) (*v1beta1.ClusterServingRuntime, error) {
	csr := &v1beta1.ClusterServingRuntime{}
	if err := f.client.Get(ctx, client.ObjectKey{Name: name}, csr); err != nil {
		return nil, err
	}
	return csr, nil
}

func sortServingRuntimeList(runtimes *v1beta1.ServingRuntimeList) {
	sortByCreationTimestampThenName(runtimes.Items, func(r *v1beta1.ServingRuntime) (metav1.Time, string) {
		return r.CreationTimestamp, r.Name
	})
}

func sortClusterServingRuntimeList(runtimes *v1beta1.ClusterServingRuntimeList) {
	sortByCreationTimestampThenName(runtimes.Items, func(r *v1beta1.ClusterServingRuntime) (metav1.Time, string) {
		return r.CreationTimestamp, r.Name
	})
}

// sortByCreationTimestampThenName sorts items in place: newest first, then
// alphabetical by name as a tiebreaker. Newest-first gives operators a way
// to roll out a replacement runtime by creating it side-by-side with the old
// one and letting auto-selection naturally prefer the fresh definition.
func sortByCreationTimestampThenName[T any](items []T, key func(*T) (metav1.Time, string)) {
	sort.Slice(items, func(i, j int) bool {
		ti, ni := key(&items[i])
		tj, nj := key(&items[j])
		if !ti.Equal(&tj) {
			return ti.After(tj.Time)
		}
		return ni < nj
	})
}
