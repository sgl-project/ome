package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var userLog = logrus.WithFields(logrus.Fields{
	"component": "project-users-example",
})

// formatProjectUser returns a clean string representation of a project user
func formatProjectUser(u *openaisdk.ProjectUser) string {
	fields := map[string]interface{}{
		"id":       u.ID,
		"name":     u.Name,
		"email":    u.Email,
		"role":     u.Role,
		"added_at": time.Unix(u.AddedAt, 0).Format(time.RFC3339),
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatProjectUserList returns a clean string representation of project user list
func formatProjectUserList(ul *openaisdk.ProjectUserListResponse) string {
	var users []map[string]interface{}
	for _, u := range ul.Data {
		users = append(users, map[string]interface{}{
			"id":       u.ID,
			"name":     u.Name,
			"email":    u.Email,
			"role":     u.Role,
			"added_at": time.Unix(u.AddedAt, 0).Format(time.RFC3339),
		})
	}

	fields := map[string]interface{}{
		"count":    len(ul.Data),
		"users":    users,
		"has_more": ul.HasMore,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatProjectUserDeleteResponse returns a clean string representation of a project user delete response
func formatProjectUserDeleteResponse(r *openaisdk.ProjectUserDeleteResponse) string {
	fields := map[string]interface{}{
		"id":      r.ID,
		"deleted": r.Deleted,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

func ProjectUsersExample() {
	client := openaisdk.NewClient(option.WithAPIKey("admin-api-key"))
	ctx := context.Background()
	projectId := "proj_nFGG9rJ8eLAXjq8dD6joN3Zz"

	// Create a new user in project
	user, err := client.ProjectUsers.Create(ctx, projectId, openaisdk.ProjectUserCreateRequest{
		UserID: "user-8iLPKDdrcTsuhyMEKRnIiNNl",
		Role:   "member",
	})
	if err != nil {
		userLog.Fatalf("Failed to create project user: %v", err)
	}
	userLog.Infof("User created: %s", formatProjectUser(user))

	// List all users in project
	users, err := client.ProjectUsers.List(ctx, projectId)
	if err != nil {
		userLog.Fatalf("Failed to list project users: %v", err)
	}
	userLog.Infof("Project users: %s", formatProjectUserList(users))

	// Delete a user from project
	deleteRes, err := client.ProjectUsers.Delete(ctx, projectId, "user_abc123")
	if err != nil {
		userLog.Fatalf("Failed to delete project user: %v", err)
	}
	userLog.Infof("User deleted: %s", formatProjectUserDeleteResponse(deleteRes))
}
