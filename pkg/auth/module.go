package auth

import (
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
)

// Module provides auth functionality with fx
var Module = fx.Options(
	fx.Provide(
		NewDefaultFactory,
	),
)

// Params defines the dependencies for auth module
type Params struct {
	fx.In

	Logger logging.Interface
}

// Result defines what the auth module provides
type Result struct {
	fx.Out

	Factory Factory
}

// NewModule creates a new auth module with providers registered
func NewModule(params Params) Result {
	factory := NewDefaultFactory(params.Logger)

	// Note: Provider registration should be done by the application
	// to avoid import cycles. See cmd/storage-example/main.go

	return Result{
		Factory: factory,
	}
}
