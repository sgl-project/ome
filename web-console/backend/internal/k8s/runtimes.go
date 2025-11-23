package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ClusterServingRuntime GVR
	ClusterServingRuntimeGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "clusterservingruntimes",
	}

	// ServingRuntime GVR (namespace-scoped)
	ServingRuntimeGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "servingruntimes",
	}
)

// ListClusterServingRuntimes returns all ClusterServingRuntimes in the cluster from cache
func (c *Client) ListClusterServingRuntimes(ctx context.Context) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ClusterServingRuntimeGVR).Lister()
	objs, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	// Convert to UnstructuredList
	items := make([]unstructured.Unstructured, len(objs))
	for i, obj := range objs {
		items[i] = *obj.(*unstructured.Unstructured)
	}

	return &unstructured.UnstructuredList{
		Items: items,
	}, nil
}

// GetClusterServingRuntime returns a specific ClusterServingRuntime by name from cache
func (c *Client) GetClusterServingRuntime(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ClusterServingRuntimeGVR).Lister()
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
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

// ListServingRuntimes returns all ServingRuntimes in a namespace from cache
func (c *Client) ListServingRuntimes(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ServingRuntimeGVR).Lister().ByNamespace(namespace)
	objs, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	// Convert to UnstructuredList
	items := make([]unstructured.Unstructured, len(objs))
	for i, obj := range objs {
		items[i] = *obj.(*unstructured.Unstructured)
	}

	return &unstructured.UnstructuredList{
		Items: items,
	}, nil
}

// GetServingRuntime returns a specific ServingRuntime by name and namespace from cache
func (c *Client) GetServingRuntime(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ServingRuntimeGVR).Lister().ByNamespace(namespace)
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}

// CreateServingRuntime creates a new ServingRuntime in a namespace
func (c *Client) CreateServingRuntime(ctx context.Context, namespace string, runtime *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ServingRuntimeGVR).Namespace(namespace).Create(ctx, runtime, metav1.CreateOptions{})
}

// UpdateServingRuntime updates an existing ServingRuntime in a namespace
func (c *Client) UpdateServingRuntime(ctx context.Context, namespace string, runtime *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ServingRuntimeGVR).Namespace(namespace).Update(ctx, runtime, metav1.UpdateOptions{})
}

// DeleteServingRuntime deletes a ServingRuntime by name and namespace
func (c *Client) DeleteServingRuntime(ctx context.Context, namespace, name string) error {
	return c.DynamicClient.Resource(ServingRuntimeGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
