package main

import (
	"context"
	"fmt"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OrgWatcher watches for OpenAI organizations and sends them to a channel for processing
type OrgWatcher struct {
	client  client.Client
	orgChan chan<- *v1beta1.Organization
	logger  *zap.SugaredLogger
}

// NewOrgWatcher creates a new organization watcher
func NewOrgWatcher(client client.Client, orgChan chan<- *v1beta1.Organization, logger *zap.SugaredLogger) *OrgWatcher {
	return &OrgWatcher{
		client:  client,
		orgChan: orgChan,
		logger:  logger,
	}
}

// Start begins watching for organizations at the specified interval
func (w *OrgWatcher) Start(interval time.Duration) {
	w.logger.Infow("Starting organization watcher", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		w.logger.Debug("Checking for organizations")
		if err := w.listOrganizations(); err != nil {
			w.logger.Errorw("Error listing organizations", "error", err)
		}
		<-ticker.C
	}
}

// listOrganizations fetches all OpenAI organizations and sends them to the channel
func (w *OrgWatcher) listOrganizations() error {
	ctx := context.Background()
	var orgList v1beta1.OrganizationList

	w.logger.Debug("Listing organizations")
	if err := w.client.List(ctx, &orgList); err != nil {
		w.logger.Errorw("Failed to list organizations", "error", err)
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	openAIOrgs := 0
	for i := range orgList.Items {
		org := &orgList.Items[i]
		if org.Spec.Vendor != nil && *org.Spec.Vendor == "openai" && org.Spec.SecretRef != nil {
			openAIOrgs++
			w.logger.Debugw("Found OpenAI organization", "name", org.Name, "namespace", org.Namespace)
			w.orgChan <- org
		}
	}

	// Update metrics
	totalOrgs.Set(float64(openAIOrgs))
	w.logger.Infow("Finished listing organizations", "count", openAIOrgs)

	return nil
}
