// Package runtimecollection reads the bounded runtime snapshot used by the
// runtime inheritance graph.
package runtimecollection

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/paging"
	"sigs.k8s.io/ome/pkg/cli/runtimegraph"
	omeclient "sigs.k8s.io/ome/pkg/client/clientset/versioned/typed/ome/v1beta1"
)

// KindCompleteness describes the bounded observations for one runtime kind.
type KindCompleteness struct {
	ObservedPages int
	ObservedItems int
	Truncated     bool
}

// Completeness keeps collection state separate for both required kinds.
type Completeness struct {
	ClusterServingRuntimes KindCompleteness
	ServingRuntimes        KindCompleteness
}

// Result is a runtime graph snapshot and its collection completeness.
type Result struct {
	Snapshot     runtimegraph.Snapshot
	Completeness Completeness
}

// Collect reads all runtime namespaces through bounded Kubernetes pagination.
func Collect(ctx context.Context, client omeclient.OmeV1beta1Interface, limits paging.Limits) (Result, error) {
	result := Result{Snapshot: runtimegraph.Snapshot{
		ClusterServingRuntimes: []omev1beta1.ClusterServingRuntime{},
		ServingRuntimes:        []omev1beta1.ServingRuntime{},
	}}

	clusterPages := 0
	clusters, err := paging.ListBounded(ctx, metav1.ListOptions{}, limits,
		func(requestCtx context.Context, options metav1.ListOptions) (paging.Page[omev1beta1.ClusterServingRuntime], error) {
			list, listErr := client.ClusterServingRuntimes().List(requestCtx, options)
			if requestErr := requestCtx.Err(); requestErr != nil {
				return paging.Page[omev1beta1.ClusterServingRuntime]{}, requestErr
			}
			if listErr != nil {
				return paging.Page[omev1beta1.ClusterServingRuntime]{}, listErr
			}
			if list == nil {
				return paging.Page[omev1beta1.ClusterServingRuntime]{}, errors.New("empty response")
			}
			clusterPages++
			return paging.Page[omev1beta1.ClusterServingRuntime]{Items: list.Items, Continue: list.Continue}, nil
		})
	result.Snapshot.ClusterServingRuntimes = copyClusterServingRuntimes(clusters.Items)
	result.Completeness.ClusterServingRuntimes = KindCompleteness{
		ObservedPages: clusterPages,
		ObservedItems: len(clusters.Items),
		Truncated:     clusters.Truncated,
	}
	if err != nil {
		return result, fmt.Errorf("list ClusterServingRuntimes: %w", err)
	}

	runtimePages := 0
	runtimes, err := paging.ListBounded(ctx, metav1.ListOptions{}, limits,
		func(requestCtx context.Context, options metav1.ListOptions) (paging.Page[omev1beta1.ServingRuntime], error) {
			list, listErr := client.ServingRuntimes(metav1.NamespaceAll).List(requestCtx, options)
			if requestErr := requestCtx.Err(); requestErr != nil {
				return paging.Page[omev1beta1.ServingRuntime]{}, requestErr
			}
			if listErr != nil {
				return paging.Page[omev1beta1.ServingRuntime]{}, listErr
			}
			if list == nil {
				return paging.Page[omev1beta1.ServingRuntime]{}, errors.New("empty response")
			}
			runtimePages++
			return paging.Page[omev1beta1.ServingRuntime]{Items: list.Items, Continue: list.Continue}, nil
		})
	result.Snapshot.ServingRuntimes = copyServingRuntimes(runtimes.Items)
	result.Completeness.ServingRuntimes = KindCompleteness{
		ObservedPages: runtimePages,
		ObservedItems: len(runtimes.Items),
		Truncated:     runtimes.Truncated,
	}
	if err != nil {
		return result, fmt.Errorf("list ServingRuntimes: %w", err)
	}
	return result, nil
}

func copyClusterServingRuntimes(items []omev1beta1.ClusterServingRuntime) []omev1beta1.ClusterServingRuntime {
	result := make([]omev1beta1.ClusterServingRuntime, len(items))
	for i := range items {
		result[i] = *items[i].DeepCopy()
	}
	return result
}

func copyServingRuntimes(items []omev1beta1.ServingRuntime) []omev1beta1.ServingRuntime {
	result := make([]omev1beta1.ServingRuntime, len(items))
	for i := range items {
		result[i] = *items[i].DeepCopy()
	}
	return result
}
