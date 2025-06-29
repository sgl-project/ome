package storage

import (
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"go.uber.org/fx"
)

// Module provides storage functionality with fx
var Module = fx.Options(
	fx.Provide(
		NewDefaultFactory,
	),
)

// Params defines the dependencies for storage module
type Params struct {
	fx.In

	AuthFactory auth.Factory
	Logger      logging.Interface
}

// Result defines what the storage module provides
type Result struct {
	fx.Out

	Factory StorageFactory
}

// NewModule creates a new storage module with providers registered
func NewModule(params Params) Result {
	factory := NewDefaultFactory(params.AuthFactory, params.Logger)

	// Note: Provider registration should be done by the application
	// to avoid import cycles. See cmd/storage-example/main.go

	return Result{
		Factory: factory,
	}
}
