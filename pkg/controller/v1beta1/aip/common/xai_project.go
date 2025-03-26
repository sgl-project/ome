package common

import (
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// XAIProject implements OrganizationScoped, ResourceOperation
type XAIProject struct {
	ResourceBase
	Resource *v1beta1.Project
}

// NewXAIProject creates a new Project resource handler
func NewXAIProject(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) *XAIProject {
	return &XAIProject{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: project,
	}
}

// Create creates a new project
func (p *XAIProject) Create(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Update updates the existing project
func (p *XAIProject) Update(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete delelts the existing project
func (p *XAIProject) Delete(ctx context.Context) error {
	// TODO: implementation
	return nil
}
