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
		return nil, fmt.Errorf("failed to get organization %s: %w", p.Resource.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}

// initializeOpenAIClient initializes the OpenAI client with proper error handling
func (p *Project) initializeOpenAIClient(ctx context.Context) (*openaisdk.Client, error) {
	org, err := p.GetOrganization(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	openAIClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// updateCondition adds or updates a condition in the project status
func (p *Project) updateCondition(ctx context.Context, status v1beta1.ProjectStatusReason) error {
	now := v1.NewTime(time.Now())
	conditionType := v1beta1.ConditionTypeReady
	conditionStatus := v1.ConditionTrue

	if status.IsError() {
		conditionType = v1beta1.ConditionTypeError
		conditionStatus = v1.ConditionFalse
	}

	condition := v1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             string(status),
		Message:            p.getStatusMessage(status),
		LastTransitionTime: now,
		ObservedGeneration: p.Resource.Generation,
	}

	// Update or append the condition
	found := false
	for i, c := range p.Resource.Status.Conditions {
		if c.Type == conditionType {
			p.Resource.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		p.Resource.Status.Conditions = append(p.Resource.Status.Conditions, condition)
	}

	if err := p.Client.Status().Update(ctx, p.Resource); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}
	return nil
}

// getStatusMessage returns a human-readable message for a given status
func (p *Project) getStatusMessage(status v1beta1.ProjectStatusReason) string {
	switch status {
	case v1beta1.ProjectStatusCreated:
		return "Project successfully created"
	case v1beta1.ProjectStatusUpdated:
		return "Project successfully updated"
	case v1beta1.ProjectStatusArchived:
		return "Project successfully archived"
	case v1beta1.ProjectStatusInitError:
		return "Failed to initialize project"
	case v1beta1.ProjectStatusAPIError:
		return "API operation failed"
	case v1beta1.ProjectStatusOrgError:
		return "Organization operation failed"
	default:
		return "Unknown status"
	}
}

// Create creates a new project in OpenAI
func (p *Project) Create(ctx context.Context) error {
	openaiClient, err := p.initializeOpenAIClient(ctx)
	if err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusInitError)
	}

	resp, err := openaiClient.Projects.Create(ctx, openaisdk.ProjectCreateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusAPIError)
	}

	// Update project status
	creationTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.ProjectID = resp.ID
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateCondition(ctx, v1beta1.ProjectStatusCreated)
}

// Update updates the project details in OpenAI
func (p *Project) Update(ctx context.Context) error {
	openaiClient, err := p.initializeOpenAIClient(ctx)
	if err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusInitError)
	}

	resp, err := openaiClient.Projects.Update(ctx, p.Resource.Status.ProjectID, openaisdk.ProjectUpdateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusAPIError)
	}

	// Update status
	updateTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.LastUpdatedTime = &updateTime

	return p.updateCondition(ctx, v1beta1.ProjectStatusUpdated)
}

// GetProject fetches the project details from OpenAI
func (p *Project) GetProject(ctx context.Context) (*openaisdk.Project, error) {
	openaiClient, err := p.initializeOpenAIClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client for project %s: %w", p.Resource.Name, err)
	}

	project, err := openaiClient.Projects.Get(ctx, p.Resource.Status.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s (ID: %s): %w", p.Resource.Name, p.Resource.Status.ProjectID, err)
	}

	return project, nil
}

// Delete archives the project in OpenAI
func (p *Project) Delete(ctx context.Context) error {
	openaiClient, err := p.initializeOpenAIClient(ctx)
	if err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusInitError)
	}

	if _, err := openaiClient.Projects.Archive(ctx, p.Resource.Status.ProjectID); err != nil {
		return p.updateCondition(ctx, v1beta1.ProjectStatusAPIError)
	}

	return p.updateCondition(ctx, v1beta1.ProjectStatusArchived)
}
