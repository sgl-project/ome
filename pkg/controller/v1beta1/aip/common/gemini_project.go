package common

import (
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GeminiProject implements OrganizationScoped, ResourceOperation
type GeminiProject struct {
	ResourceBase
	Resource *v1beta1.Project
}

// NewGeminiProject creates a new Project resource handler
func NewGeminiProject(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) *GeminiProject {
	return &GeminiProject{
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
func (p *GeminiProject) Create(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Update updates the existing project
func (p *GeminiProject) Update(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete delelts the existing project
func (p *GeminiProject) Delete(ctx context.Context) error {
	// TODO: implementation
	return nil
}
