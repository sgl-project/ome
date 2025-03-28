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

// NewOpenAIProject creates a new Project resource handler
func NewOpenAIProject(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) *OpenAIProject {
	return &OpenAIProject{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: project,
	}
}

// OpenAIProject implements OrganizationScoped, ResourceOperation, and OpenAIClientProvider
type OpenAIProject struct {
	ResourceBase
	Resource *v1beta1.Project
	// For testing purposes, allows injecting a mock client
	openAIClient *openaisdk.Client
}

// GetOpenAIClient initializes the OpenAI client with proper error handling
// Implements OpenAIClientProvider interface
func (p *OpenAIProject) GetOpenAIClient(ctx context.Context) (*openaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if p.openAIClient != nil {
		return p.openAIClient, nil
	}

	org, err := p.GetOrganizationFromProject(ctx, p.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	openAIClient, err := p.InitializeClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// SetOpenAIClient sets a custom OpenAI client for testing purposes
func (p *OpenAIProject) SetOpenAIClient(client *openaisdk.Client) {
	p.openAIClient = client
}

// Create creates a new project in OpenAI
func (p *OpenAIProject) Create(ctx context.Context) error {
	openaiClient, err := p.GetOpenAIClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}

	resp, err := openaiClient.Projects.Create(ctx, openaisdk.ProjectCreateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	// Update project status
	creationTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.ProjectID = resp.ID
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
}

// Update updates the project details in OpenAI
func (p *OpenAIProject) Update(ctx context.Context) error {
	// Only attempt to get the project if we have a project ID
	var existingProject *openaisdk.Project
	var err error
	existingProject, err = p.GetProject(ctx)
	if err != nil {
		return err
	}

	if p.Resource.Spec.Name != existingProject.Name {
		// Update project
		return p.updateInternal(ctx)
	}
	return nil
}

// Update updates the project details in OpenAI
func (p *OpenAIProject) updateInternal(ctx context.Context) error {
	openaiClient, err := p.GetOpenAIClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}

	resp, err := openaiClient.Projects.Update(ctx, p.Resource.Status.ProjectID, openaisdk.ProjectUpdateRequest{Name: p.Resource.Spec.Name})
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	// Update status
	updateTime := v1.NewTime(time.Unix(resp.CreatedAt, 0))
	p.Resource.Status.LastUpdatedTime = &updateTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusUpdated)
}

// GetProject fetches the project details from OpenAI
func (p *OpenAIProject) GetProject(ctx context.Context) (*openaisdk.Project, error) {
	openaiClient, err := p.GetOpenAIClient(ctx)
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
func (p *OpenAIProject) Delete(ctx context.Context) error {
	openaiClient, err := p.GetOpenAIClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}

	if _, err := openaiClient.Projects.Archive(ctx, p.Resource.Status.ProjectID); err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
}
