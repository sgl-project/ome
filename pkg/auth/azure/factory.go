package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Factory creates Azure credentials
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new Azure auth factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates Azure credentials based on config
func (f *Factory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
	if config.Provider != auth.ProviderAzure {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", auth.ProviderAzure, config.Provider)
	}

	var credential azcore.TokenCredential
	var tenantID, clientID string
	var err error

	switch config.AuthType {
	case auth.AzureClientSecret:
		credential, tenantID, clientID, err = f.createClientSecretCredential(config)
	case auth.AzureClientCertificate:
		credential, tenantID, clientID, err = f.createClientCertificateCredential(config)
	case auth.AzureManagedIdentity:
		credential, tenantID, clientID, err = f.createManagedIdentityCredential(config)
	case auth.AzureDeviceFlow:
		credential, tenantID, clientID, err = f.createDeviceFlowCredential(config)
	case auth.AzureDefault:
		credential, tenantID, clientID, err = f.createDefaultCredential(config)
	case auth.AzureAccountKey:
		credential, tenantID, clientID, err = f.createAccountKeyCredential(config)
	default:
		return nil, fmt.Errorf("unsupported Azure auth type: %s", config.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credentials: %w", err)
	}

	return &AzureCredentials{
		credential: credential,
		authType:   config.AuthType,
		tenantID:   tenantID,
		clientID:   clientID,
		logger:     f.logger,
	}, nil
}

// SupportedAuthTypes returns supported Azure auth types
func (f *Factory) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.AzureClientSecret,
		auth.AzureClientCertificate,
		auth.AzureManagedIdentity,
		auth.AzureDeviceFlow,
		auth.AzureDefault,
		auth.AzureAccountKey,
	}
}

// createClientSecretCredential creates client secret credentials
func (f *Factory) createClientSecretCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Extract client secret config
	csConfig := ClientSecretConfig{}

	if config.Extra != nil {
		if cs, ok := config.Extra["client_secret"].(map[string]interface{}); ok {
			if tenantID, ok := cs["tenant_id"].(string); ok {
				csConfig.TenantID = tenantID
			}
			if clientID, ok := cs["client_id"].(string); ok {
				csConfig.ClientID = clientID
			}
			if clientSecret, ok := cs["client_secret"].(string); ok {
				csConfig.ClientSecret = clientSecret
			}
		}
	}

	// Check environment variables
	if csConfig.TenantID == "" {
		csConfig.TenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if csConfig.ClientID == "" {
		csConfig.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}
	if csConfig.ClientSecret == "" {
		csConfig.ClientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	}

	// Validate
	if err := csConfig.Validate(); err != nil {
		return nil, "", "", err
	}

	// Create credential
	cred, err := azidentity.NewClientSecretCredential(csConfig.TenantID, csConfig.ClientID, csConfig.ClientSecret, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create client secret credential: %w", err)
	}

	return cred, csConfig.TenantID, csConfig.ClientID, nil
}

// createClientCertificateCredential creates client certificate credentials
func (f *Factory) createClientCertificateCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Extract client certificate config
	ccConfig := ClientCertificateConfig{}

	if config.Extra != nil {
		if cc, ok := config.Extra["client_certificate"].(map[string]interface{}); ok {
			if tenantID, ok := cc["tenant_id"].(string); ok {
				ccConfig.TenantID = tenantID
			}
			if clientID, ok := cc["client_id"].(string); ok {
				ccConfig.ClientID = clientID
			}
			if certPath, ok := cc["certificate_path"].(string); ok {
				ccConfig.CertificatePath = certPath
			}
			if certData, ok := cc["certificate_data"].([]byte); ok {
				ccConfig.CertificateData = certData
			}
			if password, ok := cc["password"].(string); ok {
				ccConfig.Password = password
			}
		}
	}

	// Check environment variables
	if ccConfig.TenantID == "" {
		ccConfig.TenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if ccConfig.ClientID == "" {
		ccConfig.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}
	if ccConfig.CertificatePath == "" {
		ccConfig.CertificatePath = os.Getenv("AZURE_CLIENT_CERTIFICATE_PATH")
	}

	// Validate
	if err := ccConfig.Validate(); err != nil {
		return nil, "", "", err
	}

	// Read certificate if path provided
	var certData []byte
	if ccConfig.CertificatePath != "" {
		data, err := os.ReadFile(ccConfig.CertificatePath)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to read certificate file: %w", err)
		}
		certData = data
	} else {
		certData = ccConfig.CertificateData
	}

	// Parse certificate
	certs, key, err := azidentity.ParseCertificates(certData, []byte(ccConfig.Password))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create credential
	cred, err := azidentity.NewClientCertificateCredential(ccConfig.TenantID, ccConfig.ClientID, certs, key, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create client certificate credential: %w", err)
	}

	return cred, ccConfig.TenantID, ccConfig.ClientID, nil
}

// createManagedIdentityCredential creates managed identity credentials
func (f *Factory) createManagedIdentityCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Extract managed identity config
	miConfig := ManagedIdentityConfig{}

	if config.Extra != nil {
		if mi, ok := config.Extra["managed_identity"].(map[string]interface{}); ok {
			if clientID, ok := mi["client_id"].(string); ok {
				miConfig.ClientID = clientID
			}
			if resourceID, ok := mi["resource_id"].(string); ok {
				miConfig.ResourceID = resourceID
			}
		}
	}

	// Check environment variables
	if miConfig.ClientID == "" {
		miConfig.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}

	// Create options
	options := &azidentity.ManagedIdentityCredentialOptions{}
	if miConfig.ClientID != "" {
		options.ID = azidentity.ClientID(miConfig.ClientID)
	} else if miConfig.ResourceID != "" {
		options.ID = azidentity.ResourceID(miConfig.ResourceID)
	}

	// Create credential
	cred, err := azidentity.NewManagedIdentityCredential(options)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create managed identity credential: %w", err)
	}

	return cred, "", miConfig.ClientID, nil
}

// createDefaultCredential creates default Azure credentials
func (f *Factory) createDefaultCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Create default credential chain
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create default credential: %w", err)
	}

	// Try to get tenant and client ID from environment
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")

	return cred, tenantID, clientID, nil
}

// createAccountKeyCredential creates storage account key credentials
func (f *Factory) createAccountKeyCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Extract account key config
	akConfig := AccountKeyConfig{}

	if config.Extra != nil {
		if ak, ok := config.Extra["account_key"].(map[string]interface{}); ok {
			if accountName, ok := ak["account_name"].(string); ok {
				akConfig.AccountName = accountName
			}
			if accountKey, ok := ak["account_key"].(string); ok {
				akConfig.AccountKey = accountKey
			}
		}
	}

	// Check environment variables
	if akConfig.AccountName == "" {
		akConfig.AccountName = os.Getenv("AZURE_STORAGE_ACCOUNT")
	}
	if akConfig.AccountKey == "" {
		akConfig.AccountKey = os.Getenv("AZURE_STORAGE_KEY")
	}

	// Validate
	if err := akConfig.Validate(); err != nil {
		return nil, "", "", err
	}

	// Create shared key credential
	cred := NewSharedKeyCredential(akConfig.AccountName, akConfig.AccountKey)

	return cred, "", "", nil
}

// createDeviceFlowCredential creates device flow credentials
func (f *Factory) createDeviceFlowCredential(config auth.Config) (azcore.TokenCredential, string, string, error) {
	// Extract device flow config
	dfConfig := DeviceFlowConfig{}

	if config.Extra != nil {
		if df, ok := config.Extra["device_flow"].(map[string]interface{}); ok {
			if tenantID, ok := df["tenant_id"].(string); ok {
				dfConfig.TenantID = tenantID
			}
			if clientID, ok := df["client_id"].(string); ok {
				dfConfig.ClientID = clientID
			}
		}
	}

	// Check environment variables
	if dfConfig.TenantID == "" {
		dfConfig.TenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if dfConfig.ClientID == "" {
		dfConfig.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}

	// Validate
	if err := dfConfig.Validate(); err != nil {
		return nil, "", "", err
	}

	// Create device code credential
	cred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: dfConfig.TenantID,
		ClientID: dfConfig.ClientID,
		UserPrompt: func(ctx context.Context, message azidentity.DeviceCodeMessage) error {
			f.logger.WithField("code", message.UserCode).
				WithField("url", message.VerificationURL).
				WithField("message", message.Message).
				Info("Device code authentication required")
			return nil
		},
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create device code credential: %w", err)
	}

	return cred, dfConfig.TenantID, dfConfig.ClientID, nil
}
