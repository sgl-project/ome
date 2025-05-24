package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var apiKeyLog = logrus.WithFields(logrus.Fields{
	"component": "api-key-example",
})

// formatAPIKey returns a clean string representation of an API key
func formatAPIKey(a *openaisdk.APIKey) string {
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

// formatAPIKeyList returns a clean string representation of API key list
func formatAPIKeyList(al *openaisdk.APIKeyListResponse) string {
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

// formatApiKeyDelete returns a clean string representation of an API key deletion response
func formatApiKeyDelete(ad *openaisdk.APIKeyDeleteResponse) string {
	b, _ := json.Marshal(ad)
	return string(b)
}

func ApiKeyExample() {
	client := openaisdk.NewClient(option.WithAPIKey("admin-api-key"))
	ctx := context.Background()

	// List all API keys
	apikeys, err := client.APIKeys.List(ctx, "proj-id")
	if err != nil {
		apiKeyLog.Fatalf("Failed to list API keys: %v", err)
	}
	apiKeyLog.Infof("Current API keys: %s", formatAPIKeyList(apikeys))

	// Get a specific API key
	apikey, err := client.APIKeys.Get(ctx, "proj-id", "key-id")
	if err != nil {
		apiKeyLog.Fatalf("Failed to get API key: %v", err)
	}
	apiKeyLog.Infof("API key: %s", formatAPIKey(apikey))

	// Delete an API key
	apikeyDelete, err := client.APIKeys.Delete(ctx, "proj-id", "key-id")
	if err != nil {
		apiKeyLog.Fatalf("Failed to delete API key: %v", err)
	}
	apiKeyLog.Infof("API key deleted: %s", formatApiKeyDelete(apikeyDelete))
}
