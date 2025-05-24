package examples

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	testing_pkg "github.com/sgl-project/sgl-ome/pkg/testing"
)

// TestMockOpenAIServer demonstrates how to use the mock OpenAI server for testing
func TestMockOpenAIServer(t *testing.T) {
	// Create a mock server
	server := testing_pkg.MockOpenAIServer()
	defer server.Close()

	// Create a client that uses the mock server
	client := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test project creation
	projectResp, err := client.Projects.Create(context.Background(), openaisdk.ProjectCreateRequest{
		Name: "test-project",
	})
	require.NoError(t, err)
	assert.Equal(t, "proj-123", projectResp.ID)
	assert.Equal(t, "test-project", projectResp.Name)
	assert.Equal(t, "active", projectResp.Status)

	// Test project retrieval
	project, err := client.Projects.Get(context.Background(), "proj-123")
	require.NoError(t, err)
	assert.Equal(t, "proj-123", project.ID)
	assert.Equal(t, "test-project-name", project.Name)
	assert.Equal(t, "active", project.Status)

	// Test project update
	updatedProject, err := client.Projects.Update(context.Background(), "proj-123", openaisdk.ProjectUpdateRequest{
		Name: "updated-project-name",
	})
	require.NoError(t, err)
	assert.Equal(t, "proj-123", updatedProject.ID)
	assert.Equal(t, "updated-project-name", updatedProject.Name)

	// Test service account creation
	saResp, err := client.ServiceAccounts.Create(context.Background(), "proj-123", openaisdk.ProjectServiceAccountCreateRequest{
		Name: "test-sa",
	})
	require.NoError(t, err)
	assert.Equal(t, "sa-123", saResp.ProjectServiceAccount.ID)
	assert.Equal(t, "test-sa", saResp.ProjectServiceAccount.Name)
	assert.Equal(t, "key-123", saResp.APIKey.ID)
	assert.Equal(t, "test-api-key-value", saResp.APIKey.Value)

	// Test service account deletion
	deleteResp, err := client.ServiceAccounts.Delete(context.Background(), "proj-123", "sa-123")
	require.NoError(t, err)
	assert.Equal(t, "sa-123", deleteResp.ID)
	assert.True(t, deleteResp.Deleted)

	// Test project archiving
	archivedProject, err := client.Projects.Archive(context.Background(), "proj-123")
	require.NoError(t, err)
	assert.Equal(t, "proj-123", archivedProject.ID)
	assert.Equal(t, "archived", archivedProject.Status)
	assert.NotNil(t, archivedProject.ArchivedAt)
}
