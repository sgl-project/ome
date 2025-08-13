package common

import (
	"context"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewRateLimit returns a new RateLimit resource handler based on the organization vendor
func NewRateLimit(ctx context.Context, c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, rateLimit *v1beta1.RateLimit) (RateLimitOperation, error) {
	// Get the organization from the rate limit
	org, err := GetOrganizationFromRateLimit(ctx, c, rateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	var vendor v1beta1.Vendor
	if org.Spec.Vendor != nil {
		vendor = *org.Spec.Vendor
	} else {
		vendor = v1beta1.VendorOpenAI
	}

	switch vendor {
	case v1beta1.VendorOpenAI:
		return NewOpenAIRateLimit(c, cs, log, scheme, rateLimit), nil
	case v1beta1.VendorGoogle:
		return NewGeminiRateLimit(c, cs, log, scheme, rateLimit), nil
	case v1beta1.VendorXAI:
		return NewXAIRateLimit(c, cs, log, scheme, rateLimit), nil
	default:
		return nil, fmt.Errorf("unsupported vendor %s", vendor)
	}
}

// updateCondition adds or updates a condition in the rate limit status
func (r *ResourceBase) updateRateLimitCondition(ctx context.Context, rateLimit *v1beta1.RateLimit, status v1beta1.RateLimitStatusReason) error {
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
		Message:            r.getRateLimitStatusMessage(status),
		LastTransitionTime: now,
		ObservedGeneration: rateLimit.Generation,
	}

	// Update or append the condition
	found := false
	for i, c := range rateLimit.Status.Conditions {
		if c.Type == conditionType {
			rateLimit.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		rateLimit.Status.Conditions = append(rateLimit.Status.Conditions, condition)
	}

	if err := r.Client.Status().Update(ctx, rateLimit); err != nil {
		return fmt.Errorf("failed to update rate limit status: %w", err)
	}
	return nil
}

// updateConditionWithError updates the status condition and returns the original error
// to ensure proper error propagation for reconciliation requeuing
func (r *ResourceBase) updateRateLimitConditionWithError(ctx context.Context, rateLimit *v1beta1.RateLimit, status v1beta1.RateLimitStatusReason, originalErr error) error {
	// Update the status
	if err := r.updateRateLimitCondition(ctx, rateLimit, status); err != nil {
		// If status update fails, log it but return the original error as that's more important
		r.Log.Error(err, "Failed to update rate limit status", "name", rateLimit.Name)
	}

	// Always return the original error to ensure reconciliation is requeued
	return originalErr
}

// getStatusMessage returns a human-readable message for a given status
func (r *ResourceBase) getRateLimitStatusMessage(status v1beta1.RateLimitStatusReason) string {
	switch status {
	case v1beta1.RateLimitStatusCreated:
		return "Rate limit successfully created"
	default:
		return "Unknown status"
	}
}

// GetProjectFromRateLimit fetches the project for the rate limit
func GetProjectFromRateLimit(ctx context.Context, c client.Client, rateLimit *v1beta1.RateLimit) (*v1beta1.Project, error) {
	project := &v1beta1.Project{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: rateLimit.Spec.ProjectRef.Namespace,
		Name:      rateLimit.Spec.ProjectRef.Name,
	}, project); err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", rateLimit.Spec.ProjectRef.Name, err)
	}
	return project, nil
}

// GetOrganizationFromRateLimit fetches the organization for the rate limit
func GetOrganizationFromRateLimit(ctx context.Context, c client.Client, rateLimit *v1beta1.RateLimit) (*v1beta1.Organization, error) {
	// Get the project first
	project, err := GetProjectFromRateLimit(ctx, c, rateLimit)
	if err != nil {
		return nil, err
	}

	// Get the organization from the project
	org := &v1beta1.Organization{}
	if err := c.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", project.Spec.OrganizationRef.Name, err)
	}

	return org, nil
}
