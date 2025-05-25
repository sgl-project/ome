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

func TestApiKeyService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.APIKeys.List(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "key-123", resp.Data[0].ID)
	assert.Equal(t, "test-api-key", resp.Data[0].Name)
}

func TestApiKeyService_Get(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.APIKeys.Get(context.Background(), "proj-123", "key-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "key-123", resp.ID)
	assert.Equal(t, "test-api-key", resp.Name)
	assert.Equal(t, "organization.project.api_key", resp.Object)
}

func TestApiKeyService_Delete(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.APIKeys.Delete(context.Background(), "proj-123", "key-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "key-123", resp.ID)
	assert.Equal(t, "organization.project.api_key", resp.Object)
	assert.True(t, resp.Deleted)
}

func TestApiKeyService_List_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.APIKeys.List(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestApiKeyService_Get_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.APIKeys.Get(context.Background(), "", "key-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty API key ID
	_, err = client.APIKeys.Get(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required apiKeyID parameter")
}

func TestApiKeyService_Delete_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.APIKeys.Delete(context.Background(), "", "key-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty API key ID
	_, err = client.APIKeys.Delete(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required apiKeyID parameter")
}
