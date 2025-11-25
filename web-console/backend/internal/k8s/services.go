package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// InferenceService GVR
	InferenceServiceGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "inferenceservices",
	}
)

// ListInferenceServices returns all InferenceServices in the specified namespace from cache
// If namespace is empty, lists across all namespaces
func (c *Client) ListInferenceServices(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	var objs []runtime.Object
	var err error

	if namespace == "" {
		// List across all namespaces
		lister := c.DynamicInformerFactory.ForResource(InferenceServiceGVR).Lister()
		objs, err = lister.List(labels.Everything())
	} else {
		// List in specific namespace
		lister := c.DynamicInformerFactory.ForResource(InferenceServiceGVR).Lister().ByNamespace(namespace)
		objs, err = lister.List(labels.Everything())
	}

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

// GetInferenceService returns a specific InferenceService by name and namespace from cache
func (c *Client) GetInferenceService(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(InferenceServiceGVR).Lister().ByNamespace(namespace)
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}

// CreateInferenceService creates a new InferenceService
func (c *Client) CreateInferenceService(ctx context.Context, namespace string, service *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Create(ctx, service, metav1.CreateOptions{})
}

// UpdateInferenceService updates an existing InferenceService
func (c *Client) UpdateInferenceService(ctx context.Context, namespace string, service *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Update(ctx, service, metav1.UpdateOptions{})
}

// DeleteInferenceService deletes an InferenceService by name and namespace
func (c *Client) DeleteInferenceService(ctx context.Context, namespace, name string) error {
	return c.DynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
