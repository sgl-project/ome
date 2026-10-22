package runtimeinheritance

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// ClusterRuntimeRef is the resolver's view of a ClusterServingRuntime: its
// name, its spec and the parent its inherit-from annotation names.
func ClusterRuntimeRef(csr *v1beta1.ClusterServingRuntime) *RuntimeRef {
	return &RuntimeRef{
		Name:       csr.Name,
		Spec:       &csr.Spec,
		ParentName: csr.Annotations[constants.RuntimeInheritFromAnnotationKey],
	}
}

// NamespacedRuntimeRef is the resolver's view of a ServingRuntime.
func NamespacedRuntimeRef(sr *v1beta1.ServingRuntime) *RuntimeRef {
	return &RuntimeRef{
		Name:       sr.Name,
		Spec:       &sr.Spec,
		ParentName: sr.Annotations[constants.RuntimeInheritFromAnnotationKey],
	}
}

// ClusterRuntimeFetcher looks up the parents of a ClusterServingRuntime
// through c. A cluster-scoped runtime inherits from cluster scope only.
func ClusterRuntimeFetcher(c client.Reader) Fetcher {
	return func(ctx context.Context, name string) (*RuntimeRef, error) {
		return fetchClusterRuntime(ctx, c, name)
	}
}

// NamespacedRuntimeFetcher looks up the parents of a ServingRuntime in
// namespace through c. The ServingRuntime of the parent's name in that
// namespace is the parent when it exists; otherwise the
// ClusterServingRuntime of that name is. A runtime in another namespace is
// never a parent, so it reads as not found.
func NamespacedRuntimeFetcher(c client.Reader, namespace string) Fetcher {
	return func(ctx context.Context, name string) (*RuntimeRef, error) {
		sr := &v1beta1.ServingRuntime{}
		err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sr)
		if err == nil {
			return NamespacedRuntimeRef(sr), nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		return fetchClusterRuntime(ctx, c, name)
	}
}

// fetchClusterRuntime reads one ClusterServingRuntime. A missing runtime
// reads as ErrParentNotFound; any other read error passes through as it is.
func fetchClusterRuntime(ctx context.Context, c client.Reader, name string) (*RuntimeRef, error) {
	csr := &v1beta1.ClusterServingRuntime{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, csr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.Join(ErrParentNotFound, err)
		}
		return nil, err
	}
	return ClusterRuntimeRef(csr), nil
}
