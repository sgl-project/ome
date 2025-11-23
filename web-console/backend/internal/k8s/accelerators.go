package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// AcceleratorClass GVR
	AcceleratorClassGVR = schema.GroupVersionResource{
		Group:    "ome.io",
		Version:  "v1beta1",
		Resource: "acceleratorclasses",
	}
)

// ListAcceleratorClasses returns all AcceleratorClasses in the cluster
func (c *Client) ListAcceleratorClasses(ctx context.Context) (*unstructured.UnstructuredList, error) {
	return c.DynamicClient.Resource(AcceleratorClassGVR).List(ctx, metav1.ListOptions{})
}

// GetAcceleratorClass returns a specific AcceleratorClass by name
func (c *Client) GetAcceleratorClass(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.DynamicClient.Resource(AcceleratorClassGVR).Get(ctx, name, metav1.GetOptions{})
}
