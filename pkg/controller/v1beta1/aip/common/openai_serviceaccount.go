package common

import (
	"context"
	"fmt"
	"time"

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

// GetProject retrieves the project associated with the service account
func (sa *OpenAIServiceAccount) GetProject(ctx context.Context) (*v1beta1.Project, error) {
	project := &v1beta1.Project{}
	if err := sa.Get(ctx, client.ObjectKey{Name: sa.Resource.Spec.ProjectRef.Name}, project); err != nil {
		// For NotFound errors, log without stack trace as this is expected in some cases
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Project not found",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)
		} else {
			// For unexpected errors, log with more details
			sa.Log.Error(err, "Failed to get project",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return project, nil
}

// GetOrganization retrieves the organization associated with the project
func (sa *OpenAIServiceAccount) GetOrganization(ctx context.Context) (*v1beta1.Organization, error) {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return nil, err
	}

	org := &v1beta1.Organization{}
	if err := sa.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, org); err != nil {
		sa.Log.Error(err, "failed to get organization from service account: ", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// GetOpenAIClient initializes the OpenAI client with proper error handling
// Implements OpenAIClientProvider interface
func (sa *OpenAIServiceAccount) GetOpenAIClient(ctx context.Context) (*openaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if sa.openAIClient != nil {
		return sa.openAIClient, nil
	}

	org, err := sa.GetOrganization(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	openAIClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// SetOpenAIClient sets a custom OpenAI client for testing purposes
func (sa *OpenAIServiceAccount) SetOpenAIClient(client *openaisdk.Client) {
	sa.openAIClient = client
}

// updateServiceAccountCondition updates the status conditions for the ServiceAccount
func (sa *OpenAIServiceAccount) updateServiceAccountCondition(ctx context.Context, status v1beta1.ServiceAccountStatusReason) error {
	now := metav1.NewTime(time.Now())
	conditionType := v1beta1.ConditionTypeReady
	conditionStatus := metav1.ConditionTrue

	if status.IsError() {
		conditionType = v1beta1.ConditionTypeError
		conditionStatus = metav1.ConditionFalse
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             string(status),
		Message:            sa.getStatusMessage(status),
		LastTransitionTime: now,
		ObservedGeneration: sa.Resource.Generation,
	}

	// Update or append the condition
	found := false
	for i, c := range sa.Resource.Status.Conditions {
		if c.Type == conditionType {
			sa.Resource.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		sa.Resource.Status.Conditions = append(sa.Resource.Status.Conditions, condition)
	}

	return sa.Client.Status().Update(ctx, sa.Resource)
}

// updateServiceAccountConditionWithError updates the status conditions and returns the original error
// to ensure proper error propagation for reconciliation requeuing
func (sa *OpenAIServiceAccount) updateServiceAccountConditionWithError(ctx context.Context, status v1beta1.ServiceAccountStatusReason, originalErr error) error {
	// Update the status
	if err := sa.updateServiceAccountCondition(ctx, status); err != nil {
		// If status update fails, log it but return the original error as that's more important
		sa.Log.Error(err, "Failed to update service account status", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
	}

	// Always return the original error to ensure reconciliation is requeued
	return originalErr
}

// getStatusMessage returns a human-readable message for a given status
func (sa *OpenAIServiceAccount) getStatusMessage(status v1beta1.ServiceAccountStatusReason) string {
	switch status {
	case v1beta1.ServiceAccountStatusCreated:
		return "Service account successfully created"
	case v1beta1.ServiceAccountStatusDeleted:
		return "Service account successfully deleted"
	case v1beta1.ServiceAccountStatusProjectError:
		return "Failed to get project information"
	case v1beta1.ServiceAccountStatusInitError:
		return "Failed to initialize service account"
	case v1beta1.ServiceAccountStatusAPIError:
		return "API operation failed"
	case v1beta1.ServiceAccountStatusSecretError:
		return "Secret operation failed"
	case v1beta1.ServiceAccountStatusConfigError:
		return "Configuration error occurred"
	default:
		return "Unknown status"
	}
}

// getCommonSecret retrieves or creates the common secret
func (sa *OpenAIServiceAccount) getCommonSecret(ctx context.Context, config *v1beta1.AIPlatformConfig) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := sa.Get(ctx, client.ObjectKey{Name: config.SecretConfig.SecretName, Namespace: config.SecretConfig.Namespace}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get common secret: %w", err)
	}
	return secret, nil
}

// Create implements ResourceOperation
func (sa *OpenAIServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusProjectError, err)
	}

	openaiClient, err := sa.GetOpenAIClient(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusInitError, err)
	}

	// Set the project as the owner of the service account
	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Update the service account resource with the owner reference
	if err := sa.Client.Update(ctx, sa.Resource); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusAPIError,
			fmt.Errorf("failed to update service account with owner reference: %w", err))
	}

	// Check if ProjectID is available
	if project.Status.ProjectID == "" {
		sa.Log.Info("ProjectID is not available yet for service account", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusProjectError,
			fmt.Errorf("project ID not available for project %s", project.Name))
	}

	// Create service account in OpenAI
	resp, err := openaiClient.ServiceAccounts.Create(ctx, project.Status.ProjectID, openaisdk.ProjectServiceAccountCreateRequest{Name: *sa.Resource.Spec.Name})
	if err != nil {
		// Log the error without stack trace for API errors as they might be transient
		sa.Log.Info("Failed to create service account in API, will retry",
			"name", sa.Resource.Name,
			"namespace", sa.Resource.Namespace,
			"projectID", project.Status.ProjectID,
			"error", err.Error())
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusAPIError, err)
	}

	// Add service account key to common secret
	aiPlatformConfig, err := v1beta1.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusConfigError, err)
	}

	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update common secret
	if commonSecret.StringData == nil {
		commonSecret.StringData = map[string]string{}
	}
	commonSecret.StringData[resp.ID] = resp.APIKey.Value
	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusSecretError, err)
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
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update status
	creationTime := metav1.NewTime(time.Unix(resp.CreatedAt, 0))
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountID: &resp.ID,
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
	return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusCreated)
}

// deleteServiceAccountKeyFromSecret removes a key from the common secret
func (sa *OpenAIServiceAccount) deleteServiceAccountKeyFromSecret(ctx context.Context, aiPlatformConfig *v1beta1.AIPlatformConfig) error {
	// Safety check - ensure we have a service account ID
	if sa.Resource.Status.ServiceAccountID == nil {
		sa.Log.Info("No service account ID found, skipping key deletion from common secret",
			"name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return nil
	}

	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		// If the secret doesn't exist, that's fine - nothing to delete
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Common secret not found, nothing to delete",
				"name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
			return nil
		}
		return fmt.Errorf("failed to get common secret: %w", err)
	}

	// Check if the key exists in the secret data
	if commonSecret.Data == nil || len(commonSecret.Data[*sa.Resource.Status.ServiceAccountID]) == 0 {
		sa.Log.Info("Service account key not found in common secret, nothing to delete",
			"name", sa.Resource.Name, "namespace", sa.Resource.Namespace,
			"serviceAccountID", *sa.Resource.Status.ServiceAccountID)
		return nil
	}

	// Delete the key from the secret
	delete(commonSecret.Data, *sa.Resource.Status.ServiceAccountID)
	return sa.Client.Update(ctx, commonSecret)
}

// Delete implements ResourceOperation
func (sa *OpenAIServiceAccount) Delete(ctx context.Context) error {
	// Try to get the project, but don't fail if it doesn't exist
	// This handles the case where the project was deleted and triggered this service account deletion
	project, err := sa.GetProject(ctx)
	if err != nil {
		// Check if the error is because the project doesn't exist
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Project not found during service account deletion, likely already deleted",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)

			// Even if the project is gone, we should still clean up the service account key from the common secret
			aiPlatformConfig, configErr := v1beta1.NewAIPlatformConfig(sa.Clientset)
			if configErr != nil {
				sa.Log.Error(configErr, "Failed to get AIPlatform config")
				// Update status but don't return error to avoid requeuing
				_ = sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusConfigError)
			} else if sa.Resource.Status.ServiceAccountID != nil {
				// Try to delete the key from the common secret
				if secretErr := sa.deleteServiceAccountKeyFromSecret(ctx, aiPlatformConfig); secretErr != nil {
					sa.Log.Error(secretErr, "Failed to delete service account key from common secret")
					// Update status but don't return error to avoid requeuing
					_ = sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusSecretError)
				}
			}

			// Update status and continue with deletion
			return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusDeleted)
		}

		// For other errors, propagate them normally
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusProjectError, err)
	}

	openaiClient, err := sa.GetOpenAIClient(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusInitError, err)
	}

	// Only try to delete the service account in the API if we have both project ID and service account ID
	if project.Status.ProjectID != "" && sa.Resource.Status.ServiceAccountID != nil {
		if _, err := openaiClient.ServiceAccounts.Delete(ctx, project.Status.ProjectID, *sa.Resource.Status.ServiceAccountID); err != nil {
			// Log the error without stack trace for API errors as they might be transient
			sa.Log.Info("Failed to delete service account in API, will retry",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectID", project.Status.ProjectID,
				"serviceAccountID", *sa.Resource.Status.ServiceAccountID,
				"error", err.Error())
			return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusAPIError, err)
		}
	}

	// Delete service account key from common secret
	aiPlatformConfig, err := v1beta1.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusConfigError, err)
	}

	if sa.Resource.Status.ServiceAccountID != nil {
		if err := sa.deleteServiceAccountKeyFromSecret(ctx, aiPlatformConfig); err != nil {
			return sa.updateServiceAccountConditionWithError(ctx, v1beta1.ServiceAccountStatusSecretError, err)
		}
	}

	return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusDeleted)
}

// Update implements ServiceAccountOperation
func (sa *OpenAIServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}
