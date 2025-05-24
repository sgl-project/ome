package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/apijson"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// ApiKeyService contains methods for interacting with API keys
type ApiKeyService struct {
	Options []option.RequestOption
}

// NewApiKeyService generates a new service that applies the given options to each request
func NewApiKeyService(opts ...option.RequestOption) (r *ApiKeyService) {
	r = &ApiKeyService{}
	r.Options = opts
	return
}

// List returns all API keys in the project
func (r *ApiKeyService) List(ctx context.Context, projectID string, opts ...option.RequestOption) (res *APIKeyListResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/api_keys", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Get retrieves a specific API key
func (r *ApiKeyService) Get(ctx context.Context, projectID string, apiKeyID string, opts ...option.RequestOption) (res *APIKey, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if apiKeyID == "" {
		err = errors.New("missing required apiKeyID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/api_keys/%s", projectID, apiKeyID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Delete removes an API key from the project
func (r *ApiKeyService) Delete(ctx context.Context, projectID string, apiKeyID string, opts ...option.RequestOption) (res *APIKeyDeleteResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if apiKeyID == "" {
		err = errors.New("missing required apiKeyID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/api_keys/%s", projectID, apiKeyID)
	err = option.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// APIKey represents an API key
type APIKey struct {
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
	// The owner of the key
	Owner string     `json:"owner"`
	JSON  apiKeyJSON `json:"-"`
}

type apiKeyJSON struct {
	Object      apijson.Field
	Value       apijson.Field
	Name        apijson.Field
	CreatedAt   apijson.Field
	ID          apijson.Field
	Owner       apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *APIKey) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type Owner struct {
	// user or service_account
	Type string `json:"type"`
	// Represents an individual user in a project.
	User ProjectUser `json:"user"`
	// Represents an individual service account in a project.
	ServiceAccount ProjectServiceAccount `json:"service_account"`
}

// APIKeyListResponse represents a response from listing API keys
type APIKeyListResponse struct {
	Object  string         `json:"object"`
	Data    []APIKey       `json:"data"`
	FirstID string         `json:"first_id"`
	LastID  string         `json:"last_id"`
	HasMore bool           `json:"has_more"`
	Json    apiKeyListJSON `json:"-"`
}

type apiKeyListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *APIKeyListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type APIKeyDeleteResponse struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
