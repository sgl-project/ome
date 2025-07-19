package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Factory creates GCP credentials
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new GCP auth factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates GCP credentials based on config
func (f *Factory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
	if config.Provider != auth.ProviderGCP {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", auth.ProviderGCP, config.Provider)
	}

	var creds *google.Credentials
	var projectID string
	var err error

	switch config.AuthType {
	case auth.GCPServiceAccount:
		creds, projectID, err = f.createServiceAccountCredentials(ctx, config)
	case auth.GCPWorkloadIdentity:
		creds, projectID, err = f.createWorkloadIdentityCredentials(ctx, config)
	case auth.GCPDefault:
		creds, projectID, err = f.createDefaultCredentials(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported GCP auth type: %s", config.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GCP credentials: %w", err)
	}

	// Use explicit project ID from config if provided
	if config.Extra != nil {
		if pid, ok := config.Extra["project_id"].(string); ok && pid != "" {
			projectID = pid
		}
	}

	return &GCPCredentials{
		tokenSource: creds.TokenSource,
		authType:    config.AuthType,
		projectID:   projectID,
		logger:      f.logger,
	}, nil
}

// SupportedAuthTypes returns supported GCP auth types
func (f *Factory) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.GCPServiceAccount,
		auth.GCPWorkloadIdentity,
		auth.GCPDefault,
	}
}

// createServiceAccountCredentials creates service account credentials
func (f *Factory) createServiceAccountCredentials(ctx context.Context, config auth.Config) (*google.Credentials, string, error) {
	var saConfig ServiceAccountConfig
	var jsonData []byte

	if config.Extra != nil {
		// Check for direct service account config
		if sa, ok := config.Extra["service_account"].(map[string]interface{}); ok {
			// Convert map to ServiceAccountConfig
			jsonBytes, err := json.Marshal(sa)
			if err != nil {
				return nil, "", fmt.Errorf("failed to marshal service account config: %w", err)
			}
			if err := json.Unmarshal(jsonBytes, &saConfig); err != nil {
				return nil, "", fmt.Errorf("failed to unmarshal service account config: %w", err)
			}
			jsonData = jsonBytes
		} else if keyFile, ok := config.Extra["key_file"].(string); ok {
			// Read from file
			data, err := os.ReadFile(keyFile)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read service account key file: %w", err)
			}
			if err := json.Unmarshal(data, &saConfig); err != nil {
				return nil, "", fmt.Errorf("failed to parse service account key file: %w", err)
			}
			jsonData = data
		} else if keyJSON, ok := config.Extra["key_json"].(string); ok {
			// Use JSON string directly
			if err := json.Unmarshal([]byte(keyJSON), &saConfig); err != nil {
				return nil, "", fmt.Errorf("failed to parse service account JSON: %w", err)
			}
			jsonData = []byte(keyJSON)
		}
	}

	// Check environment variable
	if len(jsonData) == 0 {
		if keyFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); keyFile != "" {
			data, err := os.ReadFile(keyFile)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read GOOGLE_APPLICATION_CREDENTIALS file: %w", err)
			}
			if err := json.Unmarshal(data, &saConfig); err != nil {
				return nil, "", fmt.Errorf("failed to parse GOOGLE_APPLICATION_CREDENTIALS: %w", err)
			}
			jsonData = data
		}
	}

	if len(jsonData) == 0 {
		return nil, "", fmt.Errorf("no service account credentials provided")
	}

	// Validate config
	if err := saConfig.Validate(); err != nil {
		return nil, "", err
	}

	// Create credentials
	creds, err := google.CredentialsFromJSON(ctx, jsonData,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/devstorage.full_control",
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create credentials from JSON: %w", err)
	}

	return creds, saConfig.ProjectID, nil
}

// createWorkloadIdentityCredentials creates workload identity credentials
func (f *Factory) createWorkloadIdentityCredentials(ctx context.Context, config auth.Config) (*google.Credentials, string, error) {
	// GKE Workload Identity uses Application Default Credentials
	// The GKE metadata service provides tokens for the bound service account

	var wiConfig WorkloadIdentityConfig

	if config.Extra != nil {
		// Extract workload identity config
		if wi, ok := config.Extra["workload_identity"].(map[string]interface{}); ok {
			if projectID, ok := wi["project_id"].(string); ok {
				wiConfig.ProjectID = projectID
			}
			if sa, ok := wi["service_account"].(string); ok {
				wiConfig.ServiceAccount = sa
			}
			if ksa, ok := wi["kubernetes_service_account"].(string); ok {
				wiConfig.KubernetesServiceAccount = ksa
			}
			if clusterName, ok := wi["cluster_name"].(string); ok {
				wiConfig.ClusterName = clusterName
			}
			if clusterLocation, ok := wi["cluster_location"].(string); ok {
				wiConfig.ClusterLocation = clusterLocation
			}
		}

		// Also check for direct project_id
		if wiConfig.ProjectID == "" {
			if pid, ok := config.Extra["project_id"].(string); ok {
				wiConfig.ProjectID = pid
			}
		}
	}

	// Find credentials using Application Default Credentials
	// In GKE with Workload Identity, this will use the metadata service
	creds, err := google.FindDefaultCredentials(ctx,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/devstorage.full_control",
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find workload identity credentials: %w", err)
	}

	// Try to get project ID from various sources
	projectID := wiConfig.ProjectID
	if projectID == "" && creds.ProjectID != "" {
		projectID = creds.ProjectID
	}
	if projectID == "" {
		// Try to get from metadata service
		projectID = getProjectIDFromMetadata(ctx)
	}

	f.logger.WithField("project_id", projectID).
		WithField("service_account", wiConfig.ServiceAccount).
		WithField("kubernetes_service_account", wiConfig.KubernetesServiceAccount).
		Debug("Created GKE Workload Identity credentials")

	return creds, projectID, nil
}

// createDefaultCredentials creates default credentials
func (f *Factory) createDefaultCredentials(ctx context.Context, config auth.Config) (*google.Credentials, string, error) {
	// Use Application Default Credentials
	creds, err := google.FindDefaultCredentials(ctx,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/devstorage.full_control",
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find default credentials: %w", err)
	}

	projectID := creds.ProjectID

	// Try to get project ID from metadata if not available
	if projectID == "" {
		projectID = getProjectIDFromMetadata(ctx)
	}

	// Override with explicit project ID from config
	if config.Extra != nil {
		if pid, ok := config.Extra["project_id"].(string); ok && pid != "" {
			projectID = pid
		}
	}

	return creds, projectID, nil
}

// getProjectIDFromMetadata tries to get project ID from GCE metadata
func getProjectIDFromMetadata(ctx context.Context) string {
	// Use the metadata package to get project ID from GCE metadata service
	// This is more reliable than trying to use the compute API
	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		return ""
	}
	return creds.ProjectID
}
