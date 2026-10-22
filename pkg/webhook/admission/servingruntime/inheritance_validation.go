package servingruntime

import (
	"context"
	"fmt"

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

// validateClusterRuntimeInheritance refuses a ClusterServingRuntime whose
// inherit-from chain does not resolve: a missing parent, a cycle or a chain
// deeper than the limit. A runtime with no parent passes.
func validateClusterRuntimeInheritance(ctx context.Context, c client.Reader, csr *v1beta1.ClusterServingRuntime) error {
	start := runtimeinheritance.ClusterRuntimeRef(csr)
	if start.ParentName == "" {
		return nil
	}
	_, _, err := runtimeinheritance.Resolve(ctx, start, runtimeinheritance.ClusterRuntimeFetcher(c), constants.RuntimeInheritMaxDepth)
	return err
}

// validateNamespacedRuntimeInheritance is validateClusterRuntimeInheritance
// for a ServingRuntime, whose parents are read from its own namespace first
// and from cluster scope second.
func validateNamespacedRuntimeInheritance(ctx context.Context, c client.Reader, sr *v1beta1.ServingRuntime) error {
	start := runtimeinheritance.NamespacedRuntimeRef(sr)
	if start.ParentName == "" {
		return nil
	}
	_, _, err := runtimeinheritance.Resolve(ctx, start, runtimeinheritance.NamespacedRuntimeFetcher(c, sr.Namespace), constants.RuntimeInheritMaxDepth)
	return err
}
