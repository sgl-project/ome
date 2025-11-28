package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

// ListClusterBaseModels returns all ClusterBaseModels in the cluster from cache
func (c *Client) ListClusterBaseModels(ctx context.Context) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ClusterBaseModelGVR).Lister()
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

// GetClusterBaseModel returns a specific ClusterBaseModel by name from cache
func (c *Client) GetClusterBaseModel(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(ClusterBaseModelGVR).Lister()
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
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

// ListBaseModels returns all BaseModels in a namespace from cache
func (c *Client) ListBaseModels(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(BaseModelGVR).Lister().ByNamespace(namespace)
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

// GetBaseModel returns a specific BaseModel by name and namespace from cache
func (c *Client) GetBaseModel(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(BaseModelGVR).Lister().ByNamespace(namespace)
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
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

// GetClusterBaseModelEvents returns K8s events for a ClusterBaseModel
// Since ClusterBaseModel is cluster-scoped, events are stored in the "default" namespace
func (c *Client) GetClusterBaseModelEvents(ctx context.Context, name string) (*corev1.EventList, error) {
	// For cluster-scoped resources, K8s stores events in the "default" namespace
	// The events are linked via involvedObject with kind=ClusterBaseModel
	fieldSelector := "involvedObject.kind=ClusterBaseModel,involvedObject.name=" + name
	return c.Clientset.CoreV1().Events("default").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
}

// GetBaseModelEvents returns K8s events for a namespace-scoped BaseModel
func (c *Client) GetBaseModelEvents(ctx context.Context, namespace, name string) (*corev1.EventList, error) {
	fieldSelector := "involvedObject.kind=BaseModel,involvedObject.name=" + name
	return c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
}
