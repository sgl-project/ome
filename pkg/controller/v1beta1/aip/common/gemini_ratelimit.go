package common

import (
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewGeminiRateLimit creates a new RateLimit resource handler for Gemini
func NewGeminiRateLimit(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, rateLimit *v1beta1.RateLimit) *GeminiRateLimit {
	return &GeminiRateLimit{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: rateLimit,
	}
}

// GeminiRateLimit implements RateLimitOperation for Gemini
type GeminiRateLimit struct {
	ResourceBase
	Resource *v1beta1.RateLimit
}

// Create creates a new rate limit in Gemini
func (r *GeminiRateLimit) Create(ctx context.Context) error {
	// TODO: Implement Gemini rate limit creation
	r.Log.Info("Creating Gemini rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusCreated)
}

// Update updates the rate limit in Gemini
func (r *GeminiRateLimit) Update(ctx context.Context) error {
	// TODO: Implement Gemini rate limit update
	r.Log.Info("Updating Gemini rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusUpdated)
}

// Delete deletes the rate limit from Gemini
func (r *GeminiRateLimit) Delete(ctx context.Context) error {
	// TODO: Implement Gemini rate limit deletion
	r.Log.Info("Deleting Gemini rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusArchived)
}
