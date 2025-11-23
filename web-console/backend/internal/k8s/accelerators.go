package k8s

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

// ListAcceleratorClasses returns all AcceleratorClasses in the cluster from cache
func (c *Client) ListAcceleratorClasses(ctx context.Context) (*unstructured.UnstructuredList, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(AcceleratorClassGVR).Lister()
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

// GetAcceleratorClass returns a specific AcceleratorClass by name from cache
func (c *Client) GetAcceleratorClass(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	// Use lister instead of direct API call
	lister := c.DynamicInformerFactory.ForResource(AcceleratorClassGVR).Lister()
	obj, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}
