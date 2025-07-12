package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestModelMetadataAgent_ConfigureCommand(t *testing.T) {
	// Test that required flags are properly configured
	agent := NewModelMetadataAgent()
	cmd := &cobra.Command{}

	agent.ConfigureCommand(cmd)

	// Check required flags
	modelPathFlag := cmd.Flags().Lookup("model-path")
	assert.NotNil(t, modelPathFlag)
	assert.Equal(t, "model-path", modelPathFlag.Name)

	baseModelNameFlag := cmd.Flags().Lookup("basemodel-name")
	assert.NotNil(t, baseModelNameFlag)
	assert.Equal(t, "basemodel-name", baseModelNameFlag.Name)

	baseModelNamespaceFlag := cmd.Flags().Lookup("basemodel-namespace")
	assert.NotNil(t, baseModelNamespaceFlag)
	assert.Equal(t, "basemodel-namespace", baseModelNamespaceFlag.Name)

	clusterScopedFlag := cmd.Flags().Lookup("cluster-scoped")
	assert.NotNil(t, clusterScopedFlag)
	assert.Equal(t, "cluster-scoped", clusterScopedFlag.Name)
}

func TestModelMetadataAgent_Name(t *testing.T) {
	agent := NewModelMetadataAgent()
	assert.Equal(t, "model-metadata", agent.Name())
}

func TestModelMetadataAgent_ShortDescription(t *testing.T) {
	agent := NewModelMetadataAgent()
	assert.Equal(t, "Extract model metadata from PVC-mounted models", agent.ShortDescription())
}

func TestModelMetadataAgent_LongDescription(t *testing.T) {
	agent := NewModelMetadataAgent()
	expected := "Model metadata agent mounts PVCs and extracts model metadata to update BaseModel/ClusterBaseModel CRs"
	assert.Equal(t, expected, agent.LongDescription())
}
