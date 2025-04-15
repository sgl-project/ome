package common

import (
	"context"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// NewServiceAccount returns a new ServiceAccount
func NewServiceAccount(ctx context.Context, c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) (ServiceAccountOperation, error) {
	project := &v1beta1.Project{}
	if err := c.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name}, project); err != nil {
		// For NotFound errors, log without stack trace as this is expected in some cases
		if apierrors.IsNotFound(err) {
			log.Info("Project not found",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		} else {
			// For unexpected errors, log with more details
			log.Error(err, "Failed to get project",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	org := &v1beta1.Organization{}
	if err := c.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", project.Spec.OrganizationRef.Name, err)
	}

	var vendor v1beta1.Vendor
	if org.Spec.Vendor != nil {
		vendor = *org.Spec.Vendor
	} else {
		vendor = v1beta1.VendorOpenAI
	}

	switch vendor {
	case v1beta1.VendorOpenAI:
		return NewOpenAIServiceAccount(c, cs, log, scheme, sa), nil
	case v1beta1.VendorGoogle:
		return NewGeminiServiceAccount(c, cs, log, scheme, sa), nil
	case v1beta1.VendorXAI:
		return NewXAIServiceAccount(c, cs, log, scheme, sa), nil
	default:
		return nil, fmt.Errorf("Unsupport vendor %s", *org.Spec.Vendor)
	}
}

// updateServiceAccountConditionWithError updates the status conditions and returns the original error
// to ensure proper error propagation for reconciliation requeuing
func (r *ResourceBase) updateServiceAccountConditionWithError(ctx context.Context, sa *v1beta1.ServiceAccount, status v1beta1.ServiceAccountStatusReason, originalErr error) error {
	// Update the status
	if err := r.updateServiceAccountCondition(ctx, sa, status); err != nil {
		// If status update fails, log it but return the original error as that's more important
		r.Log.Error(err, "Failed to update service account status", "name", sa.Name, "namespace", sa.Namespace)
	}

	// Always return the original error to ensure reconciliation is requeued
	return originalErr
}

// updateServiceAccountCondition updates the status conditions for the ServiceAccount
func (r *ResourceBase) updateServiceAccountCondition(ctx context.Context, sa *v1beta1.ServiceAccount, status v1beta1.ServiceAccountStatusReason) error {
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
		Message:            r.getServiceAccountStatusMessage(status),
		LastTransitionTime: now,
		ObservedGeneration: sa.Generation,
	}

	// Update or append the condition
	found := false
	for i, c := range sa.Status.Conditions {
		if c.Type == conditionType {
			sa.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		sa.Status.Conditions = append(sa.Status.Conditions, condition)
	}

	return r.Client.Status().Update(ctx, sa)
}

// getStatusMessage returns a human-readable message for a given status
func (r *ResourceBase) getServiceAccountStatusMessage(status v1beta1.ServiceAccountStatusReason) string {
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
func (r *ResourceBase) getCommonSecret(ctx context.Context, config *controllerconfig.AIPlatformConfig) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: config.SecretConfig.SecretName, Namespace: config.SecretConfig.Namespace}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get common secret: %w", err)
	}
	return secret, nil
}

// Update implements ServiceAccountOperation
func (r *ResourceBase) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return r.Client.Update(ctx, resource)
}

// GetProject retrieves the project associated with the service account
func (r *ResourceBase) GetProject(ctx context.Context, sa *v1beta1.ServiceAccount) (*v1beta1.Project, error) {
	project := &v1beta1.Project{}
	if err := r.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name}, project); err != nil {
		// For NotFound errors, log without stack trace as this is expected in some cases
		if apierrors.IsNotFound(err) {
			r.Log.Info("Project not found",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		} else {
			// For unexpected errors, log with more details
			r.Log.Error(err, "Failed to get project",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return project, nil
}

// GetOrganization retrieves the organization associated with the project
func (r *ResourceBase) GetOrganization(ctx context.Context, sa *v1beta1.ServiceAccount) (*v1beta1.Organization, error) {
	project, err := r.GetProject(ctx, sa)
	if err != nil {
		return nil, err
	}

	org := &v1beta1.Organization{}
	if err := r.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, org); err != nil {
		r.Log.Error(err, "failed to get organization from service account: ", "name", sa.Name, "namespace", sa.Namespace)
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// deleteServiceAccountKeyFromSecret removes a key from the common secret
func (r *ResourceBase) deleteServiceAccountKeyFromSecret(ctx context.Context, sa *v1beta1.ServiceAccount, aiPlatformConfig *controllerconfig.AIPlatformConfig) error {
	// Safety check - ensure we have a service account ID
	if sa.Status.ServiceAccountId == nil {
		r.Log.Info("No service account ID found, skipping key deletion from common secret",
			"name", sa.Name, "namespace", sa.Namespace)
		return nil
	}

	commonSecret, err := r.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		// If the secret doesn't exist, that's fine - nothing to delete
		if apierrors.IsNotFound(err) {
			r.Log.Info("Common secret not found, nothing to delete",
				"name", sa.Name, "namespace", sa.Namespace)
			return nil
		}
		return fmt.Errorf("failed to get common secret: %w", err)
	}

	// Check if the key exists in the secret data
	if commonSecret.Data == nil || len(commonSecret.Data[*sa.Status.ServiceAccountId]) == 0 {
		r.Log.Info("Service account key not found in common secret, nothing to delete",
			"name", sa.Name, "namespace", sa.Namespace,
			"serviceAccountID", *sa.Status.ServiceAccountId)
		return nil
	}

	// Delete the key from the secret
	delete(commonSecret.Data, *sa.Status.ServiceAccountId)
	return r.Client.Update(ctx, commonSecret)
}
