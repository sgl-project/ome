package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ListNamespaces returns all namespaces in the cluster from cache
func (c *Client) ListNamespaces(ctx context.Context) (*corev1.NamespaceList, error) {
	// Use lister instead of direct API call
	lister := c.InformerFactory.Core().V1().Namespaces().Lister()
	namespaces, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	// Convert to NamespaceList
	return &corev1.NamespaceList{
		Items: func() []corev1.Namespace {
			items := make([]corev1.Namespace, len(namespaces))
			for i, ns := range namespaces {
				items[i] = *ns
			}
			return items
		}(),
	}, nil
}

// GetNamespace returns a specific namespace by name from cache
func (c *Client) GetNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	// Use lister instead of direct API call
	lister := c.InformerFactory.Core().V1().Namespaces().Lister()
	return lister.Get(name)
}
