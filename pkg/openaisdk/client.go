package openaisdk

import (
	"context"
	"net/http"
	"os"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

type Client struct {
	Options []option.RequestOption

	AdminAPIKeys      *AdminApiKeyService
	Projects          *ProjectService
	ProjectUsers      *ProjectUserService
	ServiceAccounts   *ProjectServiceAccountService
	APIKeys           *ApiKeyService
	ProjectRateLimits *ProjectRateLimitService
	AuditLogs         *AuditLogService
}

// NewClient generates a new client with the default option read from the
// environment (OPENAI_API_KEY, OPENAI_ORG_ID, OPENAI_PROJECT_ID). The option
// passed in as arguments are applied after these default arguments, and all option
// will be passed down to the services and requests that this client makes.
func NewClient(opts ...option.RequestOption) (r *Client) {
	defaults := []option.RequestOption{option.WithEnvironmentProduction()}
	if o, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
		defaults = append(defaults, option.WithAPIKey(o))
	}
	if o, ok := os.LookupEnv("OPENAI_ORG_ID"); ok {
		defaults = append(defaults, option.WithOrganization(o))
	}
	if o, ok := os.LookupEnv("OPENAI_PROJECT_ID"); ok {
		defaults = append(defaults, option.WithProject(o))
	}
	opts = append(defaults, opts...)

	r = &Client{Options: opts}

	r.AdminAPIKeys = NewAdminApiKeyService(opts...)
	r.Projects = NewProjectService(opts...)
	r.ProjectUsers = NewProjectUserService(opts...)
	r.ServiceAccounts = NewProjectServiceAccountService(opts...)
	r.APIKeys = NewApiKeyService(opts...)
	r.ProjectRateLimits = NewProjectRateLimitService(opts...)
	r.AuditLogs = NewAuditLogService(opts...)

	return
}

func (r *Client) Execute(ctx context.Context, method string, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	opts = append(r.Options, opts...)
	return option.ExecuteNewRequest(ctx, method, path, params, res, opts...)
}

// Get makes a GET request with the given URL, params, and optionally deserializes
// to a response. See [Execute] documentation on the params and response.
func (r *Client) Get(ctx context.Context, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	return r.Execute(ctx, http.MethodGet, path, params, res, opts...)
}

// Post makes a POST request with the given URL, params, and optionally
// deserializes to a response. See [Execute] documentation on the params and
// response.
func (r *Client) Post(ctx context.Context, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	return r.Execute(ctx, http.MethodPost, path, params, res, opts...)
}

// Put makes a PUT request with the given URL, params, and optionally deserializes
// to a response. See [Execute] documentation on the params and response.
func (r *Client) Put(ctx context.Context, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	return r.Execute(ctx, http.MethodPut, path, params, res, opts...)
}

// Patch makes a PATCH request with the given URL, params, and optionally
// deserializes to a response. See [Execute] documentation on the params and
// response.
func (r *Client) Patch(ctx context.Context, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	return r.Execute(ctx, http.MethodPatch, path, params, res, opts...)
}

// Delete makes a DELETE request with the given URL, params, and optionally
// deserializes to a response. See [Execute] documentation on the params and
// response.
func (r *Client) Delete(ctx context.Context, path string, params interface{}, res interface{}, opts ...option.RequestOption) error {
	return r.Execute(ctx, http.MethodDelete, path, params, res, opts...)
}
