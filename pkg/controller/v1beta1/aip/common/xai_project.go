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
	creationTime := v1.NewTime(time.Now())
	p.Resource.Status.ProjectId = GenerateId("proj_", p.Resource.UID)
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
}

// Update updates the existing project
func (p *XAIProject) Update(ctx context.Context) error {
	// TODO: implementation
	return nil
}

// Delete deletes the existing project
func (p *XAIProject) Delete(ctx context.Context) error {
	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
}
