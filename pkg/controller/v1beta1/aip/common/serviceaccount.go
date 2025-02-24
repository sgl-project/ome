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
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// updateServiceAccountCondition updates the status conditions for the ServiceAccount
func (sa *ServiceAccount) updateServiceAccountCondition(ctx context.Context, conditionType, reason, message string, status metav1.ConditionStatus) error {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
		ObservedGeneration: sa.Resource.Generation,
	}
	sa.Resource.Status.Conditions = append(sa.Resource.Status.Conditions, condition)
	return sa.Client.Status().Update(ctx, sa.Resource)
}

// Create implements ResourceOperation
func (sa *ServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToGetProject", err.Error(), metav1.ConditionFalse)
	}

	org, err := sa.GetOrganization(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToGetOrganization", err.Error(), metav1.ConditionFalse)
	}

	openaiClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToInitializeClient", err.Error(), metav1.ConditionFalse)
	}

	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Create service account in OpenAI
	resp, err := openaiClient.ServiceAccounts.Create(ctx, project.Status.ProjectID, openaisdk.ProjectServiceAccountCreateRequest{Name: *sa.Resource.Spec.Name})
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToCreateServiceAccount", err.Error(), metav1.ConditionFalse)
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
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToCreateSecret", err.Error(), metav1.ConditionFalse)
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
	return sa.updateServiceAccountCondition(ctx, "Ready", "ServiceAccountCreated", "Service account successfully created", metav1.ConditionTrue)
}

// Update implements ResourceOperation (currently no-op)
func (sa *ServiceAccount) Update(ctx context.Context) error {
	return nil
}

// Delete implements ResourceOperation
func (sa *ServiceAccount) Delete(ctx context.Context) error {
	project, err := sa.GetProject(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToGetProject", err.Error(), metav1.ConditionFalse)
	}

	org, err := sa.GetOrganization(ctx)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToGetOrganization", err.Error(), metav1.ConditionFalse)
	}

	openaiClient, err := sa.InitializeClient(ctx, org)
	if err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToInitializeClient", err.Error(), metav1.ConditionFalse)
	}

	// Delete service account in OpenAI
	if _, err := openaiClient.ServiceAccounts.Delete(ctx, project.Status.ProjectID, *sa.Resource.Status.ServiceAccountID); err != nil {
		return sa.updateServiceAccountCondition(ctx, "Error", "FailedToDeleteServiceAccount", err.Error(), metav1.ConditionFalse)
	}

	// Delete API key secret if it exists
	if sa.Resource.Status.APIKey != nil && sa.Resource.Status.APIKey.APIKeySecretRef != nil {
		secret := &v1.Secret{}
		secretKey := client.ObjectKey{
			Name:      sa.Resource.Status.APIKey.APIKeySecretRef.Name,
			Namespace: sa.Resource.Status.APIKey.APIKeySecretRef.Namespace,
		}

		if err := sa.Get(ctx, secretKey, secret); err == nil {
			if err := sa.Client.Delete(ctx, secret); err != nil {
				return fmt.Errorf("failed to delete secret: %w", err)
			}
		}
	}

	return nil
}
