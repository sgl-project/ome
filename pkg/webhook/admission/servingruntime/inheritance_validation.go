package servingruntime

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

// validateProfileMarker enforces runtime-profile="true" ⇒ disabled=true.
func validateProfileMarker(annotations map[string]string, disabled *bool) error {
	if annotations[constants.RuntimeProfileAnnotationKey] != "true" {
		return nil
	}
	if disabled == nil || !*disabled {
		return fmt.Errorf("runtimes carrying %s=\"true\" must also set spec.disabled: true", constants.RuntimeProfileAnnotationKey)
	}
	return nil
}

func validateClusterRuntimeInheritance(ctx context.Context, c client.Reader, csr *v1beta1.ClusterServingRuntime) error {
	parentName := csr.Annotations[constants.RuntimeInheritFromAnnotationKey]
	if parentName == "" {
		return nil
	}
	fetch := clusterFetcher(c)
	start := &runtimeinheritance.RuntimeRef{
		Name:       csr.Name,
		Spec:       &csr.Spec,
		ParentName: parentName,
	}
	_, _, err := runtimeinheritance.Resolve(ctx, start, fetch, constants.RuntimeInheritMaxDepth)
	return err
}

func validateNamespacedRuntimeInheritance(ctx context.Context, c client.Reader, sr *v1beta1.ServingRuntime) error {
	parentName := sr.Annotations[constants.RuntimeInheritFromAnnotationKey]
	if parentName == "" {
		return nil
	}
	fetch := namespacedFetcher(c, sr.Namespace)
	start := &runtimeinheritance.RuntimeRef{
		Name:       sr.Name,
		Spec:       &sr.Spec,
		ParentName: parentName,
	}
	_, _, err := runtimeinheritance.Resolve(ctx, start, fetch, constants.RuntimeInheritMaxDepth)
	return err
}

func clusterFetcher(c client.Reader) runtimeinheritance.Fetcher {
	return func(ctx context.Context, name string) (*runtimeinheritance.RuntimeRef, error) {
		csr := &v1beta1.ClusterServingRuntime{}
		if err := c.Get(ctx, types.NamespacedName{Name: name}, csr); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, errors.Join(runtimeinheritance.ErrParentNotFound, err)
			}
			return nil, err
		}
		return &runtimeinheritance.RuntimeRef{
			Name:       csr.Name,
			Spec:       &csr.Spec,
			ParentName: csr.Annotations[constants.RuntimeInheritFromAnnotationKey],
		}, nil
	}
}

func namespacedFetcher(c client.Reader, namespace string) runtimeinheritance.Fetcher {
	return func(ctx context.Context, name string) (*runtimeinheritance.RuntimeRef, error) {
		sr := &v1beta1.ServingRuntime{}
		err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sr)
		if err == nil {
			return &runtimeinheritance.RuntimeRef{
				Name:       sr.Name,
				Spec:       &sr.Spec,
				ParentName: sr.Annotations[constants.RuntimeInheritFromAnnotationKey],
			}, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		csr := &v1beta1.ClusterServingRuntime{}
		if err := c.Get(ctx, types.NamespacedName{Name: name}, csr); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, errors.Join(runtimeinheritance.ErrParentNotFound, err)
			}
			return nil, err
		}
		return &runtimeinheritance.RuntimeRef{
			Name:       csr.Name,
			Spec:       &csr.Spec,
			ParentName: csr.Annotations[constants.RuntimeInheritFromAnnotationKey],
		}, nil
	}
}
