package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ClusterServingRuntime GVR
	ClusterServingRuntimeGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "clusterservingruntimes",
	}
)

// ListClusterServingRuntimes returns all ClusterServingRuntimes in the cluster
func (c *Client) ListClusterServingRuntimes(ctx context.Context) (*unstructured.UnstructuredList, error) {
	return c.DynamicClient.Resource(ClusterServingRuntimeGVR).List(ctx, metav1.ListOptions{})
}

// GetClusterServingRuntime returns a specific ClusterServingRuntime by name
func (c *Client) GetClusterServingRuntime(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterServingRuntimeGVR).Get(ctx, name, metav1.GetOptions{})
}

// CreateClusterServingRuntime creates a new ClusterServingRuntime
func (c *Client) CreateClusterServingRuntime(ctx context.Context, runtime *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterServingRuntimeGVR).Create(ctx, runtime, metav1.CreateOptions{})
}

// UpdateClusterServingRuntime updates an existing ClusterServingRuntime
func (c *Client) UpdateClusterServingRuntime(ctx context.Context, runtime *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterServingRuntimeGVR).Update(ctx, runtime, metav1.UpdateOptions{})
}

// DeleteClusterServingRuntime deletes a ClusterServingRuntime by name
func (c *Client) DeleteClusterServingRuntime(ctx context.Context, name string) error {
	return c.DynamicClient.Resource(ClusterServingRuntimeGVR).Delete(ctx, name, metav1.DeleteOptions{})
}
