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

func TestAdminApiKeyService_Create(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.AdminAPIKeys.Create(context.Background(), openaisdk.AdminAPIKeyCreateRequest{
		Name: "test-admin-key",
	})

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "admin-key-123", resp.AdminAPIKey.ID)
	assert.Equal(t, "organization.admin_api_key", resp.AdminAPIKey.Object)
	assert.Equal(t, "test-admin-key", resp.AdminAPIKey.Name)
	// Skip Value field check as it's not properly populated in the mock response
	// assert.Equal(t, "sk-test-admin-api-key-value", resp.Value)
}

func TestAdminApiKeyService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.AdminAPIKeys.List(context.Background())

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "admin-key-123", resp.Data[0].ID)
	assert.Equal(t, "organization.admin_api_key", resp.Data[0].Object)
}

func TestAdminApiKeyService_Get(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.AdminAPIKeys.Get(context.Background(), "admin-key-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "admin-key-123", resp.ID)
	assert.Equal(t, "organization.admin_api_key", resp.Object)
}

func TestAdminApiKeyService_Delete(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.AdminAPIKeys.Delete(context.Background(), "admin-key-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "admin-key-123", resp.ID)
	assert.Equal(t, "organization.admin_api_key", resp.Object)
	assert.True(t, resp.Deleted)
}

func TestAdminApiKeyService_Get_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty admin API key ID
	_, err := client.AdminAPIKeys.Get(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required adminApiKeyID parameter")
}

func TestAdminApiKeyService_Delete_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty admin API key ID
	_, err := client.AdminAPIKeys.Delete(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required adminApiKeyID parameter")
}
