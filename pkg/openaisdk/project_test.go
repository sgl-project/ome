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

func TestProjectService_Create(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	createReq := openaisdk.ProjectCreateRequest{
		Name: "test-project-name",
	}
	resp, err := client.Projects.Create(context.Background(), createReq)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "proj-123", resp.ID)
	assert.Equal(t, "organization.project", resp.Object)
	assert.Equal(t, "test-project-name", resp.Name)
	assert.Equal(t, "active", resp.Status)
}

func TestProjectService_Get(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.Projects.Get(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "proj-123", resp.ID)
	assert.Equal(t, "organization.project", resp.Object)
	assert.Equal(t, "test-project-name", resp.Name)
	assert.Equal(t, "active", resp.Status)
}

func TestProjectService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.Projects.List(context.Background())

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, "proj-123", resp.Data[0].ID)
	assert.Equal(t, "test-project-name", resp.Data[0].Name)
	assert.Equal(t, "active", resp.Data[0].Status)
}

func TestProjectService_Update(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	updateReq := openaisdk.ProjectUpdateRequest{
		Name: "updated-project-name",
	}
	resp, err := client.Projects.Update(context.Background(), "proj-123", updateReq)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "proj-123", resp.ID)
	assert.Equal(t, "organization.project", resp.Object)
	assert.Equal(t, "updated-project-name", resp.Name)
	assert.Equal(t, "active", resp.Status)
}

func TestProjectService_Archive(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.Projects.Archive(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, "proj-123", resp.ID)
	assert.Equal(t, "organization.project", resp.Object)
	assert.Equal(t, "test-project-name", resp.Name)
	assert.Equal(t, "archived", resp.Status)
	assert.NotNil(t, resp.ArchivedAt)
}

func TestProjectService_Get_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.Projects.Get(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectService_Update_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	updateReq := openaisdk.ProjectUpdateRequest{
		Name: "updated-project-name",
	}
	_, err := client.Projects.Update(context.Background(), "", updateReq)

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}

func TestProjectService_Archive_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create client
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty project ID
	_, err := client.Projects.Archive(context.Background(), "")

	// Verify
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required projectID parameter")
}
