package principals

import (
	"github.com/oracle/oci-go-sdk/v65/common"
)

// AuthProvider defines the interface for OCI authentication providers.
type AuthProvider interface {
	ConfigurationProvider() (common.ConfigurationProvider, error)
}
