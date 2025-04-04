package common

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// XAIServiceAccount implements ProjectScoped and ResourceOperation
type XAIServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
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

// Create implements ResourceOperation
func (sa *XAIServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
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

	// Check if ProjectID is available
	if project.Status.ProjectID == "" {
		sa.Log.Info("ProjectID is not available yet for service account", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError,
			fmt.Errorf("project ID not available for project %s", project.Name))
	}

	// Add service account key to common secret
	aiPlatformConfig, err := v1beta1.NewAIPlatformConfig(sa.Clientset)
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

	serviceAccountID := GenerateId("user_", sa.Resource.UID)
	// Update status
	creationTime := metav1.NewTime(time.Now())
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountID: &serviceAccountID,
		CreationTime:     &creationTime,
	}
	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
}

// Delete implements ResourceOperation
func (sa *XAIServiceAccount) Delete(ctx context.Context) error {
	_, err := sa.GetProject(ctx, sa.Resource)
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

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
}

// Update implements ServiceAccountOperation
func (sa *XAIServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}
