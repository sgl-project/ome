package common

import (
	"context"

	"cloud.google.com/go/billing/apiv1/billingpb"
	"cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
)

type GcpProjectClient interface {
	GetProject(ctx context.Context, req *resourcemanagerpb.GetProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error)
	CreateProject(ctx context.Context, req *resourcemanagerpb.CreateProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error)
	UpdateProject(ctx context.Context, req *resourcemanagerpb.UpdateProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error)
	DeleteProject(ctx context.Context, req *resourcemanagerpb.DeleteProjectRequest, opts ...gax.CallOption) (*resourcemanagerpb.Project, error)
	GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest, opts ...gax.CallOption) (*iampb.Policy, error)
	SetIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest, opts ...gax.CallOption) (*iampb.Policy, error)
	Close() error
}

type GcpIamClient interface {
	CreateServiceAccount(ctx context.Context, req *adminpb.CreateServiceAccountRequest, opts ...gax.CallOption) (*adminpb.ServiceAccount, error)
	CreateServiceAccountKey(ctx context.Context, req *adminpb.CreateServiceAccountKeyRequest, opts ...gax.CallOption) (*adminpb.ServiceAccountKey, error)
	DeleteServiceAccount(ctx context.Context, req *adminpb.DeleteServiceAccountRequest, opts ...gax.CallOption) error
	Close() error
}

type GcpBillingClient interface {
	UpdateProjectBillingInfo(ctx context.Context, req *billingpb.UpdateProjectBillingInfoRequest, opts ...gax.CallOption) (*billingpb.ProjectBillingInfo, error)
	Close() error
}

type GcpServiceUsageClient interface {
	Enable(projectId string, apiName string) error
}

type GcpBudgetClient interface {
	CreateBudget(ctx context.Context, req *budgetspb.CreateBudgetRequest) (*budgetspb.Budget, error)
	Close() error
}

type GcpRestfulClient interface {
	SetCacheConfig(ctx context.Context, projectId string, disableCache bool) error
}
