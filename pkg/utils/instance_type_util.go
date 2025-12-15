package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sgl-project/ome/pkg/imds"
	"github.com/sgl-project/ome/pkg/logging"
)

const (
	// InstanceTypeMapEnvVar is the environment variable name for instance type map
	InstanceTypeMapEnvVar = "INSTANCE_TYPE_MAP"
)

// defaultInstanceTypeMap is the fallback map when env var is not set
var defaultInstanceTypeMap = map[string]string{
	// Oracle Cloud (OCI) shapes
	"BM.GPU.A10.4":     "A10",
	"BM.GPU.A100-v2.8": "A100-80G",
	"BM.GPU4.8":        "A100-40G",
	"BM.GPU.B4.8":      "A100-40G",
	"BM.GPU.H100.8":    "H100",
	"BM.GPU.H100-NC.8": "H100",
	"BM.GPU.H200.8":    "H200",
	"BM.GPU.H200-NC.8": "H200",

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

	// Nebius instance types
	"gpu-h100-sxm": "H100",
	"gpu-h200-sxm": "H200",
	"gpu-b200-sxm": "B200",
	"gpu-l40s":     "L40S",
}

var (
	instanceTypeMap     map[string]string
	instanceTypeMapErr  error
	instanceTypeMapOnce sync.Once
)

// getInstanceTypeMap returns the instance type map, loading from env var if available
func getInstanceTypeMap() (map[string]string, error) {
	instanceTypeMapOnce.Do(func() {
		instanceTypeMap, instanceTypeMapErr = loadInstanceTypeMapFromEnv()
	})
	return instanceTypeMap, instanceTypeMapErr
}

// loadInstanceTypeMapFromEnv loads the instance type map from environment variable
// Falls back to default map if env var is not set, empty, or parsing fails
func loadInstanceTypeMapFromEnv() (map[string]string, error) {
	envValue := os.Getenv(InstanceTypeMapEnvVar)
	// Check if ConfigMap doesn't exist (env var not set)
	if envValue == "" {
		return defaultInstanceTypeMap, nil
	}

	var configMap map[string]string
	if err := json.Unmarshal([]byte(envValue), &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", InstanceTypeMapEnvVar, err)
	}

	// Check if ConfigMap exists but is empty
	if len(configMap) == 0 {
		return defaultInstanceTypeMap, nil
	}

	return configMap, nil
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
	typeMap, err := getInstanceTypeMap()
	if err != nil {
		return "", err
	}
	if shortName, ok := typeMap[currentInstanceType]; ok {
		return shortName, nil
	}
	// Return the original instance type as a fallback for unknown shapes
	return currentInstanceType, nil
}
