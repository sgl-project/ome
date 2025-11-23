package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ClusterBaseModel GVR (cluster-scoped)
	ClusterBaseModelGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "clusterbasemodels",
	}

	// BaseModel GVR (namespace-scoped)
	BaseModelGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "basemodels",
	}
)

// ListClusterBaseModels returns all ClusterBaseModels in the cluster
func (c *Client) ListClusterBaseModels(ctx context.Context) (*unstructured.UnstructuredList, error) {
	return c.DynamicClient.Resource(ClusterBaseModelGVR).List(ctx, metav1.ListOptions{})
}

// GetClusterBaseModel returns a specific ClusterBaseModel by name
func (c *Client) GetClusterBaseModel(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterBaseModelGVR).Get(ctx, name, metav1.GetOptions{})
}

// CreateClusterBaseModel creates a new ClusterBaseModel
func (c *Client) CreateClusterBaseModel(ctx context.Context, model *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterBaseModelGVR).Create(ctx, model, metav1.CreateOptions{})
}

// UpdateClusterBaseModel updates an existing ClusterBaseModel
func (c *Client) UpdateClusterBaseModel(ctx context.Context, model *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(ClusterBaseModelGVR).Update(ctx, model, metav1.UpdateOptions{})
}

// DeleteClusterBaseModel deletes a ClusterBaseModel by name
func (c *Client) DeleteClusterBaseModel(ctx context.Context, name string) error {
	return c.DynamicClient.Resource(ClusterBaseModelGVR).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListBaseModels returns all BaseModels in a namespace
func (c *Client) ListBaseModels(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	return c.DynamicClient.Resource(BaseModelGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetBaseModel returns a specific BaseModel by name and namespace
func (c *Client) GetBaseModel(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(BaseModelGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// CreateBaseModel creates a new BaseModel in a namespace
func (c *Client) CreateBaseModel(ctx context.Context, namespace string, model *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(BaseModelGVR).Namespace(namespace).Create(ctx, model, metav1.CreateOptions{})
}

// UpdateBaseModel updates an existing BaseModel in a namespace
func (c *Client) UpdateBaseModel(ctx context.Context, namespace string, model *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(BaseModelGVR).Namespace(namespace).Update(ctx, model, metav1.UpdateOptions{})
}

// DeleteBaseModel deletes a BaseModel by name and namespace
func (c *Client) DeleteBaseModel(ctx context.Context, namespace, name string) error {
	return c.DynamicClient.Resource(BaseModelGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
