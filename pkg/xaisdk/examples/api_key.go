package examples

import (
	"context"
	"encoding/json"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/option"
	"github.com/sirupsen/logrus"
)

var apiKeyLog = logrus.WithFields(logrus.Fields{
	"component": "api-key-example",
})

// formatAPIKey returns a clean string representation of an API key
func formatAPIKey(a *xaisdk.APIKey) string {
	fields := map[string]interface{}{
		"id":           a.ApiKeyId,
		"value":        a.ApiKey,
		"redacedValue": a.RedactedValue,
		"name":         a.Name,
		"created_at":   a.CreatedAt,
		"key":          a.ApiKey,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatAPIKeyList returns a clean string representation of API key list
func formatAPIKeyList(al *xaisdk.APIKeyListResponse) string {
	var apikeys []map[string]interface{}
	for _, a := range al.ApiKeys {
		apikeys = append(apikeys, map[string]interface{}{
			"id":         a.ApiKeyId,
			"value":      a.RedactedValue,
			"name":       a.Name,
			"created_at": a.CreatedAt,
		})
	}

	fields := map[string]interface{}{
		"count":   len(al.ApiKeys),
		"apikeys": apikeys,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatApiKeyDelete returns a clean string representation of an API key deletion response
func formatApiKeyDelete(ad *xaisdk.APIKeyDeleteResponse) string {
	b, _ := json.Marshal(ad)
	return string(b)
}

func ApiKeyExample() {
	client := xaisdk.NewClient(option.WithAPIKey("<xai-management-key>"))
	ctx := context.Background()

	// List all API keys
	apikeys, err := client.APIKeys.List(ctx, "<team-id>")
	if err != nil {
		apiKeyLog.Fatalf("Failed to list API keys: %v", err)
	}
	apiKeyLog.Infof("Current API keys: %s", formatAPIKeyList(apikeys))

	// Create API key
	apikey, err := client.APIKeys.Create(ctx, "<team-id>", xaisdk.CreateApiKeyBody{Name: "<name>", ACLStrings: []string{"api-key:endpoint:*", "api-key:model:*"}})
	if err != nil {
		apiKeyLog.Fatalf("Failed to create API key: %v", err)
	}
	apiKeyLog.Infof("Created API key: %s", formatAPIKey(apikey))

	// Delete an API key
	apikeyDelete, err := client.APIKeys.Delete(ctx, "<api-key-id>")
	if err != nil {
		apiKeyLog.Fatalf("Failed to delete API key: %v", err)
	}
	apiKeyLog.Infof("API key deleted: %s", formatApiKeyDelete(apikeyDelete))
}
