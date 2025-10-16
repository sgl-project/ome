package utils

import (
	"github.com/sgl-project/ome/pkg/imds"
	"github.com/sgl-project/ome/pkg/logging"
)

var instanceTypeMap = map[string]string{
	// Oracle Cloud (OCI) shapes
	"BM.GPU.A10.4":     "A10",
	"BM.GPU.A100-v2.8": "A100-80G",
	"BM.GPU4.8":        "A100-40G",
	"BM.GPU.B4.8":      "A100-40G",
	"BM.GPU.H100.8":    "H100",
	// TODO: more shape

	// AWS instance types
	"p5.48xlarge": "H100",

	// Azure instance types
	"Standard_ND96isr_H100_v5": "H100",

	// Google Cloud instance types
	"a3-highgpu-8g": "H100",

	// CoreWeave instance types
	"gd-8xh100ib-i128": "H100",
	"gd-8xh200ib-i128": "H200",
	"gd-8xl40-i128":    "L40",
}

// GetNodeInstanceType retrieves the instance type of the node.
// NOTE: This implementation is currently specific to Oracle Cloud Infrastructure (OCI)
// and uses the OCI IMDS client. A future refactor is needed to support
// other cloud providers' metadata services.
func GetNodeInstanceType(logger logging.Interface) (string, error) {
	client, err := imds.NewClient(imds.DefaultConfig(), logger)
	if err != nil {
		return "", err
	}
	return client.GetInstanceShape()
}

func GetInstanceTypeShortName(currentInstanceType string) (string, error) {
	if shortName, ok := instanceTypeMap[currentInstanceType]; ok {
		return shortName, nil
	}
	// Return the original instance type as a fallback for unknown shapes
	return currentInstanceType, nil
}
