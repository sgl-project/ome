package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	mergedfinetunedadapter "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/merged-finetuned-adapter"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

// MergedFineTunedAdapterAgent implements the AgentModule interface for merged fine-tuned adapter agent
type MergedFineTunedAdapterAgent struct {
	agent *mergedfinetunedadapter.MergedFinetunedAdapter
}

// Name returns the name of the agent
func (m *MergedFineTunedAdapterAgent) Name() string {
	return "merged-fine-tuned-adapter"
}

// ShortDescription returns a short description of the agent
func (m *MergedFineTunedAdapterAgent) ShortDescription() string {
	return "Run OME merged fine-tuned adapter"
}

// LongDescription returns a detailed description of the agent
func (m *MergedFineTunedAdapterAgent) LongDescription() string {
	return "OME merged fine-tuned adapter is for downloading the merged fine-tuned weights and prepared for the serving container to consume"
}

// ConfigureCommand configures the agent command
func (m *MergedFineTunedAdapterAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, m, m.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (m *MergedFineTunedAdapterAgent) FxModules() []fx.Option {
	return []fx.Option{
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		casper.CasperDataStoreModule,
		mergedfinetunedadapter.Module,
		fx.Populate(&m.agent),
	}
}

// Start starts the agent
func (m *MergedFineTunedAdapterAgent) Start() error {
	return m.agent.Start()
}

// NewMergedFineTunedAdapterAgent creates a new merged fine-tuned adapter agent
func NewMergedFineTunedAdapterAgent() *MergedFineTunedAdapterAgent {
	return &MergedFineTunedAdapterAgent{}
}
