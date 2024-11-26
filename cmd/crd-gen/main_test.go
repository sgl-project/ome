package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestRemoveCRDValidation(t *testing.T) {
	// Create a temporary test file
	testCRD := map[string]interface{}{
		"spec": map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"schema": map[string]interface{}{
						"openAPIV3Schema": map[string]interface{}{
							"properties": map[string]interface{}{
								"spec": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"foo": map[string]interface{}{
											"type": "string",
										},
									},
								},
								"status": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"bar": map[string]interface{}{
											"type": "string",
										},
									},
								},
								"metadata": map[string]interface{}{
									"type": "object",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create temp file
	tmpFile := filepath.Join(t.TempDir(), "test-crd.yaml")
	data, err := yaml.Marshal(testCRD)
	assert.NoError(t, err)
	err = os.WriteFile(tmpFile, data, 0600)
	assert.NoError(t, err)

	// Run the function
	removeCRDValidation(tmpFile)

	// Read and verify the result
	resultData, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = yaml.Unmarshal(resultData, &result)
	assert.NoError(t, err)

	// Get the properties from the result
	spec := result["spec"].(map[string]interface{})
	versions := spec["versions"].([]interface{})
	version := versions[0].(map[string]interface{})
	properties := version["schema"].(map[string]interface{})["openAPIV3Schema"].(map[string]interface{})["properties"].(map[string]interface{})

	// Check that spec and status were modified correctly
	expectedProps := map[string]interface{}{
		"type":                                 "object",
		"x-kubernetes-preserve-unknown-fields": true,
		"x-kubernetes-map-type":                "atomic",
	}

	assert.Equal(t, expectedProps, properties["spec"])
	assert.Equal(t, expectedProps, properties["status"])
	assert.Equal(t, map[string]interface{}{"type": "object"}, properties["metadata"])
}

func TestMain_RemoveCRDValidation(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a temporary test file
	tmpFile := filepath.Join(t.TempDir(), "test-crd.yaml")
	testCRD := map[string]interface{}{
		"spec": map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"schema": map[string]interface{}{
						"openAPIV3Schema": map[string]interface{}{
							"properties": map[string]interface{}{
								"spec": map[string]interface{}{
									"type": "object",
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(testCRD)
	assert.NoError(t, err)
	err = os.WriteFile(tmpFile, data, 0600)
	assert.NoError(t, err)

	// Test valid command
	os.Args = []string{"cmd", "removecrdvalidation", tmpFile}
	assert.NotPanics(t, func() { main() })

	// Test invalid command
	os.Args = []string{"cmd", "invalidcommand"}
	assert.Panics(t, func() { main() })

	// Test missing file
	os.Args = []string{"cmd", "removecrdvalidation"}
	assert.Panics(t, func() { main() })

	// Test no args
	os.Args = []string{"cmd"}
	assert.Panics(t, func() { main() })
}
