package common

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"github.com/go-logr/logr"
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

// ClientInitializer provides methods to initialize vendor clients
type ClientInitializer interface {
	// InitializeClient initializes a vendor client using organization credentials
	InitializeClient(ctx context.Context, org *v1beta1.Organization) (*openaisdk.Client, error)
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

	secret, err := r.getSecret(ctx, &org.Spec.SecretRef)
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
