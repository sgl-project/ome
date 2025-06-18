package xaisdk_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testing_pkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/option"
)

func TestApiKeyService_List(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockXAIServer()
	defer server.Close()

	// Create client
	client := xaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	resp, err := client.APIKeys.List(context.Background(), "proj-123")

	// Verify
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.ApiKeys))
	assert.Equal(t, "key-123", resp.ApiKeys[0].ApiKey)
	assert.Equal(t, "test-api-key", resp.ApiKeys[0].Name)
}
func TestApiKeyService_Delete(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockXAIServer()
	defer server.Close()

	// Create client
	client := xaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test
	_, err := client.APIKeys.Delete(context.Background(), "key-123")

	// Verify
	require.NoError(t, err)
}

func TestApiKeyService_Delete_Error(t *testing.T) {
	// Setup mock server
	server := testing_pkg.MockXAIServer()
	defer server.Close()

	// Create client
	client := xaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Test with empty API key ID
	_, err := client.APIKeys.Delete(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required apiKeyID parameter")
}
