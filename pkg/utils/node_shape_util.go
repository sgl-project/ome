package utils

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/imds"
	"github.com/sgl-project/ome/pkg/logging"
)

var shapeMap = map[string]string{
	"BM.GPU.A10.4":     "A10",
	"BM.GPU.A100-v2.8": "A100-80G",
	"BM.GPU4.8":        "A100-40G",
	"BM.GPU.B4.8":      "A100-40G",
	"BM.GPU.H100.8":    "H100",
}

func GetOCINodeShape(logger logging.Interface) (string, error) {
	client, err := imds.NewClient(imds.DefaultConfig(), logger)
	if err != nil {
		return "", err
	}
	return client.GetInstanceShape()
}

func GetOCINodeShortVersionShape(currentNodeShape string) (string, error) {
	if shapeShort, ok := shapeMap[currentNodeShape]; ok {
		return shapeShort, nil
	}
	return "", fmt.Errorf("couldn't find shape %s in the shape mapping", currentNodeShape)
}
