package common

import (
	"context"
	"fmt"
	"os"

	admin "cloud.google.com/go/iam/admin/apiv1"

	"k8s.io/apimachinery/pkg/runtime"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk"
	xaiOptions "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/option"
	billing "cloud.google.com/go/billing/apiv1"
	budgets "cloud.google.com/go/billing/budgets/apiv1"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"github.com/go-logr/logr"
	googleOption "google.golang.org/api/option"
	serviceusage "google.golang.org/api/serviceusage/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OrganizationScoped represents a resource scoped to an organization
type OrganizationScoped interface {
	// GetOrganizationRef returns the organization reference
	GetOrganizationRef() *v1beta1.CrossReference
	// GetOrganization fetches the organization resource
	GetOrganization(ctx context.Context) (*v1beta1.Organization, error)
}

// ProjectScoped represents a resource scoped to a project
type ProjectScoped interface {
	OrganizationScoped
	// GetProjectRef returns the project reference
	GetProjectRef() *v1beta1.CrossReference
	// GetProject fetches the project resource
	GetProject(ctx context.Context) (*v1beta1.Project, error)
}

// ResourceOperation defines standard CRUD operations for a resource
type ResourceOperation interface {
	Create(ctx context.Context) error
	Update(ctx context.Context) error
	Delete(ctx context.Context) error
}

// ServiceAccountOperation defines operations for a service account
type ServiceAccountOperation interface {
	Create(ctx context.Context) error
	Update(ctx context.Context, sa *v1beta1.ServiceAccount) error
	Delete(ctx context.Context) error
}

// RateLimitOperation defines operations for a rate limit
type RateLimitOperation interface {
	Create(ctx context.Context) error
	Update(ctx context.Context) error
	Delete(ctx context.Context) error
}

// ClientInitializer provides methods to initialize vendor clients
type ClientInitializer interface {
	// InitializeClient initializes a vendor client using organization credentials
	InitializeClient(ctx context.Context, org *v1beta1.Organization) (*openaisdk.Client, error)
}

// OpenAIClientProvider defines an interface for providing OpenAI clients
type OpenAIClientProvider interface {
	// GetOpenAIClient returns an initialized OpenAI client
	GetOpenAIClient(ctx context.Context) (*openaisdk.Client, error)
}

// ResourceBase provides common functionality for all resources
type ResourceBase struct {
	client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

func (r *ResourceBase) getSecret(ctx context.Context, ref *v1beta1.SecretReference) (*v1.Secret, error) {
	secret := &v1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// InitializeClient initializes an OpenAI client using organization credentials
func (r *ResourceBase) InitializeClient(ctx context.Context, org *v1beta1.Organization) (*openaisdk.Client, error) {
	if org.Spec.Vendor == nil {
		return nil, fmt.Errorf("vendor is not specified")
	}

	if *org.Spec.Vendor != "openai" {
		return nil, fmt.Errorf("unsupported vendor: %s", *org.Spec.Vendor)
	}

	secret, err := r.getSecret(ctx, org.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}

	apiKey := string(secret.Data[org.Spec.SecretRef.Key])
	if apiKey == "" {
		return nil, fmt.Errorf("API key is empty in secret %s", org.Spec.SecretRef.Name)
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL, ok := org.Spec.Config["baseURL"]; ok {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return openaisdk.NewClient(opts...), nil
}

// InitializeGcpProjectClient initializes a GCP Project client using organization credentials
func (r *ResourceBase) InitializeGcpProjectClient(ctx context.Context, org *v1beta1.Organization) (GcpProjectClient, error) {
	adminSecret, err := r.GetGcpAdminSecret(org)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin secret: %w", err)
	}

	gcpProjectClient, err := resourcemanager.NewProjectsClient(ctx, googleOption.WithCredentialsJSON(adminSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP project client: %w", err)
	}
	return &RealGcpProjectClient{client: gcpProjectClient}, nil
}

// InitializeGcpBillingClient initializes a GCP Billing client using organization credentials
func (r *ResourceBase) InitializeGcpBillingClient(ctx context.Context, org *v1beta1.Organization) (GcpBillingClient, error) {
	adminSecret, err := r.GetGcpAdminSecret(org)

	if err != nil {
		return nil, err
	}

	billingClient, err := billing.NewCloudBillingClient(ctx, googleOption.WithCredentialsJSON(adminSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP billing client: %w", err)
	}

	return &RealGcpBillingClient{client: billingClient}, nil
}

// InitializeGcpBudgetClient initializes a GCP Budget client using organization credentials
func (r *ResourceBase) InitializeGcpBudgetClient(ctx context.Context, org *v1beta1.Organization) (GcpBudgetClient, error) {
	adminSecret, err := r.GetGcpAdminSecret(org)

	if err != nil {
		return nil, err
	}

	budgetClient, err := budgets.NewBudgetClient(ctx, googleOption.WithCredentialsJSON(adminSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP budget client: %w", err)
	}

	return &RealGcpBudgetClient{client: budgetClient}, nil
}

// InitializeGcpServiceUsage initializes a GCP Service usage client using organization credentials
func (r *ResourceBase) InitializeGcpServiceUsage(ctx context.Context, org *v1beta1.Organization) (GcpServiceUsageClient, error) {
	adminSecret, err := r.GetGcpAdminSecret(org)

	if err != nil {
		return nil, err
	}

	svc, err := serviceusage.NewService(ctx, googleOption.WithCredentialsJSON(adminSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to create service usage client:%w", err)
	}

	return &RealGcpServiceUsageClient{service: svc}, nil
}

// InitializeGcpIamClient initializes a GCP iam client using organization credentials
func (r *ResourceBase) InitializeGcpIamClient(ctx context.Context, org *v1beta1.Organization) (GcpIamClient, error) {
	adminSecret, err := r.GetGcpAdminSecret(org)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin secret: %w", err)
	}

	iamClient, err := admin.NewIamClient(ctx, googleOption.WithCredentialsJSON(adminSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP iam client:%w", err)
	}

	return &RealGcpIamClient{client: iamClient}, nil
}

func (r *ResourceBase) GetGcpAdminSecret(org *v1beta1.Organization) ([]byte, error) {
	if org.Spec.Vendor == nil {
		return nil, fmt.Errorf("vendor is not specified for organization %s", org.Name)
	}

	if *org.Spec.Vendor != "google" {
		return nil, fmt.Errorf("unsupported vendor: %s (expected: google)", *org.Spec.Vendor)
	}

	adminSecret, err := os.ReadFile("/tmp/secrets-store/google-sa-file")
	if err != nil {
		return nil, fmt.Errorf("failed to read GCP service account file: %w", err)
	}

	if len(adminSecret) == 0 {
		return nil, fmt.Errorf("GCP service account key is empty in OCI Vault secret")
	}

	return adminSecret, nil
}

// InitializeClient initializes an xAI client using organization credentials
func (r *ResourceBase) InitializeXaiClient(ctx context.Context, org *v1beta1.Organization) (*xaisdk.Client, error) {
	if org.Spec.Vendor == nil {
		return nil, fmt.Errorf("vendor is not specified")
	}

	if *org.Spec.Vendor != "xai" {
		return nil, fmt.Errorf("unsupported vendor: %s", *org.Spec.Vendor)
	}

	secret, err := r.getSecret(ctx, org.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}

	apiKey := string(secret.Data[org.Spec.SecretRef.Key])
	if apiKey == "" {
		return nil, fmt.Errorf("API key is empty in secret %s", org.Spec.SecretRef.Name)
	}

	opts := []xaiOptions.RequestOption{xaiOptions.WithAPIKey(apiKey)}
	if baseURL, ok := org.Spec.Config["baseURL"]; ok {
		opts = append(opts, xaiOptions.WithBaseURL(baseURL))
	}

	return xaisdk.NewClient(opts...), nil
}
