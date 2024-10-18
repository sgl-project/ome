package serving_init

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/imds"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

func GetCurrentNodeShape(logger logging.Interface) (string, error) {
	client, err := imds.NewClient(imds.DefaultConfig(), logger)
	if err != nil {
		return "", err
	}

	return client.GetInstanceShape()
}

func GetCurrentNodeShortVersionShape(currentNodeShape, shapeMappingFilePath string) (string, error) {
	file, err := os.Open(shapeMappingFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read the entire file content as a byte slice
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	// Unmarshal the JSON content into map
	shapeMap := make(map[string]string)
	err = json.Unmarshal(fileContent, &shapeMap)
	if err != nil {
		return "", err
	}

	if shapeShort, ok := shapeMap[currentNodeShape]; ok {
		return shapeShort, nil
	}

	return "", fmt.Errorf("couldn't find shape %s in the shape mapping", currentNodeShape)
}
