package oci

import (
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func init() {
	// Register OCI provider with the default auth factory
	// We create a factory with a discard logger initially - it will be replaced
	// when the global factory is properly initialized with a logger
	factory := auth.GetDefaultFactory()
	if defaultFactory, ok := factory.(*auth.DefaultFactory); ok {
		// Use a discard logger for registration
		// The actual logger will be set when credentials are created
		logger := logging.Discard()
		defaultFactory.RegisterProvider(auth.ProviderOCI, NewFactory(logger))
	}
}
