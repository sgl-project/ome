package common

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	// Mock implementation without using GCP resource management client
	projectName := p.Resource.Spec.Name

	creationTime := v1.NewTime(time.Now())
	p.Resource.Status.ProjectId = projectName
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
}

// Update updates the existing project
func (p *GeminiProject) Update(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete deletes the existing project
func (p *GeminiProject) Delete(ctx context.Context) error {
	// TODO: implementation with GCP management client
	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
}
