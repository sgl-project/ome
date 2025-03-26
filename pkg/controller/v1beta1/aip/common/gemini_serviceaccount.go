package common

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// GeminiServiceAccount implements ProjectScoped and ResourceOperation
type GeminiServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
}

// NewGeminiServiceAccount creates a new ServiceAccount resource handler
func NewGeminiServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *GeminiServiceAccount {
	return &GeminiServiceAccount{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: sa,
	}
}

// Create implements ResourceOperation
func (sa *GeminiServiceAccount) Create(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete implements ResourceOperation
func (sa *GeminiServiceAccount) Delete(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Update implements ServiceAccountOperation
func (sa *GeminiServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}
