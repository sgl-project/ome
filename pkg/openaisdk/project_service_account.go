package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// ProjectServiceAccountService contains methods for interacting with project service accounts
type ProjectServiceAccountService struct {
	Options []option.RequestOption
}

// NewProjectServiceAccountService generates a new service that applies the given options to each request
func NewProjectServiceAccountService(opts ...option.RequestOption) (r *ProjectServiceAccountService) {
	r = &ProjectServiceAccountService{}
	r.Options = opts
	return
}

// Create creates a new service account in the project
func (r *ProjectServiceAccountService) Create(ctx context.Context, projectID string, body ProjectServiceAccountCreateRequest, opts ...option.RequestOption) (res *ProjectServiceAccountCreateResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/service_accounts", projectID)

	// Add project ID to request options
	opts = append(opts, option.WithHeader("OpenAI-Project", projectID))

	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// List returns all service accounts in the project
func (r *ProjectServiceAccountService) List(ctx context.Context, projectID string, opts ...option.RequestOption) (res *ProjectServiceAccountListResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/service_accounts", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Get retrieves a specific service account
func (r *ProjectServiceAccountService) Get(ctx context.Context, projectID string, serviceAccountID string, opts ...option.RequestOption) (res *ProjectServiceAccount, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if serviceAccountID == "" {
		err = errors.New("missing required serviceAccountID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/service_accounts/%s", projectID, serviceAccountID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Delete removes a service account from the project
func (r *ProjectServiceAccountService) Delete(ctx context.Context, projectID string, serviceAccountID string, opts ...option.RequestOption) (res *ProjectServiceAccountDeleteResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if serviceAccountID == "" {
		err = errors.New("missing required serviceAccountID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/service_accounts/%s", projectID, serviceAccountID)
	err = option.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// ProjectServiceAccount represents a service account
type ProjectServiceAccount struct {
	Object string `json:"object"`
	// ID is the unique identifier for the service account.
	ID string `json:"id"`
	// Name is the name of the service account.
	Name string `json:"name"`
	// The role of the service account
	Role string `json:"role"`
	// The Unix timestamp (in seconds) of when the service account was created
	CreatedAt int64 `json:"created_at"`
}

// ProjectServiceAccountCreateRequest represents a request to create a service account
type ProjectServiceAccountCreateRequest struct {
	// The name of the service account
	Name string `json:"name"`
}

// ProjectServiceAccountCreateResponse represents a response from creating a service account
type ProjectServiceAccountCreateResponse struct {
	ProjectServiceAccount
	// The API key associated with the service account
	// +optional
	APIKey *ProjectServiceAccountAPIKey `json:"api_key,omitempty"`
}

// ProjectServiceAccountAPIKey represents an API key for a service account
type ProjectServiceAccountAPIKey struct {
	// The object type, which is always "organization.project.service_account.api_key"
	Object string `json:"object"`
	// The API key value
	Value string `json:"value"`
	// The name of the API key
	Name string `json:"name"`
	// The Unix timestamp (in seconds) of when the key was created
	CreatedAt int64 `json:"created_at"`
	// The identifier of the key
	ID string `json:"id"`
}

// ProjectServiceAccountListResponse represents a response from listing service accounts
type ProjectServiceAccountListResponse struct {
	Object  string                  `json:"object"`
	Data    []ProjectServiceAccount `json:"data"`
	FirstID string                  `json:"first_id"`
	LastID  string                  `json:"last_id"`
	HasMore bool                    `json:"has_more"`
}

// ProjectServiceAccountDeleteResponse represents a response from deleting a service account
type ProjectServiceAccountDeleteResponse struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
