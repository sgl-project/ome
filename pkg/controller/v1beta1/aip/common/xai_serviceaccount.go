package common

import (
	"context"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

const (
	// defaultQPM is the default QPM (queries per minute) limit assigned to each xAI API key.
	// Adjust this value if default rate limiting requirements change.
	defaultQPM int32 = 5_000
	// defaultQPS is the default QPS (queries per second) limit assigned to each xAI API key.
	// Adjust this value if default rate limiting requirements change.
	defaultQPS int32 = 84

	// defaultTPM is the default TPM (tokens per minute) limit assigned to each xAI API key.
	// Adjust this value if default rate limiting requirements change.
	defaultTPM string = "200000"
)

// XAIServiceAccount implements ProjectScoped and ResourceOperation
type XAIServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
	// For testing purposes, allows injecting a mock client
	xAIClient *xaisdk.Client
}

// NewXAIServiceAccount creates a new ServiceAccount resource handler
func NewXAIServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *XAIServiceAccount {
	return &XAIServiceAccount{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: sa,
	}
}

// GetXAIClient initializes the OpenAI client with proper error handling
// Implements OpenAIClientProvider interface
func (sa *XAIServiceAccount) GetXAIClient(ctx context.Context) (*xaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if sa.xAIClient != nil {
		return sa.xAIClient, nil
	}

	org, err := sa.GetOrganization(ctx, sa.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	xAIClient, err := sa.InitializeXaiClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return xAIClient, nil
}

// SetXAIClient sets a custom OpenAI client for testing purposes
func (sa *XAIServiceAccount) SetXAIClient(client *xaisdk.Client) {
	sa.xAIClient = client
}

// Create implements ResourceOperation
func (sa *XAIServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	xaiClient, err := sa.GetXAIClient(ctx)
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
	xaiTeamId := "ee0936c7-f897-47e8-836b-08bb2d3547e5"
	// Create API Key in xAI
	resp, err := xaiClient.APIKeys.Create(ctx, xaiTeamId,
		xaisdk.CreateApiKeyBody{
			Name:       *sa.Resource.Spec.Name,
			ACLStrings: []string{"api-key:endpoint:*", "api-key:model:*"},
			QPM:        defaultQPM,
			QPS:        defaultQPS,
			TPM:        defaultTPM,
		})
	if err != nil {
		// Log the error without stack trace for API errors as they might be transient
		sa.Log.Info("Failed to create API Key in API, will retry",
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
	serviceAccountID := GenerateId("user_", sa.Resource.UID)

	// Update common secret
	if commonSecret.StringData == nil {
		commonSecret.StringData = map[string]string{}
	}
	commonSecret.StringData[serviceAccountID] = resp.ApiKey

	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Create secret for API key
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", sa.Resource.Name),
			Namespace:    sa.Resource.Namespace,
		},
		StringData: map[string]string{serviceAccountID: resp.ApiKey},
	}
	if err := controllerutil.SetControllerReference(sa.Resource, secret, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}
	if err := sa.Client.Create(ctx, secret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update status
	creationTime := metav1.NewTime(time.Now())
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountId: &serviceAccountID,
		CreationTime:     &creationTime,
		APIKey: &v1beta1.APIKeySpec{
			Name:     &resp.ApiKeyId,
			APIKeyId: &resp.ApiKeyId,
			APIKeySecretRef: &v1beta1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
				Key:       resp.ApiKeyId,
			},
		},
	}

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
}

// Delete implements ResourceOperation
func (sa *XAIServiceAccount) Delete(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		// Check if the error is because the project doesn't exist
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Project not found during service account deletion, likely already deleted",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)

			// Update status and continue with deletion
			return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
		}

		// For other errors, propagate them normally
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}
	xaiClient, err := sa.GetXAIClient(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}

	// Only try to delete the API Key in the API if we have both project ID and API Key ID
	if project.Status.ProjectId != "" && sa.Resource.Status.APIKey != nil && sa.Resource.Status.APIKey.APIKeyId != nil {
		if _, err := xaiClient.APIKeys.Delete(ctx, *sa.Resource.Status.APIKey.APIKeyId); err != nil {
			// Log the error without stack trace for API errors as they might be transient
			sa.Log.Info("Failed to delete API key in xAI API, will retry",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectID", project.Status.ProjectId,
				"apiKeyId", *sa.Resource.Status.APIKey.APIKeyId,
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

// Update implements ServiceAccountOperation
func (sa *XAIServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}
