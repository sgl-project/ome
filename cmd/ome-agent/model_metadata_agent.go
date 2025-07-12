package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	modelmetadata "github.com/sgl-project/ome/internal/ome-agent/model-metadata"
	aferoModule "github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/logging"
)

// ModelMetadataAgent implements the AgentModule interface for model metadata extraction
type ModelMetadataAgent struct {
	extractor *modelmetadata.MetadataExtractor
}

// Name returns the name of the agent
func (m *ModelMetadataAgent) Name() string {
	return "model-metadata"
}

// ShortDescription returns a short description of the agent
func (m *ModelMetadataAgent) ShortDescription() string {
	return "Extract model metadata from PVC-mounted models"
}

// LongDescription returns a detailed description of the agent
func (m *ModelMetadataAgent) LongDescription() string {
	return "Model metadata agent mounts PVCs and extracts model metadata to update BaseModel/ClusterBaseModel CRs"
}

// ConfigureCommand configures the agent command
func (m *ModelMetadataAgent) ConfigureCommand(cmd *cobra.Command) {
	// Add flags for the model metadata agent
	// These will be provided by the BaseModel controller when running as a Job
	cmd.Flags().String("model-path", "", "Path to the model directory")
	cmd.Flags().String("basemodel-name", "", "Name of the BaseModel CR")
	cmd.Flags().String("basemodel-namespace", "", "Namespace of the BaseModel CR")
	cmd.Flags().Bool("cluster-scoped", false, "Whether this is a ClusterBaseModel")

	_ = cmd.MarkFlagRequired("model-path")
	_ = cmd.MarkFlagRequired("basemodel-name")

	// Bind flags to viper with underscore keys to match mapstructure tags
	_ = viper.BindPFlag("model_path", cmd.Flags().Lookup("model-path"))
	_ = viper.BindPFlag("basemodel_name", cmd.Flags().Lookup("basemodel-name"))
	_ = viper.BindPFlag("basemodel_namespace", cmd.Flags().Lookup("basemodel-namespace"))
	_ = viper.BindPFlag("cluster_scoped", cmd.Flags().Lookup("cluster-scoped"))

	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, m, m.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (m *ModelMetadataAgent) FxModules() []fx.Option {
	return []fx.Option{
		aferoModule.Module,
		logging.Module,
		fx.Provide(NewK8sClient),
		modelmetadata.Module,
		fx.Populate(&m.extractor),
	}
}

// Start runs the agent
func (m *ModelMetadataAgent) Start() error {
	return m.extractor.Start()
}

// NewModelMetadataAgent creates a new model metadata agent
func NewModelMetadataAgent() *ModelMetadataAgent {
	return &ModelMetadataAgent{}
}
