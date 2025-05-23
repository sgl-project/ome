package afero

import (
	"github.com/spf13/afero"
	"go.uber.org/fx"
)

var fs = NewOsFs()

// Module makes available both standard spf13 afero.Fs
// and this package's extension (+ LOwnership, LChown methods).
var Module fx.Option = fx.Provide(
	func() Fs { return fs },
	func() afero.Fs { return fs },
)
