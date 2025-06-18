package main

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/xaisdk/examples"
	"github.com/sirupsen/logrus"
)

func init() {
	// Configure logrus for better logging output
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {

	fmt.Println("\n=== Running API Key Examples ===")
	examples.ApiKeyExample()

}
