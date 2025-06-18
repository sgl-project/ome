package xaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/apijson"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/option"
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

// Create API key in the team
func (r *ApiKeyService) Create(ctx context.Context, teamId string, body CreateApiKeyBody, opts ...option.RequestOption) (res *APIKey, err error) {
	opts = append(r.Options[:], opts...)
	if teamId == "" {
		err = errors.New("missing required teamId parameter")
		return
	}
	path := fmt.Sprintf("/auth/teams/%s/api-keys", teamId)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// List returns all API keys in the team
func (r *ApiKeyService) List(ctx context.Context, teamId string, opts ...option.RequestOption) (res *APIKeyListResponse, err error) {
	opts = append(r.Options[:], opts...)
	if teamId == "" {
		err = errors.New("missing required teamId parameter")
		return
	}
	path := fmt.Sprintf("/auth/teams/%s/api-keys", teamId)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Delete removes an API key
func (r *ApiKeyService) Delete(ctx context.Context, apiKeyID string, opts ...option.RequestOption) (res *APIKeyDeleteResponse, err error) {
	opts = append(r.Options[:], opts...)
	if apiKeyID == "" {
		err = errors.New("missing required apiKeyID parameter")
		return
	}
	path := fmt.Sprintf("/auth/api-keys/%s", apiKeyID)
	err = option.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

type CreateApiKeyBody struct {
	Name       string   `json:"name"`
	ACLStrings []string `json:"acls"`
	QPS        int32    `json:"qps,omitempty"`
	QPM        int32    `json:"qpm,omitempty"`
}

type APIKey struct {
	RedactedValue string     `json:"redactedApiKey"`
	ApiKey        string     `json:"apiKey,omitempty"`
	ApiKeyHash    string     `json:"apiKeyHash"`
	ApiKeyId      string     `json:"apiKeyId"`
	UserId        string     `json:"userId"`
	Name          string     `json:"name"`
	CreatedAt     string     `json:"createTime"`
	ModifyTime    string     `json:"modifyTime"`
	TeamId        string     `json:"teamId"`
	BlockedReason string     `json:"blockedReason,omitempty"`
	Disabled      bool       `json:"disabled"`
	QPS           int32      `json:"qps,omitempty"`
	QPM           int32      `json:"qpm,omitempty"`
	ACLStrings    []string   `json:"aclStrings,omitempty"`
	JSON          apiKeyJSON `json:"-"`
}

type apiKeyJSON struct {
	Object      apijson.Field
	Value       apijson.Field
	Name        apijson.Field
	CreatedAt   apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *APIKey) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// APIKeyListResponse represents a response from listing API keys
type APIKeyListResponse struct {
	ApiKeys []APIKey `json:"apiKeys"`
}

func (r *APIKeyListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type APIKeyDeleteResponse struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
