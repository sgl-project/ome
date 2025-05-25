package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var svcAcctLog = logrus.WithFields(logrus.Fields{
	"component": "service-accounts-example",
})

// formatServiceAccount returns a clean string representation of a service account
func formatServiceAccount(sa *openaisdk.ProjectServiceAccount) string {
	createdTime := time.Unix(sa.CreatedAt, 0).Format(time.RFC3339)
	fields := map[string]interface{}{
		"id":         sa.ID,
		"name":       sa.Name,
		"role":       sa.Role,
		"created_at": createdTime,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatServiceAccountList returns a clean string representation of service account list
func formatServiceAccountList(sal *openaisdk.ProjectServiceAccountListResponse) string {
	var accounts []map[string]interface{}
	for _, sa := range sal.Data {
		accounts = append(accounts, map[string]interface{}{
			"id":         sa.ID,
			"name":       sa.Name,
			"role":       sa.Role,
			"created_at": time.Unix(sa.CreatedAt, 0).Format(time.RFC3339),
		})
	}

	fields := map[string]interface{}{
		"count":    len(sal.Data),
		"accounts": accounts,
		"has_more": sal.HasMore,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatServiceAccountDelete returns a clean string representation of a service account deletion response
func formatServiceAccountDelete(resp *openaisdk.ProjectServiceAccountDeleteResponse) string {
	b, _ := json.Marshal(resp)
	return string(b)
}

// ServiceAccountsExample demonstrates how to use the service accounts API
func ServiceAccountsExample() {
	client := openaisdk.NewClient(option.WithAPIKey("sk-admin-key"))
	ctx := context.Background()
	projectId := "proj_nFGG9rJ8eLAXjq8dD6joN3Zz"
	svcAcct, err := client.ServiceAccounts.Create(ctx, projectId, openaisdk.ProjectServiceAccountCreateRequest{
		Name: "Production API",
	})
	if err != nil {
		svcAcctLog.Fatalf("Failed to create service account: %v", err)
	}

	svcAcctLog.Infof("Service account created: %s", formatServiceAccount(&svcAcct.ProjectServiceAccount))

	// Check if API key is present
	if svcAcct.APIKey.Value != "" {
		svcAcctLog.Warnf("API Key (save this, it will only be shown once): %s", svcAcct.APIKey.Value)
	} else {
		svcAcctLog.Warn("API Key creation is not implemented yet")
	}

	// List all service accounts
	svcAccts, err := client.ServiceAccounts.List(ctx, projectId)
	if err != nil {
		svcAcctLog.Fatalf("Failed to list service accounts: %v", err)
	}
	svcAcctLog.Infof("Service accounts: %s", formatServiceAccountList(svcAccts))

	// Get a specific service account
	sa, err := client.ServiceAccounts.Get(ctx, projectId, svcAcct.ID)
	if err != nil {
		svcAcctLog.Fatalf("Failed to get service account: %v", err)
	}
	svcAcctLog.Infof("Service account details: %s", formatServiceAccount(sa))

	// Delete the service account
	resp, err := client.ServiceAccounts.Delete(ctx, projectId, svcAcct.ID)
	if err != nil {
		svcAcctLog.Fatalf("Failed to delete service account: %v", err)
	}
	svcAcctLog.Infof("Service account deleted: %s", formatServiceAccountDelete(resp))

}
