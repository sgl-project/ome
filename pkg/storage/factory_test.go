package storage

import (
	"context"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestDefaultFactory_RegisterProvider(t *testing.T) {
	logger := logging.NewNopLogger()
	authFactory := auth.NewDefaultFactory(logger)
	factory := NewDefaultFactory(authFactory, logger)

	// Test that default providers are registered
	ctx := context.Background()

	// Test with unsupported provider
	_, err := factory.Create(ctx, Provider("unsupported"), nil)
	if err == nil {
		t.Error("Expected error for unsupported provider")
	}
}

func TestStorageConfig_GetAuthConfig(t *testing.T) {
	authConfig := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: auth.OCIInstancePrincipal,
		Region:   "us-ashburn-1",
	}

	storageConfig := StorageConfig{
		Provider:   ProviderOCI,
		Region:     "us-ashburn-1",
		AuthConfig: authConfig,
	}

	got := storageConfig.GetAuthConfig()
	if got.Provider != authConfig.Provider {
		t.Errorf("Expected provider %s, got %s", authConfig.Provider, got.Provider)
	}
	if got.AuthType != authConfig.AuthType {
		t.Errorf("Expected auth type %s, got %s", authConfig.AuthType, got.AuthType)
	}
	if got.Region != authConfig.Region {
		t.Errorf("Expected region %s, got %s", authConfig.Region, got.Region)
	}
}

func TestGetDefaultFactory(t *testing.T) {
	// Test that default factory is singleton
	factory1 := GetDefaultFactory()
	factory2 := GetDefaultFactory()

	if factory1 != factory2 {
		t.Error("Expected same default factory instance")
	}
}

func TestSetDefaultFactory(t *testing.T) {
	// Create a custom factory
	logger := logging.NewNopLogger()
	authFactory := auth.NewDefaultFactory(logger)
	customFactory := NewDefaultFactory(authFactory, logger)

	// Set it as default
	SetDefaultFactory(customFactory)

	// Verify it was set
	got := GetDefaultFactory()
	if got != customFactory {
		t.Error("Expected custom factory to be set as default")
	}
}
