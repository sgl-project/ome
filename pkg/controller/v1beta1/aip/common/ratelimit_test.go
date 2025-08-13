package common

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	testingpkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

// setupTestWithMockServerForRateLimit creates a test environment with a mocked OpenAI API server
func setupTestWithMockServerForRateLimit(t *testing.T) (*OpenAIRateLimit, *httptest.Server) {
	// Setup mock server
	server := testingpkg.MockOpenAIServer()

	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			SecretRef: &v1beta1.SecretReference{
				Name:      "test-secret",
				Namespace: "default",
				Key:       "api-key",
			},
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
		},
	}

	// Create test secret
	testSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-key": []byte("test-api-key"),
		},
	}

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-project",
			Generation: 1,
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
		Status: v1beta1.ProjectStatus{
			ProjectId: "proj-123",
		},
	}

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ratelimit",
			Generation: 1,
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
			TargetRef: v1beta1.CrossReference{
				Name:      "test-target",
				Namespace: "default",
			},
			Limits: []v1beta1.RateLimitConfig{
				{
					Type:   "requests",
					Limit:  100,
					Window: "1m",
				},
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret, testProject, testRateLimit).
		WithStatusSubresource(testRateLimit).
		Build()

	// Create rate limit handler
	rateLimit := NewOpenAIRateLimit(
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testRateLimit,
	)

	// Create a custom client that uses the mock server
	customClient := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Set the client on the rate limit
	rateLimit.SetOpenAIClient(customClient)

	return rateLimit, server
}

func TestNewRateLimit(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
		},
	}

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "default",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
			TargetRef: v1beta1.CrossReference{
				Name:      "test-target",
				Namespace: "default",
			},
			Limits: []v1beta1.RateLimitConfig{
				{
					Type:   "requests",
					Limit:  100,
					Window: "1m",
				},
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testProject, testRateLimit).
		Build()

	// Test OpenAI rate limit creation
	rateLimitOp, err := NewRateLimit(
		context.Background(),
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testRateLimit,
	)

	require.NoError(t, err)
	assert.NotNil(t, rateLimitOp)

	// Verify it's an OpenAI rate limit
	_, ok := rateLimitOp.(*OpenAIRateLimit)
	assert.True(t, ok, "Expected OpenAI rate limit implementation")
}

func TestNewRateLimit_UnsupportedVendor(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test organization with unsupported vendor
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr("unsupported"),
		},
	}

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "default",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
			TargetRef: v1beta1.CrossReference{
				Name:      "test-target",
				Namespace: "default",
			},
			Limits: []v1beta1.RateLimitConfig{
				{
					Type:   "requests",
					Limit:  100,
					Window: "1m",
				},
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testProject, testRateLimit).
		Build()

	// Test unsupported vendor
	_, err := NewRateLimit(
		context.Background(),
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testRateLimit,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported vendor")
}

func TestNewRateLimit_ProjectNotFound(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test rate limit with non-existent project
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "non-existent-project",
				Namespace: "default",
			},
			TargetRef: v1beta1.CrossReference{
				Name:      "test-target",
				Namespace: "default",
			},
			Limits: []v1beta1.RateLimitConfig{
				{
					Type:   "requests",
					Limit:  100,
					Window: "1m",
				},
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Test project not found
	_, err := NewRateLimit(
		context.Background(),
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testRateLimit,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

func TestOpenAIRateLimit_Create(t *testing.T) {
	rateLimit, server := setupTestWithMockServerForRateLimit(t)
	defer server.Close()

	// Test create
	err := rateLimit.Create(context.Background())
	require.NoError(t, err)

	// Verify status was updated
	assert.NotEmpty(t, rateLimit.Resource.Status.Conditions)
}

func TestOpenAIRateLimit_Update(t *testing.T) {
	rateLimit, server := setupTestWithMockServerForRateLimit(t)
	defer server.Close()

	// Test update
	err := rateLimit.Update(context.Background())
	require.NoError(t, err)

	// Verify status was updated
	assert.NotEmpty(t, rateLimit.Resource.Status.Conditions)
}

func TestOpenAIRateLimit_Delete(t *testing.T) {
	rateLimit, server := setupTestWithMockServerForRateLimit(t)
	defer server.Close()

	// Test delete
	err := rateLimit.Delete(context.Background())
	require.NoError(t, err)

	// Verify status was updated
	assert.NotEmpty(t, rateLimit.Resource.Status.Conditions)
}

func TestOpenAIRateLimit_GetOpenAIClient(t *testing.T) {
	rateLimit, server := setupTestWithMockServerForRateLimit(t)
	defer server.Close()

	// Test getting client
	client, err := rateLimit.GetOpenAIClient(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestOpenAIRateLimit_GetOpenAIClient_Error(t *testing.T) {
	// Setup without mock server
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test rate limit with missing organization
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ratelimit",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client without organization
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testRateLimit).
		Build()

	rateLimit := NewOpenAIRateLimit(
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testRateLimit,
	)

	// Test getting client with error
	_, err := rateLimit.GetOpenAIClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

func TestGetOrganizationFromRateLimit(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
		},
	}

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "default",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testProject, testRateLimit).
		Build()

	// Test getting organization
	org, err := GetOrganizationFromRateLimit(context.Background(), fakeClient, testRateLimit)
	require.NoError(t, err)
	assert.NotNil(t, org)
	assert.Equal(t, "test-org", org.Name)
}

func TestGetOrganizationFromRateLimit_ProjectNotFound(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test rate limit with non-existent project
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "non-existent-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Test project not found
	_, err := GetOrganizationFromRateLimit(context.Background(), fakeClient, testRateLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

func TestGetOrganizationFromRateLimit_OrganizationNotFound(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test project with non-existent organization
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "default",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "non-existent-org",
			},
		},
	}

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ratelimit",
			Namespace: "default",
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject, testRateLimit).
		Build()

	// Test organization not found
	_, err := GetOrganizationFromRateLimit(context.Background(), fakeClient, testRateLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

func TestResourceBase_updateRateLimitCondition(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ratelimit",
			Generation: 1,
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testRateLimit).
		WithStatusSubresource(testRateLimit).
		Build()

	// Create resource base
	resourceBase := &ResourceBase{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: scheme,
	}

	// Test updating condition
	err := resourceBase.updateRateLimitCondition(context.Background(), testRateLimit, v1beta1.RateLimitStatusCreated)
	require.NoError(t, err)

	// Verify condition was added
	assert.NotEmpty(t, testRateLimit.Status.Conditions)
	assert.Equal(t, "Ready", testRateLimit.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, testRateLimit.Status.Conditions[0].Status)
}

func TestResourceBase_updateRateLimitConditionWithError(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	// Create test rate limit
	testRateLimit := &v1beta1.RateLimit{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ratelimit",
			Generation: 1,
		},
		Spec: v1beta1.RateLimitSpec{
			ProjectRef: v1beta1.CrossReference{
				Name:      "test-project",
				Namespace: "default",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testRateLimit).
		WithStatusSubresource(testRateLimit).
		Build()

	// Create resource base
	resourceBase := &ResourceBase{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: scheme,
	}

	// Test updating condition with error
	originalErr := errors.New("test error")
	err := resourceBase.updateRateLimitConditionWithError(context.Background(), testRateLimit, v1beta1.RateLimitStatusCreated, originalErr)

	// Should return the original error
	assert.Equal(t, originalErr, err)
}

func TestResourceBase_getRateLimitStatusMessage(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	resourceBase := &ResourceBase{
		Client: nil,
		Log:    logr.Discard(),
		Scheme: scheme,
	}

	// Test known status
	message := resourceBase.getRateLimitStatusMessage(v1beta1.RateLimitStatusCreated)
	assert.Equal(t, "Rate limit successfully created", message)

	// Test unknown status
	message = resourceBase.getRateLimitStatusMessage("unknown")
	assert.Equal(t, "Unknown status", message)
}
