package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	finetunedadapter "github.com/sgl-project/sgl-ome/internal/ome-agent/fine-tuned-adapter"
	"github.com/sgl-project/sgl-ome/pkg/afero"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
)

// FineTunedAdapterAgent implements the AgentModule interface for fine-tuned adapter agent
type FineTunedAdapterAgent struct {
	agent *finetunedadapter.FineTunedAdapter
}

// Name returns the name of the agent
func (m *FineTunedAdapterAgent) Name() string {
	return "fine-tuned-adapter"
}

// ShortDescription returns a short description of the agent
func (m *FineTunedAdapterAgent) ShortDescription() string {
	return "Run OME fine-tuned adapter"
}

// LongDescription returns a detailed description of the agent
func (m *FineTunedAdapterAgent) LongDescription() string {
	return "OME fine-tuned adapter is to download the fine-tuned weight and prepared for the serving container to consume"
}

// ConfigureCommand configures the agent command
func (m *FineTunedAdapterAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, m, m.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (m *FineTunedAdapterAgent) FxModules() []fx.Option {
	return []fx.Option{
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		ociobjectstore.OCIOSDataStoreModule,
		finetunedadapter.Module,
		fx.Populate(&m.agent),
	}
}

// Start starts the agent
func (m *FineTunedAdapterAgent) Start() error {
	return m.agent.Start()
}

// NewFineTunedAdapterAgent creates a fine-tuned adapter agent
func NewFineTunedAdapterAgent() *FineTunedAdapterAgent {
	return &FineTunedAdapterAgent{}
}
