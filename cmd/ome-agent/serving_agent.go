package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	servingAgent "github.com/sgl-project/ome/internal/ome-agent/serving-agent"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

// ServingAgent implements the AgentModule interface for serving sidecar agent
type ServingAgent struct {
	agent *servingAgent.ServingSidecar
}

// Name returns the name of the agent
func (s *ServingAgent) Name() string {
	return "serving-agent"
}

// ShortDescription returns a short description of the agent
func (s *ServingAgent) ShortDescription() string {
	return "Run OME serving sidecar"
}

// LongDescription returns a detailed description of the agent
func (s *ServingAgent) LongDescription() string {
	return "OME Serving sidecar is for assisting some of the ome serving containers"
}

// ConfigureCommand configures the agent command
func (s *ServingAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, s, s.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (s *ServingAgent) FxModules() []fx.Option {
	return []fx.Option{
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		ociobjectstore.OCIOSDataStoreModule,
		servingAgent.Module,
		fx.Populate(&s.agent),
	}
}

// Start starts the agent
func (s *ServingAgent) Start() error {
	return s.agent.Start()
}

// NewServingAgent creates a new serving sidecar agent
func NewServingAgent() *ServingAgent {
	return &ServingAgent{}
}
