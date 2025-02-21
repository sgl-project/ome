package main

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/examples"
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
}
