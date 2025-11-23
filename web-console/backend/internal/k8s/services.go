package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// ListInferenceServices returns all InferenceServices in the specified namespace
// If namespace is empty, lists across all namespaces
func (c *Client) ListInferenceServices(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if namespace == "" {
		return c.DynamicClient.Resource(InferenceServiceGVR).List(ctx, metav1.ListOptions{})
	}
	return c.DynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// GetInferenceService returns a specific InferenceService by name and namespace
func (c *Client) GetInferenceService(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(InferenceServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
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
