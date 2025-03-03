package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/hf_download"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

// HFDownloadAgent implements the AgentModule interface for HuggingFace download agent
type HFDownloadAgent struct {
	agent *hf_download.HFDownloadAgent
}

// Name returns the name of the agent
func (h *HFDownloadAgent) Name() string {
	return "hf-download"
}

// ShortDescription returns a short description of the agent
func (h *HFDownloadAgent) ShortDescription() string {
	return "Run OME HuggingFace Download Agent"
}

// LongDescription returns a detailed description of the agent
func (h *HFDownloadAgent) LongDescription() string {
	return "OME Agent HuggingFace Download Agent is dedicated for downloading any model from HF."
}

// ConfigureCommand configures the agent command
func (h *HFDownloadAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, h, h.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (h *HFDownloadAgent) FxModules() []fx.Option {
	return []fx.Option{
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		hf_download.Module,
		fx.Populate(&h.agent),
	}
}

// Start starts the agent
func (h *HFDownloadAgent) Start() error {
	return h.agent.Start()
}

// NewHFDownloadAgent creates a new HuggingFace download agent
func NewHFDownloadAgent() *HFDownloadAgent {
	return &HFDownloadAgent{}
}
