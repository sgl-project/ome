package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var projectLog = logrus.WithFields(logrus.Fields{
	"component": "project-example",
})

// formatProject returns a clean string representation of a project
func formatProject(p *openaisdk.Project) string {
	createdTime := time.Unix(p.CreatedAt, 0).Format(time.RFC3339)
	fields := map[string]interface{}{
		"id":         p.ID,
		"name":       p.Name,
		"status":     p.Status,
		"created_at": createdTime,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatProjectList returns a clean string representation of project list
func formatProjectList(pl *openaisdk.ProjectListResponse) string {
	var projects []map[string]interface{}
	for _, p := range pl.Data {
		projects = append(projects, map[string]interface{}{
			"id":         p.ID,
			"name":       p.Name,
			"status":     p.Status,
			"created_at": time.Unix(p.CreatedAt, 0).Format(time.RFC3339),
		})
	}

	fields := map[string]interface{}{
		"count":    len(pl.Data),
		"projects": projects,
		"has_more": pl.HasMore,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

func ProjectExample() {
	client := openaisdk.NewClient(option.WithAPIKey("admin-api-key"))
	ctx := context.Background()

	// List all projects
	projects, err := client.Projects.List(ctx)
	if err != nil {
		projectLog.Fatalf("Failed to list projects: %v", err)
	}
	projectLog.Infof("Current projects: %s", formatProjectList(projects))

	// Create a new project
	newProject, err := client.Projects.Create(ctx, openaisdk.ProjectCreateRequest{
		Name: "My New Project",
	})
	if err != nil {
		projectLog.Fatalf("Failed to create project: %v", err)
	}
	projectLog.Infof("Project created: %s", formatProject(newProject))

	// Get the project details
	projectDetails, err := client.Projects.Get(ctx, newProject.ID)
	if err != nil {
		projectLog.Fatalf("Failed to get project details: %v", err)
	}
	projectLog.Infof("Project details: %s", formatProject(projectDetails))

	// Update the project
	updatedProject, err := client.Projects.Update(ctx, newProject.ID, openaisdk.ProjectUpdateRequest{
		Name: "My Updated Project",
	})
	if err != nil {
		projectLog.Fatalf("Failed to update project: %v", err)
	}
	projectLog.Infof("Project updated: %s", formatProject(updatedProject))

	// List projects again to see the changes
	projectsAfter, err := client.Projects.List(ctx)
	if err != nil {
		projectLog.Fatalf("Failed to list projects: %v", err)
	}
	projectLog.Infof("Updated projects list: %s", formatProjectList(projectsAfter))

	// Delete the project
	deleteRes, err := client.Projects.Archive(ctx, newProject.ID)
	if err != nil {
		projectLog.Fatalf("Failed to delete project: %v", err)
	}
	projectLog.Infof("Project deleted: %s", formatProject(deleteRes))
}
