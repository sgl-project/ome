package common

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewProject returns a new Project
func NewProject(ctx context.Context, c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) (ResourceOperation, error) {
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
		return NewOpenAIProject(c, cs, log, scheme, project), nil
	case v1beta1.VendorGoogle:
		return NewGeminiProject(c, cs, log, scheme, project), nil
	case v1beta1.VendorXAI:
		return NewXAIProject(c, cs, log, scheme, project), nil
	case v1beta1.VendorBytePlus:
		return NewBytePlusProject(c, cs, log, scheme, project), nil
	default:
		return nil, fmt.Errorf("Unsupport vendor %s", *org.Spec.Vendor)
	}
}

// updateCondition adds or updates a condition in the project status
func (r *ResourceBase) updateCondition(ctx context.Context, p *v1beta1.Project, status v1beta1.ProjectStatusReason) error {
	now := metav1.NewTime(time.Now())
	conditionType := v1beta1.ConditionTypeReady
	conditionStatus := metav1.ConditionTrue

	if status.IsError() {
		conditionType = v1beta1.ConditionTypeError
	}

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             string(status),
		Message:            r.getStatusMessage(status),
		LastTransitionTime: now,
		ObservedGeneration: p.Generation,
	}

	// Update or append the condition
	found := false
	for i, c := range p.Status.Conditions {
		if c.Type == conditionType {
			p.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		p.Status.Conditions = append(p.Status.Conditions, condition)
	}

	if err := r.Client.Status().Update(ctx, p); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}
	return nil
}

// updateConditionWithError updates the status condition and returns the original error
// to ensure proper error propagation for reconciliation requeuing
func (r *ResourceBase) updateConditionWithError(ctx context.Context, p *v1beta1.Project, status v1beta1.ProjectStatusReason, originalErr error) error {
	// Update the status
	if err := r.updateCondition(ctx, p, status); err != nil {
		// If status update fails, log it but return the original error as that's more important
		r.Log.Error(err, "Failed to update project status", "name", p.Name)
	}

	// Always return the original error to ensure reconciliation is requeued
	return originalErr
}

// getStatusMessage returns a human-readable message for a given status
func (r *ResourceBase) getStatusMessage(status v1beta1.ProjectStatusReason) string {
	switch status {
	case v1beta1.ProjectStatusCreated:
		return "Project successfully created"
	case v1beta1.ProjectStatusUpdated:
		return "Project successfully updated"
	case v1beta1.ProjectStatusArchived:
		return "Project successfully archived"
	case v1beta1.ProjectStatusInitError:
		return "Failed to initialize project"
	case v1beta1.ProjectStatusAPIError:
		return "API operation failed"
	case v1beta1.ProjectStatusOrgError:
		return "Organization operation failed"
	case v1beta1.ProjectStatusConfigError:
		return "Configuration operation failed"
	default:
		return "Unknown status"
	}
}

// GetOrganizationRef implements OrganizationScoped
func (r *ResourceBase) GetOrganizationRef(p *v1beta1.Project) *v1beta1.CrossReference {
	return &p.Spec.OrganizationRef
}

// GetOrganization fetches the organization for the project
func (r *ResourceBase) GetOrganizationFromProject(ctx context.Context, p *v1beta1.Project) (*v1beta1.Organization, error) {
	org := &v1beta1.Organization{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: p.Spec.OrganizationRef.Name}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", p.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}
