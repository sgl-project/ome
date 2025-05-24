package openaisdk_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	testing_pkg "github.com/sgl-project/sgl-ome/pkg/testing"
)

func TestProjectRateLimitService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ProjectRateLimits.List(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "rate-limit-123", resp.Data[0].ID)
	assert.Equal(t, "project.rate_limit", resp.Data[0].Object)
	assert.Equal(t, "gpt-4", resp.Data[0].Model)
	assert.Equal(t, 100, resp.Data[0].MaxRequestsPerOneMinute)
	assert.Equal(t, 10000, resp.Data[0].MaxTokensPerOneMinute)
}

func TestProjectRateLimitService_Update(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ProjectRateLimits.Update(context.Background(), "proj-123", "rate-limit-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "rate-limit-123", resp.ID)
	assert.Equal(t, "project.rate_limit", resp.Object)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Equal(t, 200, resp.MaxRequestsPerOneMinute)
	assert.Equal(t, 20000, resp.MaxTokensPerOneMinute)
}

func TestProjectRateLimitService_List_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ProjectRateLimits.List(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectRateLimitService_Update_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ProjectRateLimits.Update(context.Background(), "", "rate-limit-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty rate limit ID
	_, err = client.ProjectRateLimits.Update(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required rateLimitID parameter")
}
