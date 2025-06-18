package testing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/option"
)

// Int64Ptr Helper function for testing

// MockXAIServer creates a test server that mocks XAI API responses
// It handles common operations like project and service account management
func MockXAIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		now := time.Now().String()
		// Handle API keys
		if strings.Contains(r.URL.Path, "/api-keys") {
			apiKeyID := "key-123"
			// Extract API key ID if it's in the path
			if parts := strings.Split(r.URL.Path, "/api-keys/"); len(parts) > 1 {
				if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
					apiKeyID = idParts[0]
				}
			}

			switch r.Method {
			case http.MethodPost:
				var createReq xaisdk.CreateApiKeyBody
				if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				response := xaisdk.APIKey{
					ApiKeyId:      "key-123",
					Name:          createReq.Name,
					ACLStrings:    createReq.ACLStrings,
					ApiKey:        "test-api-key-value",
					RedactedValue: "test-api-key-value",
					CreatedAt:     now,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				return

			case http.MethodGet:
				// Handle list API keys
				response := xaisdk.APIKeyListResponse{
					ApiKeys: []xaisdk.APIKey{
						{
							ApiKey:        apiKeyID,
							Name:          "test-api-key",
							RedactedValue: "sk-...XXXX",
							CreatedAt:     time.Now().Format(time.RFC3339),
							UserId:        "user-id-123",
						},
					},
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				return

			case http.MethodDelete:
				// Handle delete API key
				response := xaisdk.APIKeyDeleteResponse{}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				return
			}
		}

	}))
}

// NewMockXAIClient creates a new XAI client that uses the mock server
func NewMockXAIClient(server *httptest.Server) *xaisdk.Client {
	return xaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)
}
