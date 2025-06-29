package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/sgl-project/ome/pkg/logging"
)

func TestDefaultFactory(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewDefaultFactory(logger)

	// Register mock providers for testing
	for _, provider := range []Provider{ProviderOCI, ProviderAWS, ProviderGCP, ProviderAzure, ProviderGitHub} {
		factory.RegisterProvider(provider, &mockProviderFactory{provider: provider})
	}

	// Test supported providers
	providers := factory.SupportedProviders()
	expectedProviders := map[Provider]bool{
		ProviderOCI:    true,
		ProviderAWS:    true,
		ProviderGCP:    true,
		ProviderAzure:  true,
		ProviderGitHub: true,
	}

	for _, p := range providers {
		if !expectedProviders[p] {
			t.Errorf("Unexpected provider: %s", p)
		}
		delete(expectedProviders, p)
	}

	if len(expectedProviders) > 0 {
		t.Errorf("Missing providers: %v", expectedProviders)
	}

	// Test supported auth types for each provider
	tests := []struct {
		provider      Provider
		expectedTypes []AuthType
	}{
		{
			provider: ProviderOCI,
			expectedTypes: []AuthType{
				OCIUserPrincipal,
				OCIInstancePrincipal,
				OCIResourcePrincipal,
				OCIOkeWorkloadIdentity,
			},
		},
		{
			provider: ProviderAWS,
			expectedTypes: []AuthType{
				AWSAccessKey,
				AWSInstanceProfile,
				AWSAssumeRole,
				AWSWebIdentity,
			},
		},
		{
			provider: ProviderGCP,
			expectedTypes: []AuthType{
				GCPServiceAccount,
				GCPApplicationDefault,
				GCPWorkloadIdentity,
			},
		},
		{
			provider: ProviderAzure,
			expectedTypes: []AuthType{
				AzureServicePrincipal,
				AzureManagedIdentity,
				AzureDeviceFlow,
			},
		},
		{
			provider: ProviderGitHub,
			expectedTypes: []AuthType{
				GitHubToken,
				GitHubApp,
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			types := factory.SupportedAuthTypes(tt.provider)
			if len(types) != len(tt.expectedTypes) {
				t.Errorf("Expected %d auth types, got %d", len(tt.expectedTypes), len(types))
			}

			typeMap := make(map[AuthType]bool)
			for _, at := range types {
				typeMap[at] = true
			}

			for _, expected := range tt.expectedTypes {
				if !typeMap[expected] {
					t.Errorf("Missing expected auth type: %s", expected)
				}
			}
		})
	}
}

func TestChainProvider(t *testing.T) {
	ctx := context.Background()

	// Mock credentials
	mockCreds := &mockCredentials{
		provider: ProviderOCI,
		authType: OCIInstancePrincipal,
	}

	// Test successful provider
	successProvider := &mockCredentialsProvider{
		creds: mockCreds,
		err:   nil,
	}

	// Test failing provider
	failProvider := &mockCredentialsProvider{
		creds: nil,
		err:   context.DeadlineExceeded,
	}

	// Test chain with successful provider first
	chain := &ChainProvider{
		Providers: []CredentialsProvider{successProvider, failProvider},
	}

	creds, err := chain.GetCredentials(ctx)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if creds != mockCreds {
		t.Errorf("Expected mock credentials, got %v", creds)
	}

	// Test chain with failing provider first
	chain = &ChainProvider{
		Providers: []CredentialsProvider{failProvider, successProvider},
	}

	creds, err = chain.GetCredentials(ctx)
	if err != nil {
		t.Errorf("Expected success after retry, got error: %v", err)
	}
	if creds != mockCreds {
		t.Errorf("Expected mock credentials after retry, got %v", creds)
	}

	// Test chain with all failing providers
	chain = &ChainProvider{
		Providers: []CredentialsProvider{failProvider, failProvider},
	}

	_, err = chain.GetCredentials(ctx)
	if err == nil {
		t.Errorf("Expected error with all failing providers")
	}
}

// Mock implementations for testing

type mockCredentials struct {
	provider Provider
	authType AuthType
}

func (m *mockCredentials) Provider() Provider {
	return m.provider
}

func (m *mockCredentials) Type() AuthType {
	return m.authType
}

func (m *mockCredentials) Token(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer mock-token")
	return nil
}

func (m *mockCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockCredentials) IsExpired() bool {
	return false
}

type mockCredentialsProvider struct {
	creds Credentials
	err   error
}

func (m *mockCredentialsProvider) GetCredentials(ctx context.Context) (Credentials, error) {
	return m.creds, m.err
}

type mockProviderFactory struct {
	provider Provider
}

func (m *mockProviderFactory) Create(ctx context.Context, config Config) (Credentials, error) {
	return &mockCredentials{
		provider: config.Provider,
		authType: config.AuthType,
	}, nil
}

func (m *mockProviderFactory) SupportedAuthTypes() []AuthType {
	switch m.provider {
	case ProviderOCI:
		return []AuthType{
			OCIUserPrincipal,
			OCIInstancePrincipal,
			OCIResourcePrincipal,
			OCIOkeWorkloadIdentity,
		}
	case ProviderAWS:
		return []AuthType{
			AWSAccessKey,
			AWSInstanceProfile,
			AWSAssumeRole,
			AWSWebIdentity,
		}
	case ProviderGCP:
		return []AuthType{
			GCPServiceAccount,
			GCPApplicationDefault,
			GCPWorkloadIdentity,
		}
	case ProviderAzure:
		return []AuthType{
			AzureServicePrincipal,
			AzureManagedIdentity,
			AzureDeviceFlow,
		}
	case ProviderGitHub:
		return []AuthType{
			GitHubToken,
			GitHubApp,
		}
	default:
		return nil
	}
}
