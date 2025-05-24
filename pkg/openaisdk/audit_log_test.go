package openaisdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogService_List(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/organization/audit_logs", r.URL.Path)

		// Return mock response
		mockResponse := AuditLogListResponse{
			Object:  "list",
			FirstID: "auditlog_abc123",
			LastID:  "auditlog_xyz789",
			HasMore: false,
			Data: []AuditLog{
				{
					ID:          "auditlog_abc123",
					Type:        "api_key.created",
					EffectiveAt: time.Now().Unix(),
					Actor: &Actor{
						Type: "user",
						Session: &Session{
							IpAddress: "192.168.1.1",
							User: &User{
								ID:    "user_123",
								Email: "user1@example.com",
							},
						},
					},
					ApiKeyCreated: &ApiKeyCreated{
						ID: "apikey_123",
						Data: &Data{
							Scopes: []string{"read", "write"},
						},
					},
				},
				{
					ID:          "auditlog_xyz789",
					Type:        "user.added",
					EffectiveAt: time.Now().Unix(),
					Actor: &Actor{
						Type: "user",
						Session: &Session{
							IpAddress: "192.168.1.2",
							User: &User{
								ID:    "user_456",
								Email: "user2@example.com",
							},
						},
					},
					UserAdded: &UserAdded{
						ID: "user_789",
						Data: &UserAddedData{
							Role: "member",
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(mockResponse)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := &http.Client{}
	service := NewAuditLogService(option.WithBaseURL(server.URL), option.WithHTTPClient(client))

	// Call the List method
	ctx := context.Background()
	response, err := service.List(ctx)

	// Verify results
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "list", response.Object)
	assert.Equal(t, "auditlog_abc123", response.FirstID)
	assert.Equal(t, "auditlog_xyz789", response.LastID)
	assert.False(t, response.HasMore)
	assert.Len(t, response.Data, 2)

	// Verify first audit log
	log1 := response.Data[0]
	assert.Equal(t, "auditlog_abc123", log1.ID)
	assert.Equal(t, "api_key.created", log1.Type)
	assert.NotNil(t, log1.Actor)
	assert.Equal(t, "user", log1.Actor.Type)
	assert.NotNil(t, log1.Actor.Session)
	assert.Equal(t, "192.168.1.1", log1.Actor.Session.IpAddress)
	assert.NotNil(t, log1.Actor.Session.User)
	assert.Equal(t, "user_123", log1.Actor.Session.User.ID)
	assert.Equal(t, "user1@example.com", log1.Actor.Session.User.Email)
	assert.NotNil(t, log1.ApiKeyCreated)
	assert.Equal(t, "apikey_123", log1.ApiKeyCreated.ID)
	assert.NotNil(t, log1.ApiKeyCreated.Data)
	assert.Equal(t, []string{"read", "write"}, log1.ApiKeyCreated.Data.Scopes)

	// Verify second audit log
	log2 := response.Data[1]
	assert.Equal(t, "auditlog_xyz789", log2.ID)
	assert.Equal(t, "user.added", log2.Type)
	assert.NotNil(t, log2.Actor)
	assert.Equal(t, "user", log2.Actor.Type)
	assert.NotNil(t, log2.Actor.Session)
	assert.Equal(t, "192.168.1.2", log2.Actor.Session.IpAddress)
	assert.NotNil(t, log2.Actor.Session.User)
	assert.Equal(t, "user_456", log2.Actor.Session.User.ID)
	assert.Equal(t, "user2@example.com", log2.Actor.Session.User.Email)
	assert.NotNil(t, log2.UserAdded)
	assert.Equal(t, "user_789", log2.UserAdded.ID)
	assert.NotNil(t, log2.UserAdded.Data)
	assert.Equal(t, "member", log2.UserAdded.Data.Role)
}

func TestAuditLogService_List_Error(t *testing.T) {
	// Setup test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API key",
				"type":    "authentication_error",
				"code":    "invalid_api_key",
			},
		})
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := &http.Client{}
	service := NewAuditLogService(option.WithBaseURL(server.URL), option.WithHTTPClient(client))

	// Call the List method
	ctx := context.Background()
	response, err := service.List(ctx)

	// Verify error
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestAuditLogService_List_EffectiveAtParams(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Return mock response
		mockResponse := AuditLogListResponse{
			Object:  "list",
			FirstID: "auditlog_time1",
			LastID:  "auditlog_time2",
			HasMore: false,
			Data: []AuditLog{
				{
					ID:          "auditlog_time1",
					Type:        "api_key.created",
					EffectiveAt: 1500,
				},
				{
					ID:          "auditlog_time2",
					Type:        "user.added",
					EffectiveAt: 1800,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(mockResponse)
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := &http.Client{}
	service := NewAuditLogService(option.WithBaseURL(server.URL), option.WithHTTPClient(client))

	// Call the List method
	ctx := context.Background()
	response, err := service.List(ctx)

	// Verify results
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, int64(1500), response.Data[0].EffectiveAt)
	assert.Equal(t, int64(1800), response.Data[1].EffectiveAt)
}
