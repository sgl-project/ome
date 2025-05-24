package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/apijson"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// ProjectService contains methods and other services that help with interacting with
// the openai API. For more details see https://platform.openai.com/docs/api-reference/projects
type ProjectService struct {
	Options []option.RequestOption
}

// NewProjectService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewProjectService(opts ...option.RequestOption) (r *ProjectService) {
	r = &ProjectService{}
	r.Options = opts
	return
}

// Create a new project.
func (r *ProjectService) Create(ctx context.Context, body ProjectCreateRequest, opts ...option.RequestOption) (res *Project, err error) {
	opts = append(r.Options[:], opts...)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, "organization/projects", body, &res, opts...)
	return
}

// Get a project instance.
func (r *ProjectService) Get(ctx context.Context, projectID string, opts ...option.RequestOption) (res *Project, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List the currently available projects.
func (r *ProjectService) List(ctx context.Context, opts ...option.RequestOption) (res *ProjectListResponse, err error) {
	opts = append(r.Options[:], opts...)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, "organization/projects", nil, &res, opts...)
	return
}

// Update a project.
func (r *ProjectService) Update(ctx context.Context, projectID string, body ProjectUpdateRequest, opts ...option.RequestOption) (res *Project, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Archive a project
func (r *ProjectService) Archive(ctx context.Context, projectID string, opts ...option.RequestOption) (res *Project, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/archive", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// Project represents an individual project.
type Project struct {
	// The identifier, which can be referenced in API endpoints
	ID string `json:"id"`
	// The object type, which is always "organization.project"
	Object string `json:"object"`
	// The name of the project
	Name string `json:"name"`
	// The Unix timestamp (in seconds) of when the project was created
	CreatedAt int64 `json:"created_at"`
	// The Unix timestamp (in seconds) of when the project was archived or null
	ArchivedAt *int64 `json:"archived_at"`
	// Status can be 'active' or 'archived'
	Status string      `json:"status"`
	JSON   projectJSON `json:"-"`
}

// projectJSON contains the JSON metadata for the struct [Project]
type projectJSON struct {
	ID          apijson.Field
	Object      apijson.Field
	Name        apijson.Field
	CreatedAt   apijson.Field
	ArchivedAt  apijson.Field
	Status      apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *Project) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// ProjectCreateRequest is the request struct for creating a new project
type ProjectCreateRequest struct {
	// The friendly name of the project
	Name string `json:"name"`

	// The geography of the project
	Geography string `json:"geography,omitempty"`
}

// ProjectUpdateRequest is the request struct for updating a project
type ProjectUpdateRequest struct {
	// The updated name of the project
	Name string `json:"name"`
}

// ProjectListResponse is the response struct for listing projects
type ProjectListResponse struct {
	Object  string          `json:"object"`
	Data    []Project       `json:"data"`
	FirstID string          `json:"first_id"`
	LastID  string          `json:"last_id"`
	HasMore bool            `json:"has_more"`
	JSON    projectListJSON `json:"-"`
}

// projectListJSON contains the JSON metadata for the struct [ProjectListResponse]
type projectListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ProjectListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}
