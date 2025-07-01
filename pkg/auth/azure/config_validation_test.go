package azure

import (
	"strings"
	"testing"
)

func TestConfig_Validate_ImprovedValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Empty config is valid",
			config:    Config{},
			wantError: false,
		},
		{
			name: "Valid account key config",
			config: Config{
				AccountName: "storageaccount",
				AccountKey:  "base64key==",
			},
			wantError: false,
		},
		{
			name: "Account name without key",
			config: Config{
				AccountName: "storageaccount",
			},
			wantError: true,
			errorMsg:  "account_key is required when account_name is provided",
		},
		{
			name: "Account key without name",
			config: Config{
				AccountKey: "base64key==",
			},
			wantError: true,
			errorMsg:  "account_name is required when account_key is provided",
		},
		{
			name: "Valid client secret config",
			config: Config{
				TenantID:     "tenant-id",
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantError: false,
		},
		{
			name: "Client secret without tenant ID",
			config: Config{
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantError: true,
			errorMsg:  "tenant_id is required for client secret authentication",
		},
		{
			name: "Client secret without client ID",
			config: Config{
				TenantID:     "tenant-id",
				ClientSecret: "secret",
			},
			wantError: true,
			errorMsg:  "client_id is required for client secret authentication",
		},
		{
			name: "Valid certificate with path",
			config: Config{
				TenantID:        "tenant-id",
				ClientID:        "client-id",
				CertificatePath: "/path/to/cert.pfx",
			},
			wantError: false,
		},
		{
			name: "Valid certificate with data",
			config: Config{
				TenantID:        "tenant-id",
				ClientID:        "client-id",
				CertificateData: []byte("cert-data"),
			},
			wantError: false,
		},
		{
			name: "Certificate with both path and data",
			config: Config{
				TenantID:        "tenant-id",
				ClientID:        "client-id",
				CertificatePath: "/path/to/cert.pfx",
				CertificateData: []byte("cert-data"),
			},
			wantError: true,
			errorMsg:  "cannot specify both certificate_path and certificate_data",
		},
		{
			name: "Multiple auth methods - account key and client secret",
			config: Config{
				AccountName:  "storageaccount",
				AccountKey:   "base64key==",
				TenantID:     "tenant-id",
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantError: true,
			errorMsg:  "multiple authentication methods configured",
		},
		{
			name: "Multiple auth methods - client secret and certificate",
			config: Config{
				TenantID:        "tenant-id",
				ClientID:        "client-id",
				ClientSecret:    "secret",
				CertificatePath: "/path/to/cert.pfx",
			},
			wantError: true,
			errorMsg:  "multiple authentication methods configured",
		},
		{
			name: "Multiple validation errors",
			config: Config{
				AccountName:     "storageaccount", // Missing account key
				ClientSecret:    "secret",         // Missing tenant and client ID
				CertificatePath: "/path/to/cert",  // Missing tenant and client ID
			},
			wantError: true,
			errorMsg:  "validation errors:",
		},
		{
			name: "Partial client secret config with multiple errors",
			config: Config{
				ClientSecret: "secret", // Missing both tenant ID and client ID
			},
			wantError: true,
			errorMsg:  "tenant_id is required for client secret authentication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestConfig_Validate_AllErrorsReported(t *testing.T) {
	// Test that all validation errors are reported, not just the first one
	config := Config{
		AccountName:     "storageaccount", // Missing account key
		ClientSecret:    "secret",         // Missing tenant and client ID
		CertificatePath: "/path/to/cert",  // Missing tenant and client ID
		CertificateData: []byte("data"),   // Conflicts with CertificatePath
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("Expected validation error")
	}

	errorMsg := err.Error()

	// Check that all expected errors are present
	expectedErrors := []string{
		"account_key is required",
		"tenant_id is required for client secret",
		"client_id is required for client secret",
		"tenant_id is required for client certificate",
		"client_id is required for client certificate",
		"cannot specify both certificate_path and certificate_data",
		"multiple authentication methods configured",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(errorMsg, expected) {
			t.Errorf("Expected error message to contain %q, but it didn't. Full error: %s", expected, errorMsg)
		}
	}
}
