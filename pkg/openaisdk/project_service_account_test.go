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

func TestProjectServiceAccountService_Create(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	createReq := openaisdk.ProjectServiceAccountCreateRequest{
		Name: "test-service-account",
	}
	resp, err := client.ServiceAccounts.Create(context.Background(), "proj-123", createReq)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "sa-123", resp.ID)
	assert.Equal(t, "organization.project.service_account", resp.Object)
	assert.Equal(t, "test-service-account", resp.Name)
	assert.Equal(t, "member", resp.Role)

	// Verify API key was returned
	require.NotNil(t, resp.APIKey)
	assert.Equal(t, "key-123", resp.APIKey.ID)
	assert.Equal(t, "organization.project.service_account.api_key", resp.APIKey.Object)
	assert.Equal(t, "test-service-account", resp.APIKey.Name)
	assert.Equal(t, "test-api-key-value", resp.APIKey.Value)
}

func TestProjectServiceAccountService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ServiceAccounts.List(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "sa-123", resp.Data[0].ID)
	assert.Equal(t, "test-sa-name", resp.Data[0].Name)
	assert.Equal(t, "member", resp.Data[0].Role)
	assert.Equal(t, "organization.project.service_account", resp.Data[0].Object)
}

func TestProjectServiceAccountService_Get(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ServiceAccounts.Get(context.Background(), "proj-123", "sa-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "sa-123", resp.ID)
	assert.Equal(t, "test-sa-name", resp.Name)
	assert.Equal(t, "member", resp.Role)
	assert.Equal(t, "organization.project.service_account", resp.Object)
}

func TestProjectServiceAccountService_Delete(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ServiceAccounts.Delete(context.Background(), "proj-123", "sa-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "sa-123", resp.ID)
	assert.Equal(t, "organization.project.service_account", resp.Object)
	assert.True(t, resp.Deleted)
}

func TestProjectServiceAccountService_Create_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	createReq := openaisdk.ProjectServiceAccountCreateRequest{
		Name: "test-service-account",
	}
	_, err := client.ServiceAccounts.Create(context.Background(), "", createReq)

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectServiceAccountService_List_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ServiceAccounts.List(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectServiceAccountService_Get_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ServiceAccounts.Get(context.Background(), "", "sa-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty service account ID
	_, err = client.ServiceAccounts.Get(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required serviceAccountID parameter")
}

func TestProjectServiceAccountService_Delete_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ServiceAccounts.Delete(context.Background(), "", "sa-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty service account ID
	_, err = client.ServiceAccounts.Delete(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required serviceAccountID parameter")
}
