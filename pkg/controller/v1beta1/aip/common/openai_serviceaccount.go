package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
)

// OpenAIServiceAccount implements ProjectScoped and ResourceOperation

const defaultOpenAIBaseURL = "https://api.openai.com/v1/"

var openAIBaseURLByGeography = map[string]string{
	"US": "https://us.api.openai.com/v1/",
	"EU": "https://eu.api.openai.com/v1/",
}

func resolveOpenAIBaseURL(geography string) string {
	geo := strings.ToUpper(strings.TrimSpace(geography))
	if geo == "" {
		return defaultOpenAIBaseURL
	}
	if baseURL, ok := openAIBaseURLByGeography[geo]; ok {
		return baseURL
	}
	return defaultOpenAIBaseURL
}

type OpenAIServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
	// For testing purposes, allows injecting a mock client
	openAIClient *openaisdk.Client
}

// NewOpenAIServiceAccount creates a new ServiceAccount resource handler
func NewOpenAIServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *OpenAIServiceAccount {
	return &OpenAIServiceAccount{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: sa,
	}
}

// GetOpenAIClient initializes the OpenAI client with proper error handling
// Implements OpenAIClientProvider interface
func (sa *OpenAIServiceAccount) GetOpenAIClient(ctx context.Context) (*openaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if sa.openAIClient != nil {
		return sa.openAIClient, nil
	}

	org, err := sa.GetOrganization(ctx, sa.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	openAIClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// GetOpenAIClientForProject initializes the OpenAI client using project geography
func (sa *OpenAIServiceAccount) GetOpenAIClientForProject(ctx context.Context, project *v1beta1.Project) (*openaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if sa.openAIClient != nil {
		return sa.openAIClient, nil
	}

	org, err := sa.GetOrganization(ctx, sa.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	geography := ""
	if project != nil && project.Spec.Config != nil {
		geography = project.Spec.Config["geography"]
	}
	baseURL := resolveOpenAIBaseURL(geography)

	openAIClient, err := sa.InitializeClientWithBaseURL(ctx, org, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// SetOpenAIClient sets a custom OpenAI client for testing purposes
func (sa *OpenAIServiceAccount) SetOpenAIClient(client *openaisdk.Client) {
	sa.openAIClient = client
}

// Create implements ResourceOperation
func (sa *OpenAIServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	openaiClient, err := sa.GetOpenAIClientForProject(ctx, project)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}

	// Set the project as the owner of the service account
	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Update the service account resource with the owner reference
	if err := sa.Client.Update(ctx, sa.Resource); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError,
			fmt.Errorf("failed to update service account with owner reference: %w", err))
	}

	// Check if ProjectId is available
	if project.Status.ProjectId == "" {
		sa.Log.Info("ProjectId is not available yet for service account", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError,
			fmt.Errorf("project ID not available for project %s", project.Name))
	}

	// Create service account in OpenAI
	resp, err := openaiClient.ServiceAccounts.Create(ctx, project.Status.ProjectId, openaisdk.ProjectServiceAccountCreateRequest{Name: *sa.Resource.Spec.Name})
	if err != nil {
		// Log the error without stack trace for API errors as they might be transient
		sa.Log.Info("Failed to create service account in API, will retry",
			"name", sa.Resource.Name,
			"namespace", sa.Resource.Namespace,
			"projectID", project.Status.ProjectId,
			"error", err.Error())
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError, err)
	}

	// Add service account key to common secret
	aiPlatformConfig, err := controllerconfig.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusConfigError, err)
	}

	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update common secret
	if commonSecret.StringData == nil {
		commonSecret.StringData = map[string]string{}
	}
	commonSecret.StringData[resp.ID] = resp.APIKey.Value
	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Create secret for API key
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", sa.Resource.Name),
			Namespace:    sa.Resource.Namespace,
		},
		StringData: map[string]string{resp.ID: resp.APIKey.Value},
	}
	if err := controllerutil.SetControllerReference(sa.Resource, secret, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}
	if err := sa.Client.Create(ctx, secret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update status
	creationTime := metav1.NewTime(time.Unix(resp.CreatedAt, 0))
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountId: &resp.ID,
		APIKey: &v1beta1.APIKeySpec{
			Name:     &resp.APIKey.Name,
			APIKeyId: &resp.APIKey.ID,
			APIKeySecretRef: &v1beta1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
				Key:       resp.ID,
			},
		},
		CreationTime: &creationTime,
	}
	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
}

// Delete implements ResourceOperation
func (sa *OpenAIServiceAccount) Delete(ctx context.Context) error {
	// Try to get the project, but don't fail if it doesn't exist
	// This handles the case where the project was deleted and triggered this service account deletion
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		// Check if the error is because the project doesn't exist
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Project not found during service account deletion, likely already deleted",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)

			// Even if the project is gone, we should still clean up the service account key from the common secret
			_ = sa.deleteServiceAccountKey(ctx, sa.Resource, true)

			// Update status and continue with deletion
			return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
		}

		// For other errors, propagate them normally
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	openaiClient, err := sa.GetOpenAIClientForProject(ctx, project)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}

	// Only try to delete the service account in the API if we have both project ID and service account ID
	if project.Status.ProjectId != "" && sa.Resource.Status.ServiceAccountId != nil {
		if _, err := openaiClient.ServiceAccounts.Delete(ctx, project.Status.ProjectId, *sa.Resource.Status.ServiceAccountId); err != nil {
			// Log the error without stack trace for API errors as they might be transient
			sa.Log.Info("Failed to delete service account in API, will retry",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectID", project.Status.ProjectId,
				"serviceAccountID", *sa.Resource.Status.ServiceAccountId,
				"error", err.Error())
			return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError, err)
		}
	}

	// Delete service account key from common secret
	if err := sa.deleteServiceAccountKey(ctx, sa.Resource, false); err != nil {
		return err
	}

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
}
