package main

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/examples"
	"github.com/sirupsen/logrus"
)

func init() {
	// Configure logrus for better logging output
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	fmt.Println("\n=== Running Project Examples ===")
	examples.ProjectExample()

	fmt.Println("\n=== Running Service Accounts Examples ===")
	examples.ServiceAccountsExample()

	fmt.Println("\n=== Running Project Users Examples ===")
	examples.ProjectUsersExample()

	fmt.Println("\n=== Running API Key Examples ===")
	examples.ApiKeyExample()

	fmt.Println("\n=== Running Admin API Key Examples ===")
	examples.AdminApiKeyExample()

	fmt.Println("\n=== Running Project Rate Limit Examples ===")
	examples.ProjectRateLimitExample()

	fmt.Println("\n=== Running Audit Log Examples ===")
	examples.AuditLogExample()
}
