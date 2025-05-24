package principals

import (
	"github.com/sgl-project/sgl-ome/pkg/logging"
)

// Opts contains various dependencies used to construct principals.
type Opts struct {
	// Factory is the factory used to create principals.
	//
	// If not set, defaults to defaultFactory that uses oci-go-sdk common & auth packages.
	Factory Factory

	// Log is the logger.
	Log logging.Interface
}

// factory returns the specifies principal factory
// or a defaultFactory if it's not set explicitly on opts.
func (opts Opts) factory() Factory {
	if opts.Factory != nil {
		return opts.Factory
	}

	return defaultFactory
}
