package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"golang.org/x/oauth2/google"

	"net/http"

	billing "cloud.google.com/go/billing/apiv1"
	"cloud.google.com/go/billing/apiv1/billingpb"
	budgets "cloud.google.com/go/billing/budgets/apiv1"
	budgetspb "cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	admin "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"cloud.google.com/go/iam/apiv1/iampb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/serviceusage/v1"
)

type RealGcpProjectClient struct {
	client *resourcemanager.ProjectsClient
}

func (c *RealGcpProjectClient) GetProject(ctx context.Context, req *resourcemanagerpb.GetProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error) {
	return c.client.GetProject(ctx, req)
}

func (c *RealGcpProjectClient) CreateProject(ctx context.Context, req *resourcemanagerpb.CreateProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error) {
	op, err := c.client.CreateProject(ctx, req)
	if err != nil {
		return nil, err
	}

	createdProject, err := op.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return createdProject, nil
}

func (c *RealGcpProjectClient) UpdateProject(ctx context.Context, req *resourcemanagerpb.UpdateProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error) {
	op, err := c.client.UpdateProject(ctx, req)
	if err != nil {
		return nil, err
	}

	updatedProject, err := op.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return updatedProject, nil
}
func (c *RealGcpProjectClient) DeleteProject(ctx context.Context, req *resourcemanagerpb.DeleteProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error) {
	op, err := c.client.DeleteProject(ctx, req)
	if err != nil {
		return nil, err
	}

	deletedProject, err := op.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return deletedProject, nil
}

func (c *RealGcpProjectClient) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...gax.CallOption) (*iampb.Policy, error) {
	return c.client.GetIamPolicy(ctx, req)
}

func (c *RealGcpProjectClient) SetIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest, opts ...gax.CallOption) (*iampb.Policy, error) {
	return c.client.SetIamPolicy(ctx, req)
}

func (c *RealGcpProjectClient) Close() error {
	return c.client.Close()
}

type RealGcpIamClient struct {
	client *admin.IamClient
}

func (c *RealGcpIamClient) CreateServiceAccount(ctx context.Context, req *adminpb.CreateServiceAccountRequest, opts ...gax.CallOption) (*adminpb.ServiceAccount, error) {
	return c.client.CreateServiceAccount(ctx, req)
}
func (c *RealGcpIamClient) CreateServiceAccountKey(ctx context.Context, req *adminpb.CreateServiceAccountKeyRequest, opts ...gax.CallOption) (*adminpb.ServiceAccountKey, error) {
	return c.client.CreateServiceAccountKey(ctx, req)
}
func (c *RealGcpIamClient) DeleteServiceAccount(ctx context.Context, req *adminpb.DeleteServiceAccountRequest, opts ...gax.CallOption) error {
	return c.client.DeleteServiceAccount(ctx, req)
}

func (c *RealGcpIamClient) Close() error {
	return c.client.Close()
}

type RealGcpBillingClient struct {
	client *billing.CloudBillingClient
}

func (c *RealGcpBillingClient) UpdateProjectBillingInfo(ctx context.Context, req *billingpb.UpdateProjectBillingInfoRequest, opts ...gax.CallOption) (*billingpb.ProjectBillingInfo, error) {
	return c.client.UpdateProjectBillingInfo(ctx, req)
}

func (c *RealGcpBillingClient) Close() error {
	return c.client.Close()
}

type RealGcpBudgetClient struct {
	client *budgets.BudgetClient
}

func (c *RealGcpBudgetClient) CreateBudget(ctx context.Context, req *budgetspb.CreateBudgetRequest) (*budgetspb.Budget, error) {
	return c.client.CreateBudget(ctx, req)
}

func (c *RealGcpBudgetClient) Close() error {
	return c.client.Close()
}

type RealGcpServiceUsageClient struct {
	service *serviceusage.Service
}

func (c *RealGcpServiceUsageClient) Enable(projectId string, apiName string) error {
	op, err := c.service.Services.Enable(
		fmt.Sprintf("projects/%s/services/%s", projectId, apiName),
		&serviceusage.EnableServiceRequest{}).Do()

	if err != nil {
		return err
	}

	for {
		op, err = c.service.Operations.Get(op.Name).Do()
		if err != nil {
			return err
		}

		if op.Done {
			break
		}
		time.Sleep(10 * time.Second)
	}

	return nil
}

type RealGcpRestfulClient struct {
	adminSecret []byte
}

func (c *RealGcpRestfulClient) SetCacheConfig(ctx context.Context, projectId string, disableCache bool) error {
	creds, err := google.CredentialsFromJSON(ctx, c.adminSecret, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}
	tokenSource := creds.TokenSource

	url := fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/cacheConfig", projectId)
	body := []byte(fmt.Sprintf("{\"disableCache\": %v}", disableCache))
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request to set cache config: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	token, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to acquire token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-OK response: %d body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
