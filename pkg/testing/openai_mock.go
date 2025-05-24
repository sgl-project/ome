package testing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// Int64Ptr Helper function for testing
func Int64Ptr(i int64) *int64 {
	return &i
}

// StringPtr Helper function for testing
func StringPtr(s string) *string {
	return &s
}

// MockOpenAIServer creates a test server that mocks OpenAI API responses
// It handles common operations like project and service account management
func MockOpenAIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		now := time.Now().Unix()

		// Handle admin API keys
		if strings.Contains(r.URL.Path, "/organization/admin_api_keys") {
			adminApiKeyID := "admin-key-123"
			// Extract admin API key ID if it's in the path
			if parts := strings.Split(r.URL.Path, "/organization/admin_api_keys/"); len(parts) > 1 {
				if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
					adminApiKeyID = idParts[0]
				}
			}

			switch r.Method {
			case http.MethodPost:
				// Handle admin API key creation
				var createReq openaisdk.AdminAPIKeyCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				adminAPIKey := openaisdk.AdminAPIKey{
					ID:            adminApiKeyID,
					Object:        "organization.admin_api_key",
					Name:          createReq.Name,
					RedactedValue: "sk-...XXXX",
					CreatedAt:     now,
					Owner:         "user",
				}

				response := struct {
					openaisdk.AdminAPIKey
					Value string `json:"value"`
				}{
					AdminAPIKey: adminAPIKey,
					Value:       "sk-test-admin-api-key-value",
				}

				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				return

			case http.MethodGet:
				// Handle get admin API key or list admin API keys
				if strings.Count(r.URL.Path, "/") > 2 {
					// Get specific admin API key
					response := openaisdk.AdminAPIKey{
						ID:            adminApiKeyID,
						Object:        "organization.admin_api_key",
						Name:          "test-admin-key",
						RedactedValue: "sk-...XXXX",
						CreatedAt:     now,
						Owner:         "user",
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				} else {
					// List admin API keys
					response := openaisdk.AdminAPIKeyListResponse{
						Object:  "list",
						FirstID: "admin-key-123",
						LastID:  "admin-key-123",
						HasMore: false,
						Data: []openaisdk.AdminAPIKey{
							{
								ID:            adminApiKeyID,
								Object:        "organization.admin_api_key",
								Name:          "test-admin-key",
								RedactedValue: "sk-...XXXX",
								CreatedAt:     now,
								Owner:         "user",
							},
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
				return

			case http.MethodDelete:
				// Handle delete admin API key
				response := openaisdk.AdminAPIKeyDeleteResponse{
					Object:  "organization.admin_api_key",
					ID:      adminApiKeyID,
					Deleted: true,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				return
			}
		}

		// Handle project operations
		if strings.Contains(r.URL.Path, "/organization/projects") {
			projectID := "proj-123"
			// Extract project ID from URL if it's in the path
			if parts := strings.Split(r.URL.Path, "/organization/projects/"); len(parts) > 1 {
				if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 {
					projectID = idParts[0]
				}
			}

			// Handle API keys
			if strings.Contains(r.URL.Path, "/api_keys") {
				apiKeyID := "key-123"
				// Extract API key ID if it's in the path
				if parts := strings.Split(r.URL.Path, "/api_keys/"); len(parts) > 1 {
					if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
						apiKeyID = idParts[0]
					}
				}

				switch r.Method {
				case http.MethodGet:
					// Handle get API key or list API keys
					if strings.Count(r.URL.Path, "/") > 4 {
						// Get specific API key
						response := openaisdk.APIKey{
							ID:            apiKeyID,
							Object:        "organization.project.api_key",
							Name:          "test-api-key",
							RedactedValue: "sk-...XXXX",
							CreatedAt:     now,
							Owner:         "user",
						}
						if err := json.NewEncoder(w).Encode(response); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					} else {
						// List API keys
						response := openaisdk.APIKeyListResponse{
							Object:  "list",
							FirstID: "key-123",
							LastID:  "key-123",
							HasMore: false,
							Data: []openaisdk.APIKey{
								{
									ID:            apiKeyID,
									Object:        "organization.project.api_key",
									Name:          "test-api-key",
									RedactedValue: "sk-...XXXX",
									CreatedAt:     now,
									Owner:         "user",
								},
							},
						}
						if err := json.NewEncoder(w).Encode(response); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					}
					return

				case http.MethodDelete:
					// Handle delete API key
					response := openaisdk.APIKeyDeleteResponse{
						Object:  "organization.project.api_key",
						ID:      apiKeyID,
						Deleted: true,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return
				}
			}

			// Handle rate limits
			if strings.Contains(r.URL.Path, "/rate_limits") {
				rateLimitID := "rate-limit-123"
				// Extract rate limit ID if it's in the path
				if parts := strings.Split(r.URL.Path, "/rate_limits/"); len(parts) > 1 {
					if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
						rateLimitID = idParts[0]
					}
				}

				switch r.Method {
				case http.MethodGet:
					// List rate limits
					response := openaisdk.ProjectRateLimitListResponse{
						Object:  "list",
						FirstID: "rate-limit-123",
						LastID:  "rate-limit-123",
						HasMore: false,
						Data: []openaisdk.ProjectRateLimit{
							{
								ID:                            rateLimitID,
								Object:                        "project.rate_limit",
								Model:                         "gpt-4",
								MaxRequestsPerOneMinute:       100,
								MaxTokensPerOneMinute:         10000,
								MaxImagesPerOneMinute:         10,
								MaxAudioMegabytesPerOneMinute: 100,
								MaxRequestsPerDay:             1000,
								BatchOneDayMaxInputTokens:     100000,
							},
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return

				case http.MethodPost:
					// Update rate limit
					response := openaisdk.ProjectRateLimit{
						ID:                            rateLimitID,
						Object:                        "project.rate_limit",
						Model:                         "gpt-4",
						MaxRequestsPerOneMinute:       200,
						MaxTokensPerOneMinute:         20000,
						MaxImagesPerOneMinute:         20,
						MaxAudioMegabytesPerOneMinute: 200,
						MaxRequestsPerDay:             2000,
						BatchOneDayMaxInputTokens:     200000,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return
				}
			}

			// Handle users
			if strings.Contains(r.URL.Path, "/users") {
				userID := "user-123"
				// Extract user ID if it's in the path
				if parts := strings.Split(r.URL.Path, "/users/"); len(parts) > 1 {
					if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
						userID = idParts[0]
					}
				}

				switch r.Method {
				case http.MethodPost:
					// Handle user creation
					var createReq openaisdk.ProjectUserCreateRequest
					if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					response := openaisdk.ProjectUser{
						ID:      userID,
						Object:  "organization.project.user",
						Name:    "Test User",
						Email:   "test@example.com",
						Role:    createReq.Role,
						AddedAt: now,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return

				case http.MethodGet:
					// List users
					response := openaisdk.ProjectUserListResponse{
						Object:  "list",
						FirstID: "user-123",
						LastID:  "user-123",
						HasMore: false,
						Data: []openaisdk.ProjectUser{
							{
								ID:      userID,
								Object:  "organization.project.user",
								Name:    "Test User",
								Email:   "test@example.com",
								Role:    "member",
								AddedAt: now,
							},
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return

				case http.MethodDelete:
					// Handle delete user
					response := openaisdk.ProjectUserDeleteResponse{
						Object:  "organization.project.user",
						ID:      userID,
						Deleted: true,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return
				}
			}

			// Handle archive endpoint specifically
			if strings.HasSuffix(r.URL.Path, "/archive") && r.Method == http.MethodPost {
				// Extract project ID from URL for archive endpoint
				projectID := "proj-123"
				if parts := strings.Split(r.URL.Path, "/organization/projects/"); len(parts) > 1 {
					if idParts := strings.Split(parts[1], "/archive"); len(idParts) > 0 && idParts[0] != "" {
						projectID = idParts[0]
					}
				}

				response := openaisdk.Project{
					ID:         projectID,
					Object:     "organization.project",
					Name:       "test-project-name",
					CreatedAt:  now,
					ArchivedAt: Int64Ptr(now),
					Status:     "archived",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				return
			}

			// Handle service account endpoints
			if strings.Contains(r.URL.Path, "/service_accounts") {
				serviceAccountID := "sa-123"
				// Extract service account ID if it's in the path
				if parts := strings.Split(r.URL.Path, "/service_accounts/"); len(parts) > 1 {
					if idParts := strings.Split(parts[1], "/"); len(idParts) > 0 && idParts[0] != "" {
						serviceAccountID = idParts[0]
					}
				}

				switch r.Method {
				case http.MethodPost:
					// Handle service account creation
					var createReq openaisdk.ProjectServiceAccountCreateRequest
					if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					response := openaisdk.ProjectServiceAccountCreateResponse{
						ProjectServiceAccount: openaisdk.ProjectServiceAccount{
							ID:        serviceAccountID,
							Object:    "organization.project.service_account",
							Name:      createReq.Name,
							Role:      "member",
							CreatedAt: now,
						},
						APIKey: &openaisdk.ProjectServiceAccountAPIKey{
							ID:        "key-123",
							Object:    "organization.project.service_account.api_key",
							Name:      createReq.Name,
							Value:     "test-api-key-value",
							CreatedAt: now,
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return
				case http.MethodGet:
					// Handle get service account or list service accounts
					if strings.Count(r.URL.Path, "/") > 4 {
						// Get specific service account
						response := openaisdk.ProjectServiceAccount{
							ID:        serviceAccountID,
							Object:    "organization.project.service_account",
							Name:      "test-sa-name",
							Role:      "member",
							CreatedAt: now,
						}
						if err := json.NewEncoder(w).Encode(response); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					} else {
						// List service accounts
						response := openaisdk.ProjectServiceAccountListResponse{
							Object:  "list",
							FirstID: "sa-123",
							LastID:  "sa-123",
							HasMore: false,
							Data: []openaisdk.ProjectServiceAccount{
								{
									ID:        serviceAccountID,
									Object:    "organization.project.service_account",
									Name:      "test-sa-name",
									Role:      "member",
									CreatedAt: now,
								},
							},
						}
						if err := json.NewEncoder(w).Encode(response); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					}
					return
				case http.MethodDelete:
					// Handle delete service account
					response := openaisdk.ProjectServiceAccountDeleteResponse{
						ID:      serviceAccountID,
						Object:  "organization.project.service_account",
						Deleted: true,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					return
				}
			}

			// Handle project operations
			switch r.Method {
			case http.MethodPost:
				// Handle project creation
				var createReq openaisdk.ProjectCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				response := openaisdk.Project{
					ID:        projectID,
					Object:    "organization.project",
					Name:      createReq.Name,
					CreatedAt: now,
					Status:    "active",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

			case http.MethodGet:
				// Handle get project or list projects
				if strings.Count(r.URL.Path, "/") > 2 {
					// Get specific project
					response := openaisdk.Project{
						ID:        projectID,
						Object:    "organization.project",
						Name:      "test-project-name",
						CreatedAt: now,
						Status:    "active",
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				} else {
					// List projects
					response := openaisdk.ProjectListResponse{
						Object:  "list",
						FirstID: "proj-123",
						LastID:  "proj-123",
						HasMore: false,
						Data: []openaisdk.Project{
							{
								ID:        projectID,
								Object:    "organization.project",
								Name:      "test-project-name",
								CreatedAt: now,
								Status:    "active",
							},
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}

			case http.MethodPatch:
				// Handle project update
				var updateReq openaisdk.ProjectUpdateRequest
				if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				response := openaisdk.Project{
					ID:        projectID,
					Object:    "organization.project",
					Name:      updateReq.Name,
					CreatedAt: now,
					Status:    "active",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

			case http.MethodDelete:
				// Handle project archive
				response := openaisdk.Project{
					ID:         projectID,
					Object:     "organization.project",
					Name:       "test-project-name",
					CreatedAt:  now,
					ArchivedAt: Int64Ptr(now),
					Status:     "archived",
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	}))
}

// NewMockOpenAIClient creates a new OpenAI client that uses the mock server
func NewMockOpenAIClient(server *httptest.Server) *openaisdk.Client {
	return openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)
}
