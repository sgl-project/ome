package env

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
)

type computeVarFn func(e *Environment) (string, error)

var (
	// map of computed on-the-fly variables to the functions that are used to actually compute the results
	computedVars = map[vars.Var]computeVarFn{
		vars.RegionSE: computeRegionSE,
	}
)

// computeRegionSE returns the region name that can be used directly to
// construct Service Enclave endpoints (e.g. us-phoenix-1 -> r2, us-seattle-1 -> r1 etc.)
func computeRegionSE(e *Environment) (string, error) {
	region, ok := e.Region()
	if !ok {
		return "", fmt.Errorf("region not resolved")
	}

	switch region {
	case "sea", "us-seattle-1", "r1":
		return "r1", nil
	case "phx", "us-phoenix-1", "r2":
		return "r2", nil
	default:
		return region, nil
	}
}
