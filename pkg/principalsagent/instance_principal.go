package principals

import (
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

type InstancePrincipalAuthProvider struct{}

// ConfigurationProvider returns a configuration provider set up for instance principal authentication.
func (ipa *InstancePrincipalAuthProvider) ConfigurationProvider() (common.ConfigurationProvider, error) {
	instancePrincipalProvider, err := auth.InstancePrincipalConfigurationProvider()
	return instancePrincipalProvider, err
}
