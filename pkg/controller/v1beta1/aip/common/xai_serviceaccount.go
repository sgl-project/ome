package common

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// XAIServiceAccount implements ProjectScoped and ResourceOperation
type XAIServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
}

// NewXAIServiceAccount creates a new ServiceAccount resource handler
func NewXAIServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *XAIServiceAccount {
	return &XAIServiceAccount{
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
func (sa *XAIServiceAccount) Create(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete implements ResourceOperation
func (sa *XAIServiceAccount) Delete(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Update implements ServiceAccountOperation
func (sa *XAIServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}
