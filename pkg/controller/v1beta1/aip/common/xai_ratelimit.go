package common

import (
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewXAIRateLimit creates a new RateLimit resource handler for XAI
func NewXAIRateLimit(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, rateLimit *v1beta1.RateLimit) *XAIRateLimit {
	return &XAIRateLimit{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: rateLimit,
	}
}

// XAIRateLimit implements RateLimitOperation for XAI
type XAIRateLimit struct {
	ResourceBase
	Resource *v1beta1.RateLimit
}

// Create creates a new rate limit in XAI
func (r *XAIRateLimit) Create(ctx context.Context) error {
	r.Log.Info("Creating XAI rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusCreated)
}

// Update updates the rate limit in XAI
func (r *XAIRateLimit) Update(ctx context.Context) error {
	// TODO: Implement XAI rate limit update
	r.Log.Info("Updating XAI rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusUpdated)
}

// Delete deletes the rate limit from XAI
func (r *XAIRateLimit) Delete(ctx context.Context) error {
	// TODO: Implement XAI rate limit deletion
	r.Log.Info("Deleting XAI rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusArchived)
}
