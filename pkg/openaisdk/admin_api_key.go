package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/apijson"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// AdminApiKeyService contains methods for interacting with API keys
type AdminApiKeyService struct {
	Options []option.RequestOption
}

// NewAdminApiKeyService generates a new service that applies the given options to each request
func NewAdminApiKeyService(opts ...option.RequestOption) (r *AdminApiKeyService) {
	r = &AdminApiKeyService{}
	r.Options = opts
	return
}

// Create a new Admin API key
func (r *AdminApiKeyService) Create(ctx context.Context, body AdminAPIKeyCreateRequest, opts ...option.RequestOption) (res *AdminAPIKeyCreateResponse, err error) {
	opts = append(r.Options[:], opts...)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, "organization/admin_api_keys", body, &res, opts...)
	return
}

// List returns all API keys in the project
func (r *AdminApiKeyService) List(ctx context.Context, opts ...option.RequestOption) (res *AdminAPIKeyListResponse, err error) {
	opts = append(r.Options[:], opts...)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, "organization/admin_api_keys", nil, &res, opts...)
	return
}

// Get retrieves a specific API key
func (r *AdminApiKeyService) Get(ctx context.Context, adminApiKeyID string, opts ...option.RequestOption) (res *AdminAPIKey, err error) {
	opts = append(r.Options[:], opts...)
	if adminApiKeyID == "" {
		err = errors.New("missing required adminApiKeyID parameter")
		return
	}
	path := fmt.Sprintf("organization/admin_api_keys/%s", adminApiKeyID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Delete removes an API key from the project
func (r *AdminApiKeyService) Delete(ctx context.Context, adminApiKeyID string, opts ...option.RequestOption) (res *AdminAPIKeyDeleteResponse, err error) {
	opts = append(r.Options[:], opts...)
	if adminApiKeyID == "" {
		err = errors.New("missing required adminApiKeyID parameter")
		return
	}
	path := fmt.Sprintf("organization/admin_api_keys/%s", adminApiKeyID)
	err = option.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// APIKey represents an API key
type AdminAPIKey struct {
	// The object type, which is always "organization.project.api_key"
	Object string `json:"object"`
	// The API key value
	RedactedValue string `json:"redacted_value"`
	// The name of the API key
	Name string `json:"name"`
	// The Unix timestamp (in seconds) of when the key was created
	CreatedAt int64 `json:"created_at"`
	// The identifier of the key
	ID string `json:"id"`
	// The owner of the key, type struct defined in api_key.go
	Owner string          `json:"owner"`
	JSON  adminApiKeyJSON `json:"-"`
}

type adminApiKeyJSON struct {
	Object      apijson.Field
	Value       apijson.Field
	Name        apijson.Field
	CreatedAt   apijson.Field
	ID          apijson.Field
	Owner       apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *AdminAPIKey) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type AdminAPIKeyCreateRequest struct {
	// The name of the API key
	Name string `json:"name"`
}

type AdminAPIKeyCreateResponse struct {
	AdminAPIKey
	Value string `json:"value"`
}

// APIKeyListResponse represents a response from listing API keys
type AdminAPIKeyListResponse struct {
	Object  string              `json:"object"`
	Data    []AdminAPIKey       `json:"data"`
	FirstID string              `json:"first_id"`
	LastID  string              `json:"last_id"`
	HasMore bool                `json:"has_more"`
	Json    adminApiKeyListJSON `json:"-"`
}

type adminApiKeyListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *AdminAPIKeyListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type AdminAPIKeyDeleteResponse struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
