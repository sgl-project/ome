package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"

	"google.golang.org/genproto/googleapis/type/money"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"cloud.google.com/go/billing/apiv1/billingpb"
	budgetspb "cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GeminiProject implements OrganizationScoped, ResourceOperation
type GeminiProject struct {
	ResourceBase
	Resource         *v1beta1.Project
	gcpProjectClient GcpProjectClient
	gcpService       GcpServiceUsageClient
	billingClient    GcpBillingClient
	budgetClient     GcpBudgetClient
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
	if p.Resource.Spec.Name == "omcpminb2" {
		creationTime := v1.NewTime(time.Now())
		p.Resource.Status.ProjectId = p.Resource.Spec.Name
		p.Resource.Status.CreationTime = &creationTime
		p.Resource.Status.LastUpdatedTime = &creationTime
		return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
	}

	gcpClient, err := p.GetGcpProjectClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}

	defer gcpClient.Close()

	googleConfig, err := controllerconfig.NewGoogleConfig(p.Clientset)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusConfigError, err)
	}

	projectId := strings.ToLower(GenerateId("proj-", p.Resource.UID))

	// GCP doesn't allow the length of project displayName > 30
	displayName := p.Resource.Spec.Name
	if len(displayName) > 30 {
		displayName = displayName[:30]
	}

	req := &resourcemanagerpb.CreateProjectRequest{
		Project: &resourcemanagerpb.Project{
			ProjectId:   projectId,
			DisplayName: displayName,
			Parent:      googleConfig.ProjectFolder,
		},
	}

	createdProject, err := gcpClient.CreateProject(ctx, req)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError,
			fmt.Errorf("failed to create gcp project with projectId:%s:%w", projectId, err))
	}

	// Update project status
	var creationTime v1.Time
	if createdProject.CreateTime != nil {
		creationTime = v1.NewTime(createdProject.CreateTime.AsTime())
	} else {
		creationTime = v1.NewTime(time.Now())
	}
	p.Resource.Status.ProjectId = createdProject.ProjectId
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	// Enable Vertex API for the project
	err = p.EnableVertexAiAPI(ctx, createdProject.ProjectId)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	// Link predefined billing account to the project
	googleConfig, err = controllerconfig.NewGoogleConfig(p.Clientset)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusConfigError, err)
	}

	err = p.LinkToBillingAccount(ctx, createdProject.ProjectId, googleConfig.BillingAccount)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	if googleConfig.EnableBudget {
		err = p.SetProjectBillingBudget(ctx, createdProject.ProjectId, googleConfig.BillingAccount)
		if err != nil {
			return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
		}
	}

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
}

// Update updates the existing project
func (p *GeminiProject) Update(ctx context.Context) error {
	if p.Resource.Spec.Name == "omcpminb2" {
		return nil
	}

	existingProject, err := p.GetProject(ctx)
	if err != nil {
		return err
	}

	// Only update displayName if it's explicitly set and different
	if p.Resource.Spec.Name != existingProject.DisplayName {
		return p.updateInternal(ctx)
	}

	return nil
}

// Update updateInternal performs the actual Google project update
func (p *GeminiProject) updateInternal(ctx context.Context) error {
	gcpClient, err := p.GetGcpProjectClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}
	defer gcpClient.Close()

	if p.Resource.Status.ProjectId == "" {
		return fmt.Errorf("cannot update Google project: missing ProjectID in status")
	}

	updateReq := &resourcemanagerpb.UpdateProjectRequest{
		Project: &resourcemanagerpb.Project{
			Name:        "projects/" + p.Resource.Status.ProjectId,
			DisplayName: p.Resource.Spec.Name,
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"display_name"},
		},
	}

	updatedProject, err := gcpClient.UpdateProject(ctx, updateReq)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	var updateTime v1.Time
	if updatedProject.UpdateTime != nil {
		updateTime = v1.NewTime(updatedProject.UpdateTime.AsTime())
	} else {
		updateTime = v1.NewTime(time.Now())
	}

	p.Resource.Status.LastUpdatedTime = &updateTime
	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusUpdated)
}

// GetProject fetches the current Google project details
func (p *GeminiProject) GetProject(ctx context.Context) (*resourcemanagerpb.Project, error) {
	gcpClient, err := p.GetGcpProjectClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP client for project %s: %w", p.Resource.Name, err)
	}
	defer gcpClient.Close()

	projectID := p.Resource.Status.ProjectId
	if projectID == "" {
		return nil, fmt.Errorf("missing ProjectID in status for project %s", p.Resource.Name)
	}

	name := "projects/" + projectID
	project, err := gcpClient.GetProject(ctx, &resourcemanagerpb.GetProjectRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to get GCP project for %s (ID: %s): %w", p.Resource.Name, projectID, err)
	}

	return project, nil
}

// Delete deletes the existing project
func (p *GeminiProject) Delete(ctx context.Context) error {
	if p.Resource.Spec.Name == "omcpminb2" {
		return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
	}

	gcpClient, err := p.GetGcpProjectClient(ctx)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusInitError, err)
	}

	defer gcpClient.Close()

	projectId := p.Resource.Status.ProjectId

	// Query first to see if the project exist in GCP
	name := "projects/" + projectId
	_, err = gcpClient.GetProject(ctx, &resourcemanagerpb.GetProjectRequest{Name: name})
	if err != nil {
		p.Log.Info("failed get project from GCP", "projectId", projectId, "err", err)
		return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
	}

	req := &resourcemanagerpb.DeleteProjectRequest{
		Name: "projects/" + projectId,
	}

	deletedProject, err := gcpClient.DeleteProject(ctx, req)
	if err != nil {
		return p.updateConditionWithError(ctx, p.Resource, v1beta1.ProjectStatusAPIError, err)
	}

	p.Log.Info("deleted project", "projectId", deletedProject.ProjectId, "projectName", deletedProject.Name)
	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
}

// GetGcpProjectClient initializes the GCP project client with proper error handling
func (p *GeminiProject) GetGcpProjectClient(ctx context.Context) (GcpProjectClient, error) {
	if p.gcpProjectClient != nil {
		// For unit testing
		return p.gcpProjectClient, nil
	}

	org, err := p.GetOrganizationFromProject(ctx, p.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization when creating gcp client: %w", err)
	}

	gcpProjectClient, err := p.InitializeGcpProjectClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP project client: %w", err)
	}

	return gcpProjectClient, nil
}

func (p *GeminiProject) GetGcpBillingClient(ctx context.Context) (GcpBillingClient, error) {
	if p.billingClient != nil {
		// For unit testing
		return p.billingClient, nil
	}

	org, err := p.GetOrganizationFromProject(ctx, p.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization when creating gcp client: %w", err)
	}

	billingClient, err := p.InitializeGcpBillingClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP billing client: %w", err)
	}

	return billingClient, nil

}

func (p *GeminiProject) GetGcpBudgetClient(ctx context.Context) (GcpBudgetClient, error) {
	if p.budgetClient != nil {
		// For unit testing
		return p.budgetClient, nil
	}

	org, err := p.GetOrganizationFromProject(ctx, p.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization when creating gcp budget client: %w", err)
	}

	budgetClient, err := p.InitializeGcpBudgetClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP budget client: %w", err)
	}

	return budgetClient, nil
}

func (p *GeminiProject) GetGcpServiceUsage(ctx context.Context) (GcpServiceUsageClient, error) {
	if p.gcpService != nil {
		// For unit testing
		return p.gcpService, nil
	}

	org, err := p.GetOrganizationFromProject(ctx, p.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization when creating gcp service usage: %w", err)
	}

	gcpService, err := p.InitializeGcpServiceUsage(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP service usage: %w", err)
	}

	return gcpService, nil

}

func (p *GeminiProject) EnableVertexAiAPI(ctx context.Context, projectId string) error {
	svc, err := p.GetGcpServiceUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to create service usage client:%w", err)
	}

	err = svc.Enable(projectId, "aiplatform.googleapis.com")
	if err != nil {
		return fmt.Errorf("failed to enable Vertex Ai API for project %s, error:%w", projectId, err)
	}
	return err
}

func (p *GeminiProject) SetProjectBillingBudget(ctx context.Context, projectId string, billingAccount string) error {
	budgetClient, err := p.GetGcpBudgetClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP budget Client: %w", err)
	}

	defer budgetClient.Close()

	// Your billing account ID (starts with "01", like "01A234-567B89-CDEF01")
	billingAccountID := "billingAccounts/" + billingAccount

	// Create budget request
	req := &budgetspb.CreateBudgetRequest{
		Parent: billingAccountID,
		Budget: &budgetspb.Budget{
			DisplayName: fmt.Sprintf("Budget for OCI Genai project %s", projectId),
			BudgetFilter: &budgetspb.Filter{
				Projects: []string{"projects/" + projectId},
			},
			Amount: &budgetspb.BudgetAmount{
				BudgetAmount: &budgetspb.BudgetAmount_SpecifiedAmount{
					SpecifiedAmount: &money.Money{
						CurrencyCode: "USD",
						Units:        5000, // $5000
					},
				},
			},
			ThresholdRules: []*budgetspb.ThresholdRule{
				{
					ThresholdPercent: 0.5, // 50%
				},
				{
					ThresholdPercent: 0.9, // 90%
				},
				{
					ThresholdPercent: 1.0, // 100%
				},
			},
		},
	}

	resp, err := budgetClient.CreateBudget(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create budget for project %s, error:%w", projectId, err)
	}

	p.Log.Info("Budget created: %s for project %s", resp.GetName(), projectId)
	return nil
}

func (p *GeminiProject) LinkToBillingAccount(ctx context.Context, projectId string, billingAccount string) error {
	billingClient, err := p.GetGcpBillingClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP billing Client: %w", err)
	}

	defer billingClient.Close()

	updateReq := &billingpb.UpdateProjectBillingInfoRequest{
		Name: "projects/" + projectId,
		ProjectBillingInfo: &billingpb.ProjectBillingInfo{
			BillingAccountName: "billingAccounts/" + billingAccount,
			BillingEnabled:     true,
		},
	}
	projectBillingInfo, err := billingClient.UpdateProjectBillingInfo(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to enable billing for project %s, error:%w", projectId, err)
	}
	p.Log.Info("Billing account Linked to the project",
		"billingInfoName", projectBillingInfo.Name,
		"project", projectId,
		"billingAccount", billingAccount,
		"billingEnabled", projectBillingInfo.BillingEnabled)

	return nil
}

// SetGcpProjectClient sets a custom gcp project client for testing purposes
func (p *GeminiProject) SetGcpProjectClient(client GcpProjectClient) {
	p.gcpProjectClient = client
}

// SetGcpBillingClient sets a custom gcp billing client for testing purposes
func (p *GeminiProject) SetGcpBillingClient(client GcpBillingClient) {
	p.billingClient = client
}

// SetGcpBudgetClient sets a custom gcp budget client for testing purposes
func (p *GeminiProject) SetGcpBudgetClient(client GcpBudgetClient) {
	p.budgetClient = client
}

// SetGcpServiceUsageClient sets a custom gcp service usage client for testing purposes
func (p *GeminiProject) SetGcpServiceUsageClient(client GcpServiceUsageClient) {
	p.gcpService = client
}
