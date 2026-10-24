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

// ResolveClusterRuntime walks the inherit-from chain rooted at the
// named ClusterServingRuntime and returns the merged spec.
func ResolveClusterRuntime(ctx context.Context, c client.Reader, name string) (*v1beta1.ServingRuntimeSpec, []string, error) {
	start, err := fetchClusterRuntime(ctx, c, name)
	if err != nil {
		return nil, nil, err
	}
	return Resolve(ctx, start, ClusterRuntimeFetcher(c), constants.RuntimeInheritMaxDepth)
}

// ResolveNamespacedRuntime walks the inherit-from chain rooted at the
// named namespaced ServingRuntime. Parents are looked up in the same
// namespace first, falling back to cluster scope.
func ResolveNamespacedRuntime(ctx context.Context, c client.Reader, namespace, name string) (*v1beta1.ServingRuntimeSpec, []string, error) {
	sr := &v1beta1.ServingRuntime{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sr); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, errors.Join(ErrParentNotFound, err)
		}
		return nil, nil, err
	}
	return Resolve(ctx, NamespacedRuntimeRef(sr), NamespacedRuntimeFetcher(c, namespace), constants.RuntimeInheritMaxDepth)
}
