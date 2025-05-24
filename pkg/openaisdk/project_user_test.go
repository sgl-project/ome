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

func TestProjectUserService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ProjectUsers.List(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "user-123", resp.Data[0].ID)
	assert.Equal(t, "organization.project.user", resp.Data[0].Object)
	assert.Equal(t, "test@example.com", resp.Data[0].Email)
	assert.Equal(t, "member", resp.Data[0].Role)
}

func TestProjectUserService_Create(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ProjectUsers.Create(context.Background(), "proj-123", openaisdk.ProjectUserCreateRequest{
		UserID: "user-456",
		Role:   "owner",
	})

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "user-123", resp.ID)
	assert.Equal(t, "organization.project.user", resp.Object)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.Equal(t, "owner", resp.Role)
}

func TestProjectUserService_Delete(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.ProjectUsers.Delete(context.Background(), "proj-123", "user-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "user-123", resp.ID)
	assert.Equal(t, "organization.project.user", resp.Object)
	assert.True(t, resp.Deleted)
}

func TestProjectUserService_List_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ProjectUsers.List(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectUserService_Create_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ProjectUsers.Create(context.Background(), "", openaisdk.ProjectUserCreateRequest{
		UserID: "user-456",
		Role:   "owner",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectUserService_Delete_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.ProjectUsers.Delete(context.Background(), "", "user-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")

	// Test with empty user ID
	_, err = client.ProjectUsers.Delete(context.Background(), "proj-123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required userID parameter")
}
