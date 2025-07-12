package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	omev1beta1 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// NewK8sClient creates a new Kubernetes client using controller-runtime
// This follows the pattern used by other ome-agent components
func NewK8sClient() (client.Client, error) {
	// Create a new scheme with all required API types
	scheme := runtime.NewScheme()

	// Add standard Kubernetes types
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add client-go scheme: %w", err)
	}

	// Add OME custom resource types
	if err := omev1beta1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add OME v1beta1 scheme: %w", err)
	}

	// Get the Kubernetes config (in-cluster or from kubeconfig)
	config := ctrl.GetConfigOrDie()

	// Create the client with the scheme
	k8sClient, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return k8sClient, nil
}
