package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/apijson"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// ProjectUserService contains methods for interacting with project users
type ProjectUserService struct {
	Options []option.RequestOption
}

// NewProjectUserService generates a new service that applies the given options to each request
func NewProjectUserService(opts ...option.RequestOption) (r *ProjectUserService) {
	r = &ProjectUserService{}
	r.Options = opts
	return
}

// Create adds a new user to the project
func (r *ProjectUserService) Create(ctx context.Context, projectID string, body ProjectUserCreateRequest, opts ...option.RequestOption) (res *ProjectUser, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/users", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// List returns all users in the project
func (r *ProjectUserService) List(ctx context.Context, projectID string, opts ...option.RequestOption) (res *ProjectUserListResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/users", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Delete removes a user from the project
func (r *ProjectUserService) Delete(ctx context.Context, projectID string, userID string, opts ...option.RequestOption) (res *ProjectUserDeleteResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if userID == "" {
		err = errors.New("missing required userID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/users/%s", projectID, userID)
	err = option.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// ProjectUser represents an individual user in a project
type ProjectUser struct {
	// The object type, which is always "organization.project.user"
	Object string `json:"object"`
	// The identifier, which can be referenced in API endpoints
	ID string `json:"id"`
	// The name of the user
	Name string `json:"name"`
	// The email address of the user
	Email string `json:"email"`
	// Role can be 'owner' or 'member'
	Role string `json:"role"`
	// The Unix timestamp (in seconds) of when the user was added
	AddedAt int64           `json:"added_at"`
	JSON    projectUserJSON `json:"-"`
}

// projectUserJSON contains the JSON metadata for the struct [ProjectUser]
type projectUserJSON struct {
	Object      apijson.Field
	ID          apijson.Field
	Name        apijson.Field
	Email       apijson.Field
	Role        apijson.Field
	AddedAt     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ProjectUser) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// ProjectUserCreateRequest is the request struct for adding a user to a project
type ProjectUserCreateRequest struct {
	// The ID of the user
	UserID string `json:"user_id"`
	// Role can be 'owner' or 'member'
	Role string `json:"role"`
}

// ProjectUserListResponse is the response struct for listing project users
type ProjectUserListResponse struct {
	Object  string              `json:"object"`
	Data    []ProjectUser       `json:"data"`
	FirstID string              `json:"first_id"`
	LastID  string              `json:"last_id"`
	HasMore bool                `json:"has_more"`
	JSON    projectUserListJSON `json:"-"`
}

// projectUserListJSON contains the JSON metadata for the struct [ProjectUserListResponse]
type projectUserListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ProjectUserListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// ProjectUserDeleteResponse is the response struct for deleting a project user
type ProjectUserDeleteResponse struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
