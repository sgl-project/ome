package main

import (
	"context"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	"component": "main",
})

func main() {
	client := openaisdk.NewClient(option.WithAPIKey("admin-key"))
	log.Infof("Client created: %v", client)
	ctx := context.Background()
	project, err := client.Projects.List(ctx)
	if err != nil {
		log.Fatalf("Failed to create project: %v", err)
	}
	log.Infof("Project created: %v", project)

}
