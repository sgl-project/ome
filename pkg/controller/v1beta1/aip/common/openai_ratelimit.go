package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewOpenAIRateLimit creates a new RateLimit resource handler for OpenAI
func NewOpenAIRateLimit(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, rateLimit *v1beta1.RateLimit) *OpenAIRateLimit {
	return &OpenAIRateLimit{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: rateLimit,
	}
}

// OpenAIRateLimit implements RateLimitOperation for OpenAI
type OpenAIRateLimit struct {
	ResourceBase
	Resource *v1beta1.RateLimit
	// For testing purposes, allows injecting a mock client
	openAIClient *openaisdk.Client
}

// GetOpenAIClient initializes the OpenAI client with proper error handling
func (r *OpenAIRateLimit) GetOpenAIClient(ctx context.Context) (*openaisdk.Client, error) {
	// If a client is already set (for testing), return it
	if r.openAIClient != nil {
		return r.openAIClient, nil
	}

	org, err := GetOrganizationFromRateLimit(ctx, r.Client, r.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	openAIClient, err := r.InitializeClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}

	return openAIClient, nil
}

// SetOpenAIClient sets a custom OpenAI client for testing purposes
func (r *OpenAIRateLimit) SetOpenAIClient(client *openaisdk.Client) {
	r.openAIClient = client
}

// Create creates a new rate limit in OpenAI
func (r *OpenAIRateLimit) Create(ctx context.Context) error {
	r.Log.Info("Creating OpenAI rate limit", "name", r.Resource.Name)

	err := r.createOrUpdateRateLimit(ctx)
	if err != nil {
		return err
	}

	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusCreated)
}

// Update updates the rate limit in OpenAI
func (r *OpenAIRateLimit) Update(ctx context.Context) error {
	r.Log.Info("Updating OpenAI rate limit", "name", r.Resource.Name)

	err := r.createOrUpdateRateLimit(ctx)
	if err != nil {
		return err
	}

	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusUpdated)
}

// Delete deletes the rate limit from OpenAI
func (r *OpenAIRateLimit) Delete(ctx context.Context) error {
	r.Log.Info("Deleting OpenAI rate limit", "name", r.Resource.Name)
	return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusArchived)
}

func (r *OpenAIRateLimit) createOrUpdateRateLimit(ctx context.Context) error {
	rateLimitId := r.Resource.Spec.Config["rate_limit_id"]
	if rateLimitId == "" {
		return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusConfigError)
	}

	project, err := GetProjectFromRateLimit(ctx, r.Client, r.Resource)
	if err != nil {
		return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusInitError)
	}

	// Get project ID from the rate limit config or derive from project reference
	projectID := project.Status.ProjectId
	if projectID == "" {
		return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusInitError)
	}

	openAIClient, err := r.GetOpenAIClient(ctx)
	if err != nil {
		return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusInitError)
	}

	// Create request body from r.Resource.Spec.Limits
	requestBodyMap := make(map[string]interface{})
	for _, limit := range r.Resource.Spec.Limits {
		if limit.Name != "" {
			requestBodyMap[limit.Name] = limit.Limit
		}
	}

	// Convert map to JSON string
	requestBodyBytes, err := json.Marshal(requestBodyMap)
	if err != nil {
		return r.updateRateLimitCondition(ctx, r.Resource, v1beta1.RateLimitStatusConfigError)
	}
	requestBody := string(requestBodyBytes)

	// Call OpenAI API to update rate limits
	_, err = openAIClient.ProjectRateLimits.Update(ctx, projectID, rateLimitId, option.WithRequestBody("application/json", strings.NewReader(requestBody)))
	if err != nil {
		return r.updateRateLimitConditionWithError(ctx, r.Resource, v1beta1.RateLimitStatusAPIError, err)
	}

	return nil
}
