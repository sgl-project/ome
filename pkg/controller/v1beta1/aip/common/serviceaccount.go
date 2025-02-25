package common

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
)

// ServiceAccount implements ProjectScoped and ResourceOperation
type ServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
}

// NewServiceAccount creates a new ServiceAccount resource handler
func NewServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *ServiceAccount {
	return &ServiceAccount{
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
func (sa *ServiceAccount) GetProject(ctx context.Context) (*v1beta1.Project, error) {
	project := &v1beta1.Project{}
	if err := sa.Get(ctx, client.ObjectKey{Name: sa.Resource.Spec.ProjectRef.Name}, project); err != nil {
		sa.Log.Error(err, "failed to get project from service account: ", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return project, nil
}

// GetOrganization retrieves the organization associated with the project
func (sa *ServiceAccount) GetOrganization(ctx context.Context) (*v1beta1.Organization, error) {
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

// updateServiceAccountCondition updates the status conditions for the ServiceAccount
func (sa *ServiceAccount) updateServiceAccountCondition(ctx context.Context, status v1beta1.ServiceAccountStatusReason) error {
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

// getStatusMessage returns a human-readable message for a given status
func (sa *ServiceAccount) getStatusMessage(status v1beta1.ServiceAccountStatusReason) string {
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
func (sa *ServiceAccount) getCommonSecret(ctx context.Context, config *v1beta1.AIPlatformConfig) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := sa.Get(ctx, client.ObjectKey{Name: config.SecretConfig.SecretName, Namespace: config.SecretConfig.Namespace}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get common secret: %w", err)
	}
	return secret, nil
}

// Create implements ResourceOperation
func (sa *ServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusProjectError)
	}

	org, err := sa.GetOrganization(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusProjectError)
	}

	openaiClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusInitError)
	}

	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if ProjectID is available
	if project.Status.ProjectID == "" {
		sa.Log.Info("ProjectID is not available yet for service account", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusProjectError)
	}

	// Create service account in OpenAI
	resp, err := openaiClient.ServiceAccounts.Create(ctx, project.Status.ProjectID, openaisdk.ProjectServiceAccountCreateRequest{Name: *sa.Resource.Spec.Name})
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusAPIError)
	}

	// Add service account key to common secret
	aiPlatformConfig, err := v1beta1.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusConfigError)
	}

	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusSecretError)
	}

	// Update common secret
	if commonSecret.StringData == nil {
		commonSecret.StringData = map[string]string{}
	}
	commonSecret.StringData[resp.ID] = resp.APIKey.Value
	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusSecretError)
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
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusSecretError)
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
func (sa *ServiceAccount) deleteServiceAccountKeyFromSecret(ctx context.Context, aiPlatformConfig *v1beta1.AIPlatformConfig) error {
	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		return fmt.Errorf("failed to get common secret: %w", err)
	}

	delete(commonSecret.Data, *sa.Resource.Status.ServiceAccountID)
	return sa.Client.Update(ctx, commonSecret)
}

// Delete implements ResourceOperation
func (sa *ServiceAccount) Delete(ctx context.Context) error {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusProjectError)
	}

	org, err := sa.GetOrganization(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusProjectError)
	}

	openaiClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusInitError)
	}

	if _, err := openaiClient.ServiceAccounts.Delete(ctx, project.Status.ProjectID, *sa.Resource.Status.ServiceAccountID); err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusAPIError)
	}

	// Delete service account key from common secret
	aiPlatformConfig, err := v1beta1.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusConfigError)
	}

	if err := sa.deleteServiceAccountKeyFromSecret(ctx, aiPlatformConfig); err != nil {
		return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusSecretError)
	}

	return sa.updateServiceAccountCondition(ctx, v1beta1.ServiceAccountStatusDeleted)
}
