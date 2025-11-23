package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

// ListNamespaces returns all namespaces in the cluster
func (c *Client) ListNamespaces(ctx context.Context) (*corev1.NamespaceList, error) {
	return c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
}

// GetNamespace returns a specific namespace by name
func (c *Client) GetNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	return c.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
}
