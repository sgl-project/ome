package xet

import (
	"os"
	"strconv"
)

// UseXetBinding determines whether to use the xet-core binding
// This can be controlled via:
// 1. Build tags: go build -tags use_xet
// 2. Environment variable: OME_USE_XET_BINDING=1
// 3. Config file (future)
func UseXetBinding() bool {
	// Check environment variable
	if env := os.Getenv("OME_USE_XET_BINDING"); env != "" {
		if val, err := strconv.ParseBool(env); err == nil {
			return val
		}
	}

	// Check for experimental flag
	if env := os.Getenv("OME_EXPERIMENTAL_XET"); env != "" {
		if val, err := strconv.ParseBool(env); err == nil {
			return val
		}
	}

	// Default to false for now (use old implementation)
	return false
}

// EnableXetBinding enables the xet-core binding globally
func EnableXetBinding() {
	os.Setenv("OME_USE_XET_BINDING", "true")
}

// DisableXetBinding disables the xet-core binding globally
func DisableXetBinding() {
	os.Setenv("OME_USE_XET_BINDING", "false")
}