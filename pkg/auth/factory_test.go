package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/sgl-project/ome/pkg/logging"
)

func TestDefaultFactory(t *testing.T) {
	// Use zap test logger for better test output
	testLogger := zaptest.NewLogger(t)
	factory := NewDefaultFactory(logging.ForZap(testLogger))

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

func TestGetDefaultFactory(t *testing.T) {
	// Test that GetDefaultFactory returns a factory
	factory := GetDefaultFactory()
	if factory == nil {
		t.Error("GetDefaultFactory returned nil")
	}

	// Test that it returns the same instance on subsequent calls
	factory2 := GetDefaultFactory()
	if factory != factory2 {
		t.Error("GetDefaultFactory did not return the same instance")
	}
}

func TestSetDefaultFactory(t *testing.T) {
	// Backup the original default factory
	originalFactory := GetDefaultFactory()

	// Restore the original default factory after the test
	t.Cleanup(func() {
		SetDefaultFactory(originalFactory)
	})

	// Create a test factory
	testLogger := zaptest.NewLogger(t)
	testFactory := NewDefaultFactory(logging.ForZap(testLogger))

	// Set it as the default
	SetDefaultFactory(testFactory)

	// Verify it was set
	gotFactory := GetDefaultFactory()
	if gotFactory != testFactory {
		t.Error("SetDefaultFactory did not set the factory correctly")
	}
}

func TestDefaultFactoryThreadSafety(t *testing.T) {
	// Backup the original default factory
	originalFactory := GetDefaultFactory()

	// Restore the original default factory after the test
	t.Cleanup(func() {
		SetDefaultFactory(originalFactory)
	})

	// Reset the default factory to nil to test initialization
	SetDefaultFactory(nil)

	// Test concurrent access to GetDefaultFactory
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			factory := GetDefaultFactory()
			if factory == nil {
				t.Error("GetDefaultFactory returned nil")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Test concurrent SetDefaultFactory and GetDefaultFactory
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			if id%2 == 0 {
				testLogger := zaptest.NewLogger(t)
				testFactory := NewDefaultFactory(logging.ForZap(testLogger))
				SetDefaultFactory(testFactory)
			} else {
				factory := GetDefaultFactory()
				if factory == nil {
					t.Error("GetDefaultFactory returned nil during concurrent access")
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func BenchmarkGetDefaultFactory(b *testing.B) {
	// Ensure factory is initialized before benchmark
	_ = GetDefaultFactory()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			factory := GetDefaultFactory()
			if factory == nil {
				b.Fatal("GetDefaultFactory returned nil")
			}
		}
	})
}

func TestFallbackDepthLimit(t *testing.T) {
	testLogger := zaptest.NewLogger(t)
	factory := NewDefaultFactory(logging.ForZap(testLogger))

	// Register a mock provider that always fails
	factory.RegisterProvider(ProviderOCI, &failingProviderFactory{})

	t.Run("CircularFallback", func(t *testing.T) {
		// Create a circular fallback: A -> B -> A
		configB := Config{
			Provider: ProviderOCI,
			AuthType: OCIInstancePrincipal,
		}
		configA := Config{
			Provider: ProviderOCI,
			AuthType: OCIUserPrincipal,
			Fallback: &configB,
		}
		// Create the circular reference
		configB.Fallback = &configA

		// This should fail with depth limit error, not stack overflow
		_, err := factory.Create(context.Background(), configA)
		if err == nil {
			t.Fatal("Expected error for circular fallback")
		}
		if !strings.Contains(err.Error(), "maximum fallback depth") {
			t.Errorf("Expected depth limit error, got: %v", err)
		}
	})

	t.Run("DeepFallbackChain", func(t *testing.T) {
		// Create a deep fallback chain that exceeds the limit
		var configs []Config
		for i := 0; i < 15; i++ { // More than maxFallbackDepth
			configs = append(configs, Config{
				Provider: ProviderOCI,
				AuthType: OCIUserPrincipal,
			})
		}

		// Link them together
		for i := 0; i < len(configs)-1; i++ {
			configs[i].Fallback = &configs[i+1]
		}

		// This should fail with depth limit error
		_, err := factory.Create(context.Background(), configs[0])
		if err == nil {
			t.Fatal("Expected error for deep fallback chain")
		}
		if !strings.Contains(err.Error(), "maximum fallback depth") {
			t.Errorf("Expected depth limit error, got: %v", err)
		}
	})

	t.Run("NormalFallback", func(t *testing.T) {
		// Register a provider that succeeds on the second type
		factory.RegisterProvider(ProviderAWS, &selectiveProviderFactory{
			failTypes: map[AuthType]bool{AWSAccessKey: true},
		})

		// Create a normal fallback chain that should work
		config := Config{
			Provider: ProviderAWS,
			AuthType: AWSAccessKey, // This will fail
			Fallback: &Config{
				Provider: ProviderAWS,
				AuthType: AWSInstanceProfile, // This will succeed
			},
		}

		// This should succeed
		creds, err := factory.Create(context.Background(), config)
		if err != nil {
			t.Errorf("Expected success with normal fallback, got: %v", err)
		}
		if creds == nil {
			t.Error("Expected credentials, got nil")
		}
	})
}

// failingProviderFactory always fails to create credentials
type failingProviderFactory struct{}

func (f *failingProviderFactory) Create(ctx context.Context, config Config) (Credentials, error) {
	return nil, fmt.Errorf("intentional failure for testing")
}

func (f *failingProviderFactory) SupportedAuthTypes() []AuthType {
	return []AuthType{OCIUserPrincipal, OCIInstancePrincipal}
}

// selectiveProviderFactory fails for specific auth types
type selectiveProviderFactory struct {
	failTypes map[AuthType]bool
}

func (f *selectiveProviderFactory) Create(ctx context.Context, config Config) (Credentials, error) {
	if f.failTypes[config.AuthType] {
		return nil, fmt.Errorf("intentional failure for auth type %s", config.AuthType)
	}
	return &mockCredentials{
		provider: config.Provider,
		authType: config.AuthType,
	}, nil
}

func (f *selectiveProviderFactory) SupportedAuthTypes() []AuthType {
	return []AuthType{AWSAccessKey, AWSInstanceProfile}
}
