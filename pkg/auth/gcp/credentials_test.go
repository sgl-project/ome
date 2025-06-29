package gcp

import (
	"encoding/json"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestGCPCredentials_Provider(t *testing.T) {
	creds := &GCPCredentials{
		authType:  auth.GCPServiceAccount,
		projectID: "test-project",
		logger:    logging.NewNopLogger(),
	}

	if provider := creds.Provider(); provider != auth.ProviderGCP {
		t.Errorf("Expected provider %s, got %s", auth.ProviderGCP, provider)
	}
}

func TestGCPCredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Service Account",
			authType: auth.GCPServiceAccount,
		},
		{
			name:     "Workload Identity",
			authType: auth.GCPWorkloadIdentity,
		},
		{
			name:     "Default",
			authType: auth.GCPDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &GCPCredentials{
				authType: tt.authType,
			}
			if typ := creds.Type(); typ != tt.authType {
				t.Errorf("Expected type %s, got %s", tt.authType, typ)
			}
		})
	}
}

func TestGCPCredentials_GetProjectID(t *testing.T) {
	creds := &GCPCredentials{
		projectID: "my-gcp-project",
	}

	if projectID := creds.GetProjectID(); projectID != "my-gcp-project" {
		t.Errorf("Expected project ID my-gcp-project, got %s", projectID)
	}
}

func TestGCPCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *GCPCredentials
		expected bool
	}{
		{
			name:     "No cached token",
			creds:    &GCPCredentials{},
			expected: true,
		},
		// Note: Testing with valid/expired tokens requires mocking oauth2.Token
		// which is complex due to internal implementation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.creds.IsExpired()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestServiceAccountConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ServiceAccountConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
				ClientEmail: "test@test-project.iam.gserviceaccount.com",
			},
			wantError: false,
		},
		{
			name: "Wrong type",
			config: ServiceAccountConfig{
				Type:        "wrong_type",
				ProjectID:   "test-project",
				PrivateKey:  "key",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing project ID",
			config: ServiceAccountConfig{
				Type:        "service_account",
				PrivateKey:  "key",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing private key",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing client email",
			config: ServiceAccountConfig{
				Type:       "service_account",
				ProjectID:  "test-project",
				PrivateKey: "key",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestServiceAccountConfig_ToJSON(t *testing.T) {
	config := ServiceAccountConfig{
		Type:                    "service_account",
		ProjectID:               "test-project",
		PrivateKeyID:            "key-id",
		PrivateKey:              "private-key",
		ClientEmail:             "test@test-project.iam.gserviceaccount.com",
		ClientID:                "client-id",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/test%40test-project.iam.gserviceaccount.com",
	}

	jsonData, err := config.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
}

func TestWorkloadIdentityConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkloadIdentityConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: WorkloadIdentityConfig{
				ProjectID:  "test-project",
				PoolID:     "test-pool",
				ProviderID: "test-provider",
			},
			wantError: false,
		},
		{
			name: "Valid config with service account",
			config: WorkloadIdentityConfig{
				ProjectID:      "test-project",
				PoolID:         "test-pool",
				ProviderID:     "test-provider",
				ServiceAccount: "test@test-project.iam.gserviceaccount.com",
			},
			wantError: false,
		},
		{
			name: "Missing project ID",
			config: WorkloadIdentityConfig{
				PoolID:     "test-pool",
				ProviderID: "test-provider",
			},
			wantError: true,
		},
		{
			name: "Missing pool ID",
			config: WorkloadIdentityConfig{
				ProjectID:  "test-project",
				ProviderID: "test-provider",
			},
			wantError: true,
		},
		{
			name: "Missing provider ID",
			config: WorkloadIdentityConfig{
				ProjectID: "test-project",
				PoolID:    "test-pool",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
