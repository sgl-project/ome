package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var adminApiKeyLog = logrus.WithFields(logrus.Fields{
	"component": "admin-api-key-example",
})

// formatAdminAPIKey returns a clean string representation of an API key
func formatAdminAPIKey(a *openaisdk.AdminAPIKey) string {
	createdTime := time.Unix(a.CreatedAt, 0).Format(time.RFC3339)
	fields := map[string]interface{}{
		"id":         a.ID,
		"value":      a.RedactedValue,
		"name":       a.Name,
		"created_at": createdTime,
		"owner":      a.Owner,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatAdminAPIKeyList returns a clean string representation of API key list
func formatAdminAPIKeyList(al *openaisdk.AdminAPIKeyListResponse) string {
	var apikeys []map[string]interface{}
	for _, a := range al.Data {
		apikeys = append(apikeys, map[string]interface{}{
			"id":         a.ID,
			"value":      a.RedactedValue,
			"name":       a.Name,
			"created_at": time.Unix(a.CreatedAt, 0).Format(time.RFC3339),
			"owner":      a.Owner,
		})
	}

	fields := map[string]interface{}{
		"count":    len(al.Data),
		"apikeys":  apikeys,
		"has_more": al.HasMore,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatAdminApiKeyDelete returns a clean string representation of an API key deletion response
func formatAdminApiKeyDelete(ad *openaisdk.AdminAPIKeyDeleteResponse) string {
	b, _ := json.Marshal(ad)
	return string(b)
}

func AdminApiKeyExample() {
	client := openaisdk.NewClient(option.WithAPIKey("admin-api-key"))
	ctx := context.Background()

	//Create a new Admin API key
	adminAPIKeyCreateRequest := openaisdk.AdminAPIKeyCreateRequest{
		Name: "test-admin-api-key",
	}
	newAdminAPIKey, err := client.AdminAPIKeys.Create(ctx, adminAPIKeyCreateRequest)
	if err != nil {
		adminApiKeyLog.Fatalf("Failed to create admin API key: %v", err)
	}
	adminApiKeyLog.Infof("New admin API key: %s", formatAdminAPIKey(&newAdminAPIKey.AdminAPIKey))

	// List all Admin API keys
	adminAPIKeys, err := client.AdminAPIKeys.List(ctx)
	if err != nil {
		adminApiKeyLog.Fatalf("Failed to list admin API keys: %v", err)
	}
	adminApiKeyLog.Infof("Current admin API keys: %s", formatAdminAPIKeyList(adminAPIKeys))

	// Get a specific Admin API key
	adminAPIKey, err := client.AdminAPIKeys.Get(ctx, newAdminAPIKey.ID)
	if err != nil {
		adminApiKeyLog.Fatalf("Failed to get admin API key: %v", err)
	}
	adminApiKeyLog.Infof("Admin API key: %s", formatAdminAPIKey(adminAPIKey))

	// Delete an Admin API key
	adminAPIKeyDeleteResponse, err := client.AdminAPIKeys.Delete(ctx, newAdminAPIKey.ID)
	if err != nil {
		adminApiKeyLog.Fatalf("Failed to delete admin API key: %v", err)
	}
	adminApiKeyLog.Infof("Admin API key deleted: %s", formatAdminApiKeyDelete(adminAPIKeyDeleteResponse))

}
