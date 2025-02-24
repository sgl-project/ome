package common

import (
	"context"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Project implements OrganizationScoped and ResourceOperation
type Project struct {
	ResourceBase
	Resource *v1beta1.Project
}

// NewProject creates a new Project resource handler
func NewProject(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) *Project {
	return &Project{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: project,
	}
}

// GetOrganizationRef implements OrganizationScoped
func (p *Project) GetOrganizationRef() *v1beta1.CrossReference {
	return &p.Resource.Spec.OrganizationRef
}

// GetOrganization fetches the organization for the project
func (p *Project) GetOrganization(ctx context.Context) (*v1beta1.Organization, error) {
	org := &v1beta1.Organization{}
	if err := p.Client.Get(ctx, client.ObjectKey{Name: p.Resource.Spec.OrganizationRef.Name}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// updateProjectCondition adds or updates a condition in the project status
func (p *Project) updateProjectCondition(ctx context.Context, conditionType, reason, message string, status v1.ConditionStatus) error {
	condition := v1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: v1.NewTime(time.Now()),
		ObservedGeneration: p.Resource.Generation,
	}
	p.Resource.Status.Conditions = append(p.Resource.Status.Conditions, condition)
	return p.Client.Status().Update(ctx, p.Resource)
}

// Create creates a new project in OpenAI
func (p *Project) Create(ctx context.Context) error {
	// Fetch the organization
	org, err := p.GetOrganization(ctx)
	if err != nil {
		return p.updateProjectCondition(ctx, "Error", "FailedToGetOrganization", err.Error(), v1.ConditionFalse)
	}

	// Initialize OpenAI client
	openaiClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return p.updateProjectCondition(ctx, "Error", "FailedToInitializeClient", err.Error(), v1.ConditionFalse)
	}

	// Create project in OpenAI
	resp, err := openaiClient.Projects.Create(ctx, openaisdk.ProjectCreateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateProjectCondition(ctx, "Error", "FailedToCreateProject", err.Error(), v1.ConditionFalse)
	}

	// Update project status
	creationTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.ProjectID = resp.ID
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateProjectCondition(ctx, "Ready", "ProjectCreated", "Project successfully created", v1.ConditionTrue)
}

// Update updates the project details in OpenAI
func (p *Project) Update(ctx context.Context) error {
	// Fetch organization
	org, err := p.GetOrganization(ctx)
	if err != nil {
		return err
	}

	// Initialize OpenAI client
	openaiClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return p.updateProjectCondition(ctx, "Error", "Failed to initialize client", err.Error(), v1.ConditionFalse)
	}

	// Update project in OpenAI
	resp, err := openaiClient.Projects.Update(ctx, p.Resource.Status.ProjectID, openaisdk.ProjectUpdateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateProjectCondition(ctx, "Error", "Failed to update project", err.Error(), v1.ConditionFalse)
	}

	// Update status
	updateTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.LastUpdatedTime = &updateTime

	return p.updateProjectCondition(ctx, "Ready", "ProjectUpdated", "Project successfully updated", v1.ConditionTrue)
}

// GetProject fetches the project details from OpenAI
func (p *Project) GetProject(ctx context.Context) (*openaisdk.Project, error) {
	// Fetch organization
	org, err := p.GetOrganization(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize OpenAI client
	openaiClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return nil, err
	}

	// Get project from OpenAI
	project, err := openaiClient.Projects.Get(ctx, p.Resource.Status.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// Delete archives the project in OpenAI and deletes associated service accounts
func (p *Project) Delete(ctx context.Context) error {
	// Fetch organization
	org, err := p.GetOrganization(ctx)
	if err != nil {
		return err
	}

	// Initialize OpenAI client
	openaiClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return err
	}

	// Archive project in OpenAI
	if _, err := openaiClient.Projects.Archive(ctx, p.Resource.Status.ProjectID); err != nil {
		return fmt.Errorf("failed to archive project: %w", err)
	}

	return nil
}
