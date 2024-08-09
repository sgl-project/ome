package principals

import (
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

// WorkloadIdentityAuthProvider provides workload identity-based authentication.
type WorkloadIdentityAuthProvider struct{}

func (wia *WorkloadIdentityAuthProvider) ConfigurationProvider() (common.ConfigurationProvider, error) {
	// Assuming the OCI Go SDK provides a direct way to use workload identity
	rp, err := auth.OkeWorkloadIdentityConfigurationProvider()
	return rp, err
}
